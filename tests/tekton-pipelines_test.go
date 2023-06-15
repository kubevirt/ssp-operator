package tests

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	// pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	ssp "kubevirt.io/ssp-operator/api/v1beta2"
	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/tests/env"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Tekton Pipelines Operand", func() {
	Context("resource creation when DeployTektonTaskResources is set to true", func() {
		BeforeEach(func() {
			strategy.SkipSspUpdateTestsIfNeeded()

			updateSsp(func(foundSsp *ssp.SSP) {
				if foundSsp.Spec.FeatureGates == nil {
					foundSsp.Spec.FeatureGates = &ssp.FeatureGates{}
				}
				foundSsp.Spec.FeatureGates.DeployTektonTaskResources = true
			})

			waitUntilDeployed()
		})

		AfterEach(func() {
			strategy.RevertToOriginalSspCr()
		})

		// It("[test_id:TODO] should create pipelines", func() {
		// 	pipelineList := &pipeline.PipelineList{}

		// 	Eventually(func() bool {
		// 		err := apiClient.List(ctx, pipelineList,
		// 			client.MatchingLabels{
		// 				common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
		// 				common.AppKubernetesComponentLabel: string(common.AppComponentTektonPipelines),
		// 			},
		// 		)
		// 		Expect(err).ToNot(HaveOccurred())
		// 		return len(pipelineList.Items) > 0
		// 	}, env.ShortTimeout(), time.Second).Should(BeTrue())

		// 	for _, pipeline := range pipelineList.Items {
		// 		Expect(pipeline.Labels[common.AppKubernetesManagedByLabel]).To(Equal(common.AppKubernetesManagedByValue), "managed by label should equal")
		// 		Expect(pipeline.Labels[common.AppKubernetesComponentLabel]).To(Equal(string(common.AppComponentTektonPipelines)), "component label should equal")
		// 	}
		// })

		It("[test_id:TODO] should create role bindings", func() {
			roleBindingList := &rbac.RoleBindingList{}

			Eventually(func() bool {
				err := apiClient.List(ctx, roleBindingList,
					client.MatchingLabels{
						common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
						common.AppKubernetesComponentLabel: string(common.AppComponentTektonPipelines),
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(roleBindingList.Items) > 0
			}, env.ShortTimeout(), time.Second).Should(BeTrue())

			for _, roleBinding := range roleBindingList.Items {
				Expect(roleBinding.Labels[common.AppKubernetesManagedByLabel]).To(Equal(common.AppKubernetesManagedByValue), "managed by label should equal")
				Expect(roleBinding.Labels[common.AppKubernetesComponentLabel]).To(Equal(string(common.AppComponentTektonPipelines)), "component label should equal")
			}
		})

		It("[test_id:TODO] should not update pipeline SA when deployed in non openshift|kube namespace", func() {
			existingSA := &v1.ServiceAccount{}

			Eventually(func() bool {
				err := apiClient.Get(ctx, client.ObjectKey{Name: "pipeline", Namespace: strategy.GetNamespace()}, existingSA)
				Expect(err).ToNot(HaveOccurred())
				return existingSA != nil
			}, env.ShortTimeout(), time.Second).Should(BeTrue())

			Expect(existingSA.Annotations[common.AppKubernetesComponentLabel]).ToNot(Equal(common.AppComponentTektonPipelines))
		})

		It("[test_id:TODO] should create config maps", func() {
			configMapList := &v1.ConfigMapList{}

			Eventually(func() bool {
				err := apiClient.List(ctx, configMapList,
					client.MatchingLabels{
						common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
						common.AppKubernetesComponentLabel: string(common.AppComponentTektonPipelines),
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(configMapList.Items) > 0
			}, env.ShortTimeout(), time.Second).Should(BeTrue())

			for _, configMap := range configMapList.Items {
				Expect(configMap.Labels[common.AppKubernetesManagedByLabel]).To(Equal(common.AppKubernetesManagedByValue), "managed by label should equal")
				Expect(configMap.Labels[common.AppKubernetesComponentLabel]).To(Equal(string(common.AppComponentTektonPipelines)), "component label should equal")
			}
		})
	})

	Context("resource change", func() {
		BeforeEach(func() {
			strategy.SkipSspUpdateTestsIfNeeded()

			updateSsp(func(foundSsp *ssp.SSP) {
				if foundSsp.Spec.FeatureGates == nil {
					foundSsp.Spec.FeatureGates = &ssp.FeatureGates{}
				}

				foundSsp.Spec.FeatureGates.DeployTektonTaskResources = true
			})

			waitUntilDeployed()
		})

		AfterEach(func() {
			strategy.RevertToOriginalSspCr()
		})

		// It("[test_id:TODO] should revert user update on pipeline", func() {
		// 	pipelineList := &pipeline.PipelineList{}

		// 	Eventually(func() bool {
		// 		err := apiClient.List(ctx, pipelineList,
		// 			client.MatchingLabels{
		// 				common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
		// 				common.AppKubernetesComponentLabel: string(common.AppComponentTektonPipelines),
		// 			},
		// 		)
		// 		Expect(err).ToNot(HaveOccurred())
		// 		return len(pipelineList.Items) > 0
		// 	}, env.ShortTimeout(), time.Second).Should(BeTrue())

		// 	pipeline := pipelineList.Items[0]
		// 	pipeline.Spec.Description = "test"
		// 	err := apiClient.Update(ctx, &pipeline)
		// 	Expect(err).ToNot(HaveOccurred())

		// 	Eventually(func() bool {
		// 		err := apiClient.Get(ctx, client.ObjectKeyFromObject(&pipeline), &pipeline)
		// 		Expect(err).ToNot(HaveOccurred())
		// 		return pipeline.Spec.Description != "test"
		// 	}, env.ShortTimeout(), time.Second).Should(BeTrue())
		// })

		It("[test_id:TODO] should revert user update on configMap", func() {
			configMapList := &v1.ConfigMapList{}

			Eventually(func() bool {
				err := apiClient.List(ctx, configMapList,
					client.MatchingLabels{
						common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
						common.AppKubernetesComponentLabel: string(common.AppComponentTektonPipelines),
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(configMapList.Items) > 0
			}, env.ShortTimeout(), time.Second).Should(BeTrue())

			configMap := configMapList.Items[0]
			configMap.Data = map[string]string{}
			err := apiClient.Update(ctx, &configMap)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() bool {
				err := apiClient.Get(ctx, client.ObjectKeyFromObject(&configMap), &configMap)
				Expect(err).ToNot(HaveOccurred())
				return len(configMap.Data) > 0
			}, env.ShortTimeout(), time.Second).Should(BeTrue())
		})
	})
})
