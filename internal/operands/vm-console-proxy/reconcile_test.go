package vm_console_proxy

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apps "k8s.io/api/apps/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	kubevirt "kubevirt.io/api/core"
	ssp "kubevirt.io/ssp-operator/api/v1beta1"
	"kubevirt.io/ssp-operator/internal/common"
	. "kubevirt.io/ssp-operator/internal/test-utils"
	vm_console_proxy_bundle "kubevirt.io/ssp-operator/internal/vm-console-proxy-bundle"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	namespace              = "kubevirt"
	name                   = "vm-console-proxy"
	vmConsoleProxyName     = "vm-console-proxy"
	clusterRoleName        = "vm-console-proxy"
	clusterRoleBindingName = "vm-console-proxy"
	serviceAccountName     = "vm-console-proxy"
	configMapName          = "vm-console-proxy"
	serviceName            = "vm-console-proxy"
	deploymentName         = "vm-console-proxy"
)

var _ = Describe("VM Console Proxy Operand", func() {
	const (
		replicas int32 = 1
	)

	var (
		bundle  *vm_console_proxy_bundle.Bundle
		operand *vmConsoleProxy
		request common.Request
	)

	BeforeEach(func() {
		bundle = getMockedTestBundle()
		operand = New(bundle)
		request = getMockedRequest()
	})

	It("should return name correctly", func() {
		name := operand.Name()
		Expect(name).To(Equal(operandName), "should return correct name")
	})

	It("should return functions from reconcile correclty", func() {
		functions, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred(), "should not throw err")
		Expect(len(functions)).To(Equal(7), "should return correct number of reconcile functions")
	})

	It("should create vm-console-proxy resources", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		ExpectResourceExists(&bundle.ServiceAccount, request)
		ExpectResourceExists(&bundle.ClusterRole, request)
		ExpectResourceExists(&bundle.ClusterRoleBinding, request)
		ExpectResourceExists(&bundle.ConfigMap, request)
		ExpectResourceExists(&bundle.Service, request)
		ExpectResourceExists(&bundle.Deployment, request)
		ExpectResourceExists(newRoute(namespace, serviceName), request)
	})

	It("should remove cluster resources on cleanup", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		ExpectResourceExists(&bundle.ServiceAccount, request)
		ExpectResourceExists(&bundle.ClusterRole, request)
		ExpectResourceExists(&bundle.ClusterRoleBinding, request)
		ExpectResourceExists(&bundle.ConfigMap, request)
		ExpectResourceExists(&bundle.Service, request)
		ExpectResourceExists(&bundle.Deployment, request)
		ExpectResourceExists(newRoute(namespace, serviceName), request)

		_, err = operand.Cleanup(&request)
		Expect(err).ToNot(HaveOccurred())

		ExpectResourceNotExists(&bundle.ServiceAccount, request)
		ExpectResourceNotExists(&bundle.ClusterRole, request)
		ExpectResourceNotExists(&bundle.ClusterRoleBinding, request)
		ExpectResourceNotExists(&bundle.ConfigMap, request)
		ExpectResourceNotExists(&bundle.Service, request)
		ExpectResourceNotExists(&bundle.Deployment, request)
		ExpectResourceNotExists(newRoute(namespace, serviceName), request)
	})

	It("should not update service cluster IP", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		key := client.ObjectKeyFromObject(&bundle.Service)
		service := &core.Service{}
		Expect(request.Client.Get(request.Context, key, service)).ToNot(HaveOccurred())

		// This address is from a range of IP addresses reserved for documentation.
		const testClusterIp = "198.51.100.42"

		service.Spec.ClusterIP = testClusterIp
		Expect(request.Client.Update(request.Context, service)).ToNot(HaveOccurred())

		_, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		updatedService := &core.Service{}
		Expect(request.Client.Get(request.Context, key, updatedService)).ToNot(HaveOccurred())
		Expect(updatedService.Spec.ClusterIP).To(Equal(testClusterIp))
	})

	It("should report status", func() {
		reconcileResults, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		// Set status for deployment
		key := client.ObjectKeyFromObject(&bundle.Deployment)
		updateDeployment(key, &request, func(deployment *apps.Deployment) {
			deployment.Status.Replicas = replicas
			deployment.Status.ReadyReplicas = 0
			deployment.Status.AvailableReplicas = 0
			deployment.Status.UpdatedReplicas = 0
			deployment.Status.UnavailableReplicas = replicas
		})

		reconcileResults, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		// Only deployment should be progressing
		for _, reconcileResult := range reconcileResults {
			if _, ok := reconcileResult.Resource.(*apps.Deployment); ok {
				Expect(reconcileResult.Status.NotAvailable).ToNot(BeNil())
				Expect(reconcileResult.Status.Progressing).ToNot(BeNil())
				Expect(reconcileResult.Status.Degraded).ToNot(BeNil())
			} else {
				Expect(reconcileResult.Status.NotAvailable).To(BeNil())
				Expect(reconcileResult.Status.Progressing).To(BeNil())
				Expect(reconcileResult.Status.Degraded).To(BeNil())
			}
		}

		updateDeployment(key, &request, func(deployment *apps.Deployment) {
			deployment.Status.Replicas = replicas
			deployment.Status.ReadyReplicas = replicas
			deployment.Status.AvailableReplicas = replicas
			deployment.Status.UpdatedReplicas = replicas
			deployment.Status.UnavailableReplicas = 0
		})

		reconcileResults, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		// All resources should be available
		for _, reconcileResult := range reconcileResults {
			Expect(reconcileResult.Status.NotAvailable).To(BeNil())
			Expect(reconcileResult.Status.Progressing).To(BeNil())
			Expect(reconcileResult.Status.Degraded).To(BeNil())
		}
	})

	It("should deploy resources in namespace provided by annotation", func() {
		namespace := "some-namespace"
		request.Instance.GetAnnotations()[VmConsoleProxyNamespaceAnnotation] = namespace

		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		ExpectNamespacedResourceExists(&bundle.ServiceAccount, request, namespace)
		ExpectNamespacedResourceExists(&bundle.ConfigMap, request, namespace)
		ExpectNamespacedResourceExists(&bundle.Service, request, namespace)
		ExpectNamespacedResourceExists(&bundle.Deployment, request, namespace)
		ExpectNamespacedResourceExists(newRoute(getVmConsoleProxyNamespace(&request), serviceName), request, namespace)
	})

	It("should delete resources when enabled annotation is removed", func() {
		delete(request.Instance.Annotations, EnableAnnotation)

		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		ExpectResourceNotExists(&bundle.ServiceAccount, request)
		ExpectResourceNotExists(&bundle.ClusterRole, request)
		ExpectResourceNotExists(&bundle.ClusterRoleBinding, request)
		ExpectResourceNotExists(&bundle.ConfigMap, request)
		ExpectResourceNotExists(&bundle.Service, request)
		ExpectResourceNotExists(&bundle.Deployment, request)
		ExpectResourceNotExists(newRoute(namespace, serviceName), request)
	})
})

