package tests

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ssp "kubevirt.io/ssp-operator/api/v1beta1"

	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Tekton Tasks Operand", func() {
	var (
		taskResource           testResource
		clusterRoleResource    testResource
		roleBindingResource    testResource
		serviceAccountResource testResource
	)

	BeforeEach(func() {
		expectedLabels := expectedLabelsFor("tekton-tasks", "tektonTasks")
		taskResource = testResource{
			Name:           "tekton-tasks",
			Namespace:      strategy.GetNamespace(),
			Resource:       &pipeline.Task{},
			ExpectedLabels: expectedLabels,
		}
		clusterRoleResource = testResource{
			Name:           "tekton-tasks",
			Resource:       &rbac.ClusterRole{},
			ExpectedLabels: expectedLabels,
		}
		roleBindingResource = testResource{
			Name:           "tekton-tasks",
			Namespace:      strategy.GetNamespace(),
			Resource:       &rbac.RoleBinding{},
			ExpectedLabels: expectedLabels,
		}
		serviceAccountResource = testResource{
			Name:           "tekton-tasks",
			Namespace:      strategy.GetNamespace(),
			Resource:       &core.ServiceAccount{},
			ExpectedLabels: expectedLabels,
		}
	})

	BeforeEach(func() {
		strategy.SkipSspUpdateTestsIfNeeded()

		updateSsp(func(foundSsp *ssp.SSP) {
			foundSsp.Spec.FeatureGates.DeployTektonTaskResources = false
		})

		waitUntilDeployed()
	})

	DescribeTable("not created resource", func(res *testResource) {
		resource := res.NewResource()
		err := apiClient.Get(ctx, res.GetKey(), resource)
		Expect(errors.ReasonForError(err)).To(Equal(metav1.StatusReasonNotFound))
		Expect(err).To(HaveOccurred())
	},
		Entry("[test_id:TODO] task", &taskResource),
		Entry("[test_id:TODO] cluster role", &clusterRoleResource),
		Entry("[test_id:TODO] role binding", &roleBindingResource),
		Entry("[test_id:TODO] service account", &serviceAccountResource),
	)

	Context("resource creation when DeployTektonTaskResources is set to true", func() {
		BeforeEach(func() {
			strategy.SkipSspUpdateTestsIfNeeded()

			updateSsp(func(foundSsp *ssp.SSP) {
				foundSsp.Spec.FeatureGates.DeployTektonTaskResources = true
			})

			waitUntilDeployed()
		})

		AfterEach(func() {
			strategy.RevertToOriginalSspCr()
		})

		DescribeTable("created resource", func(res *testResource) {
			resource := res.NewResource()
			err := apiClient.Get(ctx, res.GetKey(), resource)
			Expect(err).ToNot(HaveOccurred())
		},
			Entry("[test_id:TODO] task", &taskResource),
			Entry("[test_id:TODO] cluster role", &clusterRoleResource),
			Entry("[test_id:TODO] role binding", &roleBindingResource),
			Entry("[test_id:TODO] service account", &serviceAccountResource),
		)

		DescribeTable("should set app labels", expectAppLabels,
			Entry("[test_id:TODO] task", &taskResource),
			Entry("[test_id:TODO] cluster role", &clusterRoleResource),
			Entry("[test_id:TODO] role binding", &roleBindingResource),
			Entry("[test_id:TODO] service account", &serviceAccountResource),
		)

		Context("resource deletion", func() {
			DescribeTable("recreate after delete", expectRecreateAfterDelete,
				Entry("[test_id:TODO] task", &taskResource),
				Entry("[test_id:TODO] cluster role", &clusterRoleResource),
				Entry("[test_id:TODO] role binding", &roleBindingResource),
				Entry("[test_id:TODO] service account", &serviceAccountResource),
			)
		})
	})
})
