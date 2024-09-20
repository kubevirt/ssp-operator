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
	"flag"
	"fmt"
	"net/http"
	"os"
	"path"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	ocpconfigv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/pkg/crypto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	ssp "kubevirt.io/ssp-operator/api/v1beta2"
	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/controllers"
	"kubevirt.io/ssp-operator/internal/env"
	sspMetrics "kubevirt.io/ssp-operator/pkg/monitoring/metrics/ssp-operator"
	"kubevirt.io/ssp-operator/pkg/monitoring/rules"
	"kubevirt.io/ssp-operator/webhooks"
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

	server := &http.Server{
		Addr:    s.serverAddress,
		Handler: mux,
	}

	certWatcher, err := certwatcher.New(s.certPath, s.keyPath)
	if err != nil {
		return err
	}

	go func() {
		// TODO: change context, so it can be closed when
		// this function returns an error
		if err := certWatcher.Start(ctx); err != nil {
			setupLog.Error(err, "certificate watcher error")
		}
	}()

	idleConnsClosed := make(chan struct{})
	go func() {
		// TODO: make sure that the goroutine finishes when
		// this function returns an error
		<-ctx.Done()
		setupLog.Info("shutting down Prometheus metrics server")

		if err := server.Shutdown(context.Background()); err != nil {
			setupLog.Error(err, "error shutting down the HTTP server")
		}
		close(idleConnsClosed)
	}()

	server.TLSConfig = s.getPrometheusTLSConfig(ctx, certWatcher)

	if err := server.ListenAndServeTLS(s.certPath, s.keyPath); err != nil && err != http.ErrServerClosed {
		setupLog.Error(err, "Failed to start Prometheus metrics endpoint server")
		return err
	}

	<-idleConnsClosed
	return nil
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

	apiClient, err := client.New(apiConfig, client.Options{
		Scheme: common.Scheme,
	})
	if err != nil {
		setupLog.Error(err, "error creating API client")
		os.Exit(1)
	}

	ctrls, err := controllers.CreateControllers(ctx, apiClient)
	if err != nil {
		setupLog.Error(err, "error creating controllers")
		os.Exit(1)
	}

	operatorNamespace, err := env.GetOperatorNamespace()
	if err != nil {
		setupLog.Error(err, "error getting operator namespace")
		os.Exit(1)
	}

	var mgr ctrl.Manager

	getTLSOptsFunc := func(cfg *tls.Config) {
		cfg.GetConfigForClient = func(_ *tls.ClientHelloInfo) (*tls.Config, error) {
			return getConfigForClient(ctx, cfg, mgr.GetCache())
		}
	}

	cacheOptions, err := createCacheOptions(ctrls, operatorNamespace)
	if err != nil {
		setupLog.Error(err, "error creating cache options")
		os.Exit(1)
	}

	mgr, err = ctrl.NewManager(apiConfig, ctrl.Options{
		Scheme: common.Scheme,
		Cache:  cacheOptions,
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
		if err = webhooks.Setup(apiClient, operatorNamespace, mgr); err != nil {
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

func createCacheOptions(ctrls []controllers.Controller, operatorNamespace string) (cache.Options, error) {
	watchObjectsMap := map[schema.GroupVersionKind]controllers.WatchObject{}
	for _, controller := range ctrls {
		watchObjects := controller.GetWatchObjects()
		for _, watchObject := range watchObjects {
			gvk, err := apiutil.GVKForObject(watchObject.Object, common.Scheme)
			if err != nil {
				return cache.Options{}, err
			}
			existingObject, exists := watchObjectsMap[gvk]
			if !exists {
				watchObjectsMap[gvk] = watchObject
				continue
			}

			// If one of the objects wants to watch all namespaces or
			// objects without labels,
			// then the resulting WatchObject should too.
			if !watchObject.WatchOnlyOperatorNamespace {
				existingObject.WatchOnlyOperatorNamespace = false
			}
			if !watchObject.WatchOnlyObjectsWithLabel {
				existingObject.WatchOnlyObjectsWithLabel = false
			}

			watchObjectsMap[gvk] = existingObject
		}
	}

	cacheOptions := cache.Options{
		ByObject: map[client.Object]cache.ByObject{},
	}
	for _, watchObject := range watchObjectsMap {
		if !watchObject.WatchOnlyOperatorNamespace && !watchObject.WatchOnlyObjectsWithLabel {
			continue
		}

		byObject := cache.ByObject{}
		if watchObject.WatchOnlyOperatorNamespace {
			byObject.Namespaces = map[string]cache.Config{
				// TODO -- verify that the label selector is defaulted to the below selector
				operatorNamespace: {},
			}
		}

		if watchObject.WatchOnlyObjectsWithLabel {
			requirement, err := labels.NewRequirement(common.WatchedObjectLabel, selection.Equals, []string{"true"})
			if err != nil {
				// It is ok to panic, because the above function has constant arguments, so any error is a programmer's mistake.
				panic(fmt.Sprintf("Could not create label selector: %v", err))
			}
			byObject.Label = labels.NewSelector().Add(*requirement)
		}

		cacheOptions.ByObject[watchObject.Object] = byObject
	}

	return cacheOptions, nil
}
