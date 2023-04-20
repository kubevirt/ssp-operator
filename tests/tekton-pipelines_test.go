package tests

import (
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kubevirt/tekton-tasks-operator/pkg/common"
	tektontasks "github.com/kubevirt/tekton-tasks-operator/pkg/tekton-tasks"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Tekton-pipelines", func() {
	Context("resource creation", func() {
		BeforeEach(func() {
			tto := strategy.GetTTO()
			tto.Spec.FeatureGates.DeployTektonTaskResources = true
			createOrUpdateTekton(tto)
			waitUntilDeployed()
		})

		It("[test_id:TODO]operator should create pipelines in correct namespace", func() {
			livePipelines := &pipeline.PipelineList{}
			Eventually(func() bool {
				err := apiClient.List(ctx, livePipelines,
					client.MatchingLabels{
						common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(livePipelines.Items) > 0
			}, tenSecondTimeout, time.Second).Should(BeTrue())

			for _, pipeline := range livePipelines.Items {
				Expect(pipeline.Labels[common.AppKubernetesComponentLabel]).To(Equal(string(common.AppComponentTektonPipelines)), "component label should equal")
				Expect(pipeline.Labels[common.AppKubernetesManagedByLabel]).To(Equal(common.AppKubernetesManagedByValue), "managed by label should equal")
			}
		})

		It("[test_id:TODO]operator should create role bindings", func() {
			liveRB := &rbac.RoleBindingList{}
			roleBindingName := "windows10-pipelines"
			Eventually(func() bool {
				err := apiClient.List(ctx, liveRB,
					client.MatchingLabels{
						common.AppKubernetesComponentLabel: string(common.AppComponentTektonPipelines),
						common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(liveRB.Items) > 0
			}, tenSecondTimeout, time.Second).Should(BeTrue())

			for _, rb := range liveRB.Items {
				if _, ok := tektontasks.AllowedTasks[strings.TrimSuffix(rb.Name, "-task")]; !ok {
					if ok = rb.Name != roleBindingName; ok {
						Expect(ok).To(BeTrue(), "only allowed role binding is deployed - "+rb.Name)
					}
				}
				Expect(rb.Labels[common.AppKubernetesManagedByLabel]).To(Equal(common.AppKubernetesManagedByValue), "managed by label should equal")
			}
		})
	})
	Context("user updates reverted", func() {
		BeforeEach(func() {
			tto := strategy.GetTTO()
			tto.Spec.FeatureGates.DeployTektonTaskResources = true
			createOrUpdateTekton(tto)
			waitUntilDeployed()
		})

		It("[test_id:TODO]operator should rever user update on pipeline", func() {
			livePipelines := &pipeline.PipelineList{}
			Eventually(func() bool {
				err := apiClient.List(ctx, livePipelines,
					client.MatchingLabels{
						common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(livePipelines.Items) > 0
			}, tenSecondTimeout, time.Second).Should(BeTrue())

			pipeline := livePipelines.Items[0]
			pipeline.Spec.Description = "test"
			err := apiClient.Update(ctx, &pipeline)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() bool {
				err := apiClient.Get(ctx, client.ObjectKeyFromObject(&pipeline), &pipeline)
				Expect(err).ToNot(HaveOccurred())
				return pipeline.Spec.Description != "test"
			}, tenSecondTimeout, time.Second).Should(BeTrue())
		})

		It("[test_id:TODO]operator should rever user update on configMap", func() {
			liveCM := &v1.ConfigMapList{}
			Eventually(func() bool {
				err := apiClient.List(ctx, liveCM,
					client.MatchingLabels{
						common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(liveCM.Items) > 0
			}, tenSecondTimeout, time.Second).Should(BeTrue())

			cm := liveCM.Items[0]
			cm.Data = map[string]string{}
			err := apiClient.Update(ctx, &cm)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() bool {
				err := apiClient.Get(ctx, client.ObjectKeyFromObject(&cm), &cm)
				Expect(err).ToNot(HaveOccurred())
				return len(cm.Data) > 0
			}, tenSecondTimeout, time.Second).Should(BeTrue())
		})
	})
	Context("resource deletion when CR is deleted", func() {
		BeforeEach(func() {
			tto := strategy.GetTTO()
			apiClient.Delete(ctx, tto)
		})

		AfterEach(func() {
			strategy.CreateTTOIfNeeded()
		})

		It("[test_id:TODO]operator should delete pipelines", func() {
			livePipelines := &pipeline.PipelineList{}
			Eventually(func() bool {
				err := apiClient.List(ctx, livePipelines,
					client.MatchingLabels{
						common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(livePipelines.Items) == 0
			}, tenSecondTimeout, time.Second).Should(BeTrue(), "there should be no pipelines left")
		})

		It("[test_id:TODO]operator should delete role bindings", func() {
			liveRB := &rbac.RoleBindingList{}
			Eventually(func() bool {
				err := apiClient.List(ctx, liveRB,
					client.MatchingLabels{
						common.AppKubernetesComponentLabel: string(common.AppComponentTektonPipelines),
						common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(liveRB.Items) == 0
			}, tenSecondTimeout, time.Second).Should(BeTrue(), "there should be no role bindings left")
		})
	})
})
