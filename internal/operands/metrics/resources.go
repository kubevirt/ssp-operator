package metrics

import (
	"fmt"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/utils/ptr"

	"kubevirt.io/ssp-operator/internal/common"
)

const (
	MonitorNamespace             = "openshift-monitoring"
	defaultRunbookURLTemplate    = "https://kubevirt.io/monitoring/runbooks/%s"
	runbookURLTemplateEnv        = "RUNBOOK_URL_TEMPLATE"
	PrometheusLabelKey           = "prometheus.ssp.kubevirt.io"
	PrometheusLabelValue         = "true"
	PrometheusClusterRoleName    = "prometheus-k8s-ssp"
	PrometheusServiceAccountName = "prometheus-k8s"
	MetricsPortName              = "http-metrics"
	MetricsServiceKey            = "metrics.ssp.kubevirt.io"
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

func ServiceMonitorLabels() map[string]string {
	return map[string]string{
		"openshift.io/cluster-monitoring": "true",
		PrometheusLabelKey:                PrometheusLabelValue,
		"k8s-app":                         "kubevirt",
	}
}

func serviceCABundle() promv1.SecretOrConfigMap {
	return promv1.SecretOrConfigMap{
		ConfigMap: &v1.ConfigMapKeySelector{
			LocalObjectReference: v1.LocalObjectReference{
				Name: "openshift-service-ca.crt",
			},
			Key: "service-ca.crt",
		},
	}
}

func olmManagedCABundle() promv1.SecretOrConfigMap {
	return promv1.SecretOrConfigMap{
		Secret: &v1.SecretKeySelector{
			LocalObjectReference: v1.LocalObjectReference{
				Name: "ssp-operator-service-cert",
			},
			Key: "olmCAKey",
		},
	}
}

func getCAConfigForServiceMonitor(olmDeployment bool) promv1.SecretOrConfigMap {
	if olmDeployment {
		return olmManagedCABundle()
	}
	return serviceCABundle()
}

func newValidatorServiceMonitor(request *common.Request) *promv1.ServiceMonitor {
	tlsConfig := &promv1.TLSConfig{
		SafeTLSConfig: promv1.SafeTLSConfig{
			CA: serviceCABundle(),
		},
	}

	tlsConfig.ServerName = ptr.To(fmt.Sprintf("%s.%s.svc", common.VirtTemplateValidator, request.Namespace))
	serviceMonitor := newServiceMonitor(common.TemplateValidatorMetricsServiceName, request.Namespace, tlsConfig, metav1.LabelSelector{
		MatchLabels: map[string]string{
			MetricsServiceKey: common.TemplateValidatorMetricsServiceName,
		},
	})
	return &serviceMonitor
}

func newSspServiceMonitor(request *common.Request) *promv1.ServiceMonitor {
	tlsConfig := &promv1.TLSConfig{
		SafeTLSConfig: promv1.SafeTLSConfig{
			CA: getCAConfigForServiceMonitor(request.OLMDeployment),
		},
	}
	tlsConfig.ServerName = ptr.To(request.SSPServiceHostname)

	serviceMonitor := newServiceMonitor(common.SspOperatorMetricsServiceName, request.Namespace, tlsConfig, metav1.LabelSelector{
		MatchLabels: map[string]string{
			MetricsServiceKey: common.SspOperatorMetricsServiceName,
		},
	})
	return &serviceMonitor
}

func newServiceMonitor(name,
	namespace string,
	tlsConfig *promv1.TLSConfig,
	selector metav1.LabelSelector) promv1.ServiceMonitor {
	return promv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Labels:    ServiceMonitorLabels(),
		},
		Spec: promv1.ServiceMonitorSpec{
			NamespaceSelector: promv1.NamespaceSelector{
				Any: true,
			},
			Selector: selector,
			Endpoints: []promv1.Endpoint{
				{
					Port:        MetricsPortName,
					Scheme:      ptr.To(promv1.Scheme("https")),
					TLSConfig:   tlsConfig,
					HonorLabels: true,
				},
			},
		},
	}
}
