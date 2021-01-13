package tests

import (
	"fmt"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	templatev1 "github.com/openshift/api/template/v1"
	authv1 "k8s.io/api/authorization/v1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
		viewRole = testResource{
			Name:      commonTemplates.ViewRoleName,
			Namespace: commonTemplates.GoldenImagesNSname,
			Resource:  &rbac.Role{},
			UpdateFunc: func(role *rbac.Role) {
				role.Rules = []rbac.PolicyRule{}
			},
			EqualsFunc: func(old *rbac.Role, new *rbac.Role) bool {
				return reflect.DeepEqual(old.Rules, new.Rules)
			},
		}
		viewRoleBinding = testResource{
			Name:      commonTemplates.ViewRoleName,
			Namespace: commonTemplates.GoldenImagesNSname,
			Resource:  &rbac.RoleBinding{},
			UpdateFunc: func(roleBinding *rbac.RoleBinding) {
				roleBinding.Subjects = nil
			},
			EqualsFunc: func(old *rbac.RoleBinding, new *rbac.RoleBinding) bool {
				return reflect.DeepEqual(old.Subjects, new.Subjects)
			},
		}
		editClusterRole = testResource{
			Name:      commonTemplates.EditClusterRoleName,
			Resource:  &rbac.ClusterRole{},
			Namespace: "",
			UpdateFunc: func(role *rbac.ClusterRole) {
				role.Rules[0].Verbs = []string{"watch"}
			},
			EqualsFunc: func(old *rbac.ClusterRole, new *rbac.ClusterRole) bool {
				return reflect.DeepEqual(old.Rules, new.Rules)
			},
		}
		goldenImageNS = testResource{
			Name:      commonTemplates.GoldenImagesNSname,
			Resource:  &core.Namespace{},
			Namespace: "",
		}
		testTemplate = testResource{
			Name:      "rhel8-desktop-tiny",
			Namespace: strategy.GetTemplatesNamespace(),
			Resource:  &templatev1.Template{},
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
			Expect(hasOwnerAnnotations(resource.GetAnnotations())).To(BeTrue())
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
					"template.kubevirt.io/version": commonTemplates.Version,
				},
			)
			Expect(err).ToNot(HaveOccurred())

			osDefaultCounts := make(map[string]int)
			for _, liveTemplate := range liveTemplates.Items {
				_, isDefaultOSVariant := liveTemplate.Labels["template.kubevirt.io/default-os-variant"]

				for labelKey := range liveTemplate.Labels {
					if strings.HasPrefix(labelKey, "os.template.kubevirt.io/") {
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
				fmt.Fprintf(GinkgoWriter, "checking osDefaultCount for %s\n", os)
				Expect(defaultCount).To(BeNumerically("==", 1))
			}
		})

		It("[test_id:5545]did not create duplicate templates", func() {
			liveTemplates := &templatev1.TemplateList{}
			err := apiClient.List(ctx, liveTemplates,
				client.InNamespace(strategy.GetTemplatesNamespace()),
				client.MatchingLabels{
					"template.kubevirt.io/version": commonTemplates.Version,
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
					if strings.HasPrefix(labelKey, "os.template.kubevirt.io/") {
						oss = append(oss, labelKey)
						continue
					}
					if strings.HasPrefix(labelKey, "workload.template.kubevirt.io/") {
						workloads = append(workloads, labelKey)
						continue
					}
					if strings.HasPrefix(labelKey, "flavor.template.kubevirt.io/") {
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
							Expect(matchingLiveTemplates).To(BeNumerically("==", 1))
						}
					}
				}
			}
		})
	})

	Context("resource change", func() {
		table.DescribeTable("should restore modified resource", expectRestoreAfterUpdate,
			table.Entry("[test_id:5315]edit cluster role", &editClusterRole),
			table.Entry("[test_id:5316]view role", &viewRole),
			table.Entry("[test_id:5317]view role binding", &viewRoleBinding),
			table.Entry("[test_id:5087]test template", &testTemplate),
		)

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
		var (
			ownerTemplate, oldTemplate *templatev1.Template
		)

		BeforeEach(func() {
			// Create a dummy template to act as an owner for the test template
			// we can't use the SSP CR as an owner for these tests because the tempaltes
			// might be deployed in a different namespace than the CR, and will be immediately
			// removed by the GC, the choice to use a template as an owner object was arbitrary
			ownerTemplate = &templatev1.Template{
				ObjectMeta: v1.ObjectMeta{
					Name:      "owner-template",
					Namespace: strategy.GetTemplatesNamespace(),
				},
			}
			Expect(apiClient.Create(ctx, ownerTemplate)).ToNot(HaveOccurred(), "failed to create dummy owner for an old template")

			oldTemplate = &templatev1.Template{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-old-template",
					Namespace: strategy.GetTemplatesNamespace(),
					Labels: map[string]string{
						"template.kubevirt.io/version":         "not-latest",
						"template.kubevirt.io/type":            "base",
						"os.template.kubevirt.io/some-os":      "true",
						"flavor.template.kubevirt.io/test":     "true",
						"workload.template.kubevirt.io/server": "true",
					},
					OwnerReferences: []v1.OwnerReference{{
						APIVersion: "template.openshift.io/v1",
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

		It("should replace ownerReference with owner annotations for older templates", func() {
			triggerReconciliation()

			// Template should eventually be updated by the operator
			Eventually(func() bool {
				updatedTpl := &templatev1.Template{}
				key := client.ObjectKey{Name: oldTemplate.Name, Namespace: oldTemplate.Namespace}
				err := apiClient.Get(ctx, key, updatedTpl)
				return err == nil &&
					len(updatedTpl.GetOwnerReferences()) == 0 &&
					hasOwnerAnnotations(updatedTpl.GetAnnotations())
			}, shortTimeout).Should(BeTrue(), "ownerReference was not replaced by owner annotations on the old template")
		})
		It("should remove labels from old templates", func() {
			triggerReconciliation()
			// Template should eventually be updated by the operator
			Eventually(func() bool {
				updatedTpl := &templatev1.Template{}
				key := client.ObjectKey{Name: oldTemplate.Name, Namespace: oldTemplate.Namespace}
				err := apiClient.Get(ctx, key, updatedTpl)
				return err == nil &&
					updatedTpl.Labels["os.template.kubevirt.io/some-os"] == "" &&
					updatedTpl.Labels["flavor.template.kubevirt.io/test"] == "" &&
					updatedTpl.Labels["workload.template.kubevirt.io/server"] == "" &&
					updatedTpl.Labels["template.kubevirt.io/type"] == "base" &&
					updatedTpl.Labels["template.kubevirt.io/version"] == "not-latest"
			}, shortTimeout).Should(BeTrue(), "labels were not removed from older templates")
		})
		It("should continue to have labels on latest templates", func() {
			triggerReconciliation()

			baseRequirement, err := labels.NewRequirement("template.kubevirt.io/type", selection.Equals, []string{"base"})
			Expect(err).To(BeNil())
			versionRequirement, err := labels.NewRequirement("template.kubevirt.io/version", selection.Equals, []string{commonTemplates.Version})
			Expect(err).To(BeNil())
			labelsSelector := labels.NewSelector().Add(*baseRequirement, *versionRequirement)
			opts := client.ListOptions{
				LabelSelector: labelsSelector,
				Namespace:     strategy.GetTemplatesNamespace(),
			}

			var latestTemplates templatev1.TemplateList
			err = apiClient.List(ctx, &latestTemplates, &opts)
			Expect(err).To(BeNil())
			Expect(len(latestTemplates.Items)).To(BeNumerically(">", 0))

			for _, template := range latestTemplates.Items {
				for label, value := range template.Labels {
					if strings.HasPrefix(label, "os.template.kubevirt.io/") ||
						strings.HasPrefix(label, "flavor.template.kubevirt.io/") ||
						strings.HasPrefix(label, "workload.template.kubevirt.io/") {
						Expect(value).To(Equal("true"))
					}
				}
				Expect(template.Labels["template.kubevirt.io/type"]).To(Equal("base"))
				Expect(template.Labels["template.kubevirt.io/version"]).To(Equal(commonTemplates.Version))

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
						Name:      "regular-sa",
						Namespace: strategy.GetNamespace(),
					},
				}
				regularSAFullName = fmt.Sprintf("serviceaccount:%s:%s", regularSA.GetNamespace(), regularSA.GetName())

				Expect(apiClient.Create(ctx, regularSA)).ToNot(HaveOccurred(), "creation of regular service account failed")
				Expect(apiClient.Get(ctx, getResourceKey(regularSA), regularSA)).ToNot(HaveOccurred())
			})

			AfterEach(func() {
				Expect(apiClient.Delete(ctx, regularSA)).NotTo(HaveOccurred())
			})

			table.DescribeTable("regular service account namespace RBAC", expectUserCan,
				table.Entry("should be able to 'get' namespaces",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "get",
							Namespace: commonTemplates.GoldenImagesNSname,
							Version:   "v1",
							Resource:  "namespaces",
						},
					}),
				table.Entry("should be able to 'list' namespaces",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "list",
							Namespace: commonTemplates.GoldenImagesNSname,
							Version:   "v1",
							Resource:  "namespaces",
						},
					}),
				table.Entry("should be able to 'watch' namespaces",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "list",
							Namespace: commonTemplates.GoldenImagesNSname,
							Version:   "v1",
							Resource:  "namespaces",
						},
					}))

			table.DescribeTable("regular service account DV RBAC", expectUserCan,
				table.Entry("should be able to 'get' datavolumes",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "get",
							Namespace: commonTemplates.GoldenImagesNSname,
							Group:     "cdi.kubevirt.io",
							Version:   "v1beta1",
							Resource:  "datavolumes",
						},
					}),
				table.Entry("should be able to 'list' datavolumes",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "list",
							Namespace: commonTemplates.GoldenImagesNSname,
							Group:     "cdi.kubevirt.io",
							Version:   "v1beta1",
							Resource:  "datavolumes",
						},
					}),
				table.Entry("should be able to 'watch' datavolumes",
					&authv1.SubjectAccessReviewSpec{
						User:   regularSAFullName,
						Groups: sasGroup,
						ResourceAttributes: &authv1.ResourceAttributes{
							Verb:      "watch",
							Namespace: commonTemplates.GoldenImagesNSname,
							Group:     "cdi.kubevirt.io",
							Version:   "v1beta1",
							Resource:  "datavolumes",
						},
					}))
		})
	})
})
