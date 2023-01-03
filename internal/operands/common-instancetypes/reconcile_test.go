package common_instancetypes

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	internalmeta "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	instancetypev1alpha2 "kubevirt.io/api/instancetype/v1alpha2"
	ssp "kubevirt.io/ssp-operator/api/v1beta1"
	"kubevirt.io/ssp-operator/internal/common"
	crd_watch "kubevirt.io/ssp-operator/internal/crd-watch"
	"kubevirt.io/ssp-operator/internal/operands"
	. "kubevirt.io/ssp-operator/internal/test-utils"
)

var log = logf.Log.WithName("common-instancetypes-operand")

func TestInstancetypes(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Common Instance types Suite")
}

var _ = Describe("Common-Instancetypes operand", func() {

	var (
		err                                     error
		operand                                 operands.Operand
		request                                 common.Request
		virtualMachineClusterInstancetypeCrdObj *apiextensions.CustomResourceDefinition
		virtualMachineClusterPreferenceCrdObj   *apiextensions.CustomResourceDefinition
	)

	const (
		namespace        = "kubevirt"
		name             = "test-ssp"
		instancetypePath = "../../../" + BundleDir + ClusterInstancetypesBundlePrefix + ".yaml"
		preferencePath   = "../../../" + BundleDir + ClusterPreferencesBundlePrefix + ".yaml"
	)

	BeforeEach(func() {
		operand, err = New(instancetypePath, preferencePath)
		Expect(err).ToNot(HaveOccurred())

		Expect(internalmeta.AddToScheme(scheme.Scheme)).To(Succeed())
		Expect(apiextensions.AddToScheme(scheme.Scheme)).To(Succeed())
		Expect(addConversionFunctions(scheme.Scheme)).To(Succeed())
		Expect(instancetypev1alpha2.AddToScheme(scheme.Scheme)).To(Succeed())

		client := fake.NewClientBuilder().Build()

		virtualMachineClusterInstancetypeCrdObj = &apiextensions.CustomResourceDefinition{
			TypeMeta: metav1.TypeMeta{
				APIVersion: apiextensions.SchemeGroupVersion.String(),
				Kind:       "CustomResourceDefinition",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: virtualMachineClusterInstancetypeCrd,
			},
		}
		Expect(client.Create(context.Background(), virtualMachineClusterInstancetypeCrdObj)).To(Succeed())

		virtualMachineClusterPreferenceCrdObj = &apiextensions.CustomResourceDefinition{
			TypeMeta: metav1.TypeMeta{
				APIVersion: apiextensions.SchemeGroupVersion.String(),
				Kind:       "CustomResourceDefinition",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: virtualMachineClusterPreferenceCrd,
			},
		}
		Expect(client.Create(context.Background(), virtualMachineClusterPreferenceCrdObj)).To(Succeed())

		crdWatch := crd_watch.New(virtualMachineClusterInstancetypeCrd, virtualMachineClusterPreferenceCrd)
		Expect(crdWatch.Init(context.Background(), client)).To(Succeed())

		request = common.Request{
			Request: reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: namespace,
					Name:      name,
				},
			},
			Client:  client,
			Context: context.Background(),
			Instance: &ssp.SSP{
				TypeMeta: metav1.TypeMeta{
					Kind:       "SSP",
					APIVersion: ssp.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			},
			Logger:       log,
			VersionCache: common.VersionCache{},
			CrdWatch:     crdWatch,
		}
	})

	It("should create and cleanup common-instancetypes resources", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		virtualMachineClusterInstancetypes, err := fetchClusterResources[instancetypev1alpha2.VirtualMachineClusterInstancetype](instancetypePath)
		Expect(err).ToNot(HaveOccurred())
		Expect(virtualMachineClusterInstancetypes).ToNot(BeEmpty())

		virtualMachineClusterPreferences, err := fetchClusterResources[instancetypev1alpha2.VirtualMachineClusterPreference](preferencePath)
		Expect(err).ToNot(HaveOccurred())
		Expect(virtualMachineClusterPreferences).ToNot(BeEmpty())

		for _, instancetype := range virtualMachineClusterInstancetypes {
			ExpectResourceExists(&instancetype, request)
		}

		for _, preference := range virtualMachineClusterPreferences {
			ExpectResourceExists(&preference, request)
		}

		// Assert that CrdWatch can see the required CRDs before we call Cleanup
		Expect(request.CrdWatch.CrdExists(virtualMachineClusterInstancetypeCrd)).To(BeTrue())
		Expect(request.CrdWatch.CrdExists(virtualMachineClusterPreferenceCrd)).To(BeTrue())

		_, err = operand.Cleanup(&request)
		Expect(err).ToNot(HaveOccurred())

		for _, instancetype := range virtualMachineClusterInstancetypes {
			ExpectResourceNotExists(&instancetype, request)
		}

		for _, preference := range virtualMachineClusterPreferences {
			ExpectResourceNotExists(&preference, request)
		}
	})

	It("should not fail cleanup if CRDs do not exist", func() {
		// Replace the client with a new one without the CRDs or instancetype schema present
		Expect(internalmeta.AddToScheme(common.Scheme)).To(Succeed())
		Expect(apiextensions.AddToScheme(common.Scheme)).To(Succeed())
		request.Client = fake.NewClientBuilder().WithScheme(common.Scheme).Build()
		Expect(request.CrdWatch.Init(request.Context, request.Client)).To(Succeed())

		// Assert that the CRDs are not present before we call Cleanup
		Expect(request.CrdWatch.CrdExists(virtualMachineClusterInstancetypeCrd)).To(BeFalse())
		Expect(request.CrdWatch.CrdExists(virtualMachineClusterPreferenceCrd)).To(BeFalse())
		Expect(request.CrdWatch.MissingCrds()).To(HaveLen(2))
		Expect(request.CrdWatch.MissingCrds()).To(ContainElements(virtualMachineClusterInstancetypeCrd, virtualMachineClusterPreferenceCrd))

		// Cleanup should not fail without the CRDs present
		cleanupResult, err := operand.Cleanup(&request)
		Expect(cleanupResult).To(BeNil())
		Expect(err).ToNot(HaveOccurred())
	})
})

func addConversionFunctions(s *runtime.Scheme) error {
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
