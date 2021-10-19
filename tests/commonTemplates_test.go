package tests

import (
	"fmt"
	"reflect"
	"strings"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	templatev1 "github.com/openshift/api/template/v1"
	authv1 "k8s.io/api/authorization/v1"
	core "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ssp "kubevirt.io/ssp-operator/api/v1beta1"
	sspv1beta1 "kubevirt.io/ssp-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cdiv1beta1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/ssp-operator/internal/common"
	commonTemplates "kubevirt.io/ssp-operator/internal/operands/common-templates"
)

var _ = Describe("Common templates", func() {
	var (
		viewRole        testResource
		viewRoleBinding testResource
		editClusterRole testResource
		goldenImageNS   testResource
		testTemplate    testResource
	)

	BeforeEach(func() {
		expectedLabels := expectedLabelsFor("common-templates", common.AppComponentTemplating)
		viewRole = testResource{
			Name:           commonTemplates.ViewRoleName,
			Namespace:      ssp.GoldenImagesNSname,
			Resource:       &rbac.Role{},
			ExpectedLabels: expectedLabels,
			UpdateFunc: func(role *rbac.Role) {
				role.Rules = []rbac.PolicyRule{}
			},
			EqualsFunc: func(old *rbac.Role, new *rbac.Role) bool {
				return reflect.DeepEqual(old.Rules, new.Rules)
			},
		}
		viewRoleBinding = testResource{
			Name:           commonTemplates.ViewRoleName,
			Namespace:      ssp.GoldenImagesNSname,
			Resource:       &rbac.RoleBinding{},
			ExpectedLabels: expectedLabels,
			UpdateFunc: func(roleBinding *rbac.RoleBinding) {
				roleBinding.Subjects = nil
			},
			EqualsFunc: func(old *rbac.RoleBinding, new *rbac.RoleBinding) bool {
				return reflect.DeepEqual(old.Subjects, new.Subjects)
			},
		}
		editClusterRole = testResource{
			Name:           commonTemplates.EditClusterRoleName,
			Resource:       &rbac.ClusterRole{},
			ExpectedLabels: expectedLabels,
			Namespace:      "",
			UpdateFunc: func(role *rbac.ClusterRole) {
				role.Rules[0].Verbs = []string{"watch"}
			},
			EqualsFunc: func(old *rbac.ClusterRole, new *rbac.ClusterRole) bool {
				return reflect.DeepEqual(old.Rules, new.Rules)
			},
		}
		goldenImageNS = testResource{
			Name:           ssp.GoldenImagesNSname,
			Resource:       &core.Namespace{},
			ExpectedLabels: expectedLabels,
			Namespace:      "",
		}
		testTemplate = testResource{
			Name:           "rhel8-desktop-tiny",
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

		waitUntilDeployed()
	})

	Context("resource creation", func() {
		table.DescribeTable("created cluster resource", func(res *testResource) {
			resource := res.NewResource()
			err := apiClient.Get(ctx, res.GetKey(), resource)
			Expect(err).ToNot(HaveOccurred())
			Expect(hasOwnerAnnotations(resource.GetAnnotations())).To(BeTrue(), "Missing owner annotations")
		},
			table.Entry("[test_id:4584]edit role", &editClusterRole),
			table.Entry("[test_id:4494]golden images namespace", &goldenImageNS),
		)

		table.DescribeTable("created namespaced resource", func(res *testResource) {
			err := apiClient.Get(ctx, res.GetKey(), res.NewResource())
			Expect(err).ToNot(HaveOccurred())
		},
			table.Entry("[test_id:4777]view role", &viewRole),
			table.Entry("[test_id:4772]view role binding", &viewRoleBinding),
			table.Entry("[test_id:5086]common-template in custom NS", &testTemplate),
		)

		It("[test_id:5352]creates only one default variant per OS", func() {
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

		It("[test_id:5545]did not create duplicate templates", func() {
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

				for labelKey := range template.ObjectMeta.Labels {
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

		table.DescribeTable("should set app labels", expectAppLabels,
			table.Entry("[test_id:6215] edit role", &editClusterRole),
			table.Entry("[test_id:6216] golden images namespace", &goldenImageNS),
			table.Entry("[test_id:6217] view role", &viewRole),
			table.Entry("[test_id:6218] view role binding", &viewRoleBinding),
			table.Entry("[test_id:6219] common-template in custom NS", &testTemplate),
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
				Expect(err).To(BeNil(), "err should be nil")

				newTemplateNamespace = namespaceObj.GetName()
			})

			AfterEach(func() {
				strategy.RevertToOriginalSspCr()
				err := apiClient.Delete(ctx, namespaceObj)
				Expect(err).To(BeNil(), "err should be nil")
			})

			It("[test_id:6057] should update commonTemplates.namespace", func() {
				foundSsp := getSsp()
				Expect(foundSsp.Spec.CommonTemplates.Namespace).ToNot(Equal(newTemplateNamespace), "namespaces should not equal")

				updateSsp(func(foundSsp *sspv1beta1.SSP) {
					foundSsp.Spec.CommonTemplates.Namespace = newTemplateNamespace
				})
				waitUntilDeployed()

				templates := &templatev1.TemplateList{}
				Eventually(func() bool {
					err := apiClient.List(ctx, templates, &client.ListOptions{
						Namespace: newTemplateNamespace,
					})
					return len(templates.Items) > 0 && err == nil
				}, shortTimeout).Should(BeTrue(), "templates should be in new namespace")
			})
		})
		table.DescribeTable("should restore modified resource", expectRestoreAfterUpdate,
			table.Entry("[test_id:5315]edit cluster role", &editClusterRole),
			table.Entry("[test_id:5316]view role", &viewRole),
			table.Entry("[test_id:5317]view role binding", &viewRoleBinding),
			table.Entry("[test_id:5087]test template", &testTemplate),
		)

		It("[test_id: 7340] should increase metrics when restoring tamplate", func() {
			expectMetricsIncreaseAfterRestore(&testTemplate)
		})

		Context("with pause", func() {
			BeforeEach(func() {
				strategy.SkipSspUpdateTestsIfNeeded()
			})

			JustAfterEach(func() {
				unpauseSsp()
			})

			table.DescribeTable("should restore modified resource with pause", expectRestoreAfterUpdateWithPause,
				table.Entry("[test_id:5388]view role", &viewRole),
				table.Entry("[test_id:5389]view role binding", &viewRoleBinding),
				table.Entry("[test_id:5391]testTemplate in custom NS", &testTemplate),
				table.Entry("[test_id:5393]edit cluster role", &editClusterRole),
			)
		})

		table.DescribeTable("should restore app labels", expectAppLabelsRestoreAfterUpdate,
			table.Entry("[test_id:6210] edit role", &editClusterRole),
			table.Entry("[test_id:6211] golden images namespace", &goldenImageNS),
			table.Entry("[test_id:6212] view role", &viewRole),
			table.Entry("[test_id:6213] view role binding", &viewRoleBinding),
			table.Entry("[test_id:6214] common-template in custom NS", &testTemplate),
		)
	})

	Context("resource deletion", func() {
		table.DescribeTable("recreate after delete", expectRecreateAfterDelete,
			table.Entry("[test_id:4773]view role", &viewRole),
			table.Entry("[test_id:4842]view role binding", &viewRoleBinding),
			table.Entry("[test_id:5088]testTemplate in custom NS", &testTemplate),
			table.Entry("[test_id:4771]edit cluster role", &editClusterRole),
			table.Entry("[test_id:4770]golden image NS", &goldenImageNS),
		)
	})

	Context("older templates update", func() {
		const (
			testOsLabel       = commonTemplates.TemplateOsLabelPrefix + "some-os"
			testFlavorLabel   = commonTemplates.TemplateFlavorLabelPrefix + "test"
			testWorkflowLabel = commonTemplates.TemplateWorkloadLabelPrefix + "server"
		)

		var (
			ownerTemplate, oldTemplate *templatev1.Template
		)

		BeforeEach(func() {
			// Create a dummy template to act as an owner for the test template
			// we can't use the SSP CR as an owner for these tests because the tempaltes
			// might be deployed in a different namespace than the CR, and will be immediately
			// removed by the GC, the choice to use a template as an owner object was arbitrary
			ownerTemplate = &templatev1.Template{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "owner-template-",
					Namespace:    strategy.GetTemplatesNamespace(),
				},
			}
			Expect(apiClient.Create(ctx, ownerTemplate)).ToNot(HaveOccurred(), "failed to create dummy owner for an old template")

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
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: templatev1.GroupVersion.String(),
						Kind:       "Template",
						Name:       ownerTemplate.Name,
						UID:        ownerTemplate.UID,
					}},
				},
			}
			Expect(apiClient.Create(ctx, oldTemplate)).ToNot(HaveOccurred(), "creation of dummy old template failed")
		})

		AfterEach(func() {
			Expect(apiClient.Delete(ctx, oldTemplate)).ToNot(HaveOccurred(), "deletion of dummy old template failed")
			Expect(apiClient.Delete(ctx, ownerTemplate)).ToNot(HaveOccurred(), "deletion of dummy owner template failed")
		})

		It("[test_id:5621]should replace ownerReference with owner annotations for older templates", func() {
			triggerReconciliation()

			// Template should eventually be updated by the operator
			Eventually(func() (bool, error) {
				updatedTpl := &templatev1.Template{}
				key := client.ObjectKey{Name: oldTemplate.Name, Namespace: oldTemplate.Namespace}
				err := apiClient.Get(ctx, key, updatedTpl)
				if err != nil {
					return false, err
				}
				return len(updatedTpl.GetOwnerReferences()) == 0 &&
					hasOwnerAnnotations(updatedTpl.GetAnnotations()), nil
			}, shortTimeout).Should(BeTrue(), "ownerReference was not replaced by owner annotations on the old template")
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
			}, shortTimeout).Should(BeTrue(), "labels were not removed from older templates")
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
			}, shortTimeout).Should(BeTrue(), "deprecated annotation should be added to old template")
		})
		It("[test_id:5622]should continue to have labels on latest templates", func() {
			triggerReconciliation()

			var latestTemplates templatev1.TemplateList
			err := apiClient.List(ctx, &latestTemplates,
				client.InNamespace(strategy.GetTemplatesNamespace()),
				client.MatchingLabels{
					commonTemplates.TemplateTypeLabel:    commonTemplates.TemplateTypeLabelBaseValue,
					commonTemplates.TemplateVersionLabel: commonTemplates.Version,
				})
			Expect(err).ToNot(HaveOccurred())
			Expect(len(latestTemplates.Items)).To(BeNumerically(">", 0), "Latest templates are missing")

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

	Context("rbac", func() {
		Context("os-images", func() {
			var (
				regularSA         *core.ServiceAccount
				regularSAFullName string
				sasGroup          = []string{"system:serviceaccounts"}
			)

			BeforeEach(func() {
				regularSA = &core.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "regular-sa-",
						Namespace:    strategy.GetNamespace(),
					},
				}
				Expect(apiClient.Create(ctx, regularSA)).To(Succeed(), "creation of regular service account failed")

				regularSAFullName = fmt.Sprintf("system:serviceaccount:%s:%s", regularSA.GetNamespace(), regularSA.GetName())
			})

			AfterEach(func() {
				Expect(apiClient.Delete(ctx, regularSA)).NotTo(HaveOccurred())
			})

			table.DescribeTable("regular service account namespace RBAC", expectUserCan,
				table.Entry("[test_id:6069] should be able to 'get' namespaces",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "get",
							Namespace: ssp.GoldenImagesNSname,
							Version:   core.SchemeGroupVersion.Version,
							Resource:  "namespaces",
						},
					}),
				table.Entry("[test_id:6070] should be able to 'list' namespaces",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "list",
							Namespace: ssp.GoldenImagesNSname,
							Version:   core.SchemeGroupVersion.Version,
							Resource:  "namespaces",
						},
					}),
				table.Entry("[test_id:6071] should be able to 'watch' namespaces",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "watch",
							Namespace: ssp.GoldenImagesNSname,
							Version:   core.SchemeGroupVersion.Version,
							Resource:  "namespaces",
						},
					}))

			table.DescribeTable("regular service account DV RBAC", expectUserCan,
				table.Entry("[test_id:6072] should be able to 'get' datavolumes",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "get",
							Namespace: ssp.GoldenImagesNSname,
							Group:     cdiv1beta1.SchemeGroupVersion.Group,
							Version:   cdiv1beta1.SchemeGroupVersion.Version,
							Resource:  "datavolumes",
						},
					}),
				table.Entry("[test_id:6073] should be able to 'list' datavolumes",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "list",
							Namespace: ssp.GoldenImagesNSname,
							Group:     cdiv1beta1.SchemeGroupVersion.Group,
							Version:   cdiv1beta1.SchemeGroupVersion.Version,
							Resource:  "datavolumes",
						},
					}),
				table.Entry("[test_id:6074] should be able to 'watch' datavolumes",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "watch",
							Namespace: ssp.GoldenImagesNSname,
							Group:     cdiv1beta1.SchemeGroupVersion.Group,
							Version:   cdiv1beta1.SchemeGroupVersion.Version,
							Resource:  "datavolumes",
						},
					}),
				table.Entry("[test_id:5005]: ServiceAccounts with only view role can create dv/source",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:        "create",
							Namespace:   ssp.GoldenImagesNSname,
							Group:       cdiv1beta1.SchemeGroupVersion.Group,
							Version:     cdiv1beta1.SchemeGroupVersion.Version,
							Resource:    "datavolumes",
							Subresource: "source",
						},
					}),
			)

			table.DescribeTable("regular service account DV RBAC", expectUserCannot,
				table.Entry("[test_id:4873]: ServiceAccounts with only view role cannot delete DVs",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "delete",
							Namespace: ssp.GoldenImagesNSname,
							Group:     cdiv1beta1.SchemeGroupVersion.Group,
							Version:   cdiv1beta1.SchemeGroupVersion.Version,
							Resource:  "datavolumes",
						},
					}),
				table.Entry("[test_id:4874]: ServiceAccounts with only view role cannot create DVs",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "create",
							Namespace: ssp.GoldenImagesNSname,
							Group:     cdiv1beta1.SchemeGroupVersion.Group,
							Version:   cdiv1beta1.SchemeGroupVersion.Version,
							Resource:  "datavolumes",
						},
					}),
			)
			table.DescribeTable("regular service account PVC RBAC", expectUserCan,
				table.Entry("[test_id:4775]: ServiceAccounts with view role can view PVCs",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "get",
							Namespace: ssp.GoldenImagesNSname,
							Version:   core.SchemeGroupVersion.Version,
							Resource:  "persistentvolumeclaims",
						},
					}))
			table.DescribeTable("regular service account RBAC", expectUserCannot,
				table.Entry("[test_id:4776]: ServiceAccounts with only view role cannot create PVCs",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "create",
							Namespace: ssp.GoldenImagesNSname,
							Version:   core.SchemeGroupVersion.Version,
							Resource:  "persistentvolumeclaims",
						},
					}),
				table.Entry("[test_id:4846]: ServiceAccounts with only view role cannot delete PVCs",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "delete",
							Namespace: ssp.GoldenImagesNSname,
							Version:   core.SchemeGroupVersion.Version,
							Resource:  "persistentvolumeclaims",
						},
					}),
				table.Entry("[test_id:4879]: ServiceAccounts with only view role cannot create any other resources other than the ones listed in the View role",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "create",
							Namespace: ssp.GoldenImagesNSname,
							Version:   core.SchemeGroupVersion.Version,
							Resource:  "pods",
						},
					}),
			)
			Context("With Edit permission", func() {
				var (
					privilegedSA         *core.ServiceAccount
					privilegedSAFullName string

					editObj *rbac.RoleBinding
				)
				BeforeEach(func() {
					privilegedSA = &core.ServiceAccount{
						ObjectMeta: metav1.ObjectMeta{
							GenerateName: "privileged-sa-",
							Namespace:    strategy.GetNamespace(),
						},
					}

					Expect(apiClient.Create(ctx, privilegedSA)).To(Succeed(), "creation of regular service account failed")
					privilegedSAFullName = fmt.Sprintf("system:serviceaccount:%s:%s", privilegedSA.GetNamespace(), privilegedSA.GetName())

					editObj = &rbac.RoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							GenerateName: "test-edit-",
							Namespace:    ssp.GoldenImagesNSname,
						},
						Subjects: []rbac.Subject{{
							Kind:      "ServiceAccount",
							Name:      privilegedSA.GetName(),
							Namespace: privilegedSA.GetNamespace(),
						}},
						RoleRef: rbac.RoleRef{
							Kind:     "ClusterRole",
							Name:     commonTemplates.EditClusterRoleName,
							APIGroup: rbac.GroupName,
						},
					}
					Expect(apiClient.Create(ctx, editObj)).ToNot(HaveOccurred(), "Failed to create RoleBinding")
				})
				AfterEach(func() {
					Expect(apiClient.Delete(ctx, editObj)).ToNot(HaveOccurred())
					Expect(apiClient.Delete(ctx, privilegedSA)).NotTo(HaveOccurred())
				})
				table.DescribeTable("should verify resource permissions", func(sars *authv1.SubjectAccessReviewSpec) {
					// Because privilegedSAFullName is filled after test Tree generation
					sars.User = privilegedSAFullName
					expectUserCan(sars)
				},
					table.Entry("[test_id:4774]: ServiceAcounts with edit role can create PVCs",
						&authv1.SubjectAccessReviewSpec{
							ResourceAttributes: &authv1.ResourceAttributes{
								Verb:      "create",
								Namespace: ssp.GoldenImagesNSname,
								Version:   core.SchemeGroupVersion.Version,
								Resource:  "persistentvolumeclaims",
							},
						}),
					table.Entry("[test_id:4845]: ServiceAcounts with edit role can delete PVCs",
						&authv1.SubjectAccessReviewSpec{
							ResourceAttributes: &authv1.ResourceAttributes{
								Verb:      "delete",
								Namespace: ssp.GoldenImagesNSname,
								Version:   core.SchemeGroupVersion.Version,
								Resource:  "persistentvolumeclaims",
							},
						}),
					table.Entry("[test_id:4877]: ServiceAccounts with edit role can view DVs",
						&authv1.SubjectAccessReviewSpec{
							ResourceAttributes: &authv1.ResourceAttributes{
								Verb:      "get",
								Namespace: ssp.GoldenImagesNSname,
								Group:     cdiv1beta1.SchemeGroupVersion.Group,
								Version:   cdiv1beta1.SchemeGroupVersion.Version,
								Resource:  "datavolumes",
							},
						}),
					table.Entry("[test_id:4872]: ServiceAccounts with edit role can create DVs",
						&authv1.SubjectAccessReviewSpec{
							ResourceAttributes: &authv1.ResourceAttributes{
								Verb:      "create",
								Namespace: ssp.GoldenImagesNSname,
								Group:     cdiv1beta1.SchemeGroupVersion.Group,
								Version:   cdiv1beta1.SchemeGroupVersion.Version,
								Resource:  "datavolumes",
							},
						}),
					table.Entry("[test_id:4876]: ServiceAccounts with edit role can delete DVs",
						&authv1.SubjectAccessReviewSpec{
							ResourceAttributes: &authv1.ResourceAttributes{
								Verb:      "delete",
								Namespace: ssp.GoldenImagesNSname,
								Group:     cdiv1beta1.SchemeGroupVersion.Group,
								Version:   cdiv1beta1.SchemeGroupVersion.Version,
								Resource:  "datavolumes",
							},
						}),
				)
				It("[test_id:4878]should not create any other resurces than the ones listed in the Edit Cluster role", func() {
					sars := &authv1.SubjectAccessReviewSpec{
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "create",
							Namespace: ssp.GoldenImagesNSname,
							Version:   core.SchemeGroupVersion.Version,
							Resource:  "pods",
						},
					}
					sars.User = privilegedSAFullName
					expectUserCannot(sars)
				})
			})
		})
	})
})
