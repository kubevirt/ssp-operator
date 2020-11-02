package tests

import (
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	secv1 "github.com/openshift/api/security/v1"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	nodelabeller "kubevirt.io/ssp-operator/internal/operands/node-labeller"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

var (
	nlClusterRoleRes = &testResource{
		Name:       nodelabeller.ClusterRoleName,
		resource:   &rbac.ClusterRole{},
		Namsespace: "",
	}
	nlClusterRoleBindingRes = &testResource{
		Name:       nodelabeller.ClusterRoleBindingName,
		resource:   &rbac.ClusterRoleBinding{},
		Namsespace: "",
	}
	nlServiceAccountRes = &testResource{
		Name:       nodelabeller.ServiceAccountName,
		Namsespace: testNamespace,
		resource:   &core.ServiceAccount{},
	}
	securityContextConstraintRes = &testResource{
		Name:       nodelabeller.SecurityContextName,
		Namsespace: "",
		resource:   &secv1.SecurityContextConstraints{},
	}
	configMapRes = &testResource{
		Name:       nodelabeller.ConfigMapName,
		Namsespace: testNamespace,
		resource:   &core.ConfigMap{},
	}
	daemonSetRes = &testResource{
		Name:       nodelabeller.DaemonSetName,
		Namsespace: testNamespace,
		resource:   &apps.DaemonSet{},
	}
)

var _ = Describe("Node Labeller", func() {
	Context("resource creation", func() {
		table.DescribeTable("created cluster resource", func(res *testResource) {
			resource := res.NewResource()
			err := apiClient.Get(ctx, client.ObjectKey{Name: res.Name}, resource)
			Expect(err).ToNot(HaveOccurred())
			Expect(hasOwnerAnnotations(resource.GetAnnotations())).To(BeTrue())
		},
			table.Entry("[test_id:5193] cluster role", nlClusterRoleRes),
			table.Entry("[test_id:5196] cluster role binding", nlClusterRoleBindingRes),
			table.Entry("[test_id:5202] security context constraint", securityContextConstraintRes),
		)

		table.DescribeTable("created namespaced resource", func(res *testResource) {
			err := apiClient.Get(ctx, client.ObjectKey{
				Name: res.Name, Namespace: testNamespace,
			}, res.NewResource())
			Expect(err).ToNot(HaveOccurred())
		},
			table.Entry("[test_id:5205] service account", nlServiceAccountRes),
			table.Entry("[test_id:5199] configMap", configMapRes),
			table.Entry("[test_id:5190] daemonSet", daemonSetRes),
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
			table.Entry("[test_id:5194] cluster role", nlClusterRoleRes),
			table.Entry("[test_id:5198] cluster role binding", nlClusterRoleBindingRes),
			table.Entry("[test_id:5203] security context constraint", securityContextConstraintRes),
			table.Entry("[test_id:5206] service account", nlServiceAccountRes),
			table.Entry("[test_id:5200] configMap", configMapRes),
			table.Entry("[test_id:5191] daemonSet", daemonSetRes),
		)
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
			table.Entry("[test_id:5195] cluster role", nlClusterRoleRes,
				func(role *rbac.ClusterRole) {
					role.Rules[0].Verbs = []string{"watch"}
				},
				func(old *rbac.ClusterRole, new *rbac.ClusterRole) bool {
					return reflect.DeepEqual(old.Rules, new.Rules)
				}),

			table.Entry("[test_id:5197] cluster role binding", nlClusterRoleBindingRes,
				func(roleBinding *rbac.ClusterRoleBinding) {
					roleBinding.Subjects = []rbac.Subject{}
				},
				func(old *rbac.ClusterRoleBinding, new *rbac.ClusterRoleBinding) bool {
					return reflect.DeepEqual(old.Subjects, new.Subjects)
				}),

			table.Entry("[test_id:5204] security context constraint", securityContextConstraintRes,
				func(scc *secv1.SecurityContextConstraints) {
					scc.Users = []string{"test-user"}
				},
				func(old *secv1.SecurityContextConstraints, new *secv1.SecurityContextConstraints) bool {
					return reflect.DeepEqual(old.Users, new.Users)
				}),

			table.Entry("[test_id:5201] Config Map", configMapRes,
				func(configMap *core.ConfigMap) {
					configMap.Data = map[string]string{
						"cpu-plugin-configmap.yaml": "change data",
					}
				},
				func(old *core.ConfigMap, new *core.ConfigMap) bool {
					return reflect.DeepEqual(old.Data, new.Data)
				}),

			table.Entry("[test_id:5192] daemonSet", daemonSetRes,
				func(daemonSet *apps.DaemonSet) {
					daemonSet.Labels = map[string]string{
						"test": "test-label",
					}
				},
				func(old *apps.DaemonSet, new *apps.DaemonSet) bool {
					return reflect.DeepEqual(old.Spec, new.Spec)
				}),
		)
	})
})
