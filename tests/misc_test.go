package tests

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	lifecycleapi "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api"

	sspv1beta1 "kubevirt.io/ssp-operator/api/v1beta1"
)

var _ = Describe("Observed generation", func() {
	BeforeEach(func() {
		strategy.SkipSspUpdateTestsIfNeeded()
	})

	AfterEach(func() {
		strategy.RevertToOriginalSspCr()
		waitUntilDeployed()
	})

	It("[test_id:6058] after deployment observedGeneration equals generation", func() {
		ssp := getSsp()
		Expect(ssp.Status.ObservedGeneration).To(Equal(ssp.Generation))
	})

	It("[test_id:6059] should update observed generation after CR update", func() {
		watch, err := StartWatch(sspListerWatcher)
		Expect(err).ToNot(HaveOccurred())
		defer watch.Stop()

		var newValidatorReplicas int32 = 0
		updateSsp(func(foundSsp *sspv1beta1.SSP) {
			foundSsp.Spec.TemplateValidator.Replicas = &newValidatorReplicas
		})

		// Watch changes until above change
		err = WatchChangesUntil(watch, func(updatedSsp *sspv1beta1.SSP) bool {
			return *updatedSsp.Spec.TemplateValidator.Replicas == newValidatorReplicas &&
				updatedSsp.Generation > updatedSsp.Status.ObservedGeneration
		}, shortTimeout)
		Expect(err).ToNot(HaveOccurred())

		// Watch changes until SSP operator updates ObservedGeneration
		err = WatchChangesUntil(watch, func(updatedSsp *sspv1beta1.SSP) bool {
			return *updatedSsp.Spec.TemplateValidator.Replicas == newValidatorReplicas &&
				updatedSsp.Generation == updatedSsp.Status.ObservedGeneration
		}, shortTimeout)
		Expect(err).ToNot(HaveOccurred())
	})

	It("[test_id:6060] should update observed generation when removing CR", func() {
		watch, err := StartWatch(sspListerWatcher)
		Expect(err).ToNot(HaveOccurred())
		defer watch.Stop()

		ssp := getSsp()
		Expect(apiClient.Delete(ctx, ssp)).ToNot(HaveOccurred())

		// Check for deletion timestamp before the SSP operator notices change
		err = WatchChangesUntil(watch, func(updatedSsp *sspv1beta1.SSP) bool {
			return updatedSsp.DeletionTimestamp != nil &&
				updatedSsp.Generation > updatedSsp.Status.ObservedGeneration
		}, shortTimeout)
		Expect(err).ToNot(HaveOccurred())

		// SSP operator enters Deleting phase
		err = WatchChangesUntil(watch, func(updatedSsp *sspv1beta1.SSP) bool {
			return updatedSsp.DeletionTimestamp != nil &&
				updatedSsp.Status.Phase == lifecycleapi.PhaseDeleting &&
				updatedSsp.Generation == updatedSsp.Status.ObservedGeneration
		}, shortTimeout)
		Expect(err).ToNot(HaveOccurred())
	})
})
