package tests

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Validation webhook", func() {
	BeforeEach(func() {
		waitUntilDeployed()
	})

	Context("creation", func() {
		It("[test_id:5242] should fail to create a second SSP CR", func() {
			foundSsp := getSsp()
			ssp2 := foundSsp.DeepCopy()
			ssp2.Name = "test-ssp2"

			err := apiClient.Create(ctx, ssp2)
			if err == nil {
				apiClient.Delete(ctx, ssp2)
				Fail("Second SSP resource created.")
			}
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf(
				"creation failed, an SSP CR already exists in namespace %v: %v",
				foundSsp.Namespace,
				foundSsp.Name,
			)))
		})
	})

	Context("update", func() {
		BeforeEach(func() {
			strategy.SkipSspUpdateTestsIfNeeded()
		})

		AfterEach(func() {
			strategy.RevertToOriginalSspCr()
		})

		It("should fail to update commonTemplates.namespace", func() {
			foundSsp := getSsp()
			originalNs := foundSsp.Spec.CommonTemplates.Namespace
			foundSsp.Spec.CommonTemplates.Namespace = originalNs + "-updated"
			err := apiClient.Update(ctx, foundSsp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("commonTemplates.namespace cannot be changed."))
		})
	})
})
