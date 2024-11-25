package tests

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"reflect"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	templatev1 "github.com/openshift/api/template/v1"
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
	admission "k8s.io/api/admissionregistration/v1"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	kubevirtv1 "kubevirt.io/api/core/v1"
	lifecycleapi "kubevirt.io/controller-lifecycle-operator-sdk/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ssp "kubevirt.io/ssp-operator/api/v1beta2"
	"kubevirt.io/ssp-operator/internal/common"
	validator "kubevirt.io/ssp-operator/internal/operands/template-validator"
	"kubevirt.io/ssp-operator/internal/template-validator/labels"
	"kubevirt.io/ssp-operator/tests/env"
)

func testDeploymentResource() testResource {
	return testResource{
		Name:           validator.DeploymentName,
		Namespace:      strategy.GetNamespace(),
		Resource:       &apps.Deployment{},
		ExpectedLabels: expectedLabelsFor("template-validator", common.AppComponentTemplating),
		UpdateFunc: func(deployment *apps.Deployment) {
			deployment.Spec.Replicas = ptr.To[int32](0)
		},
		EqualsFunc: func(old *apps.Deployment, new *apps.Deployment) bool {
			return reflect.DeepEqual(old.Spec, new.Spec)
		},
	}
}

var _ = Describe("Template validator operand", func() {
	var (
		clusterRoleRes        testResource
		clusterRoleBindingRes testResource
		webhookConfigRes      testResource
		serviceAccountRes     testResource
		serviceRes            testResource
		configMapRes          testResource
		serviceMetricsRes     testResource
		deploymentRes         testResource

		replicas int32 = 2
	)

	BeforeEach(func() {
		expectedLabels := expectedLabelsFor("template-validator", common.AppComponentTemplating)
		clusterRoleRes = testResource{
			Name:           validator.ClusterRoleName,
			Resource:       &rbac.ClusterRole{},
			ExpectedLabels: expectedLabels,
			UpdateFunc: func(role *rbac.ClusterRole) {
				role.Rules[0].Verbs = []string{"watch"}
			},
			EqualsFunc: func(old *rbac.ClusterRole, new *rbac.ClusterRole) bool {
				return reflect.DeepEqual(old.Rules, new.Rules)
			},
		}
		clusterRoleBindingRes = testResource{
			Name:           validator.ClusterRoleBindingName,
			Resource:       &rbac.ClusterRoleBinding{},
			ExpectedLabels: expectedLabels,
			UpdateFunc: func(roleBinding *rbac.ClusterRoleBinding) {
				roleBinding.Subjects = nil
			},
			EqualsFunc: func(old *rbac.ClusterRoleBinding, new *rbac.ClusterRoleBinding) bool {
				return reflect.DeepEqual(old.RoleRef, new.RoleRef) &&
					reflect.DeepEqual(old.Subjects, new.Subjects)
			},
		}
		webhookConfigRes = testResource{
			Name:           validator.WebhookName,
			Resource:       &admission.ValidatingWebhookConfiguration{},
			ExpectedLabels: expectedLabels,
			UpdateFunc: func(webhook *admission.ValidatingWebhookConfiguration) {
				webhook.Webhooks[0].Rules = nil
			},
			EqualsFunc: func(old *admission.ValidatingWebhookConfiguration, new *admission.ValidatingWebhookConfiguration) bool {
				return reflect.DeepEqual(old.Webhooks, new.Webhooks)
			},
		}
		serviceAccountRes = testResource{
			Name:           validator.ServiceAccountName,
			Namespace:      strategy.GetNamespace(),
			Resource:       &core.ServiceAccount{},
			ExpectedLabels: expectedLabels,
		}
		serviceRes = testResource{
			Name:           validator.ServiceName,
			Namespace:      strategy.GetNamespace(),
			Resource:       &core.Service{},
			ExpectedLabels: expectedLabels,
			UpdateFunc: func(service *core.Service) {
				service.Spec.Ports[0].Port = 44331
				service.Spec.Ports[0].TargetPort = intstr.FromInt(44331)
			},
			EqualsFunc: func(old *core.Service, new *core.Service) bool {
				return reflect.DeepEqual(old.Spec, new.Spec)
			},
		}
		configMapRes = testResource{
			Name:           validator.ConfigMapName,
			Namespace:      strategy.GetNamespace(),
			Resource:       &core.ConfigMap{},
			ExpectedLabels: expectedLabels,
			UpdateFunc: func(configMap *core.ConfigMap) {
				configMap.Data = nil
			},
			EqualsFunc: func(old *core.ConfigMap, new *core.ConfigMap) bool {
				return reflect.DeepEqual(old.Immutable, new.Immutable) &&
					reflect.DeepEqual(old.Data, new.Data) &&
					reflect.DeepEqual(old.BinaryData, new.BinaryData)
			},
		}
		serviceMetricsRes = testResource{
			Name:           validator.MetricsServiceName,
			Namespace:      strategy.GetNamespace(),
			Resource:       &core.Service{},
			ExpectedLabels: validator.PrometheusServiceLabels(),
			UpdateFunc: func(service *core.Service) {
				service.Spec.Ports[0].Port = 443
				service.Spec.Ports[0].TargetPort = intstr.FromInt(8443)
			},
			EqualsFunc: func(old *core.Service, new *core.Service) bool {
				return reflect.DeepEqual(old.Spec, new.Spec)
			},
		}
		deploymentRes = testDeploymentResource()

		waitUntilDeployed()
	})

	Context("resource creation", func() {
		DescribeTable("created cluster resource", func(res *testResource) {
			resource := res.NewResource()
			err := apiClient.Get(ctx, res.GetKey(), resource)
			Expect(err).ToNot(HaveOccurred())
			Expect(hasOwnerAnnotations(resource.GetAnnotations())).To(BeTrue(), "owner annotations are missing")
		},
			Entry("[test_id:4907] cluster role", &clusterRoleRes),
			Entry("[test_id:4908] cluster role binding", &clusterRoleBindingRes),
			Entry("[test_id:4909] validating webhook configuration", &webhookConfigRes),
		)

		DescribeTable("created namespaced resource", func(res *testResource) {
			err := apiClient.Get(ctx, res.GetKey(), res.NewResource())
			Expect(err).ToNot(HaveOccurred())
		},
			Entry("[test_id:4910] service account", &serviceAccountRes),
			Entry("[test_id:4911] service", &serviceRes),
			Entry("[test_id:TODO] ConfigMap", &configMapRes),
			Entry("[test_id:8366] metrics service", &serviceMetricsRes),
			Entry("[test_id:4912] deployment", &deploymentRes),
		)

		DescribeTable("should set app labels", expectAppLabels,
			Entry("[test_id:5824]cluster role", &clusterRoleRes),
			Entry("[test_id:5825]cluster role binding", &clusterRoleBindingRes),
			Entry("[test_id:5826]validating webhook configuration", &webhookConfigRes),
			Entry("[test_id:6201]service account", &serviceAccountRes),
			Entry("[test_id:5827]service", &serviceRes),
			Entry("[test_id:TODO]ConfigMap", &configMapRes),
			Entry("[test_id:8367]metrics service", &serviceMetricsRes),
			Entry("[test_id:5828]deployment", &deploymentRes),
		)
	})

	Context("resource deletion", func() {
		DescribeTable("recreate after delete", expectRecreateAfterDelete,
			Entry("[test_id:4914] cluster role", &clusterRoleRes),
			Entry("[test_id:4916] cluster role binding", &clusterRoleBindingRes),
			Entry("[test_id:4918] validating webhook configuration", &webhookConfigRes),
			Entry("[test_id:4920] service account", &serviceAccountRes),
			Entry("[test_id:4922] service", &serviceRes),
			Entry("[test_id:TODO] ConfigMap", &configMapRes),
			Entry("[test_id:8370] metrics service", &serviceMetricsRes),
			Entry("[test_id:4924] deployment", &deploymentRes),
		)
	})

	Context("resource change", func() {
		DescribeTable("should restore modified resource", expectRestoreAfterUpdate,
			Entry("[test_id:4915] cluster role", &clusterRoleRes),
			Entry("[test_id:4917] cluster role binding", &clusterRoleBindingRes),
			Entry("[test_id:4919] validating webhook configuration", &webhookConfigRes),
			Entry("[test_id:4923] service", &serviceRes),
			Entry("[test_id:TODO] ConfigMap", &configMapRes),
			Entry("[test_id:8371] metrics service", &serviceMetricsRes),
			Entry("[test_id:4925] deployment", &deploymentRes),
		)

		Context("with pause", func() {
			BeforeEach(func() {
				strategy.SkipSspUpdateTestsIfNeeded()
			})

			JustAfterEach(func() {
				unpauseSsp()
			})

			DescribeTable("should restore modified resource with pause", expectRestoreAfterUpdateWithPause,
				Entry("[test_id:5534] cluster role", &clusterRoleRes),
				Entry("[test_id:5535] cluster role binding", &clusterRoleBindingRes),
				Entry("[test_id:5536] validating webhook configuration", &webhookConfigRes),
				Entry("[test_id:5538] service", &serviceRes),
				Entry("[test_id:TODO] ConfigMap", &configMapRes),
				Entry("[test_id:8368] metrics service", &serviceMetricsRes),
				Entry("[test_id:5539] deployment", &deploymentRes),
			)
		})

		DescribeTable("should restore modified app labels", expectAppLabelsRestoreAfterUpdate,
			Entry("[test_id:6205] cluster role", &clusterRoleRes),
			Entry("[test_id:6206] cluster role binding", &clusterRoleBindingRes),
			Entry("[test_id:6207] validating webhook configuration", &webhookConfigRes),
			Entry("[test_id:6208] service", &serviceRes),
			Entry("[test_id:TODO] ConfigMap", &configMapRes),
			Entry("[test_id:8369] metrics service", &serviceMetricsRes),
			Entry("[test_id:6209] deployment", &deploymentRes),
		)
	})

	It("[test_id:4913] should successfully start template-validator pod", func() {
		strategy.SkipUnlessHighlyAvailableTopologyMode()
		labels := map[string]string{"kubevirt.io": "virt-template-validator"}
		Eventually(func() int {
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
			return runningCount
		}, env.Timeout(), time.Second).Should(BeNumerically("==", strategy.GetValidatorReplicas()))
	})

	It("[test_id:6204]should set Deployed phase and conditions when validator pods are running", func() {
		foundSsp := getSsp()

		Expect(foundSsp.Status.Phase).To(Equal(lifecycleapi.PhaseDeployed), "SSP should be in phase Deployed")

		conditions := foundSsp.Status.Conditions
		Expect(conditionsv1.FindStatusCondition(conditions, conditionsv1.ConditionAvailable).Status).To(Equal(core.ConditionTrue),
			fmt.Sprintf("Condition '%s' should be '%s'.", conditionsv1.ConditionAvailable, core.ConditionTrue),
		)
		Expect(conditionsv1.FindStatusCondition(conditions, conditionsv1.ConditionProgressing).Status).To(Equal(core.ConditionFalse),
			fmt.Sprintf("Condition '%s' should be '%s'.", conditionsv1.ConditionProgressing, core.ConditionFalse),
		)
		Expect(conditionsv1.FindStatusCondition(conditions, conditionsv1.ConditionDegraded).Status).To(Equal(core.ConditionFalse),
			fmt.Sprintf("Condition '%s' should be '%s'.", conditionsv1.ConditionDegraded, core.ConditionFalse),
		)

		deployment := &apps.Deployment{}
		Expect(apiClient.Get(ctx, deploymentRes.GetKey(), deployment)).ToNot(HaveOccurred())
		strategy.SkipUnlessHighlyAvailableTopologyMode()
		Expect(deployment.Status.AvailableReplicas).To(Equal(int32(strategy.GetValidatorReplicas())), "deployment available replicas")
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
			defaultPodAntiAffinity := &core.PodAntiAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []core.WeightedPodAffinityTerm{
					{
						Weight: 1,
						PodAffinityTerm: core.PodAffinityTerm{
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      "kubevirt.io",
										Operator: metav1.LabelSelectorOpIn,
										Values:   []string{"virt-template-validator"},
									},
								},
							},
							TopologyKey: "kubernetes.io/hostname",
						},
					},
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

			updateSsp(func(foundSsp *ssp.SSP) {
				foundSsp.Spec.TemplateValidator = &ssp.TemplateValidator{
					Placement: &lifecycleapi.NodePlacement{
						Affinity:     affinity,
						NodeSelector: nodeSelector,
						Tolerations:  tolerations,
					},
				}
			})

			waitUntilDeployed()

			// Test that placement nodeAffinity, nodeSelector and Toleration was added
			Eventually(func() bool {
				deployment := apps.Deployment{}
				key := deploymentRes.GetKey()
				Expect(apiClient.Get(ctx, key, &deployment)).ToNot(HaveOccurred())
				podSpec := &deployment.Spec.Template.Spec
				return reflect.DeepEqual(podSpec.Affinity.NodeAffinity, affinity.NodeAffinity) &&
					reflect.DeepEqual(podSpec.Affinity.PodAntiAffinity, defaultPodAntiAffinity) &&
					reflect.DeepEqual(podSpec.NodeSelector, nodeSelector) &&
					reflect.DeepEqual(podSpec.Tolerations, tolerations)
			}, env.Timeout(), 1*time.Second).Should(BeTrue(), "placement is different")

			updateSsp(func(foundSsp *ssp.SSP) {
				foundSsp.Spec.TemplateValidator = &ssp.TemplateValidator{
					Placement: &lifecycleapi.NodePlacement{
						Affinity:     nil,
						NodeSelector: nil,
						Tolerations:  nil,
					},
				}
			})

			waitUntilDeployed()

			// Test that placement nodeAffinity, nodeSelector and Toleration was removed
			Eventually(func() bool {
				deployment := apps.Deployment{}
				key := deploymentRes.GetKey()
				Expect(apiClient.Get(ctx, key, &deployment)).ToNot(HaveOccurred())
				podSpec := &deployment.Spec.Template.Spec
				return reflect.DeepEqual(podSpec.Affinity.PodAntiAffinity, defaultPodAntiAffinity) &&
					podSpec.Affinity.NodeAffinity == nil &&
					podSpec.NodeSelector == nil &&
					podSpec.Tolerations == nil
			}, env.Timeout(), 1*time.Second).Should(BeTrue(), "placement should be nil")
		})

		// TODO - This test is currently pending, because it can be flaky.
		//        If the operator is too slow and does not notice Deployment
		//        state when not all pods are running, the test would fail.
		PIt("[test_id:5830]should set available condition when at least one validator pod is running", func() {
			watch, err := StartWatch(sspListerWatcher)
			Expect(err).ToNot(HaveOccurred())
			defer watch.Stop()

			updateSsp(func(foundSsp *ssp.SSP) {
				foundSsp.Spec.TemplateValidator = &ssp.TemplateValidator{
					Replicas: ptr.To(replicas),
				}
			})

			err = WatchChangesUntil(watch, isStatusDeploying, env.Timeout())
			Expect(err).ToNot(HaveOccurred(), "SSP status should be deploying.")

			err = WatchChangesUntil(watch, func(obj *ssp.SSP) bool {
				available := conditionsv1.FindStatusCondition(obj.Status.Conditions, conditionsv1.ConditionAvailable)
				progressing := conditionsv1.FindStatusCondition(obj.Status.Conditions, conditionsv1.ConditionProgressing)

				return obj.Status.Phase == lifecycleapi.PhaseDeploying &&
					available.Status == core.ConditionTrue &&
					progressing.Status == core.ConditionTrue
			}, env.Timeout())
			Expect(err).ToNot(HaveOccurred(), "SSP should be available, but progressing.")

			err = WatchChangesUntil(watch, isStatusDeployed, env.Timeout())
			Expect(err).ToNot(HaveOccurred(), "SSP status should be deployed.")
		})
	})
})

