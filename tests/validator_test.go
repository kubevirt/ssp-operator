package tests

import (
	"reflect"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	libhandler "github.com/operator-framework/operator-lib/handler"
	admission "k8s.io/api/admissionregistration/v1"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	sspv1alpha1 "kubevirt.io/ssp-operator/api/v1alpha1"
	validator "kubevirt.io/ssp-operator/internal/operands/template-validator"
)

var (
	clusterRoleRes = &testResource{
		Name:     validator.ClusterRoleName(testNamespace),
		resource: &rbac.ClusterRole{},
	}
	clusterRoleBindingRes = &testResource{
		Name:     validator.ClusterRoleBindingName(testNamespace),
		resource: &rbac.ClusterRoleBinding{},
	}
	webhookConfigRes = &testResource{
		Name:     validator.ValidatingWebhookName(testNamespace),
		resource: &admission.ValidatingWebhookConfiguration{},
	}
	serviceAccountRes = &testResource{
		Name:       validator.ServiceAccountName,
		Namsespace: testNamespace,
		resource:   &core.ServiceAccount{},
	}
	serviceRes = &testResource{
		Name:       validator.ServiceName,
		Namsespace: testNamespace,
		resource:   &core.Service{},
	}
	deploymentRes = &testResource{
		Name:       validator.DeploymentName,
		Namsespace: testNamespace,
		resource:   &apps.Deployment{},
	}
)

