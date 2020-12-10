package tests

import (
	"reflect"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	secv1 "github.com/openshift/api/security/v1"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"

	nodelabeller "kubevirt.io/ssp-operator/internal/operands/node-labeller"
)

var _ = Describe("Node Labeller", func() {
	var (
		clusterRoleRes               testResource
		clusterRoleBindingRes        testResource
		serviceAccountRes            testResource
		securityContextConstraintRes testResource
		configMapRes                 testResource
		daemonSetRes                 testResource
	)

	BeforeEach(func() {
		clusterRoleRes = testResource{
			Name:      nodelabeller.ClusterRoleName,
			Resource:  &rbac.ClusterRole{},
			Namespace: "",
			UpdateFunc: func(role *rbac.ClusterRole) {
				role.Rules[0].Verbs = []string{"watch"}
			},
			EqualsFunc: func(old *rbac.ClusterRole, new *rbac.ClusterRole) bool {
				return reflect.DeepEqual(old.Rules, new.Rules)
			},
		}
		clusterRoleBindingRes = testResource{
			Name:      nodelabeller.ClusterRoleBindingName,
			Resource:  &rbac.ClusterRoleBinding{},
			Namespace: "",
			UpdateFunc: func(roleBinding *rbac.ClusterRoleBinding) {
				roleBinding.Subjects = []rbac.Subject{}
			},
			EqualsFunc: func(old *rbac.ClusterRoleBinding, new *rbac.ClusterRoleBinding) bool {
				return reflect.DeepEqual(old.Subjects, new.Subjects)
			},
		}
		serviceAccountRes = testResource{
			Name:      nodelabeller.ServiceAccountName,
			Namespace: strategy.GetNamespace(),
			Resource:  &core.ServiceAccount{},
		}
		securityContextConstraintRes = testResource{
			Name:      nodelabeller.SecurityContextName,
			Namespace: "",
			Resource:  &secv1.SecurityContextConstraints{},
			UpdateFunc: func(scc *secv1.SecurityContextConstraints) {
				scc.Users = []string{"test-user"}
			},
			EqualsFunc: func(old *secv1.SecurityContextConstraints, new *secv1.SecurityContextConstraints) bool {
				return reflect.DeepEqual(old.Users, new.Users)
			},
		}
		configMapRes = testResource{
			Name:      nodelabeller.ConfigMapName,
			Namespace: strategy.GetNamespace(),
			Resource:  &core.ConfigMap{},
			UpdateFunc: func(configMap *core.ConfigMap) {
				configMap.Data = map[string]string{
					"cpu-plugin-configmap.yaml": "change data",
				}
			},
			EqualsFunc: func(old *core.ConfigMap, new *core.ConfigMap) bool {
				return reflect.DeepEqual(old.Data, new.Data)
			},
		}
		daemonSetRes = testResource{
			Name:      nodelabeller.DaemonSetName,
			Namespace: strategy.GetNamespace(),
			Resource:  &apps.DaemonSet{},
			UpdateFunc: func(daemonSet *apps.DaemonSet) {
				daemonSet.Spec.Template.Spec.ServiceAccountName = "test-account"
			},
			EqualsFunc: func(old *apps.DaemonSet, new *apps.DaemonSet) bool {
				return reflect.DeepEqual(old.Spec, new.Spec)
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
			table.Entry("[test_id:5193] cluster role", &clusterRoleRes),
			table.Entry("[test_id:5196] cluster role binding", &clusterRoleBindingRes),
			table.Entry("[test_id:5202] security context constraint", &securityContextConstraintRes),
		)

		table.DescribeTable("created namespaced resource", func(res *testResource) {
			err := apiClient.Get(ctx, res.GetKey(), res.NewResource())
			Expect(err).ToNot(HaveOccurred())
		},
			table.Entry("[test_id:5205] service account", &serviceAccountRes),
			table.Entry("[test_id:5199] configMap", &configMapRes),
			table.Entry("[test_id:5190] daemonSet", &daemonSetRes),
		)
	})

	Context("resource deletion", func() {
		table.DescribeTable("recreate after delete", expectRecreateAfterDelete,
			table.Entry("[test_id:5194] cluster role", &clusterRoleRes),
			table.Entry("[test_id:5198] cluster role binding", &clusterRoleBindingRes),
			table.Entry("[test_id:5203] security context constraint", &securityContextConstraintRes),
			table.Entry("[test_id:5206] service account", &serviceAccountRes),
			table.Entry("[test_id:5200] configMap", &configMapRes),
			table.Entry("[test_id:5191] daemonSet", &daemonSetRes),
		)
	})

	Context("resource change", func() {
		table.DescribeTable("should restore modified resource", expectRestoreAfterUpdate,
			table.Entry("[test_id:5195] cluster role", &clusterRoleRes),
			table.Entry("[test_id:5197] cluster role binding", &clusterRoleBindingRes),
			table.Entry("[test_id:5204] security context constraint", &securityContextConstraintRes),
			table.Entry("[test_id:5201] Config Map", &configMapRes),
			table.Entry("[test_id:5192] daemonSet", &daemonSetRes),
		)
	})

	It("all pods should be ready when deployed", func() {
		daemonSet := &apps.DaemonSet{}
		Expect(apiClient.Get(ctx, daemonSetRes.GetKey(), daemonSet)).ToNot(HaveOccurred())
		Expect(daemonSet.Status.NumberReady).To(Equal(daemonSet.Status.DesiredNumberScheduled))
	})
})