var _ = Describe("Template validator webhooks", func() {
	var (
		vm       *kubevirtv1.VirtualMachine
		vmi      *kubevirtv1.VirtualMachineInstance
		template *templatev1.Template
	)
	const (
		TemplateNameAnnotation      = "vm.kubevirt.io/template"
		TemplateNamespaceAnnotation = "vm.kubevirt.io/template-namespace"
	)

	BeforeEach(func() {
		vmi = NewRandomVMIWithBridgeInterface(strategy.GetNamespace())
		vm = nil
		template = nil

		waitUntilDeployed()
	})
	AfterEach(func() {
		if vm != nil {
			err := apiClient.Delete(ctx, vm)
			if !errors.IsNotFound(err) {
				Expect(err).ToNot(HaveOccurred(), "Failed to Delete VM")
			}
			// Need to wait until VM is removed, otherwise the webhook may
			// not allow template to be removed.
			waitForDeletion(client.ObjectKeyFromObject(vm), vm)
		}
		if template != nil {
			Eventually(func(g Gomega) {
				if err := apiClient.Delete(ctx, template); err != nil {
					g.Expect(errors.ReasonForError(err)).To(Equal(metav1.StatusReasonNotFound))
				}
			}, env.ShortTimeout(), time.Second).Should(Succeed(), "Template should be deleted")
		}
	})

	Context("Validation defined on template", func() {
		It("[test_id:5584]should create VM without template", func() {
			vm = NewVirtualMachine(vmi)
			Expect(apiClient.Create(ctx, vm)).ToNot(HaveOccurred(), "Failed to create VM")
		})
		It("[test_id:5585]be created from template with no rules", func() {
			template = TemplateWithoutRules()
			Expect(apiClient.Create(ctx, template)).ToNot(HaveOccurred(), "Failed to create template: %s", template.Name)

			vm = NewVirtualMachine(vmi)
			vm.ObjectMeta.Annotations = map[string]string{
				TemplateNameAnnotation:      template.Name,
				TemplateNamespaceAnnotation: template.Namespace,
			}
			eventuallyCreateVm(vm)
		})
		It("[test_id:5033]: Template with validations, VM without validations", func() {
			template = TemplateWithRules()
			Expect(apiClient.Create(ctx, template)).ToNot(HaveOccurred(), "Failed to create template: %s", template.Name)

			vmi = addDomainResourcesToVMI(vmi, 2, "q35", "128M")
			vm = NewVirtualMachine(vmi)
			vm.ObjectMeta.Annotations = map[string]string{
				TemplateNameAnnotation:      template.Name,
				TemplateNamespaceAnnotation: template.Namespace,
			}
			eventuallyCreateVm(vm)
		})
		It("[test_id:2960] Negative test - Create a VM with machine type violation", func() {
			template = TemplateWithRules()
			Expect(apiClient.Create(ctx, template)).ToNot(HaveOccurred(), "Failed to create template: %s", template.Name)
			// set value unfulfilling validation
			vmi = addDomainResourcesToVMI(vmi, 2, "test", "128M")
			vm = NewVirtualMachine(vmi)
			vm.ObjectMeta.Annotations = map[string]string{
				TemplateNameAnnotation:      template.Name,
				TemplateNamespaceAnnotation: template.Namespace,
			}
			eventuallyFailToCreateVm(vm)
		})

		It("[test_id:7060]vm rejected metrics increases by one when vm is rejected", func() {
			template = TemplateWithRules()
			Expect(apiClient.Create(ctx, template)).ToNot(HaveOccurred(), "Failed to create template: %s", template.Name)
			failVmCreationToIncreaseRejectedVmsMetrics(template)
		})

		It("[test_id:5586]test with template optional rules unfulfilled", func() {
			template = TemplateWithRulesOptional()
			Expect(apiClient.Create(ctx, template)).ToNot(HaveOccurred(), "Failed to create template: %s", template.Name)

			vmi = addDomainResourcesToVMI(vmi, 0, "q35", "128M")
			vmi.Spec.Domain.CPU = nil
			vm = NewVirtualMachine(vmi)
			vm.ObjectMeta.Annotations = map[string]string{
				TemplateNameAnnotation:      template.Name,
				TemplateNamespaceAnnotation: template.Namespace,
			}
			eventuallyCreateVm(vm)
		})
		It("[test_id:5587]test with cpu jsonpath nil should fail", func() {
			template = TemplateWithRules()
			Expect(apiClient.Create(ctx, template)).ToNot(HaveOccurred(), "Failed to create template: %s", template.Name)

			vmi = addDomainResourcesToVMI(vmi, 0, "q35", "128M")
			vmi.Spec.Domain.CPU = nil
			vm = NewVirtualMachine(vmi)
			vm.ObjectMeta.Annotations = map[string]string{
				TemplateNameAnnotation:      template.Name,
				TemplateNamespaceAnnotation: template.Namespace,
			}
			eventuallyFailToCreateVm(vm)
		})
		It("[test_id:5589]Test template with incorrect rules satisfied", func() {
			template = TemplateWithIncorrectRules()
			Expect(apiClient.Create(ctx, template)).ToNot(HaveOccurred(), "Failed to create template: %s", template.Name)

			vmi = addDomainResourcesToVMI(vmi, 0, "q35", "128M")
			vmi.Spec.Domain.CPU = nil
			vm = NewVirtualMachine(vmi)
			vm.ObjectMeta.Annotations = map[string]string{
				TemplateNameAnnotation:      template.Name,
				TemplateNamespaceAnnotation: template.Namespace,
			}
			eventuallyFailToCreateVm(vm)
		})
		It("[test_id:5590]Test template with incorrect rules unfulfilled", func() {
			template = TemplateWithIncorrectRules()
			Expect(apiClient.Create(ctx, template)).ToNot(HaveOccurred(), "Failed to create template: %s", template.Name)

			vmi = addDomainResourcesToVMI(vmi, 0, "q35", "32M")
			vmi.Spec.Domain.CPU = nil
			vm = NewVirtualMachine(vmi)
			vm.ObjectMeta.Annotations = map[string]string{
				TemplateNameAnnotation:      template.Name,
				TemplateNamespaceAnnotation: template.Namespace,
			}
			eventuallyFailToCreateVm(vm)
		})
		It("[test_id:2959] Create a VM with memory restrictions violation that succeeds with a warning", func() {
			template = TemplateWithIncorrectRulesJustWarning()
			Expect(apiClient.Create(ctx, template)).ToNot(HaveOccurred(), "Failed to create template: %s", template.Name)

			vmi = addDomainResourcesToVMI(vmi, 0, "q35", "1G")
			vmi.Spec.Domain.CPU = nil
			vm = NewVirtualMachine(vmi)
			vm.ObjectMeta.Annotations = map[string]string{
				TemplateNameAnnotation:      template.Name,
				TemplateNamespaceAnnotation: template.Namespace,
			}
			eventuallyCreateVm(vm)

			pods, err := GetRunningPodsByLabel(validator.VirtTemplateValidator, validator.KubevirtIo, strategy.GetNamespace())
			Expect(err).ToNot(HaveOccurred(), "Could not find the validator pods")
			Eventually(func() bool {
				for _, pod := range pods.Items {
					logs, err := GetPodLogs(pod.Name, pod.Namespace)
					Expect(err).ToNot(HaveOccurred())
					if strings.Contains(logs, "Memory size not within range") {
						return true
					}
				}
				return false
			}, env.ShortTimeout()).Should(BeTrue(), "Failed to find error msg in the logs")
		})
		It("[test_id:5591]test with partial annotations", func() {
			vmi = addDomainResourcesToVMI(vmi, 2, "q35", "128M")
			vm = NewVirtualMachine(vmi)
			vm.ObjectMeta.Annotations = map[string]string{
				"vm.kubevirt.io/template-namespace": strategy.GetNamespace(),
			}
			eventuallyCreateVm(vm)
		})
		It("[test_id:6199]Test vm with UI style annotations", func() {
			template = TemplateWithRules()
			Expect(apiClient.Create(ctx, template)).ToNot(HaveOccurred(), "Failed to create template: %s", template.Name)

			vmi = addDomainResourcesToVMI(vmi, 2, "q35", "128M")
			vm = NewVirtualMachine(vmi)
			vm.ObjectMeta.Annotations = map[string]string{
				TemplateNameAnnotation:              template.Name,
				"vm.kubevirt.io/template.namespace": template.Namespace,
			}
			eventuallyCreateVm(vm)
		})
		It("[test_id:5592]Test vm with template info in labels", func() {
			template = TemplateWithRules()
			Expect(apiClient.Create(ctx, template)).ToNot(HaveOccurred(), "Failed to create template: %s", template.Name)

			vmi = addDomainResourcesToVMI(vmi, 2, "q35", "128M")
			vm = NewVirtualMachine(vmi)
			vm.ObjectMeta.Labels = map[string]string{
				TemplateNameAnnotation:              template.Name,
				"vm.kubevirt.io/template.namespace": template.Namespace,
			}
			eventuallyCreateVm(vm)
		})
		It("[test_id:5593]test template with incomplete CPU info", func() {
			template = TemplateWithRules()
			Expect(apiClient.Create(ctx, template)).ToNot(HaveOccurred(), "Failed to create template: %s", template.Name)

			vmi = addDomainResourcesToVMI(vmi, 0, "q35", "128M")
			vmi.Spec.Domain.CPU = &kubevirtv1.CPU{
				Sockets: 1,
			}
			vm = NewVirtualMachine(vmi)
			vm.ObjectMeta.Annotations = map[string]string{
				TemplateNameAnnotation:      template.Name,
				TemplateNamespaceAnnotation: template.Namespace,
			}
			eventuallyCreateVm(vm)
		})
	})

	Context("Validation inside VM object", func() {
		It("[test_id:5173]: should create a VM that passes validation", func() {
			template = TemplateWithoutRules()
			Expect(apiClient.Create(ctx, template)).ToNot(HaveOccurred(), "Failed to create template: %s", template.Name)

			vmi = addDomainResourcesToVMI(vmi, 1, "q35", "64M")
			vm = NewVirtualMachine(vmi)
			vm.ObjectMeta.Annotations = map[string]string{
				TemplateNameAnnotation:      template.Name,
				TemplateNamespaceAnnotation: template.Namespace,
				"vm.kubevirt.io/validations": `[
												 {
														"name": "LimitCores",
														"path": "jsonpath::.spec.domain.cpu.cores",
														"message": "Core amount not within range",
														"rule": "integer",
														"min": 1,
														"max": 4
        										 }
												]`,
			}
			eventuallyCreateVm(vm)
		})
		It("[test_id:5034]: should fail to create VM that fails validation", func() {
			template = TemplateWithoutRules()
			Expect(apiClient.Create(ctx, template)).ToNot(HaveOccurred(), "Failed to create template: %s", template.Name)

			vmi = addDomainResourcesToVMI(vmi, 5, "q35", "64M")
			vm = NewVirtualMachine(vmi)
			vm.ObjectMeta.Annotations = map[string]string{
				TemplateNameAnnotation:      template.Name,
				TemplateNamespaceAnnotation: template.Namespace,
				"vm.kubevirt.io/validations": `[
												 {
														"name": "LimitCores",
														"path": "jsonpath::.spec.domain.cpu.cores",
														"message": "Core amount not within range",
														"rule": "integer",
														"min": 1,
														"max": 4
        										 }
												]`,
			}
			eventuallyFailToCreateVm(vm)
		})
		It("[test_id:5035]: Template with validations, VM with validations", func() {
			template = TemplateWithRules()
			Expect(apiClient.Create(ctx, template)).ToNot(HaveOccurred(), "Failed to create template: %s", template.Name)

			vmi = addDomainResourcesToVMI(vmi, 5, "q35", "64M")
			vm = NewVirtualMachine(vmi)
			vm.ObjectMeta.Annotations = map[string]string{
				TemplateNameAnnotation:      template.Name,
				TemplateNamespaceAnnotation: template.Namespace,
				"vm.kubevirt.io/validations": `[
												 {
														"name": "LimitCores",
														"path": "jsonpath::.spec.domain.cpu.cores",
														"message": "Core amount not within range",
														"rule": "integer",
														"min": 1,
														"max": 4
        										 }
												]`,
			}
			eventuallyFailToCreateVm(vm)
		})
		It("[test_id:5036]: should successfully create a VM based on the VM validation rules priority over template rules", func() {
			template = TemplateWithRules()
			Expect(apiClient.Create(ctx, template)).ToNot(HaveOccurred(), "Failed to create template: %s", template.Name)

			vmi = addDomainResourcesToVMI(vmi, 5, "q35", "64M")
			vm = NewVirtualMachine(vmi)
			vm.ObjectMeta.Annotations = map[string]string{
				TemplateNameAnnotation:      template.Name,
				TemplateNamespaceAnnotation: template.Namespace,
				"vm.kubevirt.io/validations": `[
												 {
														"name": "LimitCores",
														"path": "jsonpath::.spec.domain.cpu.cores",
														"message": "Core amount not within range",
														"rule": "integer",
														"min": 1,
														"max": 6
        										 }
												]`,
			}
			eventuallyCreateVm(vm)
		})
		It("[test_id:5174]: VM with validations and deleted template", func() {
			vmi = addDomainResourcesToVMI(vmi, 3, "q35", "64M")
			vm = NewVirtualMachine(vmi)
			vm.ObjectMeta.Annotations = map[string]string{
				TemplateNameAnnotation:      "nonexisting-vm-template",
				TemplateNamespaceAnnotation: strategy.GetTemplatesNamespace(),
				"vm.kubevirt.io/validations": `[
												 {
														"name": "LimitCores",
														"path": "jsonpath::.spec.domain.cpu.cores",
														"message": "Core amount not within range",
														"rule": "integer",
														"min": 1,
														"max": 4
        										 }
												]`,
			}
			eventuallyCreateVm(vm)
		})
		It("[test_id:5046]: should override template rules and fail to create a VM based on the VM validation rules", func() {
			vmi = addDomainResourcesToVMI(vmi, 5, "q35", "64M")
			vm = NewVirtualMachine(vmi)
			vm.ObjectMeta.Annotations = map[string]string{
				TemplateNameAnnotation:      "nonexisting-vm-template",
				TemplateNamespaceAnnotation: strategy.GetTemplatesNamespace(),
				"vm.kubevirt.io/validations": `[
												 {
														"name": "LimitCores",
														"path": "jsonpath::.spec.domain.cpu.cores",
														"message": "Core amount not within range",
														"rule": "integer",
														"min": 1,
														"max": 4
        										 }
												]`,
			}
			eventuallyFailToCreateVm(vm)
		})
		It("[test_id:5047]: should fail to create a VM based on the VM validation rules", func() {
			vmi = addDomainResourcesToVMI(vmi, 5, "q35", "64M")
			vm = NewVirtualMachine(vmi)
			vm.ObjectMeta.Annotations = map[string]string{
				"vm.kubevirt.io/validations": `[
												 {
														"name": "LimitCores",
														"path": "jsonpath::.spec.domain.cpu.cores",
														"message": "Core amount not within range",
														"rule": "integer",
														"min": 1,
														"max": 4
        										 }
												]`,
			}
			eventuallyFailToCreateVm(vm)
		})
		It("[test_id:5175]: VM with validations without template", func() {
			vmi = addDomainResourcesToVMI(vmi, 3, "q35", "64M")
			vm = NewVirtualMachine(vmi)
			vm.ObjectMeta.Annotations = map[string]string{
				"vm.kubevirt.io/validations": `[
												 {
														"name": "LimitCores",
														"path": "jsonpath::.spec.domain.cpu.cores",
														"message": "Core amount not within range",
														"rule": "integer",
														"min": 1,
														"max": 4
        										 }
												]`,
			}
			eventuallyCreateVm(vm)
		})
	})

	Context("Deleting template", func() {
		It("[test_id:7026] Can delete template if no VM uses it", func() {
			template = TemplateWithRules()
			Expect(apiClient.Create(ctx, template)).To(Succeed())

			Consistently(func() error {
				return apiClient.Delete(ctx, template, client.DryRunAll)
			}, 5*time.Second, 500*time.Millisecond).Should(Succeed())
		})

		It("[test_id:7027] Should fail to delete template if a VM uses it", func() {
			template = TemplateWithRules()
			Expect(apiClient.Create(ctx, template)).To(Succeed())

			vmi = addDomainResourcesToVMI(vmi, 2, "q35", "128M")
			vm = NewVirtualMachine(vmi)
			vm.ObjectMeta.Annotations = map[string]string{
				TemplateNameAnnotation:      template.Name,
				TemplateNamespaceAnnotation: template.Namespace,
			}
			eventuallyCreateVm(vm)

			Eventually(func() metav1.StatusReason {
				err := apiClient.Delete(ctx, template, client.DryRunAll)
				return errors.ReasonForError(err)
			}, env.ShortTimeout(), 1*time.Second).Should(Equal(metav1.StatusReasonForbidden), "Should have given forbidden error")
		})

		It("[test_id:7037] Can delete template without validations if a VM uses it", func() {
			template = TemplateWithoutRules()
			delete(template.Annotations, "validations")

			Expect(apiClient.Create(ctx, template)).To(Succeed())

			vm = NewVirtualMachine(vmi)
			vm.ObjectMeta.Annotations = map[string]string{
				TemplateNameAnnotation:      template.Name,
				TemplateNamespaceAnnotation: template.Namespace,
			}
			eventuallyCreateVm(vm)

			Consistently(func() error {
				return apiClient.Delete(ctx, template, client.DryRunAll)
			}, 5*time.Second, 500*time.Millisecond).Should(Succeed())
		})

		It("[test_id:7035] Can delete template if validations are defined on VM", func() {
			template = TemplateWithRules()
			Expect(apiClient.Create(ctx, template)).To(Succeed())

			vmi = addDomainResourcesToVMI(vmi, 2, "q35", "128M")
			vm = NewVirtualMachine(vmi)
			vm.ObjectMeta.Annotations = map[string]string{
				TemplateNameAnnotation:      template.Name,
				TemplateNamespaceAnnotation: template.Namespace,
				"vm.kubevirt.io/validations": `[
												 {
														"name": "LimitCores",
														"path": "jsonpath::.spec.domain.cpu.cores",
														"message": "Core amount not within range",
														"rule": "integer",
														"min": 1,
														"max": 4
        										 }
												]`,
			}

			eventuallyCreateVm(vm)

			Consistently(func() error {
				return apiClient.Delete(ctx, template, client.DryRunAll)
			}, 5*time.Second, 500*time.Millisecond).Should(Succeed())
		})
	})

	It("[test_id:4375] Test refreshing of certificates", func() {
		pods, err := GetRunningPodsByLabel(validator.VirtTemplateValidator, validator.KubevirtIo, strategy.GetNamespace())
		Expect(err).ToNot(HaveOccurred())
		Expect(pods.Items).ToNot(BeEmpty(), "no pods found")

		validatorPod := pods.Items[0]
		oldCerts, err := getWebhookServerCertificates(&validatorPod)
		Expect(err).ToNot(HaveOccurred())

		By("deleting the secret with certificate")
		secret := &core.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      validator.SecretName,
				Namespace: strategy.GetNamespace(),
			},
		}
		err = apiClient.Delete(ctx, secret)
		Expect(err).ToNot(HaveOccurred())

		By("checking that the secret gets restored with a new certificate")
		Eventually(func() (bool, error) {
			newCerts, err := getWebhookServerCertificates(&validatorPod)
			if err != nil {
				return true, err
			}
			return certsEqual(newCerts, oldCerts), nil
		}, 5*time.Minute, 1*time.Second).Should(BeFalse(), "Certificates should be different")
	})
})

