package tests

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
	virtv1 "kubevirt.io/api/core/v1"
	instancetypev1beta1 "kubevirt.io/api/instancetype/v1beta1"
	ssp "kubevirt.io/ssp-operator/api/v1beta2"
	common_instancetypes "kubevirt.io/ssp-operator/internal/operands/common-instancetypes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/krusty"
)

var _ = Describe("Common Instance Types", func() {
	BeforeEach(func() {
		strategy.SkipSspUpdateTestsIfNeeded()
	})

	AfterEach(func() {
		strategy.RevertToOriginalSspCr()
	})

	Context("operand", func() {
		It("should reconcile resources from internal bundle by default", func() {
			virtualMachineClusterInstancetypes, err := common_instancetypes.FetchBundleResource[instancetypev1beta1.VirtualMachineClusterInstancetype]("../" + common_instancetypes.BundleDir + common_instancetypes.ClusterInstancetypesBundle)
			Expect(err).ToNot(HaveOccurred())

			virtualMachineClusterPreferences, err := common_instancetypes.FetchBundleResource[instancetypev1beta1.VirtualMachineClusterPreference]("../" + common_instancetypes.BundleDir + common_instancetypes.ClusterPreferencesBundle)
			Expect(err).ToNot(HaveOccurred())

			for _, instancetype := range virtualMachineClusterInstancetypes {
				Expect(apiClient.Get(ctx, client.ObjectKey{Name: instancetype.Name}, &instancetypev1beta1.VirtualMachineClusterInstancetype{})).To(Succeed())
			}

			for _, preference := range virtualMachineClusterPreferences {
				Expect(apiClient.Get(ctx, client.ObjectKey{Name: preference.Name}, &instancetypev1beta1.VirtualMachineClusterPreference{})).To(Succeed())
			}
		})
		It("should reconcile from URL when provided", func() {
			URL := "https://github.com/kubevirt/common-instancetypes//VirtualMachineClusterPreferences?ref=v0.3.3"
			sspObj := getSsp()
			sspObj.Spec.CommonInstancetypes = &ssp.CommonInstancetypes{
				URL: pointer.String(URL),
			}
			createOrUpdateSsp(sspObj)
			waitUntilDeployed()

			k := krusty.MakeKustomizer(krusty.MakeDefaultOptions())
			c := common_instancetypes.CommonInstancetypes{
				KustomizeRunFunc: k.Run,
			}
			virtualMachineClusterInstancetypes, virtualMachineClusterPreferences, err := c.FetchResourcesFromURL(URL)
			Expect(err).ToNot(HaveOccurred())

			for _, instancetype := range virtualMachineClusterInstancetypes {
				Expect(apiClient.Get(ctx, client.ObjectKey{Name: instancetype.Name}, &instancetypev1beta1.VirtualMachineClusterInstancetype{})).To(Succeed())
			}

			for _, preference := range virtualMachineClusterPreferences {
				Expect(apiClient.Get(ctx, client.ObjectKey{Name: preference.Name}, &instancetypev1beta1.VirtualMachineClusterPreference{})).To(Succeed())
			}
		})
		It("should ignore resources owned by virt-operator", func() {
			virtualMachineClusterInstancetypes, err := common_instancetypes.FetchBundleResource[instancetypev1beta1.VirtualMachineClusterInstancetype]("../" + common_instancetypes.BundleDir + common_instancetypes.ClusterInstancetypesBundle)
			Expect(err).ToNot(HaveOccurred())
			Expect(virtualMachineClusterInstancetypes).ToNot(BeEmpty())

			virtualMachineClusterPreferences, err := common_instancetypes.FetchBundleResource[instancetypev1beta1.VirtualMachineClusterPreference]("../" + common_instancetypes.BundleDir + common_instancetypes.ClusterPreferencesBundle)
			Expect(err).ToNot(HaveOccurred())
			Expect(virtualMachineClusterPreferences).ToNot(BeEmpty())

			// Mutate the preference while also adding the labels for virt-operator
			instancetypeToUpdate := &virtualMachineClusterInstancetypes[0]
			Expect(apiClient.Get(ctx, client.ObjectKey{Name: instancetypeToUpdate.Name}, instancetypeToUpdate)).To(Succeed())
			updatedCPUGuestCount := instancetypeToUpdate.Spec.CPU.Guest + 1
			instancetypeToUpdate.Spec.CPU.Guest = updatedCPUGuestCount
			instancetypeToUpdate.Labels = map[string]string{
				virtv1.ManagedByLabel: virtv1.ManagedByLabelOperatorValue,
			}
			Expect(apiClient.Update(ctx, instancetypeToUpdate)).To(Succeed())

			// Mutate the preference while also adding the labels for virt-operator
			preferenceToUpdate := &virtualMachineClusterPreferences[0]
			Expect(apiClient.Get(ctx, client.ObjectKey{Name: preferenceToUpdate.Name}, preferenceToUpdate)).To(Succeed())
			updatedPreferredCPUTopology := instancetypev1beta1.PreferCores
			updatedPreferenceCPU := &instancetypev1beta1.CPUPreferences{
				PreferredCPUTopology: &updatedPreferredCPUTopology,
			}
			preferenceToUpdate.Spec.CPU = updatedPreferenceCPU
			preferenceToUpdate.Labels = map[string]string{
				virtv1.ManagedByLabel: virtv1.ManagedByLabelOperatorValue,
			}
			Expect(apiClient.Update(ctx, preferenceToUpdate)).To(Succeed())

			triggerReconciliation()

			// Assert that the mutations made above persist as the reconcile is being ignored
			Expect(apiClient.Get(ctx, client.ObjectKey{Name: instancetypeToUpdate.Name}, instancetypeToUpdate)).To(Succeed())
			Expect(instancetypeToUpdate.Spec.CPU.Guest).To(Equal(updatedCPUGuestCount))
			Expect(apiClient.Get(ctx, client.ObjectKey{Name: preferenceToUpdate.Name}, preferenceToUpdate)).To(Succeed())
			Expect(preferenceToUpdate.Spec.CPU).To(Equal(updatedPreferenceCPU))
		})
	})
	Context("webhook", func() {
		DescribeTable("should reject URL", func(URL string) {
			sspObj := getSsp()
			sspObj.Spec.CommonInstancetypes = &ssp.CommonInstancetypes{
				URL: pointer.String(URL),
			}
			err := apiClient.Update(ctx, sspObj)
			Expect(err).To(HaveOccurred())
		},
			Entry("with file://", "file://foo/bar"),
			Entry("with foo://", "foo://foo/bar"),
			Entry("without ?ref=", "https://foo/bar"),
		)
	})
})
