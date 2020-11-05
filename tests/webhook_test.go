package tests

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sspv1alpha1 "kubevirt.io/ssp-operator/api/v1alpha1"
)

var _ = Describe("Validation webhook", func() {
	BeforeEach(func() {
		waitUntilDeployed()
	})

	Context("creation", func() {
		It("[test_id:5242] should fail to create a second SSP CR", func() {
			ssp2 := ssp.DeepCopy()
			ssp2.Name = "test-ssp2"

			err := apiClient.Create(ctx, ssp2)
			if err == nil {
				apiClient.Delete(ctx, ssp2)
				Fail("Second SSP resource created.")
			}
			Expect(err.Error()).To(ContainSubstring("creation failed, an SSP CR already exists in namespace ssp-operator-functests: test-s"))
		})
	})

	Context("update", func() {
		It("should fail to update commonTemplates.namespace", func() {
			key := client.ObjectKey{Name: ssp.Name, Namespace: ssp.Namespace}
			foundSsp := &sspv1alpha1.SSP{}
			Expect(apiClient.Get(ctx, key, foundSsp)).ToNot(HaveOccurred())

			foundSsp.Spec.CommonTemplates.Namespace = commonTemplatesTestNS + "-updated"
			err := apiClient.Update(ctx, foundSsp)
			if err == nil {
				foundSsp.Spec.CommonTemplates.Namespace = commonTemplatesTestNS
				Expect(apiClient.Update(ctx, foundSsp)).ToNot(HaveOccurred())
				Fail("update succeeded")
			}

			Expect(err.Error()).To(ContainSubstring("commonTemplates.namespace cannot be changed."))
		})
	})
})
