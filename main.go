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
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	sspv1beta1 "kubevirt.io/ssp-operator/api/v1beta1"
	"kubevirt.io/ssp-operator/controllers"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
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
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(sspv1beta1.AddToScheme(scheme))
	utilruntime.Must(controllers.InitScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func runPrometheusServer(metricsAddr string) {
	setupLog.Info("Starting Prometheus metrics endpoint server with TLS")
	// we need to unregister a couple of collectors, otherwise the metrics gathering will fail
	// due to duplicate metrics collection
	metrics.Registry.Unregister(prometheus.NewGoCollector())
	metrics.Registry.Unregister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	gatherers := prometheus.Gatherers{
		metrics.Registry,
		prometheus.DefaultGatherer,
	}
	handler := promhttp.HandlerFor(gatherers, promhttp.HandlerOpts{})
	mux := http.NewServeMux()
	mux.Handle("/metrics", handler)
	server := http.Server{
		Addr:    metricsAddr,
		Handler: mux,
	}

	err := server.ListenAndServeTLS(path.Join(sdkTLSDir, sdkTLSCrt), path.Join(sdkTLSDir, sdkTLSKey))
	if err != nil {
		setupLog.Error(err, "Failed to start Prometheus metrics endpoint server")
	}
}

// Give permissions to use leases for leader election.
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete

func main() {
	var metricsAddr string
	var readyProbeAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":8443", "The address the metric endpoint binds to.")
	flag.StringVar(&readyProbeAddr, "ready-probe-addr", ":9440", "The address the readiness probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	err := copyCertificates()
	if err != nil {
		setupLog.Error(err, "Error copying certificates")
		os.Exit(1)
	}

	go runPrometheusServer(metricsAddr)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     "0",
		HealthProbeBindAddress: readyProbeAddr,
		Port:                   9443,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       leaderElectionID,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.SSPReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("SSP"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "SSP")
		os.Exit(1)
	}
	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		if err = (&sspv1beta1.SSP{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "SSP")
			os.Exit(1)
		}
	}
	err = mgr.AddReadyzCheck("ready", healthz.Ping)
	if err != nil {
		setupLog.Error(err, "unable to register readiness check")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func copyCertificates() error {
	olmDir, olmDirErr := os.Stat(olmTLSDir)
	_, sdkDirErr := os.Stat(sdkTLSDir)

	// If certificates are generated by OLM, we should use OLM certificates mount path
	if olmDirErr == nil && olmDir.IsDir() && os.IsNotExist(sdkDirErr) {
		// For some reason, OLM maps the cert/key files to apiserver.crt/apiserver.key
		// instead of tls.crt/tls.key like the SDK expects, so copying those files to allow
		// the operator to find and use them.
		setupLog.Info("OLM cert directory found, copying cert files")

		err := os.MkdirAll(sdkTLSDir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create %s: %w", sdkTLSCrt, err)
		}

		err = copyFile(path.Join(olmTLSDir, olmTLSCrt), path.Join(sdkTLSDir, sdkTLSCrt))
		if err != nil {
			return fmt.Errorf("failed to copy %s/%s to %s/%s: %w", olmTLSDir, olmTLSCrt, sdkTLSDir, sdkTLSCrt, err)
		}

		err = copyFile(path.Join(olmTLSDir, olmTLSKey), path.Join(sdkTLSDir, sdkTLSKey))
		if err != nil {
			return fmt.Errorf("failed to copy %s/%s to %s/%s: %w", olmTLSDir, olmTLSKey, sdkTLSDir, sdkTLSKey, err)
		}
	} else {
		setupLog.Info("OLM cert directory not found, using default")
	}

	return nil
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