func eventuallyCreateVm(vm *kubevirtv1.VirtualMachine) bool {
	// Using Eventually, because it can take a while until
	// new templates propagate to template validator
	return EventuallyWithOffset(1, func() error {
		return apiClient.Create(ctx, vm)
	}, env.ShortTimeout()).Should(Succeed(), "Failed to create VM")
}

func eventuallyFailToCreateVm(vm *kubevirtv1.VirtualMachine) bool {
	// Using Eventually, because it can take a while until
	// new templates propagate to template validator
	return EventuallyWithOffset(1, func() (metav1.StatusReason, error) {
		// Check if VM is still being deleted from previous call
		foundVm := &kubevirtv1.VirtualMachine{}
		err := apiClient.Get(ctx, client.ObjectKeyFromObject(vm), foundVm)
		if err == nil {
			if foundVm.GetDeletionTimestamp().IsZero() {
				// This should not happen
				return metav1.StatusReasonUnknown, fmt.Errorf("created VM is not being deleted")
			}
			return metav1.StatusReasonUnknown, fmt.Errorf("VM was created")
		}

		err = apiClient.Create(ctx, vm)
		if err == nil {
			// VM was created, but it should not have been
			err := apiClient.Delete(ctx, vm.DeepCopy())
			if err != nil {
				Fail(fmt.Sprintf("Unexpected error when deleting created VM: %v", err))
			}
			return metav1.StatusReasonUnknown, fmt.Errorf("VM was created")
		}
		return errors.ReasonForError(err), nil
	}, env.ShortTimeout()).Should(Equal(metav1.StatusReasonInvalid), "Should have given the invalid error")
}

