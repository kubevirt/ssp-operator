package common_instancetypes

import (
	"context"
	"encoding/json"
	"fmt"
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
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/resource"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/kustomize/kyaml/kio"

	virtv1 "kubevirt.io/api/core/v1"
	instancetypeapi "kubevirt.io/api/instancetype"
	instancetypev1beta1 "kubevirt.io/api/instancetype/v1beta1"
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
		virtualMachineClusterInstancetypeCrdObj *apiextensions.CustomResourceDefinition
		virtualMachineClusterPreferenceCrdObj   *apiextensions.CustomResourceDefinition
	)

	const (
		namespace        = "kubevirt"
		name             = "test-ssp"
		instancetypePath = "../../../" + BundleDir + ClusterInstancetypesBundle
		preferencePath   = "../../../" + BundleDir + ClusterPreferencesBundle
	)

	assertResoucesExist := func(request common.Request, virtualMachineClusterInstancetypes []instancetypev1beta1.VirtualMachineClusterInstancetype, virtualMachineClusterPreferences []instancetypev1beta1.VirtualMachineClusterPreference) {
		for _, instancetype := range virtualMachineClusterInstancetypes {
			ExpectResourceExists(&instancetype, request)
		}
		for _, preference := range virtualMachineClusterPreferences {
			ExpectResourceExists(&preference, request)
		}
	}

	assertResoucesDoNotExist := func(request common.Request, virtualMachineClusterInstancetypes []instancetypev1beta1.VirtualMachineClusterInstancetype, virtualMachineClusterPreferences []instancetypev1beta1.VirtualMachineClusterPreference) {
		for _, instancetype := range virtualMachineClusterInstancetypes {
			ExpectResourceNotExists(&instancetype, request)
		}
		for _, preference := range virtualMachineClusterPreferences {
			ExpectResourceNotExists(&preference, request)
		}
	}

	BeforeEach(func() {
		operand = New(instancetypePath, preferencePath)
		Expect(err).ToNot(HaveOccurred())

		Expect(internalmeta.AddToScheme(scheme.Scheme)).To(Succeed())
		Expect(apiextensions.AddToScheme(scheme.Scheme)).To(Succeed())
		Expect(addConversionFunctions(scheme.Scheme)).To(Succeed())
		Expect(instancetypev1beta1.AddToScheme(scheme.Scheme)).To(Succeed())

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
			CrdList:      crdWatch,
		}
	})

	It("should not fail cleanup if CRDs do not exist", func() {
		// Replace the client with a new one without the CRDs or instancetype schema present
		testScheme := runtime.NewScheme()
		Expect(internalmeta.AddToScheme(testScheme)).To(Succeed())
		Expect(apiextensions.AddToScheme(testScheme)).To(Succeed())

		request.Client = fake.NewClientBuilder().WithScheme(testScheme).Build()

		crdWatch := crd_watch.New(virtualMachineClusterInstancetypeCrd, virtualMachineClusterPreferenceCrd)
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

	It("should create and cleanup resources from internal bundle", func() {
		_, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		virtualMachineClusterInstancetypes, err := FetchBundleResource[instancetypev1beta1.VirtualMachineClusterInstancetype](instancetypePath)
		Expect(err).ToNot(HaveOccurred())
		Expect(virtualMachineClusterInstancetypes).ToNot(BeEmpty())

		virtualMachineClusterPreferences, err := FetchBundleResource[instancetypev1beta1.VirtualMachineClusterPreference](preferencePath)
		Expect(err).ToNot(HaveOccurred())
		Expect(virtualMachineClusterPreferences).ToNot(BeEmpty())

		assertResoucesExist(request, virtualMachineClusterInstancetypes, virtualMachineClusterPreferences)

		// Assert that CrdList can see the required CRDs before we call Cleanup
		Expect(request.CrdList.CrdExists(virtualMachineClusterInstancetypeCrd)).To(BeTrue())
		Expect(request.CrdList.CrdExists(virtualMachineClusterPreferenceCrd)).To(BeTrue())

		_, err = operand.Cleanup(&request)
		Expect(err).ToNot(HaveOccurred())

		assertResoucesDoNotExist(request, virtualMachineClusterInstancetypes, virtualMachineClusterPreferences)
	})

	It("should cleanup any resources no longer provided by the bundle", func() {
		// Create an instancetype and preference, marking both as owned by this operand
		instancetype := newVirtualMachineClusterInstancetype("no-longer-provided-instancetype")
		instancetype.ObjectMeta.Labels = map[string]string{common.AppKubernetesNameLabel: operand.Name()}
		Expect(request.Client.Create(request.Context, instancetype, &client.CreateOptions{})).To(Succeed())

		preference := newVirtualMachineClusterPreference("no-longer-provided-preference")
		preference.ObjectMeta.Labels = map[string]string{common.AppKubernetesNameLabel: operand.Name()}
		Expect(request.Client.Create(request.Context, preference, &client.CreateOptions{})).To(Succeed())

		_, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())
		ExpectResourceNotExists(instancetype, request)
		ExpectResourceNotExists(preference, request)
	})

	It("should revert any user modifications to bundled resources when reconciling", func() {
		_, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		instancetypeList := &instancetypev1beta1.VirtualMachineClusterInstancetypeList{}
		Expect(request.Client.List(request.Context, instancetypeList, &client.ListOptions{})).To(Succeed())

		preferenceList := &instancetypev1beta1.VirtualMachineClusterPreferenceList{}
		Expect(request.Client.List(request.Context, preferenceList, &client.ListOptions{})).To(Succeed())

		instancetypeToUpdate := instancetypeList.Items[0]
		originalCPUGuestCount := instancetypeToUpdate.Spec.CPU.Guest
		updatedCPUGuestCount := originalCPUGuestCount + 1
		instancetypeToUpdate.Spec.CPU.Guest = updatedCPUGuestCount
		Expect(request.Client.Update(request.Context, &instancetypeToUpdate, &client.UpdateOptions{})).To(Succeed())
		Expect(instancetypeToUpdate.Spec.CPU.Guest).To(Equal(updatedCPUGuestCount))

		preferenceToUpdate := preferenceList.Items[0]
		originalPreferenceCPU := preferenceToUpdate.Spec.CPU
		updatedPreferredCPUTopology := instancetypev1beta1.PreferCores
		updatedPreferenceCPU := &instancetypev1beta1.CPUPreferences{
			PreferredCPUTopology: &updatedPreferredCPUTopology,
		}
		preferenceToUpdate.Spec.CPU = updatedPreferenceCPU
		Expect(request.Client.Update(request.Context, &preferenceToUpdate, &client.UpdateOptions{})).To(Succeed())
		Expect(preferenceToUpdate.Spec.CPU).To(Equal(updatedPreferenceCPU))

		_, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		Expect(request.Client.Get(request.Context, client.ObjectKeyFromObject(&instancetypeToUpdate), &instancetypeToUpdate, &client.GetOptions{})).To(Succeed())
		Expect(instancetypeToUpdate.Spec.CPU.Guest).To(Equal(originalCPUGuestCount))
		Expect(request.Client.Get(request.Context, client.ObjectKeyFromObject(&preferenceToUpdate), &preferenceToUpdate, &client.GetOptions{})).To(Succeed())
		Expect(preferenceToUpdate.Spec.CPU).To(Equal(originalPreferenceCPU))
	})

	It("should not cleanup any user resources when reconciling the bundle", func() {
		instancetype := newVirtualMachineClusterInstancetype("user-instancetype")
		Expect(request.Client.Create(request.Context, instancetype, &client.CreateOptions{})).To(Succeed())

		preference := newVirtualMachineClusterPreference("user-preference")
		Expect(request.Client.Create(request.Context, preference, &client.CreateOptions{})).To(Succeed())

		_, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())
		ExpectResourceExists(instancetype, request)
		ExpectResourceExists(preference, request)
	})

	It("should ignore virt-operator owned objects during reconcile when also provided by bundle", func() {
		_, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		instancetypeList := &instancetypev1beta1.VirtualMachineClusterInstancetypeList{}
		Expect(request.Client.List(request.Context, instancetypeList, &client.ListOptions{})).To(Succeed())
		Expect(len(instancetypeList.Items) > 0).To(BeTrue())

		preferenceList := &instancetypev1beta1.VirtualMachineClusterPreferenceList{}
		Expect(request.Client.List(request.Context, preferenceList, &client.ListOptions{})).To(Succeed())
		Expect(len(preferenceList.Items) > 0).To(BeTrue())

		// Mutate the instance type while also adding the labels for virt-operator
		instancetypeToUpdate := instancetypeList.Items[0]
		updatedCPUGuestCount := instancetypeToUpdate.Spec.CPU.Guest + 1
		instancetypeToUpdate.Spec.CPU.Guest = updatedCPUGuestCount
		instancetypeToUpdate.Labels = map[string]string{
			virtv1.ManagedByLabel: virtv1.ManagedByLabelOperatorValue,
		}
		Expect(request.Client.Update(request.Context, &instancetypeToUpdate, &client.UpdateOptions{})).To(Succeed())

		// Mutate the preference while also adding the labels for virt-operator
		preferenceToUpdate := preferenceList.Items[0]
		updatedPreferredCPUTopology := instancetypev1beta1.PreferCores
		updatedPreferenceCPU := &instancetypev1beta1.CPUPreferences{
			PreferredCPUTopology: &updatedPreferredCPUTopology,
		}
		preferenceToUpdate.Spec.CPU = updatedPreferenceCPU
		preferenceToUpdate.Labels = map[string]string{
			virtv1.ManagedByLabel: virtv1.ManagedByLabelOperatorValue,
		}
		Expect(request.Client.Update(request.Context, &preferenceToUpdate, &client.UpdateOptions{})).To(Succeed())

		results, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		// Assert that we have reported ignoring the attempt to reconcile the objects owned by virt-operator
		for _, res := range results {
			Expect(res.Resource.GetName()).ToNot(Equal(instancetypeToUpdate.Name))
			Expect(res.Resource.GetName()).ToNot(Equal(preferenceToUpdate.Name))
		}

		// Assert that the mutations made above persist as the reconcile is being ignored
		Expect(request.Client.Get(request.Context, client.ObjectKeyFromObject(&instancetypeToUpdate), &instancetypeToUpdate, &client.GetOptions{})).To(Succeed())
		Expect(instancetypeToUpdate.Spec.CPU.Guest).To(Equal(updatedCPUGuestCount))
		Expect(request.Client.Get(request.Context, client.ObjectKeyFromObject(&preferenceToUpdate), &preferenceToUpdate, &client.GetOptions{})).To(Succeed())
		Expect(preferenceToUpdate.Spec.CPU).To(Equal(updatedPreferenceCPU))
	})

	It("should create and cleanup resources from an external URL", func() {
		// Generate a mock ResMap and resources for the test
		mockResMap, virtualMachineClusterInstancetypes, virtualMachineClusterPreferences, err := newMockResources(10, 10)
		Expect(err).ToNot(HaveOccurred())

		// Use a mock Run function to return our fake ResMap
		operand.KustomizeRunFunc = func(_ filesys.FileSystem, _ string) (resmap.ResMap, error) {
			return mockResMap, nil
		}

		// Update the SSP CR to use a URL so that it calls our mock KustomizeRunFunc
		request.Instance.Spec.CommonInstancetypes = &ssp.CommonInstancetypes{
			URL: pointer.String("https://foo.com/bar?ref=1"),
		}

		// Run Reconcile and assert the results
		_, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		// Assert the expected resources have been created
		assertResoucesExist(request, virtualMachineClusterInstancetypes, virtualMachineClusterPreferences)

		_, err = operand.Cleanup(&request)
		Expect(err).ToNot(HaveOccurred())

		// Assert the expected resources have been cleaned up
		assertResoucesDoNotExist(request, virtualMachineClusterInstancetypes, virtualMachineClusterPreferences)
	})

	It("should create and cleanup resources when an external URL changes", func() {
		// Generate a mock ResMap and resources for the test
		mockResMap, originalInstancetypes, originalPreferences, err := newMockResources(0, 10)
		Expect(err).ToNot(HaveOccurred())
		Expect(originalInstancetypes).To(HaveLen(0))
		Expect(originalPreferences).To(HaveLen(10))

		operand.KustomizeRunFunc = func(_ filesys.FileSystem, _ string) (resmap.ResMap, error) {
			return mockResMap, nil
		}

		request.Instance.Spec.CommonInstancetypes = &ssp.CommonInstancetypes{
			URL: pointer.String("https://foo.com/bar?ref=1"),
		}

		_, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		assertResoucesExist(request, originalInstancetypes, originalPreferences)

		// Generate a new set of resources, this time without instancetypes
		mockResMap, updatedInstancetypes, updatedPreferences, err := newMockResources(10, 0)
		Expect(err).ToNot(HaveOccurred())
		Expect(updatedInstancetypes).To(HaveLen(10))
		Expect(updatedPreferences).To(HaveLen(0))

		operand.KustomizeRunFunc = func(_ filesys.FileSystem, _ string) (resmap.ResMap, error) {
			return mockResMap, nil
		}
		request.Instance.Spec.CommonInstancetypes = &ssp.CommonInstancetypes{
			URL: pointer.String("https://foo.com/bar?ref=2"),
		}

		_, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		// Assert the expected resources have been created
		assertResoucesExist(request, updatedInstancetypes, updatedPreferences)

		// Assert the expected resources have been removed
		assertResoucesDoNotExist(request, originalInstancetypes, originalPreferences)
	})

	It("should not cleanup any user resources when reconciling from a URL", func() {
		instancetype := newVirtualMachineClusterInstancetype("user-instancetype")
		Expect(request.Client.Create(request.Context, instancetype, &client.CreateOptions{})).To(Succeed())

		preference := newVirtualMachineClusterPreference("user-preference")
		Expect(request.Client.Create(request.Context, preference, &client.CreateOptions{})).To(Succeed())

		// Generate a mock ResMap and resources for the test
		mockResMap, _, _, err := newMockResources(10, 10)
		Expect(err).ToNot(HaveOccurred())

		// Use a mock Run function to return our fake ResMap
		operand.KustomizeRunFunc = func(_ filesys.FileSystem, _ string) (resmap.ResMap, error) {
			return mockResMap, nil
		}

		// Update the SSP CR to use a URL so that it calls KustomizeRunFunc
		request.Instance.Spec.CommonInstancetypes = &ssp.CommonInstancetypes{
			URL: pointer.String("https://foo.com/bar?ref=1"),
		}

		// Run Reconcile and assert the results
		_, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())
		ExpectResourceExists(instancetype, request)
		ExpectResourceExists(preference, request)
	})

	It("should not deploy internal bundle resources when featureGate is disabled", func() {
		request.Instance.Spec.FeatureGates = &ssp.FeatureGates{
			DeployCommonInstancetypes: pointer.Bool(false),
		}

		_, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		virtualMachineClusterInstancetypes, err := FetchBundleResource[instancetypev1beta1.VirtualMachineClusterInstancetype](instancetypePath)
		Expect(err).ToNot(HaveOccurred())
		Expect(virtualMachineClusterInstancetypes).ToNot(BeEmpty())

		virtualMachineClusterPreferences, err := FetchBundleResource[instancetypev1beta1.VirtualMachineClusterPreference](preferencePath)
		Expect(err).ToNot(HaveOccurred())
		Expect(virtualMachineClusterPreferences).ToNot(BeEmpty())

		assertResoucesDoNotExist(request, virtualMachineClusterInstancetypes, virtualMachineClusterPreferences)
	})

	It("should cleanup internal bundle resources from when featureGate is disabled after being enabled", func() {
		_, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		virtualMachineClusterInstancetypes, err := FetchBundleResource[instancetypev1beta1.VirtualMachineClusterInstancetype](instancetypePath)
		Expect(err).ToNot(HaveOccurred())
		Expect(virtualMachineClusterInstancetypes).ToNot(BeEmpty())

		virtualMachineClusterPreferences, err := FetchBundleResource[instancetypev1beta1.VirtualMachineClusterPreference](preferencePath)
		Expect(err).ToNot(HaveOccurred())
		Expect(virtualMachineClusterPreferences).ToNot(BeEmpty())

		assertResoucesExist(request, virtualMachineClusterInstancetypes, virtualMachineClusterPreferences)

		// Assert that CrdList can see the required CRDs before we Reconcile and Cleanup
		Expect(request.CrdList.CrdExists(virtualMachineClusterInstancetypeCrd)).To(BeTrue())
		Expect(request.CrdList.CrdExists(virtualMachineClusterPreferenceCrd)).To(BeTrue())

		request.Instance.Spec.FeatureGates = &ssp.FeatureGates{
			DeployCommonInstancetypes: pointer.Bool(false),
		}

		_, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		assertResoucesDoNotExist(request, virtualMachineClusterInstancetypes, virtualMachineClusterPreferences)
	})

	It("should not deploy external URI resources resources when featureGate is disabled", func() {
		// Generate a mock ResMap and resources for the test
		mockResMap, virtualMachineClusterInstancetypes, virtualMachineClusterPreferences, err := newMockResources(10, 10)
		Expect(err).ToNot(HaveOccurred())

		// Use a mock Run function to return our fake ResMap
		operand.KustomizeRunFunc = func(_ filesys.FileSystem, _ string) (resmap.ResMap, error) {
			return mockResMap, nil
		}

		request.Instance.Spec.FeatureGates = &ssp.FeatureGates{
			DeployCommonInstancetypes: pointer.Bool(false),
		}

		// Update the SSP CR to use a URL so that it calls our mock KustomizeRunFunc
		request.Instance.Spec.CommonInstancetypes = &ssp.CommonInstancetypes{
			URL: pointer.String("https://foo.com/bar?ref=1"),
		}

		// Run Reconcile and assert the results
		_, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		assertResoucesDoNotExist(request, virtualMachineClusterInstancetypes, virtualMachineClusterPreferences)
	})

	It("should cleanup external URI resources from when featureGate is disabled after being enabled", func() {
		// Generate a mock ResMap and resources for the test
		mockResMap, virtualMachineClusterInstancetypes, virtualMachineClusterPreferences, err := newMockResources(10, 10)
		Expect(err).ToNot(HaveOccurred())

		// Use a mock Run function to return our fake ResMap
		operand.KustomizeRunFunc = func(_ filesys.FileSystem, _ string) (resmap.ResMap, error) {
			return mockResMap, nil
		}

		// Update the SSP CR to use a URL so that it calls our mock KustomizeRunFunc
		request.Instance.Spec.CommonInstancetypes = &ssp.CommonInstancetypes{
			URL: pointer.String("https://foo.com/bar?ref=1"),
		}

		// Run Reconcile and assert the results
		_, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		assertResoucesExist(request, virtualMachineClusterInstancetypes, virtualMachineClusterPreferences)

		// Assert that CrdList can see the required CRDs before we Reconcile and Cleanup
		Expect(request.CrdList.CrdExists(virtualMachineClusterInstancetypeCrd)).To(BeTrue())
		Expect(request.CrdList.CrdExists(virtualMachineClusterPreferenceCrd)).To(BeTrue())

		request.Instance.Spec.FeatureGates = &ssp.FeatureGates{
			DeployCommonInstancetypes: pointer.Bool(false),
		}

		_, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		assertResoucesDoNotExist(request, virtualMachineClusterInstancetypes, virtualMachineClusterPreferences)
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

func convertToResMapResources(obj runtime.Object) ([]*resource.Resource, error) {
	var resources []*resource.Resource
	objBytes, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	ObjNodes, err := kio.FromBytes(objBytes)
	if err != nil {
		return nil, err
	}
	for _, node := range ObjNodes {
		resources = append(resources, &resource.Resource{RNode: *node})
	}
	return resources, nil
}

func newMockResources(countInstancetypes, countPreferences int) (*MockResMap, []instancetypev1beta1.VirtualMachineClusterInstancetype, []instancetypev1beta1.VirtualMachineClusterPreference, error) {
	var (
		resources     []*resource.Resource
		instancetypes []instancetypev1beta1.VirtualMachineClusterInstancetype
		preferences   []instancetypev1beta1.VirtualMachineClusterPreference
	)
	for i := 0; i < countInstancetypes; i++ {
		instancetype := newVirtualMachineClusterInstancetype(fmt.Sprintf("instancetype-%d", i))
		instancetypes = append(instancetypes, *instancetype)
		resource, err := convertToResMapResources(instancetype)
		if err != nil {
			return nil, nil, nil, err
		}
		resources = append(resources, resource...)
	}
	for i := 0; i < countPreferences; i++ {
		preference := newVirtualMachineClusterPreference(fmt.Sprintf("preference-%d", i))
		preferences = append(preferences, *preference)
		resource, err := convertToResMapResources(preference)
		if err != nil {
			return nil, nil, nil, err
		}
		resources = append(resources, resource...)
	}
	return &MockResMap{resources: resources}, instancetypes, preferences, nil
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
