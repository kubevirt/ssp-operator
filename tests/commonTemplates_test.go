package tests

import (
	"reflect"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	templatev1 "github.com/openshift/api/template/v1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"

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
			Name:       "centos6-server-large-v0.11.3",
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
