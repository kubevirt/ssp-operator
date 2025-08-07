package service

import (
	goflag "flag"
	"fmt"
	"strconv"

	flag "github.com/spf13/pflag"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type Service interface {
	Run()
	AddFlags()
}

type ServiceListen struct {
	Name        string
	BindAddress string
	MetricsPort int
	WebhookPort int
}

type ServiceLibvirt struct {
	LibvirtUri string
}

func (service *ServiceListen) MetricsAddress() string {
	return fmt.Sprintf("%s:%s", service.BindAddress, strconv.Itoa(service.MetricsPort))
}

func (service *ServiceListen) WebhookAddress() string {
	return fmt.Sprintf("%s:%s", service.BindAddress, strconv.Itoa(service.WebhookPort))
}

func (service *ServiceListen) InitFlags() {
	flag.CommandLine.AddGoFlagSet(goflag.CommandLine)
}

func (service *ServiceListen) AddCommonFlags() {
	flag.StringVar(&service.BindAddress, "listen", service.BindAddress, "Address where to listen on")
	flag.IntVar(&service.MetricsPort, "metrics-port", service.MetricsPort, "Port to listen on for metrics")
	flag.IntVar(&service.WebhookPort, "webhook-port", service.WebhookPort, "Port to listen on for webhooks")
}

func (service *ServiceLibvirt) AddLibvirtFlags() {
	flag.StringVar(&service.LibvirtUri, "libvirt-uri", service.LibvirtUri, "Libvirt connection string")
}

func Setup(service Service) {
	service.AddFlags()

	zapOpts := zap.Options{}
	zapOpts.BindFlags(goflag.CommandLine)

	// FIXME - Remove call to AddGoFlagSet once glog is no longer an indirect dependency
	// https://github.com/spf13/pflag/blob/master/README.md#supporting-go-flags-when-using-pflag
	flag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zapOpts)))
}