var _ = Describe("Template validator", func() {
	Context("resource creation", func() {
		table.DescribeTable("created cluster resource", func(res *testResource) {
			resource := res.NewResource()
			err := apiClient.Get(ctx, client.ObjectKey{Name: res.Name}, resource)
			Expect(err).ToNot(HaveOccurred())
			Expect(hasOwnerAnnotations(resource.GetAnnotations())).To(BeTrue())
		},
			table.Entry("[test_id:4907] cluster role", clusterRoleRes),
			table.Entry("[test_id:4908] cluster role binding", clusterRoleBindingRes),
			table.Entry("[test_id:4909] validating webhook configuration", webhookConfigRes),
		)

		table.DescribeTable("created namespaced resource", func(res *testResource) {
			err := apiClient.Get(ctx, client.ObjectKey{
				Name: res.Name, Namespace: testNamespace,
			}, res.NewResource())
			Expect(err).ToNot(HaveOccurred())
		},
			table.Entry("[test_id:4910] service account", serviceAccountRes),
			table.Entry("[test_id:4911] service", serviceRes),
			table.Entry("[test_id:4912] deployment", deploymentRes),
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
			table.Entry("[test_id:4914] cluster role", clusterRoleRes),
			table.Entry("[test_id:4916] cluster role binding", clusterRoleBindingRes),
			table.Entry("[test_id:4918] validating webhook configuration", webhookConfigRes),
			table.Entry("[test_id:4920] service account", serviceAccountRes),
			table.Entry("[test_id:4922] service", serviceRes),
			table.Entry("[test_id:4924] deployment", deploymentRes),
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
			table.Entry("[test_id:4915] cluster role", clusterRoleRes,
				func(role *rbac.ClusterRole) {
					role.Rules[0].Verbs = []string{"watch"}
				},
				func(old *rbac.ClusterRole, new *rbac.ClusterRole) bool {
					return reflect.DeepEqual(old.Rules, new.Rules)
				}),

			table.Entry("[test_id:4917] cluster role binding", clusterRoleBindingRes,
				func(roleBinding *rbac.ClusterRoleBinding) {
					roleBinding.Subjects = []rbac.Subject{}
				},
				func(old *rbac.ClusterRoleBinding, new *rbac.ClusterRoleBinding) bool {
					return reflect.DeepEqual(old.RoleRef, new.RoleRef) &&
						reflect.DeepEqual(old.Subjects, new.Subjects)
				}),

			table.Entry("[test_id:4919] validating webhook configuration", webhookConfigRes,
				func(webhook *admission.ValidatingWebhookConfiguration) {
					webhook.Webhooks[0].Rules = []admission.RuleWithOperations{}
				},
				func(old *admission.ValidatingWebhookConfiguration, new *admission.ValidatingWebhookConfiguration) bool {
					return reflect.DeepEqual(old.Webhooks, new.Webhooks)
				}),

			table.Entry("[test_id:4923] service", serviceRes,
				func(service *core.Service) {
					service.Spec.Ports[0].Port = 44331
					service.Spec.Ports[0].TargetPort = intstr.FromInt(44331)
				},
				func(old *core.Service, new *core.Service) bool {
					return reflect.DeepEqual(old.Spec, new.Spec)
				}),

			table.Entry("[test_id:4925] deployment", deploymentRes,
				func(deployment *apps.Deployment) {
					deployment.Spec.Replicas = pointer.Int32Ptr(0)
				},
				func(old *apps.Deployment, new *apps.Deployment) bool {
					return reflect.DeepEqual(old.Spec, new.Spec)
				}),
		)
	})

	It("[test_id:4913] should successfully start template-validator pod", func() {
		labels := map[string]string{"kubevirt.io": "virt-template-validator"}
		Eventually(func() bool {
			pods := core.PodList{}
			err := apiClient.List(ctx, &pods, client.InNamespace(testNamespace), client.MatchingLabels(labels))
			Expect(err).ToNot(HaveOccurred())
			if len(pods.Items) != 1 {
				return false
			}
			return pods.Items[0].Status.Phase == core.PodRunning
		}, timeout, 1*time.Second).Should(BeTrue())
	})

	Context("placement API", func() {
		var originalSSP *sspv1alpha1.SSP

		BeforeEach(func() {
			originalSSP = ssp.DeepCopy()
		})

		AfterEach(func() {
			key := client.ObjectKey{
				Name:      originalSSP.Name,
				Namespace: originalSSP.Namespace,
			}
			foundSsp := &sspv1alpha1.SSP{}
			err := apiClient.Get(ctx, key, foundSsp)
			if err == nil {
				foundSsp.Spec = originalSSP.Spec
				err = apiClient.Update(ctx, foundSsp)
			} else {
				if !errors.IsNotFound(err) {
					Expect(err).ToNot(HaveOccurred())
				}
				err = apiClient.Create(ctx, originalSSP)
			}
			Expect(err).ToNot(HaveOccurred())
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

			updateSsp(func(foundSsp *sspv1alpha1.SSP) {
				placement := &foundSsp.Spec.TemplateValidator.Placement
				placement.Affinity = affinity
				placement.NodeSelector = nodeSelector
				placement.Tolerations = tolerations
			})

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

			updateSsp(func(foundSsp *sspv1alpha1.SSP) {
				placement := &foundSsp.Spec.TemplateValidator.Placement
				placement.Affinity = nil
				placement.NodeSelector = nil
				placement.Tolerations = nil
			})

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
	})
})

func hasOwnerAnnotations(annotations map[string]string) bool {
	const typeName = "SSP.ssp.kubevirt.io"
	namespacedName := ssp.Namespace + "/" + ssp.Name

	if annotations == nil {
		return false
	}

	return annotations[libhandler.TypeAnnotation] == typeName &&
		annotations[libhandler.NamespacedNameAnnotation] == namespacedName
}

func updateSsp(updateFunc func(foundSsp *sspv1alpha1.SSP)) {
	key := client.ObjectKey{Name: ssp.Name, Namespace: ssp.Namespace}
	foundSsp := &sspv1alpha1.SSP{}
	Expect(apiClient.Get(ctx, key, foundSsp)).ToNot(HaveOccurred())

	updateFunc(foundSsp)
	Expect(apiClient.Update(ctx, foundSsp)).ToNot(HaveOccurred())
}

type testResource struct {
	Name       string
	Namsespace string
	resource   controllerutil.Object
}

func (r *testResource) NewResource() controllerutil.Object {
	return r.resource.DeepCopyObject().(controllerutil.Object)
}

func (r *testResource) GetKey() client.ObjectKey {
	return client.ObjectKey{
		Name:      r.Name,
		Namespace: r.Namsespace,
	}
}