func failVmCreationToIncreaseRejectedVmsMetrics(template *templatev1.Template) {
	rejectedCountBefore := totalRejectedVmsMetricsValue()
	vmi := NewRandomVMIWithBridgeInterface(strategy.GetNamespace())
	// set values that will fail validation
	vmi = addDomainResourcesToVMI(vmi, 2, "test", "128M")
	vm := NewVirtualMachine(vmi)
	vm.ObjectMeta.Annotations = map[string]string{
		labels.AnnotationTemplateNameKey:      template.Name,
		labels.AnnotationTemplateNamespaceKey: template.Namespace,
	}
	eventuallyFailToCreateVm(vm)
	Expect(totalRejectedVmsMetricsValue() - rejectedCountBefore).To(Equal(1))
	err := apiClient.Delete(ctx, vm)
	if !errors.IsNotFound(err) {
		Expect(err).ToNot(HaveOccurred(), "Failed to Delete VM")
	}
	waitForDeletion(client.ObjectKeyFromObject(vm), vm)
}

func totalRejectedVmsMetricsValue() (sum int) {
	pods, err := GetRunningPodsByLabel(validator.VirtTemplateValidator, validator.KubevirtIo, strategy.GetNamespace())
	Expect(err).ToNot(HaveOccurred(), "Could not find the validator pods")
	Expect(pods.Items).ToNot(BeEmpty())
	for _, validatorPod := range pods.Items {
		sum += intMetricValue("kubevirt_ssp_template_validator_rejected_total", validator.MetricsPort, &validatorPod)
	}
	return
}