func TestVmConsoleProxyBundle(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "VM Console Proxy Operand Suite")
}

func getMockedRequest() common.Request {
	var log = logf.Log.WithName("vm_console_proxy_operand")
	client := fake.NewClientBuilder().WithScheme(common.Scheme).Build()

	return common.Request{
		Request: reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: namespace,
				Name:      name,
			},
		},
		Client:  client,
		Context: context.Background(),
		Instance: &ssp.SSP{
			TypeMeta: metav1.TypeMeta{
				Kind:       "SSP",
				APIVersion: ssp.GroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Annotations: map[string]string{
					EnableAnnotation:                  "true",
					VmConsoleProxyNamespaceAnnotation: namespace,
				},
			},
		},
		Logger:       log,
		VersionCache: common.VersionCache{},
	}
}

func getMockedTestBundle() *vm_console_proxy_bundle.Bundle {
	image := "quay.io/kubevirt/vm-console-proxy:latest"
	trueValue := true
	falseValue := false
	replicas := int32(1)
	terminationGracePeriodSecondsValue := int64(10)

	return &vm_console_proxy_bundle.Bundle{
		ServiceAccount: core.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceAccountName,
				Namespace: namespace,
				Labels:    map[string]string{"control-plane": "vm-console-proxy"},
			},
		},
		ClusterRole: rbac.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterRoleName,
				Namespace: namespace,
				Labels:    map[string]string{},
			},
			Rules: []rbac.PolicyRule{{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "list", "watch"},
			}, {
				APIGroups: []string{kubevirt.GroupName},
				Resources: []string{"virtualmachineinstances"},
				Verbs:     []string{"get", "list", "watch"},
			}, {
				APIGroups: []string{authenticationv1.GroupName},
				Resources: []string{"tokenreviews"},
				Verbs:     []string{"create"},
			}, {
				APIGroups: []string{authenticationv1.GroupName},
				Resources: []string{"subjectaccessreviews"},
				Verbs:     []string{"create"},
			}},
		},
		ClusterRoleBinding: rbac.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterRoleBindingName,
				Namespace: namespace,
				Labels:    map[string]string{},
			},
			RoleRef: rbac.RoleRef{
				Kind:     "ClusterRole",
				Name:     clusterRoleName,
				APIGroup: rbac.GroupName,
			},
			Subjects: []rbac.Subject{{
				Kind:      "ServiceAccount",
				Name:      serviceAccountName,
				Namespace: namespace,
			}},
		},
		ConfigMap: core.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: namespace,
			},
			Data: map[string]string{
				"tls-profile-v1alpha1.yaml": `
	type: Intermediate
	intermediate: {}
	`,
			},
		},
		Service: core.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: namespace,
				Labels:    map[string]string{},
				Annotations: map[string]string{
					"service.beta.openshift.io/serving-cert-secret-name": "vm-console-proxy-cert",
				},
			},
			Spec: core.ServiceSpec{
				Ports: []core.ServicePort{{
					Port:       443,
					TargetPort: intstr.FromInt(8768),
				}},
				Selector: map[string]string{
					"vm-console-proxy.kubevirt.io": vmConsoleProxyName,
				},
			},
		},
		Deployment: apps.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deploymentName,
				Namespace: namespace,
				Labels: map[string]string{
					"name":                         deploymentName,
					"vm-console-proxy.kubevirt.io": deploymentName,
				},
			},
			Spec: apps.DeploymentSpec{
				Replicas: &replicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"vm-console-proxy.kubevirt.io": deploymentName,
					},
				},
				Template: core.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Name: vmConsoleProxyName,
						Labels: map[string]string{
							"name":                         deploymentName,
							"vm-console-proxy.kubevirt.io": deploymentName,
						},
					},
					Spec: core.PodSpec{
						Containers: []core.Container{{
							Name:            "console",
							Image:           image,
							ImagePullPolicy: core.PullAlways,
							Resources: core.ResourceRequirements{
								Requests: core.ResourceList{
									core.ResourceCPU:    resource.MustParse("200m"),
									core.ResourceMemory: resource.MustParse("200Mi"),
								},
							},
							Args: []string{},
							Command: []string{
								"/console",
							},
							VolumeMounts: []core.VolumeMount{{
								Name:      "config",
								MountPath: "/config",
								ReadOnly:  true,
							}, {
								Name:      "vm-console-proxy-cert",
								MountPath: "/tmp/vm-console-proxy-cert",
								ReadOnly:  true,
							}, {
								Name:      "kubevirt-virt-handler-certs",
								MountPath: "/etc/virt-handler/clientcertificates",
								ReadOnly:  true,
							}},
							SecurityContext: &core.SecurityContext{
								AllowPrivilegeEscalation: &falseValue,
								Capabilities: &core.Capabilities{
									Drop: []core.Capability{"ALL"},
								},
							},
							Ports: []core.ContainerPort{{
								Name:          "api",
								ContainerPort: 8768,
								Protocol:      core.ProtocolTCP,
							}},
						}},
						SecurityContext: &core.PodSecurityContext{
							RunAsNonRoot: &trueValue,
							SeccompProfile: &core.SeccompProfile{
								Type: core.SeccompProfileTypeRuntimeDefault,
							},
						},
						ServiceAccountName:            serviceAccountName,
						TerminationGracePeriodSeconds: &terminationGracePeriodSecondsValue,
						Volumes: []core.Volume{{
							Name: "config",
							VolumeSource: core.VolumeSource{
								ConfigMap: &core.ConfigMapVolumeSource{
									LocalObjectReference: core.LocalObjectReference{
										Name: "vm-console-proxy",
									},
								},
							},
						}, {
							Name: "vm-console-proxy-cert",
							VolumeSource: core.VolumeSource{
								Secret: &core.SecretVolumeSource{
									SecretName: "vm-console-proxy-cert",
								},
							},
						}, {
							Name: "kubevirt-virt-handler-certs",
							VolumeSource: core.VolumeSource{
								Secret: &core.SecretVolumeSource{
									SecretName: "kubevirt-virt-handler-certs",
								},
							},
						}},
					},
				},
			},
		},
	}
}

func updateDeployment(key client.ObjectKey, request *common.Request, updateFunc func(deployment *apps.Deployment)) {
	deployment := &apps.Deployment{}
	Expect(request.Client.Get(request.Context, key, deployment)).ToNot(HaveOccurred())
	updateFunc(deployment)
	Expect(request.Client.Update(request.Context, deployment)).ToNot(HaveOccurred())
	Expect(request.Client.Status().Update(request.Context, deployment)).ToNot(HaveOccurred())
}
