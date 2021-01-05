package tests

import (
	"fmt"
	"reflect"
	"strings"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	templatev1 "github.com/openshift/api/template/v1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sspv1beta1 "kubevirt.io/ssp-operator/api/v1beta1"
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
			Name:      "centos6-server-large",
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
			ssp           *sspv1beta1.SSP
			ownerTemplate *templatev1.Template
		)

		BeforeEach(func() {
			ssp = getSsp()

			// Create a dummy template to act as an owner for the test template
			// we can't use the SSP CR as an owner for these tests because the tempaltes
			// might be deployed in a different namespace than the CR, and will be immediately
			// removed by the GC, the choice to use a template as an owner object was arbitrary
			ownerTemplate = func() *templatev1.Template {
				tpl := &templatev1.Template{
					ObjectMeta: v1.ObjectMeta{
						Name:      "owner-template",
						Namespace: ssp.Spec.CommonTemplates.Namespace,
					},
				}
				Expect(apiClient.Create(ctx, tpl)).ToNot(HaveOccurred(), "failed to create dummy owner for an old template")
				key, err := client.ObjectKeyFromObject(tpl)
				Expect(err).ToNot(HaveOccurred(), "failed to read template object key")
				Expect(apiClient.Get(ctx, key, tpl)).ToNot(HaveOccurred())

				return tpl
			}()
		})

		AfterEach(func() {
			Expect(apiClient.Delete(ctx, ownerTemplate)).ToNot(HaveOccurred(), "deletion of dummy owner template failed")
		})

		It("should replace ownerReference with owner annotations for older templates", func() {
			oldTpl := func() *templatev1.Template {
				tpl := &templatev1.Template{
					ObjectMeta: v1.ObjectMeta{
						Name:      "test-old-template",
						Namespace: ssp.Spec.CommonTemplates.Namespace,
						Labels: map[string]string{
							"template.kubevirt.io/version": "not-latest",
							"template.kubevirt.io/type":    "base",
						},
						OwnerReferences: []v1.OwnerReference{{
							APIVersion: "template.openshift.io/v1",
							Kind:       "Template",
							Name:       ownerTemplate.Name,
							UID:        ownerTemplate.UID,
						}},
					},
				}
				Expect(apiClient.Create(ctx, tpl)).ToNot(HaveOccurred(), "creation of dummy old template failed")

				return tpl
			}()

			triggerReconciliation()

			// Template should eventually be updated by the operator
			Eventually(func() bool {
				updatedTpl := &templatev1.Template{}
				key, err := client.ObjectKeyFromObject(oldTpl)
				Expect(err).ToNot(HaveOccurred(), "failed to read template object key")
				err = apiClient.Get(ctx, key, updatedTpl)
				if err != nil {
					return false
				}

				if len(updatedTpl.GetOwnerReferences()) == 0 && hasOwnerAnnotations(updatedTpl.GetAnnotations()) {
					return true
				}

				return false
			}, shortTimeout).Should(BeTrue(), "ownerReference was not replaced by owner annotations on the old template")

			// Cleanup
			Expect(apiClient.Delete(ctx, oldTpl)).ToNot(HaveOccurred(), "deletion of dummy old template failed")
		})
	})
})
