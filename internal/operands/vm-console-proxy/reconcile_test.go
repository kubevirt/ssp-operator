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
	vm_console_proxy_bundle "kubevirt.io/ssp-operator/internal/vm-console-proxy-bundle"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
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
	var v *vmConsoleProxy
	var mockedRequest *common.Request

	BeforeEach(func() {
		v = getMockedVmConsoleProxyOperand()
		mockedRequest = getMockedRequest()
	})

	It("should return new with test bundle correctly", func() {
		vmConsoleProxy := New(getMockedTestBundle())
		Expect(vmConsoleProxy.serviceAccount).ToNot(BeNil(), "service account should not be nil")
		Expect(vmConsoleProxy.clusterRole).ToNot(BeNil(), "cluster role should not be nil")
		Expect(vmConsoleProxy.clusterRoleBinding).ToNot(BeNil(), "cluster role binding should not be nil")
		Expect(vmConsoleProxy.service).ToNot(BeNil(), "service should not be nil")
		Expect(vmConsoleProxy.deployment).ToNot(BeNil(), "deployment should not be nil")
		Expect(vmConsoleProxy.configMap).ToNot(BeNil(), "config map should not be nil")
	})

	It("should return name correctly", func() {
		name := v.Name()
		Expect(name).To(Equal(operandName), "should return correct name")
	})

	It("should return functions from reconcile correclty", func() {
		functions, err := v.Reconcile(mockedRequest)
		Expect(err).ToNot(HaveOccurred(), "should not throw err")
		Expect(len(functions)).To(Equal(6), "should return correct number of reconcile functions")
	})
})

func TestVmConsoleProxyBundle(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "VM Console Proxy Operand Suite")
}

func getMockedRequest() *common.Request {
	var log = logf.Log.WithName("metrics_operand")
	client := fake.NewClientBuilder().WithScheme(common.Scheme).Build()

	return &common.Request{
		Request: reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: "kubevirt",
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
				Name:        name,
				Namespace:   "kubevirt",
				Annotations: map[string]string{enableAnnotation: "true"},
			},
		},
		Logger:       log,
		VersionCache: common.VersionCache{},
	}
}

func getMockedVmConsoleProxyOperand() *vmConsoleProxy {
	replicas := int32(1)

	return &vmConsoleProxy{
		serviceAccount: &core.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceAccountName,
				Namespace: "",
				Labels:    map[string]string{"control-plane": "vm-console-proxy"},
			},
		},
		clusterRole: &rbac.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterRoleName,
				Namespace: "",
				Labels:    map[string]string{},
			},
		},
		clusterRoleBinding: &rbac.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterRoleBindingName,
				Namespace: "",
				Labels:    map[string]string{},
			},
		},
		service: &core.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: "",
				Labels:    map[string]string{},
				Annotations: map[string]string{
					"service.beta.openshift.io/serving-cert-secret-name": "vm-console-proxy-cert",
				},
			},
		},
		deployment: &apps.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deploymentName,
				Namespace: "",
				Labels: map[string]string{
					"name":                         deploymentName,
					"vm-console-proxy.kubevirt.io": deploymentName,
				},
			},
			Spec: apps.DeploymentSpec{
				Replicas: &replicas,
				Template: core.PodTemplateSpec{
					Spec: core.PodSpec{
						Containers: []core.Container{{
							Name:  "test-container",
							Image: "test-image",
						}},
					},
				},
			},
		},
		configMap: &core.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: "",
			},
		},
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
				Namespace: "",
				Labels:    map[string]string{"control-plane": "vm-console-proxy"},
			},
		},
		ClusterRole: rbac.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterRoleName,
				Namespace: "",
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
				Namespace: "",
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
				Namespace: "",
			}},
		},
		Service: core.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: "",
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
				Namespace: "",
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
