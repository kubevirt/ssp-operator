package metrics

import (
	"fmt"

	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"kubevirt.io/ssp-operator/internal/common"
	template_validator "kubevirt.io/ssp-operator/internal/operands/template-validator"
	"kubevirt.io/ssp-operator/internal/resources"
)

const (
	MonitorNamespace             = "openshift-monitoring"
	defaultRunbookURLTemplate    = "https://kubevirt.io/monitoring/runbooks/%s"
	runbookURLTemplateEnv        = "RUNBOOK_URL_TEMPLATE"
	PrometheusClusterRoleName    = "prometheus-k8s-ssp"
	PrometheusServiceAccountName = "prometheus-k8s"

	ServiceCABundle    = "openshift-service-ca.crt"
	ServiceCABUndleKey = "service-ca.crt"
	OLMManagedCert     = "ssp-operator-service-cert"
	OLMManagedCertKey  = "olmCAKey"
)

func newMonitoringClusterRole() *rbac.ClusterRole {
	return &rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: PrometheusClusterRoleName,
		},
		Rules: []rbac.PolicyRule{{
			APIGroups: []string{""},
			Resources: []string{"services", "endpoints", "pods"},
			Verbs:     []string{"get", "list", "watch"},
		}},
	}
}

func newMonitoringClusterRoleBinding() *rbac.ClusterRoleBinding {
	return &rbac.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: PrometheusClusterRoleName,
		},
		Subjects: []rbac.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      PrometheusServiceAccountName,
				Namespace: MonitorNamespace,
			},
		},
		RoleRef: rbac.RoleRef{
			Kind:     "ClusterRole",
			Name:     PrometheusClusterRoleName,
			APIGroup: rbac.GroupName,
		},
	}
}

func serviceMonitorLabels(serviceLabels map[string]string) map[string]string {
	labels := map[string]string{
		"openshift.io/cluster-monitoring": "true",
		"k8s-app":                         "kubevirt",
	}

	for k, v := range serviceLabels {
		labels[k] = v
	}

	return labels
}

func serviceCABundle() promv1.SecretOrConfigMap {
	return promv1.SecretOrConfigMap{
		ConfigMap: &v1.ConfigMapKeySelector{
			LocalObjectReference: v1.LocalObjectReference{
				Name: ServiceCABundle,
			},
			Key: ServiceCABUndleKey,
		},
	}
}

func olmManagedCABundle() promv1.SecretOrConfigMap {
	return promv1.SecretOrConfigMap{
		Secret: &v1.SecretKeySelector{
			LocalObjectReference: v1.LocalObjectReference{
				Name: OLMManagedCert,
			},
			Key: OLMManagedCertKey,
		},
	}
}

// when deployed via OLM, we need to retrieve the CABundle from the secret containing the certificate itself,
// otherwise certificate issued by service-ca operator is used and the CABundle is available in a configmap
// which is present in every namespace
func getCAConfigForServiceMonitor(olmDeployment bool) promv1.SecretOrConfigMap {
	if olmDeployment {
		return olmManagedCABundle()
	}
	return serviceCABundle()
}

func TemplateValidatorServiceMonitorLabels() map[string]string {
	return serviceMonitorLabels(
		template_validator.PrometheusServiceLabels(),
	)
}

func newValidatorServiceMonitor(request common.Request) *promv1.ServiceMonitor {
	tlsConfig := &promv1.TLSConfig{
		SafeTLSConfig: promv1.SafeTLSConfig{
			CA:         serviceCABundle(),
			ServerName: ptr.To(fmt.Sprintf("%s.%s.svc", template_validator.VirtTemplateValidator, request.Namespace)),
		},
	}

	return &promv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      template_validator.MetricsServiceName,
			Namespace: request.Namespace,
			Labels:    TemplateValidatorServiceMonitorLabels(),
		},
		Spec: promv1.ServiceMonitorSpec{
			NamespaceSelector: promv1.NamespaceSelector{
				Any: true,
			},
			Selector: metav1.LabelSelector{
				MatchLabels: template_validator.PrometheusServiceLabels(),
			},
			Endpoints: []promv1.Endpoint{
				{
					Port:        template_validator.MetricsPortName,
					Scheme:      "https",
					TLSConfig:   tlsConfig,
					HonorLabels: true,
				},
			},
		},
	}
}

func SspServiceMonitorLabels() map[string]string {
	return serviceMonitorLabels(
		resources.PrometheusServiceLabels(),
	)
}

func newSspServiceMonitor(request common.Request) *promv1.ServiceMonitor {
	tlsConfig := &promv1.TLSConfig{
		SafeTLSConfig: promv1.SafeTLSConfig{
			CA:         getCAConfigForServiceMonitor(request.OLMDeployment),
			ServerName: ptr.To(request.SSPServiceHostname),
		},
	}

	return &promv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resources.MetricsServiceName,
			Namespace: request.Namespace,
			Labels:    SspServiceMonitorLabels(),
		},
		Spec: promv1.ServiceMonitorSpec{
			NamespaceSelector: promv1.NamespaceSelector{
				Any: true,
			},
			Selector: metav1.LabelSelector{
				MatchLabels: resources.PrometheusServiceLabels(),
			},
			Endpoints: []promv1.Endpoint{
				{
					Port:        resources.MetricsPortName,
					Scheme:      "https",
					TLSConfig:   tlsConfig,
					HonorLabels: true,
				},
			},
		},
	}
}
