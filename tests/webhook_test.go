package tests

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	sspv1beta1 "kubevirt.io/ssp-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"kubevirt.io/controller-lifecycle-operator-sdk/api"
	sspv1beta2 "kubevirt.io/ssp-operator/api/v1beta2"
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

			err := apiClient.Create(ctx, ssp2, client.DryRunAll)
			if err == nil {
				Fail("Second SSP resource created.")
				return
			}
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf(
				"creation failed, an SSP CR already exists in namespace %v: %v",
				foundSsp.Namespace,
				foundSsp.Name,
			)))
		})

		Context("removed existing SSP CR", func() {
			var (
				newSsp *sspv1beta2.SSP
			)

			BeforeEach(func() {
				strategy.SkipSspUpdateTestsIfNeeded()

				foundSsp := getSsp()
				Expect(apiClient.Delete(ctx, foundSsp)).ToNot(HaveOccurred())
				waitForDeletion(client.ObjectKey{Name: foundSsp.GetName(), Namespace: foundSsp.GetNamespace()}, &sspv1beta2.SSP{})

				foundSsp.ObjectMeta = v1.ObjectMeta{
					Name:      foundSsp.GetName(),
					Namespace: foundSsp.GetNamespace(),
				}

				newSsp = foundSsp
			})

			AfterEach(func() {
				strategy.RevertToOriginalSspCr()
			})

			Context("Placement API validation", func() {
				It("[test_id:5988]should succeed with valid template-validator placement fields", func() {
					newSsp.Spec.TemplateValidator = &sspv1beta2.TemplateValidator{
						Placement: &placementAPIValidationValidPlacement,
					}

					Expect(apiClient.Create(ctx, newSsp, client.DryRunAll)).ToNot(HaveOccurred(),
						"failed to create SSP CR with valid template-validator placement fields")
				})

				It("[test_id:5987]should fail with invalid template-validator placement fields", func() {
					newSsp.Spec.TemplateValidator = &sspv1beta2.TemplateValidator{
						Placement: &placementAPIValidationInvalidPlacement,
					}

					Expect(apiClient.Create(ctx, newSsp, client.DryRunAll)).To(HaveOccurred(),
						"created SSP CR with invalid template-validator placement fields")
				})
			})

			Context("with second namespace", func() {
				var (
					namespace *corev1.Namespace
				)

				BeforeEach(func() {
					namespace = &corev1.Namespace{
						ObjectMeta: v1.ObjectMeta{
							GenerateName: "test-webhook-namespace-",
						},
					}
					Expect(apiClient.Create(ctx, namespace)).To(Succeed())
				})

				AfterEach(func() {
					Expect(apiClient.Delete(ctx, namespace)).To(Succeed())
				})

				It("[test_id:TODO] should fail to create SSP CR in a different namespace", func() {
					newSsp.Namespace = namespace.Name
					Expect(apiClient.Create(ctx, newSsp, client.DryRunAll)).
						To(MatchError(ContainSubstring("SSP object must be in namespace:")))
				})
			})

			It("[test_id:TODO] should fail when DataImportCronTemplate does not have a name", func() {
				newSsp.Spec.CommonTemplates.DataImportCronTemplates = []sspv1beta2.DataImportCronTemplate{{
					ObjectMeta: metav1.ObjectMeta{Name: ""},
				}}
				err := apiClient.Create(ctx, newSsp, client.DryRunAll)
				Expect(err).To(MatchError(ContainSubstring("missing name in DataImportCronTemplate")))
			})

			It("[test_id:TODO] should accept v1beta1 SSP object", func() {
				ssp := &sspv1beta1.SSP{
					ObjectMeta: metav1.ObjectMeta{
						Name:      newSsp.GetName(),
						Namespace: newSsp.GetNamespace(),
					},
					Spec: sspv1beta1.SSPSpec{
						CommonTemplates: sspv1beta1.CommonTemplates{
							Namespace: newSsp.Spec.CommonTemplates.Namespace,
						},
					},
				}

				Expect(apiClient.Create(ctx, ssp, client.DryRunAll)).To(Succeed())
			})
		})
	})

	Context("update", func() {
		var (
			foundSsp *sspv1beta2.SSP
		)

		BeforeEach(func() {
			strategy.SkipSspUpdateTestsIfNeeded()
		})

		AfterEach(func() {
			strategy.RevertToOriginalSspCr()
		})

		Context("Placement API validation", func() {
			It("[test_id:5990]should succeed with valid template-validator placement fields", func() {
				Eventually(func() error {
					foundSsp = getSsp()
					foundSsp.Spec.TemplateValidator = &sspv1beta2.TemplateValidator{
						Placement: &placementAPIValidationValidPlacement,
					}
					return apiClient.Update(ctx, foundSsp, client.DryRunAll)
				}, 20*time.Second, time.Second).ShouldNot(HaveOccurred(), "failed to update SSP CR with valid template-validator placement fields")
			})

			It("[test_id:5989]should fail with invalid template-validator placement fields", func() {
				Eventually(func() v1.StatusReason {
					foundSsp = getSsp()
					foundSsp.Spec.TemplateValidator = &sspv1beta2.TemplateValidator{
						Placement: &placementAPIValidationInvalidPlacement,
					}
					err := apiClient.Update(ctx, foundSsp, client.DryRunAll)
					return errors.ReasonForError(err)
				}, 20*time.Second, time.Second).Should(Equal(metav1.StatusReasonInvalid), "SSP CR updated with invalid template-validator placement fields")
			})
		})

		It("[test_id:TODO] should fail when DataImportCronTemplate does not have a name", func() {
			Eventually(func() error {
				foundSsp = getSsp()
				foundSsp.Spec.CommonTemplates.DataImportCronTemplates = []sspv1beta2.DataImportCronTemplate{{
					ObjectMeta: metav1.ObjectMeta{Name: ""},
				}}
				return apiClient.Update(ctx, foundSsp, client.DryRunAll)
			}, 20*time.Second, time.Second).Should(MatchError(ContainSubstring("missing name in DataImportCronTemplate")))
		})
	})
})
