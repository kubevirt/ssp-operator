/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"golang.org/x/sync/errgroup"
	"net/http"
	"os"
	"path"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	ocpconfigv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/pkg/crypto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	ssp "kubevirt.io/ssp-operator/api/v1beta3"
	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/controllers"
	sspMetrics "kubevirt.io/ssp-operator/pkg/monitoring/metrics/ssp-operator"
	"kubevirt.io/ssp-operator/pkg/monitoring/rules"
	"kubevirt.io/ssp-operator/webhooks"
	// +kubebuilder:scaffold:imports
)

var (
	setupLog = ctrl.Log.WithName("setup")

	// Default certificate directory operator-sdk expects to have
	sdkTLSDir = fmt.Sprintf("%s/k8s-webhook-server/serving-certs", os.TempDir())
)

const (
	// Do not change the leader election ID, otherwise multiple SSP operator instances
	// can be running during upgrade.
	leaderElectionID = "734f7229.kubevirt.io"

	// Certificate directory and file names OLM mounts certificates to
	olmTLSDir = "/apiserver.local.config/certificates"
	olmTLSCrt = "apiserver.crt"
	olmTLSKey = "apiserver.key"

	// Default cert file names operator-sdk expects to have
	sdkTLSCrt = "tls.crt"
	sdkTLSKey = "tls.key"

	webhookPort = 9443
)

// This callback executes on each client call returning a new config to be used
// please be aware that the APIServer is using http keepalive so this is going to
// be executed only after a while for fresh connections and not on existing ones
func getConfigForClient(ctx context.Context, cfg *tls.Config, cache cache.Cache) (*tls.Config, error) {
	var sspList ssp.SSPList
	err := cache.List(ctx, &sspList)
	if err != nil {
		return nil, err
	}

	if len(sspList.Items) == 0 || sspList.Items[0].Spec.TLSSecurityProfile == nil {
		cfg.MinVersion = crypto.DefaultTLSVersion()
		cfg.CipherSuites = nil
		return cfg, nil
	}

	tlsProfile := sspList.Items[0].Spec.TLSSecurityProfile
	if tlsProfile.Type == ocpconfigv1.TLSProfileCustomType {
		minVersion, err := crypto.TLSVersion(string(tlsProfile.Custom.MinTLSVersion))
		if err != nil {
			return nil, err
		}
		cfg.MinVersion = minVersion
		cfg.CipherSuites = common.CipherIDs(tlsProfile.Custom.Ciphers, &ctrl.Log)
		return cfg, nil
	}

	minVersion, err := crypto.TLSVersion(string(ocpconfigv1.TLSProfiles[tlsProfile.Type].MinTLSVersion))
	if err != nil {
		return nil, err
	}
	cfg.MinVersion = minVersion
	cfg.CipherSuites = common.CipherIDs(ocpconfigv1.TLSProfiles[tlsProfile.Type].Ciphers, &ctrl.Log)

	return cfg, nil
}

type prometheusServer struct {
	cache         cache.Cache
	certPath      string
	keyPath       string
	serverAddress string
}

func startCertWatcher(ctx context.Context, group *errgroup.Group, certWatcher *certwatcher.CertWatcher) {
	group.Go(func() error {
		if err := certWatcher.Start(ctx); err != nil {
			setupLog.Error(err, "certificate watcher error")
			return err
		}
		return nil
	})
}

func startServer(ctx context.Context, group *errgroup.Group, server *http.Server) {
	group.Go(func() error {
		if err := server.ListenAndServeTLS("", ""); !errors.Is(err, http.ErrServerClosed) {
			setupLog.Error(err, "Failed to start Prometheus metrics endpoint server")
			return err
		}
		return nil
	})

	group.Go(func() error {
		<-ctx.Done()
		setupLog.Info("shutting down Prometheus metrics server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			setupLog.Error(err, "error shutting down the HTTP server")
			return err
		}
		return nil
	})
}

// NeedLeaderElection implements the LeaderElectionRunnable interface, which indicates
// the prometheus server doesn't need leader election.
func (s *prometheusServer) NeedLeaderElection() bool {
	return false
}

func (s *prometheusServer) Start(ctx context.Context) error {
	setupLog.Info("Starting Prometheus metrics endpoint server with TLS")
	handler := promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{})
	mux := http.NewServeMux()
	mux.Handle("/metrics", handler)

	certWatcher, err := certwatcher.New(s.certPath, s.keyPath)
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:      s.serverAddress,
		Handler:   mux,
		TLSConfig: s.getPrometheusTLSConfig(ctx, certWatcher),
	}

	group, subCtx := errgroup.WithContext(ctx)
	startCertWatcher(subCtx, group, certWatcher)
	startServer(subCtx, group, server)
	return group.Wait()
}

