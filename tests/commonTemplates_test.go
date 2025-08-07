package tests

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	templatev1 "github.com/openshift/api/template/v1"
	libhandler "github.com/operator-framework/operator-lib/handler"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ssp "kubevirt.io/ssp-operator/api/v1beta3"
	"kubevirt.io/ssp-operator/internal/architecture"
	"kubevirt.io/ssp-operator/internal/common"
	commonTemplates "kubevirt.io/ssp-operator/internal/operands/common-templates"
	"kubevirt.io/ssp-operator/tests/decorators"
	"kubevirt.io/ssp-operator/tests/env"
)

func createTestTemplate() testResource {
	expectedLabels := expectedLabelsFor("common-templates", common.AppComponentTemplating)
	return testResource{
		Name:           "fedora-desktop-medium" + templatesSuffix,
		Namespace:      strategy.GetTemplatesNamespace(),
		Resource:       &templatev1.Template{},
		ExpectedLabels: expectedLabels,
		UpdateFunc: func(t *templatev1.Template) {
			t.Parameters = nil
		},
		EqualsFunc: func(old *templatev1.Template, new *templatev1.Template) bool {
			return reflect.DeepEqual(old.Parameters, new.Parameters)
		},
	}
}

var _ = Describe("Common templates", func() {
	var (
		testTemplate testResource
	)

	BeforeEach(func() {
		testTemplate = createTestTemplate()
		waitUntilDeployed()
	})

	Context("resource creation", func() {
		DescribeTable("created namespaced resource", func(res *testResource) {
			err := apiClient.Get(ctx, res.GetKey(), res.NewResource())
			Expect(err).ToNot(HaveOccurred())
		},
			Entry("[test_id:5086]common-template in custom NS", decorators.Conformance, &testTemplate),
		)

		It("[test_id:5352]creates only one default variant per OS", decorators.Conformance, func() {
			liveTemplates := &templatev1.TemplateList{}
			err := apiClient.List(ctx, liveTemplates,
				client.InNamespace(strategy.GetTemplatesNamespace()),
				client.MatchingLabels{
					commonTemplates.TemplateVersionLabel: commonTemplates.Version,
				},
			)
			Expect(err).ToNot(HaveOccurred())

			osDefaultCounts := make(map[string]int)
			for _, liveTemplate := range liveTemplates.Items {
				_, isDefaultOSVariant := liveTemplate.Labels["template.kubevirt.io/default-os-variant"]

				for labelKey := range liveTemplate.Labels {
					if strings.HasPrefix(labelKey, commonTemplates.TemplateOsLabelPrefix) {
						if isDefaultOSVariant {
							osDefaultCounts[labelKey]++
							continue
						}
						if _, knownOSVariant := osDefaultCounts[labelKey]; !knownOSVariant {
							osDefaultCounts[labelKey] = 0
						}
					}
				}
			}

			for os, defaultCount := range osDefaultCounts {
				Expect(defaultCount).To(BeNumerically("==", 1), fmt.Sprintf("osDefaultCount for %s is not 1", os))
			}
		})

		It("[test_id:5545]did not create duplicate templates", decorators.Conformance, func() {
			liveTemplates := &templatev1.TemplateList{}
			err := apiClient.List(ctx, liveTemplates,
				client.InNamespace(strategy.GetTemplatesNamespace()),
				client.MatchingLabels{
					commonTemplates.TemplateVersionLabel: commonTemplates.Version,
				},
			)
			Expect(err).ToNot(HaveOccurred())

			for _, template := range liveTemplates.Items {
				Expect(template.ObjectMeta).NotTo(BeNil())
				Expect(template.ObjectMeta.Labels).NotTo(BeNil())

				var (
					oss       []string
					workloads []string
					flavors   []string
				)

				for labelKey := range template.Labels {
					if strings.HasPrefix(labelKey, commonTemplates.TemplateOsLabelPrefix) {
						oss = append(oss, labelKey)
						continue
					}
					if strings.HasPrefix(labelKey, commonTemplates.TemplateWorkloadLabelPrefix) {
						workloads = append(workloads, labelKey)
						continue
					}
					if strings.HasPrefix(labelKey, commonTemplates.TemplateFlavorLabelPrefix) {
						flavors = append(flavors, labelKey)
						continue
					}
				}

				for _, os := range oss {
					for _, workload := range workloads {
						for _, flavor := range flavors {
							matchingLiveTemplates := 0
							for _, liveTemplate := range liveTemplates.Items {
								_, osMatch := liveTemplate.Labels[os]
								_, workloadMatch := liveTemplate.Labels[workload]
								_, flavorMatch := liveTemplate.Labels[flavor]
								if osMatch && workloadMatch && flavorMatch {
									matchingLiveTemplates++
								}
							}
							Expect(matchingLiveTemplates).To(BeNumerically("==", 1),
								fmt.Sprintf("More than 1 matching live template for (%s, %s, %s)", os, workload, flavor),
							)
						}
					}
				}
			}
		})

		DescribeTable("should set app labels", expectAppLabels,
			Entry("[test_id:6219] common-template in custom NS", &testTemplate),
		)
	})

	Context("resource change", func() {
		Context("namespace change", func() {
			var newTemplateNamespace string
			var namespaceObj *v1.Namespace
			BeforeEach(func() {
				strategy.SkipSspUpdateTestsIfNeeded()

				namespaceObj = &v1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "ssp-operator-functests-new-templates"}}
				err := apiClient.Create(ctx, namespaceObj)
				Expect(err).ToNot(HaveOccurred(), "err should be nil")

				newTemplateNamespace = namespaceObj.GetName()
			})

			AfterEach(func() {
				strategy.RevertToOriginalSspCr()
				err := apiClient.Delete(ctx, namespaceObj)
				Expect(err).ToNot(HaveOccurred(), "err should be nil")
			})

			It("[test_id:6057] should update commonTemplates.namespace", decorators.Conformance, func() {
				foundSsp := getSsp()
				Expect(foundSsp.Spec.CommonTemplates.Namespace).ToNot(Equal(newTemplateNamespace), "namespaces should not equal")

				updateSsp(func(foundSsp *ssp.SSP) {
					foundSsp.Spec.CommonTemplates.Namespace = newTemplateNamespace
				})
				waitUntilDeployed()

				templates := &templatev1.TemplateList{}
				Eventually(func() bool {
					err := apiClient.List(ctx, templates, &client.ListOptions{
						Namespace: newTemplateNamespace,
					})
					return len(templates.Items) > 0 && err == nil
				}, env.ShortTimeout()).Should(BeTrue(), "templates should be in new namespace")
			})
		})

		DescribeTable("should restore modified resource", expectRestoreAfterUpdate,
			Entry("[test_id:5087]test template", decorators.Conformance, &testTemplate),
		)

		It("[test_id: 7340] should increase metrics when restoring template", func() {
			expectTemplateUpdateToIncreaseTotalRestoredTemplatesCount(testTemplate)
		})

		Context("with pause", func() {
			BeforeEach(func() {
				strategy.SkipSspUpdateTestsIfNeeded()
			})

			JustAfterEach(func() {
				unpauseSsp()
			})

			DescribeTable("should restore modified resource with pause", expectRestoreAfterUpdateWithPause,
				Entry("[test_id:5391]testTemplate in custom NS", decorators.Conformance, &testTemplate),
			)
		})

		DescribeTable("should restore app labels", expectAppLabelsRestoreAfterUpdate,
			Entry("[test_id:6214] common-template in custom NS", &testTemplate),
		)
	})

	Context("resource deletion", func() {
		DescribeTable("recreate after delete", expectRecreateAfterDelete,
			Entry("[test_id:5088]testTemplate in custom NS", decorators.Conformance, &testTemplate),
		)
	})

	Context("older templates update", func() {
		const (
			testOsLabel       = commonTemplates.TemplateOsLabelPrefix + "some-os"
			testFlavorLabel   = commonTemplates.TemplateFlavorLabelPrefix + "test"
			testWorkflowLabel = commonTemplates.TemplateWorkloadLabelPrefix + "server"
		)

		var (
			oldTemplate *templatev1.Template
		)

		BeforeEach(func() {
			ssp := getSsp()

			oldTemplate = &templatev1.Template{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-old-template-",
					Namespace:    strategy.GetTemplatesNamespace(),
					Labels: map[string]string{
						commonTemplates.TemplateVersionLabel: "not-latest",
						commonTemplates.TemplateTypeLabel:    commonTemplates.TemplateTypeLabelBaseValue,
						testOsLabel:                          "true",
						testFlavorLabel:                      "true",
						testWorkflowLabel:                    "true",
					},
				},
			}
			Expect(libhandler.SetOwnerAnnotations(ssp, oldTemplate)).To(Succeed())

			Expect(apiClient.Create(ctx, oldTemplate)).ToNot(HaveOccurred(), "creation of dummy old template failed")
		})

		AfterEach(func() {
			Expect(apiClient.Delete(ctx, oldTemplate)).ToNot(HaveOccurred(), "deletion of dummy old template failed")
		})

		It("[test_id:5620]should remove labels from old templates", func() {
			triggerReconciliation()
			// Template should eventually be updated by the operator
			Eventually(func() (bool, error) {
				updatedTpl := &templatev1.Template{}
				key := client.ObjectKey{Name: oldTemplate.Name, Namespace: oldTemplate.Namespace}
				err := apiClient.Get(ctx, key, updatedTpl)
				if err != nil {
					return false, err
				}
				return updatedTpl.Labels[testOsLabel] == "" &&
					updatedTpl.Labels[testFlavorLabel] == "" &&
					updatedTpl.Labels[testWorkflowLabel] == "" &&
					updatedTpl.Labels[commonTemplates.TemplateTypeLabel] == commonTemplates.TemplateTypeLabelBaseValue &&
					updatedTpl.Labels[commonTemplates.TemplateVersionLabel] == "not-latest", nil
			}, env.ShortTimeout()).Should(BeTrue(), "labels were not removed from older templates")
		})
		It("[test_id:5969] should add deprecated annotation to old templates", func() {
			triggerReconciliation()

			Eventually(func() (bool, error) {
				updatedTpl := &templatev1.Template{}
				key := client.ObjectKey{Name: oldTemplate.Name, Namespace: oldTemplate.Namespace}
				err := apiClient.Get(ctx, key, updatedTpl)
				if err != nil {
					return false, err
				}
				return updatedTpl.Annotations[commonTemplates.TemplateDeprecatedAnnotation] == "true", nil
			}, env.ShortTimeout()).Should(BeTrue(), "deprecated annotation should be added to old template")
		})
		It("[test_id:5622]should continue to have labels on latest templates", decorators.Conformance, func() {
			triggerReconciliation()

			var latestTemplates templatev1.TemplateList
			err := apiClient.List(ctx, &latestTemplates,
				client.InNamespace(strategy.GetTemplatesNamespace()),
				client.MatchingLabels{
					commonTemplates.TemplateTypeLabel:    commonTemplates.TemplateTypeLabelBaseValue,
					commonTemplates.TemplateVersionLabel: commonTemplates.Version,
				})
			Expect(err).ToNot(HaveOccurred())
			Expect(latestTemplates.Items).ToNot(BeEmpty(), "Latest templates are missing")

			for _, template := range latestTemplates.Items {
				for label, value := range template.Labels {
					if strings.HasPrefix(label, commonTemplates.TemplateOsLabelPrefix) ||
						strings.HasPrefix(label, commonTemplates.TemplateFlavorLabelPrefix) ||
						strings.HasPrefix(label, commonTemplates.TemplateWorkloadLabelPrefix) {
						Expect(value).To(Equal("true"),
							fmt.Sprintf("Required label for template is not 'true': {template: %s/%s, label: %s}",
								template.GetNamespace(), template.GetName(), label),
						)
					}
				}
				Expect(template.Labels[commonTemplates.TemplateTypeLabel]).To(Equal(commonTemplates.TemplateTypeLabelBaseValue),
					fmt.Sprintf("Label '%s' is not equal 'base' for template %s/%s",
						commonTemplates.TemplateTypeLabel,
						template.GetNamespace(), template.GetName()),
				)
				Expect(template.Labels[commonTemplates.TemplateVersionLabel]).To(Equal(commonTemplates.Version),
					fmt.Sprintf("Label '%s' is not equal '%s' for template %s/%s",
						commonTemplates.TemplateVersionLabel,
						commonTemplates.Version,
						template.GetNamespace(), template.GetName()),
				)
			}
		})
	})
})

