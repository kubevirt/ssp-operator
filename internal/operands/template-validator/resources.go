package template_validator

import (
	"fmt"

	templatev1 "github.com/openshift/api/template/v1"
	admission "k8s.io/api/admissionregistration/v1"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	kubevirt "kubevirt.io/api/core"
	kubevirtv1 "kubevirt.io/api/core/v1"

	"kubevirt.io/ssp-operator/internal/env"
	common_templates "kubevirt.io/ssp-operator/internal/operands/common-templates"
	metrics "kubevirt.io/ssp-operator/internal/operands/metrics"
	"kubevirt.io/ssp-operator/internal/template-validator/tlsinfo"
	webhook "kubevirt.io/ssp-operator/internal/template-validator/webhooks"
)

const (
	ContainerPort                 = 8443
	MetricsPort                   = 8443
	KubevirtIo                    = "kubevirt.io"
	SecretName                    = "virt-template-validator-certs"
	VirtTemplateValidator         = "virt-template-validator"
	ClusterRoleName               = "template:view"
	ClusterRoleBindingName        = "template-validator"
	WebhookName                   = VirtTemplateValidator
	ServiceAccountName            = "template-validator"
	ServiceName                   = VirtTemplateValidator
	MetricsServiceName            = "template-validator-metrics"
	DeploymentName                = VirtTemplateValidator
	ConfigMapName                 = VirtTemplateValidator
	PrometheusLabel               = "prometheus.ssp.kubevirt.io"
	kubernetesHostnameTopologyKey = "kubernetes.io/hostname"
)

func CommonLabels() map[string]string {
	return map[string]string{
		KubevirtIo: VirtTemplateValidator,
	}
}

func getTemplateValidatorImage() string {
	return env.EnvOrDefault(env.TemplateValidatorImageKey, defaultTemplateValidatorImage)
}

func newClusterRole() *rbac.ClusterRole {
	return &rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ClusterRoleName,
			Namespace: "",
			Labels: map[string]string{
				KubevirtIo: "",
			},
		},
		Rules: []rbac.PolicyRule{{
			APIGroups: []string{templatev1.GroupName},
			Resources: []string{"templates"},
			Verbs:     []string{"list", "watch"},
		}, {
			APIGroups: []string{kubevirt.GroupName},
			Resources: []string{"virtualmachines"},
			Verbs:     []string{"list", "watch"},
		}},
	}
}

func newServiceAccount(namespace string) *core.ServiceAccount {
	return &core.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ServiceAccountName,
			Namespace: namespace,
			Labels:    CommonLabels(),
		},
	}
}

func newClusterRoleBinding(namespace string) *rbac.ClusterRoleBinding {
	return &rbac.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ClusterRoleBindingName,
			Namespace: "",
			Labels:    CommonLabels(),
		},
		RoleRef: rbac.RoleRef{
			Kind:     "ClusterRole",
			Name:     ClusterRoleName,
			APIGroup: rbac.GroupName,
		},
		Subjects: []rbac.Subject{{
			Kind:      "ServiceAccount",
			Name:      ServiceAccountName,
			Namespace: namespace,
		}},
	}
}

func newService(namespace string) *core.Service {
	return &core.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ServiceName,
			Namespace: namespace,
			Labels:    CommonLabels(),
			Annotations: map[string]string{
				"service.beta.openshift.io/serving-cert-secret-name": SecretName,
			},
		},
		Spec: core.ServiceSpec{
			Ports: []core.ServicePort{{
				Name:       "webhook",
				Port:       443,
				TargetPort: intstr.FromInt(ContainerPort),
			}},
			Selector: CommonLabels(),
		},
	}
}

func newPodAntiAffinity(key, topologyKey string, operator metav1.LabelSelectorOperator, values []string) *core.PodAntiAffinity {
	return &core.PodAntiAffinity{
		PreferredDuringSchedulingIgnoredDuringExecution: []core.WeightedPodAffinityTerm{
			{
				Weight: 1,
				PodAffinityTerm: core.PodAffinityTerm{
					LabelSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      key,
								Operator: operator,
								Values:   values,
							},
						},
					},
					TopologyKey: topologyKey,
				},
			},
		},
	}
}

