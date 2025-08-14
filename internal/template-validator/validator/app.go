package validator

import (
	"context"
	"crypto/tls"
	"net/http"
	"os"
	"time"

	templatev1 "github.com/openshift/api/template/v1"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	flag "github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	kubevirtv1 "kubevirt.io/api/core/v1"

	"kubevirt.io/ssp-operator/internal/template-validator/logger"
	"kubevirt.io/ssp-operator/internal/template-validator/service"
	"kubevirt.io/ssp-operator/internal/template-validator/tlsinfo"
	"kubevirt.io/ssp-operator/internal/template-validator/version"
	"kubevirt.io/ssp-operator/internal/template-validator/virtinformers"
	validating "kubevirt.io/ssp-operator/internal/template-validator/webhooks"
	validatorMetrics "kubevirt.io/ssp-operator/pkg/monitoring/metrics/template-validator"
)

const (
	defaultMetricsPort = 8443
	defaultWebhookPort = 9443
	defaultHost        = "0.0.0.0"

	tlsOptionsDirectory = "/tls-options"

	metricsServerType = "metrics"
	webhookServerType = "webhook"
)

type App struct {
	service.ServiceListen
	certsDir    string
	versionOnly bool
}

var _ service.Service = &App{}

func (app *App) AddFlags() {
	app.InitFlags()
	app.BindAddress = defaultHost
	app.MetricsPort = defaultMetricsPort
	app.WebhookPort = defaultWebhookPort
	app.AddCommonFlags()

	flag.StringVarP(&app.certsDir, "cert-dir", "c", "", "specify path to the directory containing TLS key and certificate - this enables TLS")
	flag.BoolVarP(&app.versionOnly, "version", "V", false, "show version and exit")
}

func (app *App) Run() {
	logger.Log.Info("Starting",
		"component", version.COMPONENT,
		"version", version.VERSION,
		"revision", version.REVISION,
	)
	if app.versionOnly {
		return
	}

	// We cannot use default scheme.Scheme, because it contains duplicate definitions
	// for kubevirt resources and the client would fail with an error:
	// "multiple group-version-kinds associated with type *v1.VirtualMachineList, refusing to guess at one"
	apiScheme := createScheme()

	informers, err := virtinformers.NewInformers(apiScheme)
	if err != nil {
		logger.Log.Error(err, "Error creating informers")
		panic(err)
	}

	informers.Start()
	defer informers.Stop()

	if err := validatorMetrics.SetupMetrics(); err != nil {
		logger.Log.Error(err, "Error setting up metrics")
		panic(err)
	}

	tlsInfo, err := app.setupTLS()
	if err != nil {
		panic(err)
	}
	if tlsInfo != nil {
		defer tlsInfo.Clean()
	}

	metricsServer := app.createMetricsServer()
	webhookServer := app.createWebhookServer(informers)
	if tlsInfo != nil {
		metricsServer.TLSConfig = createTLSConfig(tlsInfo)
		webhookServer.TLSConfig = createTLSConfig(tlsInfo)
	}

	g, ctx := errgroup.WithContext(context.Background())
	g.Go(func() error { return startServer(metricsServer, metricsServerType) })
	g.Go(func() error { return startServer(webhookServer, webhookServerType) })
	g.Go(func() error { return shutdownServer(metricsServer, metricsServerType, ctx) })
	g.Go(func() error { return shutdownServer(webhookServer, webhookServerType, ctx) })

	if err := g.Wait(); err != nil {
		panic(err)
	}
}

func (app *App) setupTLS() (*tlsinfo.TLSInfo, error) {
	if app.certsDir == "" {
		return nil, nil
	}

	logger.Log.Info("TLS certs directory", "directory", app.certsDir)

	tlsInfo := &tlsinfo.TLSInfo{
		CertsDirectory:      app.certsDir,
		TLSOptionsDirectory: tlsOptionsDirectory,
	}

	if err := tlsInfo.Init(); err != nil {
		logger.Log.Error(err, "Failed initializing TLSInfo")
		return nil, err
	}

	return tlsInfo, nil
}

func (app *App) createMetricsServer() *http.Server {
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())

	return &http.Server{
		Addr:    app.MetricsAddress(),
		Handler: metricsMux,
	}
}

func (app *App) createWebhookServer(informers *virtinformers.Informers) *http.Server {
	webhookMux := http.NewServeMux()
	validating.NewWebhooks(informers).Register(webhookMux)

	webhookMux.HandleFunc("/readyz", func(resp http.ResponseWriter, req *http.Request) {
		if _, err := resp.Write([]byte("ok")); err != nil {
			logger.Log.Error(err, "Failed to write response to /readyz")
		}
	})

	return &http.Server{
		Addr:    app.WebhookAddress(),
		Handler: webhookMux,
	}
}

func startServer(server *http.Server, serverType string) error {
	if server.TLSConfig != nil {
		logger.Log.Info("TLS configured, serving "+serverType+" over HTTPS", "address", server.Addr)
		if err := server.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			logger.Log.Error(err, "Error listening "+serverType+" TLS")
			return err
		}
	} else {
		logger.Log.Info("TLS disabled, serving "+serverType+" over HTTP", "address", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Log.Error(err, "Error listening "+serverType)
			return err
		}
	}
	return nil
}

func shutdownServer(server *http.Server, serverType string, ctx context.Context) error {
	<-ctx.Done()
	logger.Log.Info("Shutting down " + serverType + " server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}

func createTLSConfig(tlsInfo *tlsinfo.TLSInfo) *tls.Config {
	return &tls.Config{
		GetConfigForClient: func(_ *tls.ClientHelloInfo) (*tls.Config, error) {
			return tlsInfo.CreateTlsConfig()
		},
		GetCertificate: func(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
			// This function is not called, but it needs to be non-nil, otherwise
			// the server tries to load certificate from filenames passed to
			// ListenAndServe().
			panic("function should not be called")
		},
	}
}

func createScheme() *runtime.Scheme {
	sch := runtime.NewScheme()

	utilruntime.Must(clientgoscheme.AddToScheme(sch))
	utilruntime.Must(templatev1.Install(sch))

	// Setting API version of kubevirt that we want to register
	utilruntime.Must(os.Setenv(kubevirtv1.KubeVirtClientGoSchemeRegistrationVersionEnvVar, "v1"))
	utilruntime.Must(kubevirtv1.AddToScheme(sch))

	return sch
}
