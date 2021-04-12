package tests

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api"
	ssp "kubevirt.io/ssp-operator/api/v1beta1"
)

// Placement API tests variables
var (
	placementAPIValidationTestKey         = "testKey"
	placementAPIValidationValidOperator   = corev1.NodeSelectorOpIn
	placementAPIValidationInvalidOperator = corev1.NodeSelectorOperator("Invalid")

	placementAPIValidationValidPlacement = api.NodePlacement{
		Affinity: &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      placementAPIValidationTestKey,
									Operator: placementAPIValidationValidOperator,
									Values:   []string{"val1", "val2"},
								},
							},
						},
					},
				},
			},
		},
	}

	placementAPIValidationInvalidPlacement = api.NodePlacement{
		Affinity: &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      placementAPIValidationTestKey,
									Operator: placementAPIValidationInvalidOperator,
									Values:   []string{"val1", "val2"},
								},
							},
						},
					},
				},
			},
		},
	}
)

var _ = Describe("Validation webhook", func() {
	BeforeEach(func() {
		waitUntilDeployed()
	})

	Context("creation", func() {
		It("[test_id:5242] should fail to create a second SSP CR", func() {
			foundSsp := getSsp()
			ssp2 := foundSsp.DeepCopy()
			ssp2.ObjectMeta = v1.ObjectMeta{
				Name:      "test-ssp2",
				Namespace: foundSsp.GetNamespace(),
			}

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
			var (
				newSsp *ssp.SSP
			)

			BeforeEach(func() {
				strategy.SkipSspUpdateTestsIfNeeded()

				foundSsp := getSsp()
				Expect(apiClient.Delete(ctx, foundSsp)).ToNot(HaveOccurred())
				waitForDeletion(client.ObjectKey{Name: foundSsp.GetName(), Namespace: foundSsp.GetNamespace()}, &ssp.SSP{})

				foundSsp.ObjectMeta = v1.ObjectMeta{
					Name:      foundSsp.GetName(),
					Namespace: foundSsp.GetNamespace(),
				}

				newSsp = foundSsp
			})

			AfterEach(func() {
				strategy.RevertToOriginalSspCr()
			})

			It("should fail to create SSP CR with invalid commonTemplates.namespace", func() {
				newSsp.Spec.CommonTemplates.Namespace = "nonexisting-templates-namespace"

				err := apiClient.Create(ctx, newSsp)
				if err == nil {
					Fail("SSP CR with invalid commonTemplates.namespace created.")
					return
				}
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf(
					"creation failed, the configured namespace for common templates does not exist: %v",
					newSsp.Spec.CommonTemplates.Namespace,
				)))
			})

			Context("Placement API validation", func() {
				It("should succeed with valid template-validator placement fields", func() {
					newSsp.Spec.TemplateValidator.Placement = &placementAPIValidationValidPlacement

					Expect(apiClient.Create(ctx, newSsp)).ToNot(HaveOccurred(),
						"failed to create SSP CR with valid template-validator placement fields")
				})

				It("should fail with invalid template-validator placement fields", func() {
					newSsp.Spec.TemplateValidator.Placement = &placementAPIValidationInvalidPlacement

					Expect(apiClient.Create(ctx, newSsp)).To(HaveOccurred(),
						"created SSP CR with invalid template-validator placement fields")
				})
			})
		})
	})

	Context("update", func() {
		var (
			foundSsp *ssp.SSP
		)

		BeforeEach(func() {
			strategy.SkipSspUpdateTestsIfNeeded()

			foundSsp = getSsp()
		})

		AfterEach(func() {
			strategy.RevertToOriginalSspCr()
		})

		It("should fail to update commonTemplates.namespace", func() {
			originalNs := foundSsp.Spec.CommonTemplates.Namespace
			foundSsp.Spec.CommonTemplates.Namespace = originalNs + "-updated"
			err := apiClient.Update(ctx, foundSsp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("commonTemplates.namespace cannot be changed."))
		})

		Context("Placement API validation", func() {
			It("should succeed with valid template-validator placement fields", func() {
				foundSsp.Spec.TemplateValidator.Placement = &placementAPIValidationValidPlacement
				Expect(apiClient.Update(ctx, foundSsp)).ToNot(HaveOccurred(),
					"failed to update SSP CR with valid template-validator placement fields")
			})

			It("should fail with invalid template-validator placement fields", func() {
				foundSsp.Spec.TemplateValidator.Placement = &placementAPIValidationInvalidPlacement
				Expect(apiClient.Update(ctx, foundSsp)).To(HaveOccurred(),
					"SSP CR updated with invalid template-validator placement fields")
			})
		})
	})
})
