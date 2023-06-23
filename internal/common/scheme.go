package common

import (
	osconfv1 "github.com/openshift/api/config/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	internalmeta "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	instancetypev1alpha2 "kubevirt.io/api/instancetype/v1alpha2"
	sspv1beta1 "kubevirt.io/ssp-operator/api/v1beta1"
	sspv1beta2 "kubevirt.io/ssp-operator/api/v1beta2"
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
	utilruntime.Must(sspv1beta2.AddToScheme(Scheme))
	utilruntime.Must(osconfv1.Install(Scheme))
	utilruntime.Must(instancetypev1alpha2.AddToScheme(Scheme))
}

// This function is useful in operand unit tests only
func AddConversionFunctions(s *runtime.Scheme) error {
	err := s.AddConversionFunc((*apiextensions.CustomResourceDefinition)(nil), (*metav1.PartialObjectMetadata)(nil), func(a, b interface{}, scope conversion.Scope) error {
		crd := a.(*apiextensions.CustomResourceDefinition)
		partialMeta := b.(*metav1.PartialObjectMetadata)

		partialMeta.TypeMeta = crd.TypeMeta
		partialMeta.ObjectMeta = crd.ObjectMeta
		return nil
	})
	if err != nil {
		return err
	}

	return s.AddConversionFunc((*apiextensions.CustomResourceDefinitionList)(nil), (*metav1.PartialObjectMetadataList)(nil), func(a, b interface{}, scope conversion.Scope) error {
		crdList := a.(*apiextensions.CustomResourceDefinitionList)
		partialMetaList := b.(*metav1.PartialObjectMetadataList)

		partialMetaList.TypeMeta = crdList.TypeMeta
		partialMetaList.ListMeta = crdList.ListMeta

		partialMetaList.Items = make([]metav1.PartialObjectMetadata, len(crdList.Items))
		for i := range crdList.Items {
			if err := scope.Convert(&crdList.Items[i], &partialMetaList.Items[i]); err != nil {
				return err
			}
		}
		return nil
	})
}