func addObjectsToTemplates(genName, validation string) *templatev1.Template {
	editable := `/objects[0].spec.template.spec.domain.cpu.sockets
				/objects[0].spec.template.spec.domain.cpu.cores
 				/objects[0].spec.template.spec.domain.cpu.threads
				/objects[0].spec.template.spec.domain.resources.requests.memory
				/objects[0].spec.template.spec.domain.devices.disks
				/objects[0].spec.template.spec.volumes
				/objects[0].spec.template.spec.networks`
	userData := `#cloud-config
				password: fedora
				chpasswd: { expire: False }`
	running := false
	liveMigrate := kubevirtv1.EvictionStrategyLiveMigrate
	template := &templatev1.Template{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: genName + "-",
			Namespace:    strategy.GetNamespace(),
			Annotations: map[string]string{
				"openshift.io/display-name":             "Fedora 23+ VM",
				"description":                           "This template can be used to create a VM",
				"tags":                                  "kubevirt,virtualmachine,fedora,rhel",
				"iconClass":                             "icon-fedora",
				"openshift.io/provider-display-name":    "KubeVirt",
				"openshift.io/documentation-url":        "https://github.com/kubevirt/common-templates",
				"openshift.io/support-url":              "https://github.com/kubevirt/common-templates/issues",
				"template.openshift.io/bindable":        "false",
				"template.kubevirt.io/version":          "v1alpha1",
				"defaults.template.kubevirt.io/disk":    "rootdisk",
				"template.kubevirt.io/editable":         editable,
				"name.os.template.kubevirt.io/fedora26": "Fedora 26",
				"name.os.template.kubevirt.io/fedora27": "Fedora 27",
				"name.os.template.kubevirt.io/fedora28": "Fedora 28",
				"validations":                           validation,
			},
			Labels: map[string]string{
				"os.template.kubevirt.io/fedora26":      "true",
				"os.template.kubevirt.io/fedora27":      "true",
				"os.template.kubevirt.io/fedora28":      "true",
				"workload.template.kubevirt.io/generic": "true",
				"flavor.template.kubevirt.io/small":     "true",
				"template.kubevirt.io/type":             "base",
			},
		},
		Parameters: []templatev1.Parameter{
			{
				Description: "VM name",
				From:        "fedora-[a-z0-9]{16}",
				Generate:    "expression",
				Name:        "NAME",
			},
			{
				Name:        "PVCNAME",
				Description: "Name of the PVC with the disk image",
				Required:    true,
			},
		},
	}

	codec := serializer.NewCodecFactory(testScheme).LegacyCodec(kubevirtv1.GroupVersion)
	template.Objects = append(template.Objects,
		runtime.RawExtension{
			Raw: []byte(runtime.EncodeOrDie(codec, &kubevirtv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "${NAME}",
					Namespace: strategy.GetNamespace(),
					Labels: map[string]string{
						"vm.kubevirt.io/template": "fedora-desktop-small",
						"app":                     "${NAME}",
					},
				},
				Spec: kubevirtv1.VirtualMachineSpec{
					Running: &running,
					Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
						Spec: kubevirtv1.VirtualMachineInstanceSpec{
							Domain: kubevirtv1.DomainSpec{
								CPU: &kubevirtv1.CPU{
									Sockets: 1,
									Cores:   1,
									Threads: 1,
								},
								Resources: kubevirtv1.ResourceRequirements{
									Requests: map[core.ResourceName]resource.Quantity{
										"memory": resource.MustParse("2Gi"),
									},
								},
								Devices: kubevirtv1.Devices{
									Rng: &kubevirtv1.Rng{},
									Disks: []kubevirtv1.Disk{
										{
											Name: "rootdisk",
											DiskDevice: kubevirtv1.DiskDevice{
												Disk: &kubevirtv1.DiskTarget{
													Bus: "virtio",
												},
											},
										},
									},
								},
							},
							EvictionStrategy: &liveMigrate,
							Volumes: []kubevirtv1.Volume{
								{
									Name: "rootdisk",
									VolumeSource: kubevirtv1.VolumeSource{
										PersistentVolumeClaim: &kubevirtv1.PersistentVolumeClaimVolumeSource{
											PersistentVolumeClaimVolumeSource: core.PersistentVolumeClaimVolumeSource{
												ClaimName: "${PVCNAME}",
											},
										},
									},
								},
								{
									Name: "cloudinitvolume",
									VolumeSource: kubevirtv1.VolumeSource{
										CloudInitNoCloud: &kubevirtv1.CloudInitNoCloudSource{
											UserData: userData,
										},
									},
								},
							},
						},
					},
				},
			})),
		})
	return template
}

