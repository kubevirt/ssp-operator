package tests

import (
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	ssp "kubevirt.io/ssp-operator/api/v1beta1"
	"kubevirt.io/ssp-operator/internal/common"
	tekton_tasks "kubevirt.io/ssp-operator/internal/operands/tekton-tasks"
	"kubevirt.io/ssp-operator/tests/env"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Tekton Tasks Operand", func() {
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

		It("[test_id:TODO] should create tasks", func() {
			taskList := &pipeline.TaskList{}

			Eventually(func() bool {
				err := apiClient.List(ctx, taskList,
					client.MatchingLabels{
						common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
						common.AppKubernetesComponentLabel: string(common.AppComponentTektonTasks),
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(taskList.Items) > 0
			}, env.ShortTimeout(), time.Second).Should(BeTrue())

			for _, task := range taskList.Items {
				if _, found := tekton_tasks.AllowedTasks[strings.TrimSuffix(task.Name, "-task")]; found {
					Expect(found).To(BeTrue(), "only allowed task is deployed - "+task.Name)
				}

				Expect(task.Labels[tekton_tasks.TektonTasksVersionLabel]).To(Equal(common.TektonTasksVersion), "version label should equal")
				Expect(task.Labels[common.AppKubernetesManagedByLabel]).To(Equal(common.AppKubernetesManagedByValue), "managed by label should equal")
				Expect(task.Labels[common.AppKubernetesComponentLabel]).To(Equal(string(common.AppComponentTektonTasks)), "component label should equal")
			}
		})

		It("[test_id:TODO] should create service accounts", func() {
			serviceAccountList := &v1.ServiceAccountList{}

			Eventually(func() bool {
				err := apiClient.List(ctx, serviceAccountList,
					client.MatchingLabels{
						common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
						common.AppKubernetesComponentLabel: string(common.AppComponentTektonTasks),
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(serviceAccountList.Items) > 0
			}, env.ShortTimeout(), time.Second).Should(BeTrue())

			for _, serviceAccount := range serviceAccountList.Items {
				if _, found := tekton_tasks.AllowedTasks[strings.TrimSuffix(serviceAccount.Name, "-task")]; found {
					Expect(found).To(BeTrue(), "only allowed service account is deployed - "+serviceAccount.Name)
				}

				Expect(serviceAccount.Labels[common.AppKubernetesManagedByLabel]).To(Equal(common.AppKubernetesManagedByValue), "managed by label should equal")
				Expect(serviceAccount.Labels[common.AppKubernetesComponentLabel]).To(Equal(string(common.AppComponentTektonTasks)), "component label should equal")
			}
		})

		It("[test_id:TODO] should create cluster roles", func() {
			clusterRoleList := &rbac.ClusterRoleList{}

			Eventually(func() bool {
				err := apiClient.List(ctx, clusterRoleList,
					client.MatchingLabels{
						common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
						common.AppKubernetesComponentLabel: string(common.AppComponentTektonTasks),
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(clusterRoleList.Items) > 0
			}, env.ShortTimeout(), time.Second).Should(BeTrue())

			for _, clusterRole := range clusterRoleList.Items {
				if _, found := tekton_tasks.AllowedTasks[strings.TrimSuffix(clusterRole.Name, "-task")]; found {
					Expect(found).To(BeTrue(), "only allowed cluster role is deployed - "+clusterRole.Name)
				}

				Expect(clusterRole.Labels[common.AppKubernetesManagedByLabel]).To(Equal(common.AppKubernetesManagedByValue), "managed by label should equal")
				Expect(clusterRole.Labels[common.AppKubernetesComponentLabel]).To(Equal(string(common.AppComponentTektonTasks)), "component label should equal")
			}
		})

		It("[test_id:TODO] should create role bindings", func() {
			roleBindingList := &rbac.RoleBindingList{}

			Eventually(func() bool {
				err := apiClient.List(ctx, roleBindingList,
					client.MatchingLabels{
						common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
						common.AppKubernetesComponentLabel: string(common.AppComponentTektonTasks),
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(roleBindingList.Items) > 0
			}, env.ShortTimeout(), time.Second).Should(BeTrue())

			for _, roleBinding := range roleBindingList.Items {
				if _, found := tekton_tasks.AllowedTasks[strings.TrimSuffix(roleBinding.Name, "-task")]; found {
					Expect(found).To(BeTrue(), "only allowed role binding is deployed - "+roleBinding.Name)
				}

				Expect(roleBinding.Labels[common.AppKubernetesManagedByLabel]).To(Equal(common.AppKubernetesManagedByValue), "managed by label should equal")
				Expect(roleBinding.Labels[common.AppKubernetesComponentLabel]).To(Equal(string(common.AppComponentTektonTasks)), "component label should equal")
			}
		})
	})
})
