package metrics

import (
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubevirt.io/ssp-operator/pkg/monitoring/rules"
)

const (
	MonitorNamespace             = "openshift-monitoring"
	defaultRunbookURLTemplate    = "https://kubevirt.io/monitoring/runbooks/%s"
	runbookURLTemplateEnv        = "RUNBOOK_URL_TEMPLATE"
	PrometheusLabelKey           = "prometheus.ssp.kubevirt.io"
	PrometheusLabelValue         = "true"
	PrometheusClusterRoleName    = "prometheus-k8s-ssp"
	PrometheusServiceAccountName = "prometheus-k8s"
	MetricsPortName              = "metrics"
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

func newServiceMonitorCR(namespace string) *promv1.ServiceMonitor {
	return &promv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      rules.RuleName,
			Labels:    ServiceMonitorLabels(),
		},
		Spec: promv1.ServiceMonitorSpec{
			NamespaceSelector: promv1.NamespaceSelector{
				Any: true,
			},
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					PrometheusLabelKey: PrometheusLabelValue,
				},
			},
			Endpoints: []promv1.Endpoint{
				{
					Port:   MetricsPortName,
					Scheme: "https",
					TLSConfig: &promv1.TLSConfig{
						SafeTLSConfig: promv1.SafeTLSConfig{
							InsecureSkipVerify: true,
						},
					},
					HonorLabels: true,
				},
			},
		},
	}
}
