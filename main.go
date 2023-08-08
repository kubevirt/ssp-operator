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
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"kubevirt.io/ssp-operator/controllers"
	"kubevirt.io/ssp-operator/internal/common"
	common_templates "kubevirt.io/ssp-operator/internal/operands/common-templates"
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

func runPrometheusServer(metricsAddr string, tlsOptions common.SSPTLSOptions) error {
	setupLog.Info("Starting Prometheus metrics endpoint server with TLS")
	metrics.Registry.MustRegister(common_templates.CommonTemplatesRestored)
	metrics.Registry.MustRegister(common.SSPOperatorReconcileSucceeded)
	handler := promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{})
	mux := http.NewServeMux()
	mux.Handle("/metrics", handler)

	minTlsVersion, err := tlsOptions.MinTLSVersionId()
	if err != nil {
		return err
	}

	tlsConfig := tls.Config{
		CipherSuites: tlsOptions.CipherIDs(&setupLog),
		MinVersion:   minTlsVersion,
	}

	server := http.Server{
		Addr:      metricsAddr,
		Handler:   mux,
		TLSConfig: &tlsConfig,
	}

	go func() {
		err := server.ListenAndServeTLS(path.Join(sdkTLSDir, sdkTLSCrt), path.Join(sdkTLSDir, sdkTLSKey))
		if err != nil {
			setupLog.Error(err, "Failed to start Prometheus metrics endpoint server")
		}
	}()
	return nil
}

func getWebhookServer(sspTLSOptions common.SSPTLSOptions) *webhook.Server {
	// If TLSSecurityProfile is empty, we want to return nil so that the default
	// webhook server configuration is used.
	if sspTLSOptions.IsEmpty() {
		return nil
	}

	tlsCfgFunc := func(cfg *tls.Config) {
		cfg.CipherSuites = sspTLSOptions.CipherIDs(&setupLog)
		setupLog.Info("Configured ciphers", "ciphers", cfg.CipherSuites)
	}

	funcs := []func(*tls.Config){tlsCfgFunc}
	return &webhook.Server{Port: webhookPort, TLSMinVersion: sspTLSOptions.MinTLSVersion, TLSOpts: funcs}
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

	tlsOptions, err := common.GetSspTlsOptions(ctx)
	if err != nil {
		setupLog.Error(err, "Error while getting tls profile")
		os.Exit(1)
	}

	err = runPrometheusServer(metricsAddr, *tlsOptions)
	if err != nil {
		setupLog.Error(err, "unable to start prometheus server")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 common.Scheme,
		MetricsBindAddress:     "0",
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       leaderElectionID,
		// If WebhookServer is set to nil, a default one will be created.
		WebhookServer: getWebhookServer(*tlsOptions),
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
	if err := mgr.AddReadyzCheck("check", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}
	if err := mgr.AddHealthzCheck("health", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder
	if err = controllers.CreateAndStartReconciler(ctx, mgr); err != nil {
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
