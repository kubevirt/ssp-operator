package service

import (
	goflag "flag"
	"fmt"
	"strconv"

	flag "github.com/spf13/pflag"
)

var (
	defaultServiceVerbosity = "2"
)

type Service interface {
	Run()
	AddFlags()
}

type ServiceListen struct {
	Name        string
	BindAddress string
	Port        int
}

type ServiceLibvirt struct {
	LibvirtUri string
}

func (service *ServiceListen) Address() string {
	return fmt.Sprintf("%s:%s", service.BindAddress, strconv.Itoa(service.Port))
}

func (service *ServiceListen) InitFlags() {
	flag.CommandLine.AddGoFlagSet(goflag.CommandLine)
}

func (service *ServiceListen) AddCommonFlags() {
	flag.StringVar(&service.BindAddress, "listen", service.BindAddress, "Address where to listen on")
	flag.IntVar(&service.Port, "port", service.Port, "Port to listen on")
}

func (service *ServiceLibvirt) AddLibvirtFlags() {
	flag.StringVar(&service.LibvirtUri, "libvirt-uri", service.LibvirtUri, "Libvirt connection string")
}

func Setup(service Service) {
	service.AddFlags()

	if err := flag.Set("v", defaultServiceVerbosity); err != nil {
		panic(fmt.Sprintf("%v", err))
	}
	if err := flag.Set("logtostderr", "true"); err != nil {
		panic(fmt.Sprintf("%v", err))
	}

	// FIXME - Remove call to AddGoFlagSet once glog is no longer an indirect dependency
	// https://github.com/spf13/pflag/blob/master/README.md#supporting-go-flags-when-using-pflag
	flag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	flag.Parse()
}
