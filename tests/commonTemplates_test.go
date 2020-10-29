package tests

import (
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	templatev1 "github.com/openshift/api/template/v1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	commonTemplates "kubevirt.io/ssp-operator/internal/operands/common-templates"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

var (
	viewRole = &testResource{
		Name:       commonTemplates.ViewRoleName,
		Namsespace: commonTemplates.GoldenImagesNSname,
		resource:   &rbac.Role{},
	}
	viewRoleBinding = &testResource{
		Name:       commonTemplates.ViewRoleName,
		Namsespace: commonTemplates.GoldenImagesNSname,
		resource:   &rbac.RoleBinding{},
	}
	editClusterRole = &testResource{
		Name:       commonTemplates.EditClusterRoleName,
		resource:   &rbac.ClusterRole{},
		Namsespace: "",
	}
	goldenImageNS = &testResource{
		Name:       commonTemplates.GoldenImagesNSname,
		resource:   &core.Namespace{},
		Namsespace: "",
	}
	testTemplate = &testResource{
		Name:       "centos6-server-large-v0.11.3",
		Namsespace: commonTemplatesTestNS,
		resource:   &templatev1.Template{},
	}
)

var _ = Describe("Common templates", func() {
	Context("resource creation", func() {
		table.DescribeTable("created cluster resource", func(res *testResource) {
			resource := res.NewResource()
			err := apiClient.Get(ctx, client.ObjectKey{Name: res.Name}, resource)
			Expect(err).ToNot(HaveOccurred())
			Expect(hasOwnerAnnotations(resource.GetAnnotations())).To(BeTrue())
		},
			table.Entry("edit role", editClusterRole),
			table.Entry("[test_id:4494]golden images namespace", goldenImageNS),
		)

		table.DescribeTable("created namespaced resource", func(res *testResource) {
			err := apiClient.Get(ctx, client.ObjectKey{
				Name: res.Name, Namespace: res.Namsespace,
			}, res.NewResource())
			Expect(err).ToNot(HaveOccurred())
		},
			table.Entry("[test_id:4777]view role", viewRole),
			table.Entry("[test_id:4772]view role binding", viewRoleBinding),
		)

		It("[test_id:5086]Create common-template in custom NS", func() {
			err := apiClient.Get(ctx, client.ObjectKey{
				Name: testTemplate.Name, Namespace: commonTemplatesTestNS,
			}, testTemplate.NewResource())
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("resource change", func() {
		table.DescribeTable("should restore modified resource", func(
			res *testResource,
			updateFunc interface{},
			equalsFunc interface{},
		) {
			key := res.GetKey()
			original := res.NewResource()
			Expect(apiClient.Get(ctx, key, original)).ToNot(HaveOccurred())

			changed := original.DeepCopyObject()
			reflect.ValueOf(updateFunc).Call([]reflect.Value{reflect.ValueOf(changed)})
			Expect(apiClient.Update(ctx, changed)).ToNot(HaveOccurred())

			newRes := res.NewResource()
			Eventually(func() bool {
				Expect(apiClient.Get(ctx, key, newRes)).ToNot(HaveOccurred())
				res := reflect.ValueOf(equalsFunc).Call([]reflect.Value{
					reflect.ValueOf(original),
					reflect.ValueOf(newRes),
				})
				return res[0].Interface().(bool)
			}, timeout, time.Second).Should(BeTrue())
		},
			table.Entry("edit cluster role", editClusterRole,
				func(role *rbac.ClusterRole) {
					role.Rules[0].Verbs = []string{"watch"}
				},
				func(old *rbac.ClusterRole, new *rbac.ClusterRole) bool {
					return reflect.DeepEqual(old.Rules, new.Rules)
				}),

			table.Entry("view role", viewRole,
				func(roleBinding *rbac.Role) {
					roleBinding.Rules = []rbac.PolicyRule{}
				},
				func(old *rbac.Role, new *rbac.Role) bool {
					return reflect.DeepEqual(old.Rules, new.Rules)
				}),

			table.Entry("view role binding", viewRoleBinding,
				func(roleBinding *rbac.RoleBinding) {
					roleBinding.Subjects = []rbac.Subject{}
				},
				func(old *rbac.RoleBinding, new *rbac.RoleBinding) bool {
					return reflect.DeepEqual(old.Subjects, new.Subjects)
				}),
			table.Entry("[test_id:5087]test template", testTemplate,
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
		table.DescribeTable("recreate after delete", func(res *testResource) {
			resource := res.NewResource()
			resource.SetName(res.Name)
			resource.SetNamespace(res.Namsespace)
			Expect(apiClient.Delete(ctx, resource)).ToNot(HaveOccurred())

			Eventually(func() error {
				return apiClient.Get(ctx, client.ObjectKey{
					Name: res.Name, Namespace: res.Namsespace,
				}, resource)
			}, timeout, time.Second).ShouldNot(HaveOccurred())
		},
			table.Entry("[test_id:4773]view role", viewRole),
			table.Entry("[test_id:4842]view role binding", viewRoleBinding),
			table.Entry("[test_id:5088]testTemplate in custom NS", testTemplate),
			table.Entry("edit cluster role", editClusterRole),
			table.Entry("[test_id:4770]golden image NS", goldenImageNS),
		)
	})
})