func newDeployment(namespace string, replicas int32, image string) *apps.Deployment {
	const secretVolumeName = "tls"
	const configMapVolumeName = "config-map"
	const certMountPath = "/etc/webhook/certs"
	const configMapMountPath = "/tls-options"
	trueVal := true
	falseVal := false

	podLabels := CommonLabels()
	podLabels[PrometheusLabel] = "true"
	podLabels["name"] = DeploymentName
	podAntiAffinity := newPodAntiAffinity(KubevirtIo, kubernetesHostnameTopologyKey, metav1.LabelSelectorOpIn, []string{VirtTemplateValidator})
	return &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeploymentName,
			Namespace: namespace,
			Labels: map[string]string{
				"name": DeploymentName,
			},
		},
		Spec: apps.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: CommonLabels(),
			},
			Template: core.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:   VirtTemplateValidator,
					Labels: podLabels,
				},
				Spec: core.PodSpec{
					SecurityContext: &core.PodSecurityContext{
						RunAsNonRoot: &trueVal,
						SeccompProfile: &core.SeccompProfile{
							Type: core.SeccompProfileTypeRuntimeDefault,
						},
					},
					ServiceAccountName: ServiceAccountName,
					PriorityClassName:  "system-cluster-critical",
					Containers: []core.Container{{
						Name:            "webhook",
						Image:           image,
						ImagePullPolicy: core.PullIfNotPresent,
						Resources: core.ResourceRequirements{
							Requests: core.ResourceList{
								core.ResourceCPU:    resource.MustParse("50m"),
								core.ResourceMemory: resource.MustParse("150Mi"),
							},
						},
						Args: []string{
							fmt.Sprintf("--port=%d", ContainerPort),
							fmt.Sprintf("--cert-dir=%s", certMountPath),
						},
						VolumeMounts: []core.VolumeMount{{
							Name:      secretVolumeName,
							MountPath: certMountPath,
							ReadOnly:  true,
						}, {
							Name:      configMapVolumeName,
							MountPath: configMapMountPath,
							ReadOnly:  true,
						}},
						SecurityContext: &core.SecurityContext{
							ReadOnlyRootFilesystem:   &trueVal,
							AllowPrivilegeEscalation: &falseVal,
							Capabilities: &core.Capabilities{
								Drop: []core.Capability{"ALL"},
							},
						},
						Ports: []core.ContainerPort{{
							Name:          "webhook",
							ContainerPort: ContainerPort,
							Protocol:      core.ProtocolTCP,
						}, {
							Name:          metrics.MetricsPortName,
							ContainerPort: MetricsPort,
							Protocol:      core.ProtocolTCP,
						}},
						ReadinessProbe: &core.Probe{
							ProbeHandler: core.ProbeHandler{
								HTTPGet: &core.HTTPGetAction{
									Path:   "/readyz",
									Port:   intstr.FromInt(ContainerPort),
									Scheme: core.URISchemeHTTPS,
								},
							},
							InitialDelaySeconds: 5,
							PeriodSeconds:       10,
						},
					}},
					Volumes: []core.Volume{{
						Name: secretVolumeName,
						VolumeSource: core.VolumeSource{
							Secret: &core.SecretVolumeSource{
								SecretName: SecretName,
							},
						},
					}, {
						Name: configMapVolumeName,
						VolumeSource: core.VolumeSource{
							ConfigMap: &core.ConfigMapVolumeSource{
								LocalObjectReference: core.LocalObjectReference{
									Name: ConfigMapName,
								},
							},
						},
					}},
					Affinity: &core.Affinity{
						PodAntiAffinity: podAntiAffinity,
					},
				},
			},
		},
	}
}

func newConfigMap(namespace string, tlsOptionsJson string) *core.ConfigMap {
	return &core.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConfigMapName,
			Namespace: namespace,
			Labels:    CommonLabels(),
		},
		Data: map[string]string{
			tlsinfo.TLSOptionsFilename: tlsOptionsJson,
		},
	}
}

func newValidatingWebhook(serviceNamespace string) *admission.ValidatingWebhookConfiguration {
	fail := admission.Fail
	sideEffectsNone := admission.SideEffectClassNone

	var vmRules []admission.RuleWithOperations
	for _, version := range kubevirtv1.ApiSupportedWebhookVersions {
		vmRules = append(vmRules, admission.RuleWithOperations{
			Operations: []admission.OperationType{
				admission.Create, admission.Update,
			},
			Rule: admission.Rule{
				APIGroups:   []string{kubevirt.GroupName},
				APIVersions: []string{version},
				Resources:   []string{"virtualmachines"},
			},
		})
	}

	return &admission.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: WebhookName,
			Annotations: map[string]string{
				"service.beta.openshift.io/inject-cabundle": "true",
			},
		},
		Webhooks: []admission.ValidatingWebhook{{
			Name: "virtualmachine-admission.ssp.kubevirt.io",
			ClientConfig: admission.WebhookClientConfig{
				Service: &admission.ServiceReference{
					Name:      ServiceName,
					Namespace: serviceNamespace,
					Path:      ptr.To(webhook.VmValidatePath),
				},
			},
			Rules:                   vmRules,
			FailurePolicy:           &fail,
			SideEffects:             &sideEffectsNone,
			AdmissionReviewVersions: []string{"v1"},
		}, {
			Name: "template-admission.ssp.kubevirt.io",
			ClientConfig: admission.WebhookClientConfig{
				Service: &admission.ServiceReference{
					Name:      ServiceName,
					Namespace: serviceNamespace,
					Path:      ptr.To(webhook.TemplateValidatePath),
				},
			},
			ObjectSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					common_templates.TemplateTypeLabel: common_templates.TemplateTypeLabelBaseValue,
				},
			},
			Rules: []admission.RuleWithOperations{{
				Operations: []admission.OperationType{
					admission.Delete,
				},
				Rule: admission.Rule{
					APIGroups:   []string{templatev1.GroupVersion.Group},
					APIVersions: []string{templatev1.GroupVersion.Version},
					Resources:   []string{"templates"},
				},
			}},
			FailurePolicy:           &fail,
			SideEffects:             &sideEffectsNone,
			AdmissionReviewVersions: []string{"v1"},
		}},
	}
}

func PrometheusServiceLabels() map[string]string {
	return map[string]string{
		metrics.PrometheusLabelKey: metrics.PrometheusLabelValue,
	}
}

func newPrometheusService(namespace string) *core.Service {
	return &core.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      MetricsServiceName,
			Labels:    PrometheusServiceLabels(),
		},
		Spec: core.ServiceSpec{
			Selector: CommonLabels(),
			Ports: []core.ServicePort{
				{
					Name:       metrics.MetricsPortName,
					Port:       443,
					TargetPort: intstr.FromString(metrics.MetricsPortName),
					Protocol:   core.ProtocolTCP,
				},
			},
			Type: core.ServiceTypeClusterIP,
		},
	}
}