func (s *prometheusServer) getPrometheusTLSConfig(ctx context.Context, certWatcher *certwatcher.CertWatcher) *tls.Config {
	return &tls.Config{
		GetConfigForClient: func(_ *tls.ClientHelloInfo) (*tls.Config, error) {
			cfg := &tls.Config{}
			cfg.GetCertificate = certWatcher.GetCertificate
			return getConfigForClient(ctx, cfg, s.cache)
		},
	}
}

func newPrometheusServer(metricsAddr string, cache cache.Cache) (*prometheusServer, error) {
	if err := sspMetrics.SetupMetrics(); err != nil {
		return nil, err
	}

	if err := rules.SetupRules(); err != nil {
		return nil, err
	}

	return &prometheusServer{
		certPath:      path.Join(sdkTLSDir, sdkTLSCrt),
		keyPath:       path.Join(sdkTLSDir, sdkTLSKey),
		cache:         cache,
		serverAddress: metricsAddr,
	}, nil
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8443", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	err := createCertificateSymlinks()
	if err != nil {
		setupLog.Error(err, "Error creating certificate symlinks")
		os.Exit(1)
	}

	ctx := ctrl.SetupSignalHandler()

	apiConfig, err := ctrl.GetConfig()
	if err != nil {
		setupLog.Error(err, "error getting API config")
		os.Exit(1)
	}

	// Using closure so that the temporary client is cleaned up after it is not needed anymore.
	ctrls, err := func() ([]controllers.Controller, error) {
		apiClient, err := client.New(apiConfig, client.Options{
			Scheme: common.Scheme,
		})
		if err != nil {
			return nil, err
		}
		return controllers.CreateControllers(ctx, apiClient)
	}()
	if err != nil {
		setupLog.Error(err, "error creating controllers")
		os.Exit(1)
	}

	var mgr ctrl.Manager

	getTLSOptsFunc := func(cfg *tls.Config) {
		cfg.GetConfigForClient = func(_ *tls.ClientHelloInfo) (*tls.Config, error) {
			return getConfigForClient(ctx, cfg, mgr.GetCache())
		}
	}

	mgr, err = ctrl.NewManager(apiConfig, ctrl.Options{
		Scheme: common.Scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       leaderElectionID,
		WebhookServer: webhook.NewServer(webhook.Options{
			Port:    webhookPort,
			TLSOpts: []func(*tls.Config){getTLSOptsFunc},
		}),
	})

	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		if err = webhooks.Setup(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "SSP")
			os.Exit(1)
		}
	}

	metricsServer, err := newPrometheusServer(metricsAddr, mgr.GetCache())
	if err != nil {
		setupLog.Error(err, "unable create Prometheus server")
		os.Exit(1)
	}

	if err := mgr.Add(metricsServer); err != nil {
		setupLog.Error(err, "unable to set up metrics")
		os.Exit(1)
	}

	if err := mgr.AddReadyzCheck("check", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("health", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder
	if err = controllers.StartControllers(ctx, mgr, ctrls); err != nil {
		setupLog.Error(err, "unable to create or start controller", "controller", "SSP")
		os.Exit(1)
	}
}

func createCertificateSymlinks() error {
	olmDir, olmDirErr := os.Stat(olmTLSDir)
	_, sdkDirErr := os.Stat(sdkTLSDir)

	// If certificates are generated by OLM, we should use OLM certificates mount path
	if olmDirErr == nil && olmDir.IsDir() && os.IsNotExist(sdkDirErr) {
		// For some reason, OLM maps the cert/key files to apiserver.crt/apiserver.key
		// instead of tls.crt/tls.key like the SDK expects. Creating symlinks to allow
		// the operator to find and use them.
		setupLog.Info("OLM cert directory found, copying cert files")

		err := os.MkdirAll(sdkTLSDir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create %s: %w", sdkTLSCrt, err)
		}

		err = os.Symlink(path.Join(olmTLSDir, olmTLSCrt), path.Join(sdkTLSDir, sdkTLSCrt))
		if err != nil {
			return err
		}

		err = os.Symlink(path.Join(olmTLSDir, olmTLSKey), path.Join(sdkTLSDir, sdkTLSKey))
		if err != nil {
			return err
		}
	} else {
		setupLog.Info("OLM cert directory not found, using default cert directory")
	}

	return nil
}
