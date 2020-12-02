package tests

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"

	ssp "kubevirt.io/ssp-operator/api/v1beta1"
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

		Context("removed existing SSP CR", func() {
			BeforeEach(func() {
				strategy.SkipSspUpdateTestsIfNeeded()
			})

			AfterEach(func() {
				strategy.RevertToOriginalSspCr()
			})

			It("should fail to create SSP CR with invalid commonTemplates.namespace", func() {
				foundSsp := getSsp()

				Expect(apiClient.Delete(ctx, foundSsp)).ToNot(HaveOccurred())
				waitForDeletion(client.ObjectKey{Name: foundSsp.GetName(), Namespace: foundSsp.GetNamespace()}, &ssp.SSP{})

				foundSsp.Spec.CommonTemplates.Namespace = "nonexisting-templates-namespace"

				err := apiClient.Create(ctx, foundSsp)
				if err == nil {
					Fail("SSP CR with invalid commonTemplates.namespace created.")
					return
				}
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf(
					"creation failed, the configured namespace for common templates does not exist: %v",
					foundSsp.Spec.CommonTemplates.Namespace,
				)))
			})
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
