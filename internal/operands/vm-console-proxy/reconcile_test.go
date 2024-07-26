package vm_console_proxy

import (
	"context"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	routev1 "github.com/openshift/api/route/v1"
	libhandler "github.com/operator-framework/operator-lib/handler"
	apps "k8s.io/api/apps/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	apiregv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"k8s.io/utils/ptr"
	kubevirt "kubevirt.io/api/core"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ssp "kubevirt.io/ssp-operator/api/v1beta2"
	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/env"
	"kubevirt.io/ssp-operator/internal/operands"
	. "kubevirt.io/ssp-operator/internal/test-utils"
	vm_console_proxy_bundle "kubevirt.io/ssp-operator/internal/vm-console-proxy-bundle"
)

const (
	namespace              = "kubevirt"
	name                   = "vm-console-proxy"
	vmConsoleProxyName     = "vm-console-proxy"
	clusterRoleName        = "vm-console-proxy"
	clusterRoleBindingName = "vm-console-proxy"
	roleBindingName        = "vm-console-proxy"
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
		operand operands.Operand
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

	It("should create vm-console-proxy resources", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		ExpectResourceExists(bundle.ServiceAccount, request)
		for _, clusterRole := range bundle.ClusterRoles {
			ExpectResourceExists(&clusterRole, request)
		}
		ExpectResourceExists(bundle.ClusterRoleBinding, request)
		ExpectResourceExists(bundle.RoleBinding, request)
		ExpectResourceExists(bundle.ConfigMap, request)
		ExpectResourceExists(bundle.Service, request)
		ExpectResourceExists(bundle.Deployment, request)
		ExpectResourceExists(bundle.ApiService, request)
	})

	It("should read deployment image the environment variable", func() {
		originalImage := os.Getenv(env.VmConsoleProxyImageKey)

		newImage := "www.example.org/images/vm-console-proxy:latest"
		Expect(os.Setenv(env.VmConsoleProxyImageKey, newImage)).To(Succeed())
		DeferCleanup(func() {
			Expect(os.Setenv(env.VmConsoleProxyImageKey, originalImage)).To(Succeed())
		})

		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		deployment := &apps.Deployment{}
		key := client.ObjectKeyFromObject(bundle.Deployment)
		Expect(request.Client.Get(request.Context, key, deployment)).To(Succeed())

		Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
		Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal(newImage))
	})

	It("should deploy APIService with ServiceReference pointing to the right namespace", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		apiService := &apiregv1.APIService{}
		key := client.ObjectKeyFromObject(bundle.ApiService)
		Expect(request.Client.Get(request.Context, key, apiService)).To(Succeed())

		Expect(apiService.Spec.Service.Namespace).To(Equal(namespace))
	})

	It("should remove cluster resources on cleanup", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		ExpectResourceExists(bundle.ServiceAccount, request)
		for _, clusterRole := range bundle.ClusterRoles {
			ExpectResourceExists(&clusterRole, request)
		}
		ExpectResourceExists(bundle.ClusterRoleBinding, request)
		ExpectResourceExists(bundle.RoleBinding, request)
		ExpectResourceExists(bundle.ConfigMap, request)
		ExpectResourceExists(bundle.Service, request)
		ExpectResourceExists(bundle.Deployment, request)
		ExpectResourceExists(bundle.ApiService, request)

		_, err = operand.Cleanup(&request)
		Expect(err).ToNot(HaveOccurred())

		ExpectResourceNotExists(bundle.ServiceAccount, request)
		for _, clusterRole := range bundle.ClusterRoles {
			ExpectResourceNotExists(&clusterRole, request)
		}
		ExpectResourceNotExists(bundle.ClusterRoleBinding, request)
		ExpectResourceNotExists(bundle.RoleBinding, request)
		ExpectResourceNotExists(bundle.ConfigMap, request)
		ExpectResourceNotExists(bundle.Service, request)
		ExpectResourceNotExists(bundle.Deployment, request)
		ExpectResourceNotExists(bundle.ApiService, request)
	})

	DescribeTable("should delete Route leftover from previous version", func(op func() error) {
		route := &routev1.Route{
			ObjectMeta: metav1.ObjectMeta{
				Name:      routeName,
				Namespace: namespace,
				Annotations: map[string]string{
					libhandler.TypeAnnotation:           "SSP.ssp.kubevirt.io",
					libhandler.NamespacedNameAnnotation: namespace + "/" + name,
				},
				Labels: map[string]string{
					common.AppKubernetesNameLabel:      operandName,
					common.AppKubernetesComponentLabel: operandComponent,
					common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
				},
			},
			Spec: routev1.RouteSpec{},
		}

		Expect(request.Client.Create(request.Context, route)).To(Succeed())
		Expect(op()).To(Succeed())
		ExpectResourceNotExists(route.DeepCopy(), request)
	},
		Entry("on reconcile", func() error {
			_, err := operand.Reconcile(&request)
			return err
		}),
		Entry("on cleanup", func() error {
			_, err := operand.Cleanup(&request)
			return err
		}),
	)

	It("should not update service cluster IP", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		key := client.ObjectKeyFromObject(bundle.Service)
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
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		// Set status for deployment
		key := client.ObjectKeyFromObject(bundle.Deployment)
		updateDeploymentStatus(key, &request, func(deploymentStatus *apps.DeploymentStatus) {
			deploymentStatus.Replicas = replicas
			deploymentStatus.ReadyReplicas = 0
			deploymentStatus.AvailableReplicas = 0
			deploymentStatus.UpdatedReplicas = 0
			deploymentStatus.UnavailableReplicas = replicas
		})

		reconcileResults, err := operand.Reconcile(&request)
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

		updateDeploymentStatus(key, &request, func(deploymentStatus *apps.DeploymentStatus) {
			deploymentStatus.Replicas = replicas
			deploymentStatus.ReadyReplicas = replicas
			deploymentStatus.AvailableReplicas = replicas
			deploymentStatus.UpdatedReplicas = replicas
			deploymentStatus.UnavailableReplicas = 0
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

	It("should delete resources when TokenGenerationService is disabled", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		ExpectResourceExists(bundle.ServiceAccount, request)
		for _, clusterRole := range bundle.ClusterRoles {
			ExpectResourceExists(&clusterRole, request)
		}
		ExpectResourceExists(bundle.ClusterRoleBinding, request)
		ExpectResourceExists(bundle.RoleBinding, request)
		ExpectResourceExists(bundle.ConfigMap, request)
		ExpectResourceExists(bundle.Service, request)
		ExpectResourceExists(bundle.Deployment, request)
		ExpectResourceExists(bundle.ApiService, request)

		request.Instance.Spec.TokenGenerationService.Enabled = false

		_, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		ExpectResourceNotExists(bundle.ServiceAccount, request)
		for _, clusterRole := range bundle.ClusterRoles {
			ExpectResourceNotExists(&clusterRole, request)
		}
		ExpectResourceNotExists(bundle.ClusterRoleBinding, request)
		ExpectResourceNotExists(bundle.RoleBinding, request)
		ExpectResourceNotExists(bundle.ConfigMap, request)
		ExpectResourceNotExists(bundle.Service, request)
		ExpectResourceNotExists(bundle.Deployment, request)
		ExpectResourceNotExists(bundle.ApiService, request)
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
			},
			Spec: ssp.SSPSpec{
				TokenGenerationService: &ssp.TokenGenerationService{
					Enabled: true,
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
		ServiceAccount: &core.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceAccountName,
				Namespace: namespace,
				Labels:    map[string]string{"control-plane": "vm-console-proxy"},
			},
		},
		ClusterRoles: []rbac.ClusterRole{{
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
		}, {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "token-generate",
				Namespace: namespace,
				Labels:    map[string]string{},
			},
			Rules: []rbac.PolicyRule{{
				APIGroups: []string{"token.kubevirt.io"},
				Resources: []string{"virtualmachines/vnc"},
				Verbs:     []string{"get"},
			}},
		}},
		ClusterRoleBinding: &rbac.ClusterRoleBinding{
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
		RoleBinding: &rbac.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      roleBindingName,
				Namespace: "kube-system",
			},
			RoleRef: rbac.RoleRef{
				Kind:     "Role",
				Name:     "extension-apiserver-authentication-reader",
				APIGroup: rbac.GroupName,
			},
			Subjects: []rbac.Subject{{
				Kind:      "ServiceAccount",
				Name:      serviceAccountName,
				Namespace: namespace,
			}},
		},
		ConfigMap: &core.ConfigMap{
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
		Service: &core.Service{
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
		Deployment: &apps.Deployment{
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
							ImagePullPolicy: core.PullIfNotPresent,
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
		ApiService: &apiregv1.APIService{
			ObjectMeta: metav1.ObjectMeta{
				Name: "v1alpha1.token.kubevirt.io",
			},
			Spec: apiregv1.APIServiceSpec{
				Group:                "token.kubevirt.io",
				GroupPriorityMinimum: 2000,
				Version:              "v1alpha1",
				VersionPriority:      10,
				Service: &apiregv1.ServiceReference{
					Name:      serviceName,
					Namespace: namespace,
					Port:      ptr.To[int32](443),
				},
			},
		},
	}
}

func updateDeploymentStatus(key client.ObjectKey, request *common.Request, updateFunc func(deploymentStatus *apps.DeploymentStatus)) {
	deployment := &apps.Deployment{}
	Expect(request.Client.Get(request.Context, key, deployment)).ToNot(HaveOccurred())
	updateFunc(&deployment.Status)
	Expect(request.Client.Status().Update(request.Context, deployment)).ToNot(HaveOccurred())
}
