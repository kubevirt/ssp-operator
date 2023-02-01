package common

import (
	osconfv1 "github.com/openshift/api/config/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	internalmeta "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	kubevirt "kubevirt.io/api/core/v1"
	instancetypev1alpha2 "kubevirt.io/api/instancetype/v1alpha2"
	sspv1beta1 "kubevirt.io/ssp-operator/api/v1beta1"
)

var (
	// Scheme used for the SSP operator.
	Scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(Scheme))
	utilruntime.Must(extv1.AddToScheme(Scheme))
	utilruntime.Must(internalmeta.AddToScheme(Scheme))
	utilruntime.Must(sspv1beta1.AddToScheme(Scheme))
	utilruntime.Must(osconfv1.Install(Scheme))
	utilruntime.Must(instancetypev1alpha2.AddToScheme(Scheme))
	utilruntime.Must(kubevirt.AddToScheme(Scheme))
}
