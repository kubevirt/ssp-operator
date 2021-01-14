package template_validator

import (
	"fmt"

	admission "k8s.io/api/admissionregistration/v1"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"kubevirt.io/ssp-operator/internal/common"
)

// Resources from kubevirt-template-validator version v0.7.0
// TODO - move this code to the kubevirt-template-validator
//        repository, and import it as a go module

const (
	containerPort          = 8443
	kubevirtIo             = "kubevirt.io"
	secretName             = "virt-template-validator-certs"
	virtTemplateValidator  = "virt-template-validator"
	ClusterRoleName        = "template:view"
	ClusterRoleBindingName = "template-validator"
	WebhookName            = virtTemplateValidator
	ServiceAccountName     = "template-validator"
	ServiceName            = virtTemplateValidator
	DeploymentName         = virtTemplateValidator
)

var commonLabels = map[string]string{
	kubevirtIo: virtTemplateValidator,
}

func getTemplateValidatorImage() string {
	return common.EnvOrDefault(common.TemplateValidatorImageKey, defaultTemplateValidatorImage)
}

func newClusterRole() *rbac.ClusterRole {
	return &rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ClusterRoleName,
			Namespace: "",
			Labels: map[string]string{
				kubevirtIo: "",
			},
		},
		Rules: []rbac.PolicyRule{{
			APIGroups: []string{"template.openshift.io"},
			Resources: []string{"templates"},
			Verbs:     []string{"get", "list", "watch"},
		}},
	}
}

func newServiceAccount(namespace string) *core.ServiceAccount {
	return &core.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ServiceAccountName,
			Namespace: namespace,
			Labels:    commonLabels,
		},
	}
}

func newClusterRoleBinding(namespace string) *rbac.ClusterRoleBinding {
	return &rbac.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ClusterRoleBindingName,
			Namespace: "",
			Labels:    commonLabels,
		},
		RoleRef: rbac.RoleRef{
			Kind:     "ClusterRole",
			Name:     ClusterRoleName,
			APIGroup: "rbac.authorization.k8s.io",
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
			Labels:    commonLabels,
			Annotations: map[string]string{
				"service.beta.openshift.io/serving-cert-secret-name": secretName,
			},
		},
		Spec: core.ServiceSpec{
			Ports: []core.ServicePort{{
				Name:       "webhook",
				Port:       443,
				TargetPort: intstr.FromInt(containerPort),
			}},
			Selector: map[string]string{
				kubevirtIo: virtTemplateValidator,
			},
		},
	}
}

func newDeployment(namespace string, replicas int32, image string) *apps.Deployment {
	const volumeName = "tls"
	const certMountPath = "/etc/webhook/certs"
	trueVal := true

	return &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeploymentName,
			Namespace: namespace,
			Labels: map[string]string{
				"name": virtTemplateValidator,
			},
		},
		Spec: apps.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					kubevirtIo: virtTemplateValidator,
				},
			},
			Template: core.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:   virtTemplateValidator,
					Labels: commonLabels,
				},
				Spec: core.PodSpec{
					ServiceAccountName: ServiceAccountName,
					Containers: []core.Container{{
						Name:            "webhook",
						Image:           image,
						ImagePullPolicy: core.PullAlways,
						Args: []string{
							"-v=2",
							fmt.Sprintf("--port=%d", containerPort),
							fmt.Sprintf("--cert-dir=%s", certMountPath),
						},
						VolumeMounts: []core.VolumeMount{{
							Name:      volumeName,
							MountPath: certMountPath,
							ReadOnly:  true,
						}},
						SecurityContext: &core.SecurityContext{
							ReadOnlyRootFilesystem: &trueVal,
						},
						Ports: []core.ContainerPort{{
							Name:          "webhook",
							ContainerPort: containerPort,
							Protocol:      core.ProtocolTCP,
						}},
					}},
					Volumes: []core.Volume{{
						Name: volumeName,
						VolumeSource: core.VolumeSource{
							Secret: &core.SecretVolumeSource{
								SecretName: secretName,
							},
						},
					}},
				},
			},
		},
	}
}

func newValidatingWebhook(namespace string) *admission.ValidatingWebhookConfiguration {
	path := "/virtualmachine-template-validate"
	fail := admission.Fail
	sideEffectsNone := admission.SideEffectClassNone

	return &admission.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: WebhookName,
			Annotations: map[string]string{
				"service.beta.openshift.io/inject-cabundle": "true",
			},
		},
		Webhooks: []admission.ValidatingWebhook{{
			Name: "virt-template-admission.kubevirt.io",
			ClientConfig: admission.WebhookClientConfig{
				Service: &admission.ServiceReference{
					Name:      ServiceName,
					Namespace: namespace,
					Path:      &path,
				},
			},
			Rules: []admission.RuleWithOperations{{
				Operations: []admission.OperationType{
					admission.Create, admission.Update,
				},
				Rule: admission.Rule{
					APIGroups:   []string{"kubevirt.io"},
					APIVersions: []string{"v1alpha3"},
					Resources:   []string{"virtualmachines"},
				},
			}},
			FailurePolicy: &fail,
			SideEffects:   &sideEffectsNone,
			// TODO - add "v1" to the list once the template-validator
			//        is updated to new API
			AdmissionReviewVersions: []string{"v1beta1"},
		}},
	}
}