func TemplateWithRules() *templatev1.Template {
	validations := `[
		{
          	"name": "EnoughMemory",
          	"path": "jsonpath::.spec.domain.resources.requests.memory",
          	"message": "Memory size not within range",
          	"rule": "integer",
          	"min": 67108864,
          	"max": 536870912
        },
        {
          	"name": "LimitCores",
          	"path": "jsonpath::.spec.domain.cpu.cores",
          	"message": "Core amount not within range",
          	"rule": "integer",
          	"min": 1,
          	"max": 4
        },
        {
          	"name": "SupportedChipset",
          	"path": "jsonpath::.spec.domain.machine.type",
          	"message": "Machine type is a supported value",
          	"rule": "enum",
          	"values": ["q35"]
        }
	]`
	return addObjectsToTemplates("test-fedora-desktop-small-with-rules", validations)
}

func TemplateWithRulesOptional() *templatev1.Template {
	validations := `[
		{
          "name": "EnoughMemory",
          "path": "jsonpath::.spec.domain.resources.requests.memory",
          "valid": "jsonpath::.spec.domain.resources.requests.memory",
          "message": "Memory size not within range",
          "rule": "integer",
          "min": 67108864,
          "max": 536870912
        },
        {
          "name": "LimitCores",
          "path": "jsonpath::.spec.domain.cpu.cores",
          "valid": "jsonpath::.spec.domain.cpu.cores",
          "message": "Core amount not within range",
          "rule": "integer",
          "min": 1,
          "max": 4
        }
	]`
	return addObjectsToTemplates("test-fedora-desktop-small-with-rules-optional", validations)
}

