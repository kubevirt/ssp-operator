package tests

import (
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	tekton "github.com/kubevirt/tekton-tasks-operator/api/v1alpha1"
	"github.com/kubevirt/tekton-tasks-operator/pkg/common"
	"github.com/kubevirt/tekton-tasks-operator/pkg/operands"
	tektontasks "github.com/kubevirt/tekton-tasks-operator/pkg/tekton-tasks"
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Tekton-tasks", func() {
	Context("resource creation when DeployTektonTaskResources is set to false", func() {
		BeforeEach(func() {
			tto := strategy.GetTTO()
			tto.Spec.FeatureGates.DeployTektonTaskResources = false
			createOrUpdateTekton(tto)
		})

		AfterEach(func() {
			tto := getTekton()
			deleteTekton(tto)
		})

		It("[test_id:TODO]operator should not create any tasks", func() {
			liveTasks := &pipeline.TaskList{}

			err := apiClient.List(ctx, liveTasks,
				client.MatchingLabels{
					common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
				},
			)
			Expect(err).ToNot(HaveOccurred())

			//Nothing to check, end the test successfully
			if len(liveTasks.Items) == 0 {
				return
			}

			err = apiClient.Delete(ctx, &liveTasks.Items[0])
			Expect(err).ToNot(HaveOccurred())

			deletedTask := &pipeline.Task{}
			Consistently(func() metav1.StatusReason {
				err := apiClient.Get(ctx, client.ObjectKeyFromObject(&liveTasks.Items[0]), deletedTask)
				Expect(err).To(HaveOccurred())
				return errors.ReasonForError(err)
			}, tenSecondTimeout, time.Second).Should(Equal(metav1.StatusReasonNotFound), "task should not exists")
		})

		It("[test_id:TODO]operator should not create any service accounts", func() {
			liveSA := &v1.ServiceAccountList{}

			err := apiClient.List(ctx, liveSA,
				client.MatchingLabels{
					common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
				},
			)
			Expect(err).ToNot(HaveOccurred())

			//Nothing to check, end the test successfully
			if len(liveSA.Items) == 0 {
				return
			}

			err = apiClient.Delete(ctx, &liveSA.Items[0])
			Expect(err).ToNot(HaveOccurred())

			deletedSA := &v1.ServiceAccount{}
			Consistently(func() metav1.StatusReason {
				err := apiClient.Get(ctx, client.ObjectKeyFromObject(&liveSA.Items[0]), deletedSA)
				Expect(err).To(HaveOccurred())
				return errors.ReasonForError(err)
			}, tenSecondTimeout, time.Second).Should(Equal(metav1.StatusReasonNotFound), "SA should not exists")
		})

		It("[test_id:TODO]operator should not create any cluster role", func() {
			liveCR := &rbac.ClusterRoleList{}

			err := apiClient.List(ctx, liveCR,
				client.MatchingLabels{
					common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
				},
			)
			Expect(err).ToNot(HaveOccurred())

			//Nothing to check, end the test successfully
			if len(liveCR.Items) == 0 {
				return
			}

			err = apiClient.Delete(ctx, &liveCR.Items[0])
			Expect(err).ToNot(HaveOccurred())

			deletedCR := &rbac.ClusterRole{}
			Consistently(func() metav1.StatusReason {
				err := apiClient.Get(ctx, client.ObjectKeyFromObject(&liveCR.Items[0]), deletedCR)
				Expect(err).To(HaveOccurred())
				return errors.ReasonForError(err)
			}, tenSecondTimeout, time.Second).Should(Equal(metav1.StatusReasonNotFound), "CR should not exists")
		})

		It("[test_id:TODO]operator should not create role bindings", func() {
			liveRB := &rbac.RoleBindingList{}

			err := apiClient.List(ctx, liveRB,
				client.MatchingLabels{
					common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
				},
			)
			Expect(err).ToNot(HaveOccurred())

			//Nothing to check, end the test successfully
			if len(liveRB.Items) == 0 {
				return
			}

			err = apiClient.Delete(ctx, &liveRB.Items[0])
			Expect(err).ToNot(HaveOccurred())

			deletedRB := &rbac.RoleBinding{}
			Consistently(func() metav1.StatusReason {
				err := apiClient.Get(ctx, client.ObjectKeyFromObject(&liveRB.Items[0]), deletedRB)
				Expect(err).To(HaveOccurred())
				return errors.ReasonForError(err)
			}, tenSecondTimeout, time.Second).Should(Equal(metav1.StatusReasonNotFound), "RB should not exists")
		})

	})
	Context("resource creation", func() {
		BeforeEach(func() {
			tto := strategy.GetTTO()
			tto.Spec.FeatureGates.DeployTektonTaskResources = true
			createOrUpdateTekton(tto)
			waitUntilDeployed()
		})

		AfterEach(func() {
			tto := getTekton()
			deleteTekton(tto)
		})

		It("[test_id:TODO]operator should create only allowed tekton-tasks with correct labels", func() {
			liveTasks := &pipeline.TaskList{}
			Eventually(func() bool {
				err := apiClient.List(ctx, liveTasks,
					client.MatchingLabels{
						common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(liveTasks.Items) > 0
			}, tenSecondTimeout, time.Second).Should(BeTrue())

			for _, task := range liveTasks.Items {
				if _, ok := tektontasks.AllowedTasks[strings.TrimSuffix(task.Name, "-task")]; !ok {
					Expect(ok).To(BeTrue(), "only allowed task is deployed")
				}
				Expect(task.Labels[tektontasks.TektonTasksVersionLabel]).To(Equal(operands.TektonTasksVersion), "version label should equal")
				Expect(task.Labels[common.AppKubernetesComponentLabel]).To(Equal(string(common.AppComponentTektonTasks)), "component label should equal")
				Expect(task.Labels[common.AppKubernetesManagedByLabel]).To(Equal(common.AppKubernetesManagedByValue), "managed by label should equal")
			}
		})

		It("[test_id:TODO]operator should create service accounts", func() {
			liveSA := &v1.ServiceAccountList{}
			Eventually(func() bool {
				err := apiClient.List(ctx, liveSA,
					client.MatchingLabels{
						common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
						common.AppKubernetesComponentLabel: string(common.AppComponentTektonTasks),
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(liveSA.Items) > 0
			}, tenSecondTimeout, time.Second).Should(BeTrue())
			for _, sa := range liveSA.Items {
				if _, ok := tektontasks.AllowedTasks[strings.TrimSuffix(sa.Name, "-task")]; !ok {
					Expect(ok).To(BeTrue(), "only allowed service account is deployed - "+sa.Name)
				}
				Expect(sa.Labels[common.AppKubernetesComponentLabel]).To(Equal(string(common.AppComponentTektonTasks)), "component label should equal")
				Expect(sa.Labels[common.AppKubernetesManagedByLabel]).To(Equal(common.AppKubernetesManagedByValue), "managed by label should equal")
			}
		})

		It("[test_id:TODO]operator should create cluster role", func() {
			liveCR := &rbac.ClusterRoleList{}
			clusterRoleName := "windows-pipelines"
			Eventually(func() bool {
				err := apiClient.List(ctx, liveCR,
					client.MatchingLabels{
						common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(liveCR.Items) > 0
			}, tenSecondTimeout, time.Second).Should(BeTrue())
			for _, cr := range liveCR.Items {
				if _, ok := tektontasks.AllowedTasks[strings.TrimSuffix(cr.Name, "-task")]; !ok {
					if ok = cr.Name != clusterRoleName; ok {
						Expect(ok).To(BeTrue(), "only allowed cluster role is deployed - "+cr.Name)
					}
				}

				if cr.Name == clusterRoleName {
					Expect(cr.Labels[common.AppKubernetesComponentLabel]).To(Equal(string(common.AppComponentTektonPipelines)), "component label should equal")
				} else {
					Expect(cr.Labels[common.AppKubernetesComponentLabel]).To(Equal(string(common.AppComponentTektonTasks)), "component label should equal")
				}
				Expect(cr.Labels[common.AppKubernetesManagedByLabel]).To(Equal(common.AppKubernetesManagedByValue), "managed by label should equal")
			}
		})

		It("[test_id:TODO]operator should create role bindings", func() {
			liveRB := &rbac.RoleBindingList{}
			Eventually(func() bool {
				err := apiClient.List(ctx, liveRB,
					client.MatchingLabels{
						common.AppKubernetesComponentLabel: string(common.AppComponentTektonTasks),
						common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(liveRB.Items) > 0
			}, tenSecondTimeout, time.Second).Should(BeTrue())

			for _, rb := range liveRB.Items {
				if _, ok := tektontasks.AllowedTasks[strings.TrimSuffix(rb.Name, "-task")]; !ok {
					Expect(ok).To(BeTrue(), "only allowed role binding is deployed - "+rb.Name)
				}
				Expect(rb.Labels[common.AppKubernetesManagedByLabel]).To(Equal(common.AppKubernetesManagedByValue), "managed by label should equal")
			}
		})
	})

	Context("resource deletion when CR is deleted", func() {
		BeforeEach(func() {
			strategy.CreateTTOIfNeeded()
			waitUntilDeployed()
		})

		It("[test_id:TODO]operator should delete tekton-tasks", func() {
			tto := getTekton()
			deleteTekton(tto)
			liveTasks := &pipeline.TaskList{}
			Eventually(func() bool {
				err := apiClient.List(ctx, liveTasks,
					client.MatchingLabels{
						common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(liveTasks.Items) == 0
			}, tenSecondTimeout, time.Second).Should(BeTrue(), "there should be no tasks left")
		})

		It("[test_id:TODO]operator should delete service accounts", func() {
			tto := getTekton()
			deleteTekton(tto)
			liveSA := &v1.ServiceAccountList{}
			Eventually(func() bool {
				err := apiClient.List(ctx, liveSA,
					client.MatchingLabels{
						common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(liveSA.Items) == 0
			}, tenSecondTimeout, time.Second).Should(BeTrue(), "there should be no service accounts left")
		})

		It("[test_id:TODO]operator should delete cluster role", func() {
			tto := getTekton()
			deleteTekton(tto)
			liveCR := &rbac.ClusterRoleList{}
			Eventually(func() bool {
				err := apiClient.List(ctx, liveCR,
					client.MatchingLabels{
						common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(liveCR.Items) == 0
			}, tenSecondTimeout, time.Second).Should(BeTrue(), "there should be no cluster roles left")

		})

		It("[test_id:TODO]operator should delete role bindings", func() {
			tto := getTekton()
			deleteTekton(tto)
			liveRB := &rbac.RoleBindingList{}
			Eventually(func() bool {
				err := apiClient.List(ctx, liveRB,
					client.MatchingLabels{
						common.AppKubernetesComponentLabel: string(common.AppComponentTektonTasks),
						common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(liveRB.Items) == 0
			}, tenSecondTimeout, time.Second).Should(BeTrue(), "there should be no role bindings left")
		})
	})
	Context("multiple CRs deployed", func() {
		BeforeEach(func() {
			strategy.createTekton("tto-test-1")
			strategy.createTekton("tto-test-2")

			Eventually(func() bool {
				ttos := tekton.TektonTasksList{}
				err := apiClient.List(ctx, &ttos)
				Expect(err).ToNot(HaveOccurred())
				return len(ttos.Items) == 2
			}, 60*time.Second, 2*time.Second).Should(BeTrue(), "there should be 2 CRs")
		})

		AfterEach(func() {
			tektonTasksCRList := &tekton.TektonTasksList{}
			Expect(apiClient.List(ctx, tektonTasksCRList)).To(Succeed())

			for _, tekton := range tektonTasksCRList.Items {
				deleteTekton(&tekton)
			}
		})

		It("check if correct status is set for TTO", func() {
			tektonTasksCRList := &tekton.TektonTasksList{}
			Eventually(func() bool {
				apiClient.List(ctx, tektonTasksCRList)

				for _, item := range tektonTasksCRList.Items {
					for _, condition := range item.Status.Conditions {
						if condition.Type == conditionsv1.ConditionAvailable && condition.Status == v1.ConditionTrue {
							return false
						}
						if condition.Type == conditionsv1.ConditionProgressing && condition.Status == v1.ConditionTrue {
							return false
						}
						if condition.Type == conditionsv1.ConditionDegraded && condition.Status == v1.ConditionFalse {
							return false
						}
						if condition.Message != "there are multiple CRs deployed" {
							return false
						}
					}
				}
				return true
			}, 60*time.Second, 2*time.Second).Should(BeTrue(), "conditions should have correct type, status and message")
		})
	})
})
