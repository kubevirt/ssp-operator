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
			Name:       commonTemplates.ViewRoleName,
			Namsespace: commonTemplates.GoldenImagesNSname,
			resource:   &rbac.Role{},
		}
		viewRoleBinding = testResource{
			Name:       commonTemplates.ViewRoleName,
			Namsespace: commonTemplates.GoldenImagesNSname,
			resource:   &rbac.RoleBinding{},
		}
		editClusterRole = testResource{
			Name:       commonTemplates.EditClusterRoleName,
			resource:   &rbac.ClusterRole{},
			Namsespace: "",
		}
		goldenImageNS = testResource{
			Name:       commonTemplates.GoldenImagesNSname,
			resource:   &core.Namespace{},
			Namsespace: "",
		}
		testTemplate = testResource{
			Name:       "centos6-server-large",
			Namsespace: strategy.GetTemplatesNamespace(),
			resource:   &templatev1.Template{},
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
		)

		It("[test_id:5086]Create common-template in custom NS", func() {
			err := apiClient.Get(ctx, testTemplate.GetKey(), testTemplate.NewResource())
			Expect(err).ToNot(HaveOccurred())
		})

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
			table.Entry("[test_id:5315]edit cluster role", &editClusterRole,
				func(role *rbac.ClusterRole) {
					role.Rules[0].Verbs = []string{"watch"}
				},
				func(old *rbac.ClusterRole, new *rbac.ClusterRole) bool {
					return reflect.DeepEqual(old.Rules, new.Rules)
				}),

			table.Entry("[test_id:5316]view role", &viewRole,
				func(roleBinding *rbac.Role) {
					roleBinding.Rules = []rbac.PolicyRule{}
				},
				func(old *rbac.Role, new *rbac.Role) bool {
					return reflect.DeepEqual(old.Rules, new.Rules)
				}),

			table.Entry("[test_id:5317]view role binding", &viewRoleBinding,
				func(roleBinding *rbac.RoleBinding) {
					roleBinding.Subjects = []rbac.Subject{}
				},
				func(old *rbac.RoleBinding, new *rbac.RoleBinding) bool {
					return reflect.DeepEqual(old.Subjects, new.Subjects)
				}),
			table.Entry("[test_id:5087]test template", &testTemplate,
				func(t *templatev1.Template) {
					t.Parameters = []templatev1.Parameter{}
				},
				func(old *templatev1.Template, new *templatev1.Template) bool {
					return reflect.DeepEqual(old.Parameters, new.Parameters)
				},
			),
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
})
