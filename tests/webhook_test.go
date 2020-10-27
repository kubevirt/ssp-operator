package tests

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sspv1alpha1 "kubevirt.io/ssp-operator/api/v1alpha1"
)

var _ = Describe("Validation webhook", func() {
	Context("creation", func() {
		It("should fail to create a second SSP CR", func() {
			ssp2 := &sspv1alpha1.SSP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ssp2",
					Namespace: testNamespace,
				},
				Spec: sspv1alpha1.SSPSpec{
					TemplateValidator: sspv1alpha1.TemplateValidator{
						Replicas: templateValidatorReplicas,
					},
				},
			}
			err := apiClient.Create(ctx, ssp2)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("creation failed, an SSP CR already exists: test-ssp"))
		})
	})
})