var _ = Describe("Common templates with multiple architectures enabled", Ordered, func() {
	var (
		testTemplateAmd64 testResource
		testTemplateArm64 testResource
		testTemplateS390x testResource
	)

	// Using ordered container and BeforeAll for performance reasons.
	// These tests don't need to disable and enable multi-arch before every test.
	BeforeAll(func() {
		strategy.SkipSspUpdateTestsIfNeeded()

		const templateName = "rhel9-server-small"
		expectedLabels := expectedLabelsFor("common-templates", common.AppComponentTemplating)

		testTemplateAmd64 = testResource{
			Name:           templateName,
			Namespace:      strategy.GetTemplatesNamespace(),
			Resource:       &templatev1.Template{},
			ExpectedLabels: expectedLabels,
			UpdateFunc: func(t *templatev1.Template) {
				t.Parameters = nil
			},
			EqualsFunc: func(old *templatev1.Template, new *templatev1.Template) bool {
				return reflect.DeepEqual(old.Parameters, new.Parameters)
			},
		}

		testTemplateArm64 = testResource{
			Name:           templateName + "-" + string(architecture.ARM64),
			Namespace:      strategy.GetTemplatesNamespace(),
			Resource:       &templatev1.Template{},
			ExpectedLabels: expectedLabels,
			UpdateFunc: func(t *templatev1.Template) {
				t.Parameters = nil
			},
			EqualsFunc: func(old *templatev1.Template, new *templatev1.Template) bool {
				return reflect.DeepEqual(old.Parameters, new.Parameters)
			},
		}

		testTemplateS390x = testResource{
			Name:           templateName + "-" + string(architecture.S390X),
			Namespace:      strategy.GetTemplatesNamespace(),
			Resource:       &templatev1.Template{},
			ExpectedLabels: expectedLabels,
			UpdateFunc: func(t *templatev1.Template) {
				t.Parameters = nil
			},
			EqualsFunc: func(old *templatev1.Template, new *templatev1.Template) bool {
				return reflect.DeepEqual(old.Parameters, new.Parameters)
			},
		}

		updateSsp(func(foundSsp *ssp.SSP) {
			foundSsp.Spec.EnableMultipleArchitectures = ptr.To(true)
			foundSsp.Spec.Cluster = &ssp.Cluster{
				WorkloadArchitectures: []string{
					string(architecture.AMD64),
					string(architecture.ARM64),
					string(architecture.S390X),
				},
				ControlPlaneArchitectures: []string{string(architecture.AMD64)},
			}
		})
		waitUntilDeployed()
	})

	AfterAll(func() {
		strategy.RevertToOriginalSspCr()
	})

	DescribeTable("created resource", func(res *testResource, arch architecture.Arch) {
		found := res.NewResource()
		Expect(apiClient.Get(ctx, res.GetKey(), found)).To(Succeed())
		Expect(found.GetLabels()).To(HaveKeyWithValue(commonTemplates.TemplateArchitectureLabel, string(arch)))
	},
		Entry("[test_id:TODO] template for amd64", decorators.Conformance, &testTemplateAmd64, architecture.AMD64),
		Entry("[test_id:TODO] template for arm64", decorators.Conformance, &testTemplateArm64, architecture.ARM64),
		Entry("[test_id:TODO] template for s390x", decorators.Conformance, &testTemplateS390x, architecture.S390X),
	)

	DescribeTable("should restore modified resource", expectRestoreAfterUpdate,
		Entry("[test_id:TODO] template for amd64", decorators.Conformance, &testTemplateAmd64),
		Entry("[test_id:TODO] template for arm64", decorators.Conformance, &testTemplateArm64),
		Entry("[test_id:TODO] template for s390x", decorators.Conformance, &testTemplateS390x),
	)

	DescribeTable("recreate after delete", expectRecreateAfterDelete,
		Entry("[test_id:TODO] template for amd64", decorators.Conformance, &testTemplateAmd64),
		Entry("[test_id:TODO] template for arm64", decorators.Conformance, &testTemplateArm64),
		Entry("[test_id:TODO] template for s390x", decorators.Conformance, &testTemplateS390x),
	)

	Context("when disabled multi-arch", Ordered, func() {
		BeforeAll(func() {
			updateSsp(func(foundSsp *ssp.SSP) {
				foundSsp.Spec.EnableMultipleArchitectures = ptr.To(false)
			})
			waitUntilDeployed()
		})

		AfterAll(func() {
			updateSsp(func(foundSsp *ssp.SSP) {
				foundSsp.Spec.EnableMultipleArchitectures = ptr.To(true)
			})
			waitUntilDeployed()
		})

		DescribeTable("should delete template for non-default arch", func(res *testResource, arch architecture.Arch) {
			Eventually(func() error {
				return apiClient.Get(ctx, res.GetKey(), res.NewResource())
			}, env.ShortTimeout(), time.Second).Should(MatchError(errors.IsNotFound, "errors.IsNotFound"))
		},
			Entry("[test_id:TODO] template for arm64", decorators.Conformance, &testTemplateArm64, architecture.ARM64),
			Entry("[test_id:TODO] template for s390x", decorators.Conformance, &testTemplateS390x, architecture.S390X),
		)

		It("[test_id:TODO] should keep template for default arch", decorators.Conformance, func() {
			Expect(apiClient.Get(ctx, testTemplateAmd64.GetKey(), testTemplateAmd64.NewResource())).To(Succeed())
		})
	})
})

func expectTemplateUpdateToIncreaseTotalRestoredTemplatesCount(testTemplate testResource) {
	restoredCountBefore, err := totalRestoredTemplatesCount()
	Expect(err).ToNot(HaveOccurred())

	expectRestoreAfterUpdate(&testTemplate)

	restoredCountAfter, err := totalRestoredTemplatesCount()
	Expect(err).ToNot(HaveOccurred())
	Expect(restoredCountAfter - restoredCountBefore).To(Equal(1))
}
