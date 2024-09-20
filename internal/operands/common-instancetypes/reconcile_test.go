package common_instancetypes

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	libhandler "github.com/operator-framework/operator-lib/handler"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	internalmeta "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	instancetypeapi "kubevirt.io/api/instancetype"
	instancetypev1beta1 "kubevirt.io/api/instancetype/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ssp "kubevirt.io/ssp-operator/api/v1beta2"
	"kubevirt.io/ssp-operator/internal/common"
	crd_watch "kubevirt.io/ssp-operator/internal/crd-watch"
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
		operand                                 *CommonInstancetypes
		request                                 common.Request
		virtualMachineClusterInstancetypeCrdObj *extv1.CustomResourceDefinition
		virtualMachineClusterPreferenceCrdObj   *extv1.CustomResourceDefinition
	)

	const (
		namespace = "kubevirt"
		name      = "test-ssp"
	)

	BeforeEach(func() {
		operand = New()
		Expect(err).ToNot(HaveOccurred())

		Expect(internalmeta.AddToScheme(scheme.Scheme)).To(Succeed())
		Expect(extv1.AddToScheme(scheme.Scheme)).To(Succeed())
		Expect(addConversionFunctions(scheme.Scheme)).To(Succeed())
		Expect(instancetypev1beta1.AddToScheme(scheme.Scheme)).To(Succeed())

		client := fake.NewClientBuilder().Build()

		virtualMachineClusterInstancetypeCrdObj = &extv1.CustomResourceDefinition{
			TypeMeta: metav1.TypeMeta{
				APIVersion: extv1.SchemeGroupVersion.String(),
				Kind:       "CustomResourceDefinition",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: virtualMachineClusterInstancetypeCrd,
			},
		}
		Expect(client.Create(context.Background(), virtualMachineClusterInstancetypeCrdObj)).To(Succeed())

		virtualMachineClusterPreferenceCrdObj = &extv1.CustomResourceDefinition{
			TypeMeta: metav1.TypeMeta{
				APIVersion: extv1.SchemeGroupVersion.String(),
				Kind:       "CustomResourceDefinition",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: virtualMachineClusterPreferenceCrd,
			},
		}
		Expect(client.Create(context.Background(), virtualMachineClusterPreferenceCrdObj)).To(Succeed())

		crdWatch := crd_watch.New(nil, virtualMachineClusterInstancetypeCrd, virtualMachineClusterPreferenceCrd)
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
			CrdList:      crdWatch,
		}
	})

	It("should not fail cleanup if CRDs do not exist", func() {
		// Replace the client with a new one without the CRDs or instancetype schema present
		testScheme := runtime.NewScheme()
		Expect(internalmeta.AddToScheme(testScheme)).To(Succeed())
		Expect(extv1.AddToScheme(testScheme)).To(Succeed())

		request.Client = fake.NewClientBuilder().WithScheme(testScheme).Build()

		crdWatch := crd_watch.New(nil, virtualMachineClusterInstancetypeCrd, virtualMachineClusterPreferenceCrd)
		Expect(crdWatch.Init(request.Context, request.Client)).To(Succeed())

		request.CrdList = crdWatch

		// Assert that the CRDs are not present before we call Cleanup
		Expect(request.CrdList.CrdExists(virtualMachineClusterInstancetypeCrd)).To(BeFalse())
		Expect(request.CrdList.CrdExists(virtualMachineClusterPreferenceCrd)).To(BeFalse())
		Expect(request.CrdList.MissingCrds()).To(HaveLen(2))
		Expect(request.CrdList.MissingCrds()).To(ContainElements(virtualMachineClusterInstancetypeCrd, virtualMachineClusterPreferenceCrd))

		// Cleanup should not fail without the CRDs present
		cleanupResult, err := operand.Cleanup(&request)
		Expect(cleanupResult).To(BeNil())
		Expect(err).ToNot(HaveOccurred())
	})

	It("should cleanup any resources previously created by SSP", func() {
		// Create an instancetype and preference, marking both as owned by this operand
		instancetype := newVirtualMachineClusterInstancetype("old-instancetype")
		instancetype.ObjectMeta.Annotations = map[string]string{
			libhandler.NamespacedNameAnnotation: types.NamespacedName{
				Namespace: request.Instance.Namespace,
				Name:      request.Instance.Name,
			}.String(),
			libhandler.TypeAnnotation: request.Instance.GroupVersionKind().GroupKind().String(),
		}
		instancetype.ObjectMeta.Labels = map[string]string{common.AppKubernetesNameLabel: operand.Name()}
		Expect(request.Client.Create(request.Context, instancetype, &client.CreateOptions{})).To(Succeed())

		preference := newVirtualMachineClusterPreference("old-preference")
		preference.ObjectMeta.Labels = map[string]string{common.AppKubernetesNameLabel: operand.Name()}
		preference.ObjectMeta.Annotations = map[string]string{
			libhandler.NamespacedNameAnnotation: types.NamespacedName{
				Namespace: request.Instance.Namespace,
				Name:      request.Instance.Name,
			}.String(),
			libhandler.TypeAnnotation: request.Instance.GroupVersionKind().GroupKind().String(),
		}
		Expect(request.Client.Create(request.Context, preference, &client.CreateOptions{})).To(Succeed())

		_, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())
		ExpectResourceNotExists(instancetype, request)
		ExpectResourceNotExists(preference, request)
	})

	It("should not remove resources not created by SSP", func() {
		instancetype := newVirtualMachineClusterInstancetype("user-instancetype")
		Expect(request.Client.Create(request.Context, instancetype, &client.CreateOptions{})).To(Succeed())

		preference := newVirtualMachineClusterPreference("user-preference")
		Expect(request.Client.Create(request.Context, preference, &client.CreateOptions{})).To(Succeed())

		_, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())
		ExpectResourceExists(instancetype, request)
		ExpectResourceExists(preference, request)
	})
})

func addConversionFunctions(s *runtime.Scheme) error {
	err := s.AddConversionFunc((*extv1.CustomResourceDefinition)(nil), (*metav1.PartialObjectMetadata)(nil), func(a, b interface{}, scope conversion.Scope) error {
		crd := a.(*extv1.CustomResourceDefinition)
		partialMeta := b.(*metav1.PartialObjectMetadata)

		partialMeta.TypeMeta = crd.TypeMeta
		partialMeta.ObjectMeta = crd.ObjectMeta
		return nil
	})
	if err != nil {
		return err
	}

	return s.AddConversionFunc((*extv1.CustomResourceDefinitionList)(nil), (*metav1.PartialObjectMetadataList)(nil), func(a, b interface{}, scope conversion.Scope) error {
		crdList := a.(*extv1.CustomResourceDefinitionList)
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

func newVirtualMachineClusterInstancetype(name string) *instancetypev1beta1.VirtualMachineClusterInstancetype {
	return &instancetypev1beta1.VirtualMachineClusterInstancetype{
		TypeMeta: metav1.TypeMeta{
			Kind: instancetypeapi.ClusterSingularResourceName,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: instancetypev1beta1.VirtualMachineInstancetypeSpec{
			CPU: instancetypev1beta1.CPUInstancetype{
				Guest: uint32(1),
			},
		},
	}
}

func newVirtualMachineClusterPreference(name string) *instancetypev1beta1.VirtualMachineClusterPreference {
	return &instancetypev1beta1.VirtualMachineClusterPreference{
		TypeMeta: metav1.TypeMeta{
			Kind: instancetypeapi.ClusterSingularPreferenceResourceName,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}
