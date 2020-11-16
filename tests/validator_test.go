package tests

import (
	"reflect"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
	admission "k8s.io/api/admissionregistration/v1"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	lifecycleapi "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sspv1beta1 "kubevirt.io/ssp-operator/api/v1beta1"
	validator "kubevirt.io/ssp-operator/internal/operands/template-validator"
)

var _ = Describe("Template validator", func() {
	var (
		clusterRoleRes        testResource
		clusterRoleBindingRes testResource
		webhookConfigRes      testResource
		serviceAccountRes     testResource
		serviceRes            testResource
		deploymentRes         testResource

		replicas int32 = 2
	)

	BeforeEach(func() {
		clusterRoleRes = testResource{
			Name:     validator.ClusterRoleName,
			Resource: &rbac.ClusterRole{},
			UpdateFunc: func(role *rbac.ClusterRole) {
				role.Rules[0].Verbs = []string{"watch"}
			},
			EqualsFunc: func(old *rbac.ClusterRole, new *rbac.ClusterRole) bool {
				return reflect.DeepEqual(old.Rules, new.Rules)
			},
		}
		clusterRoleBindingRes = testResource{
			Name:     validator.ClusterRoleBindingName,
			Resource: &rbac.ClusterRoleBinding{},
			UpdateFunc: func(roleBinding *rbac.ClusterRoleBinding) {
				roleBinding.Subjects = nil
			},
			EqualsFunc: func(old *rbac.ClusterRoleBinding, new *rbac.ClusterRoleBinding) bool {
				return reflect.DeepEqual(old.RoleRef, new.RoleRef) &&
					reflect.DeepEqual(old.Subjects, new.Subjects)
			},
		}
		webhookConfigRes = testResource{
			Name:     validator.WebhookName,
			Resource: &admission.ValidatingWebhookConfiguration{},
			UpdateFunc: func(webhook *admission.ValidatingWebhookConfiguration) {
				webhook.Webhooks[0].Rules = nil
			},
			EqualsFunc: func(old *admission.ValidatingWebhookConfiguration, new *admission.ValidatingWebhookConfiguration) bool {
				return reflect.DeepEqual(old.Webhooks, new.Webhooks)
			},
		}
		serviceAccountRes = testResource{
			Name:      validator.ServiceAccountName,
			Namespace: strategy.GetNamespace(),
			Resource:  &core.ServiceAccount{},
		}
		serviceRes = testResource{
			Name:      validator.ServiceName,
			Namespace: strategy.GetNamespace(),
			Resource:  &core.Service{},
			UpdateFunc: func(service *core.Service) {
				service.Spec.Ports[0].Port = 44331
				service.Spec.Ports[0].TargetPort = intstr.FromInt(44331)
			},
			EqualsFunc: func(old *core.Service, new *core.Service) bool {
				return reflect.DeepEqual(old.Spec, new.Spec)
			},
		}
		deploymentRes = testResource{
			Name:      validator.DeploymentName,
			Namespace: strategy.GetNamespace(),
			Resource:  &apps.Deployment{},
			UpdateFunc: func(deployment *apps.Deployment) {
				deployment.Spec.Replicas = pointer.Int32Ptr(0)
			},
			EqualsFunc: func(old *apps.Deployment, new *apps.Deployment) bool {
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
			table.Entry("[test_id:4907] cluster role", &clusterRoleRes),
			table.Entry("[test_id:4908] cluster role binding", &clusterRoleBindingRes),
			table.Entry("[test_id:4909] validating webhook configuration", &webhookConfigRes),
		)

		table.DescribeTable("created namespaced resource", func(res *testResource) {
			err := apiClient.Get(ctx, res.GetKey(), res.NewResource())
			Expect(err).ToNot(HaveOccurred())
		},
			table.Entry("[test_id:4910] service account", &serviceAccountRes),
			table.Entry("[test_id:4911] service", &serviceRes),
			table.Entry("[test_id:4912] deployment", &deploymentRes),
		)
	})

	Context("resource deletion", func() {
		table.DescribeTable("recreate after delete", expectRecreateAfterDelete,
			table.Entry("[test_id:4914] cluster role", &clusterRoleRes),
			table.Entry("[test_id:4916] cluster role binding", &clusterRoleBindingRes),
			table.Entry("[test_id:4918] validating webhook configuration", &webhookConfigRes),
			table.Entry("[test_id:4920] service account", &serviceAccountRes),
			table.Entry("[test_id:4922] service", &serviceRes),
			table.Entry("[test_id:4924] deployment", &deploymentRes),
		)
	})

	Context("resource change", func() {
		table.DescribeTable("should restore modified resource", expectRestoreAfterUpdate,
			table.Entry("[test_id:4915] cluster role", &clusterRoleRes),
			table.Entry("[test_id:4917] cluster role binding", &clusterRoleBindingRes),
			table.Entry("[test_id:4919] validating webhook configuration", &webhookConfigRes),
			table.Entry("[test_id:4923] service", &serviceRes),
			table.Entry("[test_id:4925] deployment", &deploymentRes),
		)

		Context("with pause", func() {
			BeforeEach(func() {
				strategy.SkipSspUpdateTestsIfNeeded()
			})

			JustAfterEach(func() {
				unpauseSsp()
			})

			table.DescribeTable("should restore modified resource with pause", expectRestoreAfterUpdateWithPause,
				table.Entry("[test_id:5534] cluster role", &clusterRoleRes),
				table.Entry("[test_id:5535] cluster role binding", &clusterRoleBindingRes),
				table.Entry("[test_id:5536] validating webhook configuration", &webhookConfigRes),
				table.Entry("[test_id:5538] service", &serviceRes),
				table.Entry("[test_id:5539] deployment", &deploymentRes),
			)
		})
	})

	It("[test_id:4913] should successfully start template-validator pod", func() {
		labels := map[string]string{"kubevirt.io": "virt-template-validator"}
		Eventually(func() bool {
			pods := core.PodList{}
			err := apiClient.List(ctx, &pods,
				client.InNamespace(strategy.GetNamespace()),
				client.MatchingLabels(labels))
			Expect(err).ToNot(HaveOccurred())

			runningCount := 0
			for _, pod := range pods.Items {
				if pod.Status.Phase == core.PodRunning {
					runningCount++
				}
			}
			return runningCount == strategy.GetValidatorReplicas()
		}, timeout, time.Second).Should(BeTrue())
	})

	It("should set Deployed phase and conditions when validator pods are running", func() {
		foundSsp := getSsp()

		Expect(foundSsp.Status.Phase).To(Equal(lifecycleapi.PhaseDeployed))

		conditions := foundSsp.Status.Conditions
		Expect(conditionsv1.FindStatusCondition(conditions, conditionsv1.ConditionAvailable).Status).To(Equal(core.ConditionTrue))
		Expect(conditionsv1.FindStatusCondition(conditions, conditionsv1.ConditionProgressing).Status).To(Equal(core.ConditionFalse))
		Expect(conditionsv1.FindStatusCondition(conditions, conditionsv1.ConditionDegraded).Status).To(Equal(core.ConditionFalse))

		deployment := &apps.Deployment{}
		Expect(apiClient.Get(ctx, deploymentRes.GetKey(), deployment)).ToNot(HaveOccurred())
		Expect(deployment.Status.AvailableReplicas).To(Equal(int32(strategy.GetValidatorReplicas())))
	})

	Context("with SSP resource modification", func() {
		BeforeEach(func() {
			strategy.SkipSspUpdateTestsIfNeeded()
		})

		AfterEach(func() {
			strategy.RevertToOriginalSspCr()
			waitUntilDeployed()
		})

		It("[test_id:4926] should add and remove placement", func() {
			const testKey = "testKey"
			const testValue = "testValue"

			affinity := &core.Affinity{
				NodeAffinity: &core.NodeAffinity{
					PreferredDuringSchedulingIgnoredDuringExecution: []core.PreferredSchedulingTerm{{
						Preference: core.NodeSelectorTerm{
							MatchExpressions: []core.NodeSelectorRequirement{{
								Key:      testKey,
								Operator: core.NodeSelectorOpIn,
								Values:   []string{testValue},
							}},
						},
						Weight: 1,
					}},
				},
			}

			nodeSelector := map[string]string{
				testKey: testValue,
			}

			tolerations := []core.Toleration{{
				Key:      testKey,
				Operator: core.TolerationOpExists,
				Effect:   core.TaintEffectNoExecute,
			}}

			waitUntilDeployed()

			updateSsp(func(foundSsp *sspv1beta1.SSP) {
				foundSsp.Spec.TemplateValidator.Placement = &lifecycleapi.NodePlacement{
					Affinity:     affinity,
					NodeSelector: nodeSelector,
					Tolerations:  tolerations,
				}
			})

			waitUntilDeployed()

			// Test that placement was added
			Eventually(func() bool {
				deployment := apps.Deployment{}
				key := deploymentRes.GetKey()
				Expect(apiClient.Get(ctx, key, &deployment)).ToNot(HaveOccurred())
				podSpec := &deployment.Spec.Template.Spec
				return reflect.DeepEqual(podSpec.Affinity, affinity) &&
					reflect.DeepEqual(podSpec.NodeSelector, nodeSelector) &&
					reflect.DeepEqual(podSpec.Tolerations, tolerations)
			}, timeout, 1*time.Second).Should(BeTrue())

			updateSsp(func(foundSsp *sspv1beta1.SSP) {
				placement := foundSsp.Spec.TemplateValidator.Placement
				placement.Affinity = nil
				placement.NodeSelector = nil
				placement.Tolerations = nil
			})

			waitUntilDeployed()

			// Test that placement was removed
			Eventually(func() bool {
				deployment := apps.Deployment{}
				key := deploymentRes.GetKey()
				Expect(apiClient.Get(ctx, key, &deployment)).ToNot(HaveOccurred())
				podSpec := &deployment.Spec.Template.Spec
				return podSpec.Affinity == nil &&
					podSpec.NodeSelector == nil &&
					podSpec.Tolerations == nil
			}, timeout, 1*time.Second).Should(BeTrue())
		})

		// TODO - This test is currently pending, because it can be flaky.
		//        If the operator is too slow and does not notice Deployment
		//        state when not all pods are running, the test would fail.
		PIt("[test_id: TODO]should set available condition when at least one validator pod is running", func() {
			watch, err := StartWatch(sspListerWatcher)
			Expect(err).ToNot(HaveOccurred())
			defer watch.Stop()

			updateSsp(func(foundSsp *sspv1beta1.SSP) {
				foundSsp.Spec.TemplateValidator.Replicas = pointer.Int32Ptr(replicas)
			})

			err = WatchChangesUntil(watch, isStatusDeploying, timeout)
			Expect(err).ToNot(HaveOccurred(), "SSP status should be deploying.")

			err = WatchChangesUntil(watch, func(obj *sspv1beta1.SSP) bool {
				available := conditionsv1.FindStatusCondition(obj.Status.Conditions, conditionsv1.ConditionAvailable)
				progressing := conditionsv1.FindStatusCondition(obj.Status.Conditions, conditionsv1.ConditionProgressing)

				return obj.Status.Phase == lifecycleapi.PhaseDeploying &&
					available.Status == core.ConditionTrue &&
					progressing.Status == core.ConditionTrue
			}, timeout)
			Expect(err).ToNot(HaveOccurred(), "SSP should be available, but progressing.")

			err = WatchChangesUntil(watch, isStatusDeployed, timeout)
			Expect(err).ToNot(HaveOccurred(), "SSP status should be deployed.")
		})
	})
})