func TemplateWithIncorrectRules() *templatev1.Template {
	// Incorrect rule named 'value-set'
	validations := `[
        {
          "name": "EnoughMemory",
          "path": "jsonpath::.spec.domain.resources.requests.memory",
          "message": "Memory size not within range",
          "rule": "integer",
          "min": 67108864,
          "max": 536870912
        },
        {
          "name": "SupportedChipset",
          "path": "jsonpath::.spec.domain.machine.type",
          "rule": "value-set",
          "values": ["q35"]
        }
      ]`
	return addObjectsToTemplates("test-fedora-desktop-small-with-rules-incorrect", validations)
}

func TemplateWithIncorrectRulesJustWarning() *templatev1.Template {
	validations := `[
		{
          "name": "EnoughMemory",
          "path": "jsonpath::.spec.domain.resources.requests.memory",
          "message": "Memory size not within range",
          "rule": "integer",
          "min": 77108864,
          "max": 536870912,
          "justWarning": true
        }
	]`
	return addObjectsToTemplates("test-fedora-desktop-small-with-rules-with-warning", validations)
}

func TemplateWithoutRules() *templatev1.Template {
	validations := `[]`
	return addObjectsToTemplates("test-fedora-desktop-small-without-rules", validations)
}

func getWebhookServerCertificates(validatorPod *core.Pod) ([]*x509.Certificate, error) {
	conn, err := portForwarder.Connect(validatorPod, validator.ContainerPort)
	if err != nil {
		return nil, err
	}

	tlsConn := tls.Client(conn, &tls.Config{InsecureSkipVerify: true})
	defer tlsConn.Close()

	err = tlsConn.Handshake()
	if err != nil {
		return nil, err
	}

	return tlsConn.ConnectionState().PeerCertificates, nil
}

func certsEqual(certs1, certs2 []*x509.Certificate) bool {
	if len(certs1) != len(certs2) {
		return false
	}
	for i := 0; i < len(certs1); i++ {
		if !certs1[i].Equal(certs2[i]) {
			return false
		}
	}
	return true
}
