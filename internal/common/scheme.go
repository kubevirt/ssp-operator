package common

import (
	osconfv1 "github.com/openshift/api/config/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	sspv1beta1 "kubevirt.io/ssp-operator/api/v1beta1"
)

var (
	// Scheme used for the SSP operator.
	Scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(Scheme))
	utilruntime.Must(extv1.AddToScheme(Scheme))
	utilruntime.Must(sspv1beta1.AddToScheme(Scheme))
	utilruntime.Must(osconfv1.Install(Scheme))
}
