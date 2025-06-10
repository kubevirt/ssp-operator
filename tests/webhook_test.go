package tests

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kubevirt.io/controller-lifecycle-operator-sdk/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sspv1beta2 "kubevirt.io/ssp-operator/api/v1beta2"
	sspv1beta3 "kubevirt.io/ssp-operator/api/v1beta3"
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
		It("[test_id:5242] [v1beta2] should fail to create a second SSP CR", func() {
			foundSsp := getSspV1Beta2()
			ssp2 := foundSsp.DeepCopy()
			ssp2.ObjectMeta = v1.ObjectMeta{
				Name:      "test-ssp2",
				Namespace: foundSsp.GetNamespace(),
			}

			err := apiClient.Create(ctx, ssp2, client.DryRunAll)
			Expect(err).To(MatchError(ContainSubstring(
				"creation failed, an SSP CR already exists in namespace %v: %v",
				foundSsp.Namespace,
				foundSsp.Name,
			)))
		})

		It("[test_id:TODO] [v1beta3] should fail to create a second SSP CR", func() {
			foundSsp := getSsp()
			ssp2 := foundSsp.DeepCopy()
			ssp2.ObjectMeta = v1.ObjectMeta{
				Name:      "test-ssp2",
				Namespace: foundSsp.GetNamespace(),
			}

			err := apiClient.Create(ctx, ssp2, client.DryRunAll)
			Expect(err).To(MatchError(ContainSubstring(
				"creation failed, an SSP CR already exists in namespace %v: %v",
				foundSsp.Namespace,
				foundSsp.Name,
			)))
		})

		Context("removed existing SSP CR", func() {
			var (
				newSspV1Beta2 *sspv1beta2.SSP
				newSspV1Beta3 *sspv1beta3.SSP
			)

			BeforeEach(func() {
				strategy.SkipSspUpdateTestsIfNeeded()

				newSspV1Beta2 = getSspV1Beta2()
				foundSsp := getSsp()

				Expect(apiClient.Delete(ctx, foundSsp)).ToNot(HaveOccurred())
				waitForDeletion(client.ObjectKey{Name: foundSsp.GetName(), Namespace: foundSsp.GetNamespace()}, &sspv1beta2.SSP{})

				foundSsp.ObjectMeta = v1.ObjectMeta{
					Name:      foundSsp.GetName(),
					Namespace: foundSsp.GetNamespace(),
				}

				newSspV1Beta3 = foundSsp
			})

			AfterEach(func() {
				strategy.RevertToOriginalSspCr()
			})

			Context("Placement API validation", func() {
				It("[test_id:5988] [v1beta2] should succeed with valid template-validator placement fields", func() {
					newSspV1Beta2.Spec.TemplateValidator = &sspv1beta2.TemplateValidator{
						Placement: &placementAPIValidationValidPlacement,
					}

					Expect(apiClient.Create(ctx, newSspV1Beta2, client.DryRunAll)).ToNot(HaveOccurred(),
						"failed to create SSP CR with valid template-validator placement fields")
				})

				It("[test_id:TODO] [v1beta3] should succeed with valid template-validator placement fields", func() {
					newSspV1Beta3.Spec.TemplateValidator = &sspv1beta3.TemplateValidator{
						Placement: &placementAPIValidationValidPlacement,
					}

					Expect(apiClient.Create(ctx, newSspV1Beta3, client.DryRunAll)).ToNot(HaveOccurred(),
						"failed to create SSP CR with valid template-validator placement fields")
				})

				It("[test_id:5987] [v1beta2] should fail with invalid template-validator placement fields", func() {
					newSspV1Beta2.Spec.TemplateValidator = &sspv1beta2.TemplateValidator{
						Placement: &placementAPIValidationInvalidPlacement,
					}

					Expect(apiClient.Create(ctx, newSspV1Beta2, client.DryRunAll)).To(HaveOccurred(),
						"created SSP CR with invalid template-validator placement fields")
				})

				It("[test_id:5987] [v1beat3] should fail with invalid template-validator placement fields", func() {
					newSspV1Beta3.Spec.TemplateValidator = &sspv1beta3.TemplateValidator{
						Placement: &placementAPIValidationInvalidPlacement,
					}

					Expect(apiClient.Create(ctx, newSspV1Beta3, client.DryRunAll)).To(HaveOccurred(),
						"created SSP CR with invalid template-validator placement fields")
				})
			})

			It("[test_id:TODO] [v1beta2] should fail when DataImportCronTemplate does not have a name", func() {
				newSspV1Beta2.Spec.CommonTemplates.DataImportCronTemplates = []sspv1beta2.DataImportCronTemplate{{
					ObjectMeta: metav1.ObjectMeta{Name: ""},
				}}
				err := apiClient.Create(ctx, newSspV1Beta2, client.DryRunAll)
				Expect(err).To(MatchError(ContainSubstring("missing name in DataImportCronTemplate")))
			})

			It("[test_id:TODO] [v1beta3] should fail when DataImportCronTemplate does not have a name", func() {
				newSspV1Beta3.Spec.CommonTemplates.DataImportCronTemplates = []sspv1beta3.DataImportCronTemplate{{
					ObjectMeta: metav1.ObjectMeta{Name: ""},
				}}
				err := apiClient.Create(ctx, newSspV1Beta3, client.DryRunAll)
				Expect(err).To(MatchError(ContainSubstring("missing name in DataImportCronTemplate")))
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

		Context("Placement API validation", func() {
			It("[test_id:5990] [v1beta2] should succeed with valid template-validator placement fields", func() {
				Eventually(func() error {
					foundSsp := getSspV1Beta2()
					foundSsp.Spec.TemplateValidator = &sspv1beta2.TemplateValidator{
						Placement: &placementAPIValidationValidPlacement,
					}
					return apiClient.Update(ctx, foundSsp, client.DryRunAll)
				}, 20*time.Second, time.Second).ShouldNot(HaveOccurred(), "failed to update SSP CR with valid template-validator placement fields")
			})

			It("[test_id:5990] [v1beta3] should succeed with valid template-validator placement fields", func() {
				Eventually(func() error {
					foundSsp := getSsp()
					foundSsp.Spec.TemplateValidator = &sspv1beta3.TemplateValidator{
						Placement: &placementAPIValidationValidPlacement,
					}
					return apiClient.Update(ctx, foundSsp, client.DryRunAll)
				}, 20*time.Second, time.Second).ShouldNot(HaveOccurred(), "failed to update SSP CR with valid template-validator placement fields")
			})

			It("[test_id:5989] [v1beta2] should fail with invalid template-validator placement fields", func() {
				Eventually(func() v1.StatusReason {
					foundSsp := getSspV1Beta2()
					foundSsp.Spec.TemplateValidator = &sspv1beta2.TemplateValidator{
						Placement: &placementAPIValidationInvalidPlacement,
					}
					err := apiClient.Update(ctx, foundSsp, client.DryRunAll)
					return errors.ReasonForError(err)
				}, 20*time.Second, time.Second).Should(Equal(metav1.StatusReasonInvalid), "SSP CR updated with invalid template-validator placement fields")
			})

			It("[test_id:5989] [v1beta3] should fail with invalid template-validator placement fields", func() {
				Eventually(func() v1.StatusReason {
					foundSsp := getSsp()
					foundSsp.Spec.TemplateValidator = &sspv1beta3.TemplateValidator{
						Placement: &placementAPIValidationInvalidPlacement,
					}
					err := apiClient.Update(ctx, foundSsp, client.DryRunAll)
					return errors.ReasonForError(err)
				}, 20*time.Second, time.Second).Should(Equal(metav1.StatusReasonInvalid), "SSP CR updated with invalid template-validator placement fields")
			})
		})

		It("[test_id:TODO] [v1beta2] should fail when DataImportCronTemplate does not have a name", func() {
			Eventually(func() error {
				foundSsp := getSspV1Beta2()
				foundSsp.Spec.CommonTemplates.DataImportCronTemplates = []sspv1beta2.DataImportCronTemplate{{
					ObjectMeta: metav1.ObjectMeta{Name: ""},
				}}
				return apiClient.Update(ctx, foundSsp, client.DryRunAll)
			}, 20*time.Second, time.Second).Should(MatchError(ContainSubstring("missing name in DataImportCronTemplate")))
		})

		It("[test_id:TODO] [v1beta3] should fail when DataImportCronTemplate does not have a name", func() {
			Eventually(func() error {
				foundSsp := getSsp()
				foundSsp.Spec.CommonTemplates.DataImportCronTemplates = []sspv1beta3.DataImportCronTemplate{{
					ObjectMeta: metav1.ObjectMeta{Name: ""},
				}}
				return apiClient.Update(ctx, foundSsp, client.DryRunAll)
			}, 20*time.Second, time.Second).Should(MatchError(ContainSubstring("missing name in DataImportCronTemplate")))
		})
	})
})
