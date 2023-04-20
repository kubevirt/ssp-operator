package tests

import (
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ssp "kubevirt.io/ssp-operator/api/v1beta1"
	"kubevirt.io/ssp-operator/internal/common"

	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Tekton Pipelines Operand", func() {
	var (
		pipelineResource       testResource
		clusterRoleResource    testResource
		roleBindingResource    testResource
		configMapResource      testResource
		serviceAccountResource testResource
	)

	BeforeEach(func() {
		expectedLabels := expectedLabelsFor("tekton-pipelines", common.AppComponentTektonPipelines)
		pipelineResource = testResource{
			Name:           "tekton-pipelines",
			Namespace:      strategy.GetTektonPipelinesNamespace(),
			Resource:       &pipeline.Pipeline{},
			ExpectedLabels: expectedLabels,
			UpdateFunc: func(pipeline *pipeline.Pipeline) {
				pipeline.Spec.Description = "test"
			},
			EqualsFunc: func(old *pipeline.Pipeline, new *pipeline.Pipeline) bool {
				return reflect.DeepEqual(old.Spec, new.Spec)
			},
		}
		clusterRoleResource = testResource{
			Name:           "tekton-pipelines",
			Resource:       &rbac.ClusterRole{},
			ExpectedLabels: expectedLabels,
		}
		roleBindingResource = testResource{
			Name:           "tekton-pipelines",
			Namespace:      strategy.GetTektonPipelinesNamespace(),
			Resource:       &rbac.RoleBinding{},
			ExpectedLabels: expectedLabels,
		}
		configMapResource = testResource{
			Name:           "tekton-pipelines",
			Namespace:      strategy.GetTektonPipelinesNamespace(),
			Resource:       &core.ConfigMap{},
			ExpectedLabels: expectedLabels,
			UpdateFunc: func(configMap *core.ConfigMap) {
				configMap.Data = map[string]string{}
			},
			EqualsFunc: func(old *core.ConfigMap, new *core.ConfigMap) bool {
				return reflect.DeepEqual(old.Immutable, new.Immutable) &&
					reflect.DeepEqual(old.Data, new.Data) &&
					reflect.DeepEqual(old.BinaryData, new.BinaryData)
			},
		}
		serviceAccountResource = testResource{
			Name:           "tekton-pipelines",
			Namespace:      strategy.GetTektonPipelinesNamespace(),
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
		Expect(err).To(HaveOccurred())
		Expect(errors.ReasonForError(err)).To(Equal(metav1.StatusReasonNotFound))
	},
		Entry("[test_id:TODO] pipeline", &pipelineResource),
		Entry("[test_id:TODO] cluster role", &clusterRoleResource),
		Entry("[test_id:TODO] role binding", &roleBindingResource),
		Entry("[test_id:TODO] config map", &configMapResource),
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
			Entry("[test_id:TODO] pipeline", &pipelineResource),
			Entry("[test_id:TODO] cluster role", &clusterRoleResource),
			Entry("[test_id:TODO] role binding", &roleBindingResource),
			Entry("[test_id:TODO] config map", &configMapResource),
			Entry("[test_id:TODO] service account", &serviceAccountResource),
		)

		DescribeTable("should set app labels", expectAppLabels,
			Entry("[test_id:TODO] pipeline", &pipelineResource),
			Entry("[test_id:TODO] cluster role", &clusterRoleResource),
			Entry("[test_id:TODO] role binding", &roleBindingResource),
			Entry("[test_id:TODO] config map", &configMapResource),
			Entry("[test_id:TODO] service account", &serviceAccountResource),
		)

		Context("resource deletion", func() {
			DescribeTable("recreate after delete", expectRecreateAfterDelete,
				Entry("[test_id:TODO] pipeline", &pipelineResource),
				Entry("[test_id:TODO] cluster role", &clusterRoleResource),
				Entry("[test_id:TODO] role binding", &roleBindingResource),
				Entry("[test_id:TODO] config map", &configMapResource),
				Entry("[test_id:TODO] service account", &serviceAccountResource),
			)
		})
	})

	Context("resource change", func() {
		BeforeEach(func() {
			strategy.SkipSspUpdateTestsIfNeeded()

			updateSsp(func(foundSsp *ssp.SSP) {
				foundSsp.Spec.FeatureGates.DeployTektonTaskResources = true
			})

			waitUntilDeployed()
		})

		DescribeTable("should restore modified resource", expectRestoreAfterUpdate,
			Entry("[test_id:TODO] pipeline", &pipelineResource),
			Entry("[test_id:TODO] config map", &configMapResource),
		)

		Context("with pause", func() {
			JustAfterEach(func() {
				unpauseSsp()
			})

			DescribeTable("should restore modified resource with pause", expectRestoreAfterUpdateWithPause,
				Entry("[test_id:TODO] pipeline", &pipelineResource),
				Entry("[test_id:TODO] config map", &configMapResource),
			)
		})
	})
})
