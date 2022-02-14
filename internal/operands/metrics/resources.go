package metrics

import (
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	v1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	PrometheusRuleName           = "prometheus-k8s-rules-cnv"
	MonitorNamespace             = "openshift-monitoring"
	runbookURLBasePath           = "http://kubevirt.io/monitoring/runbooks/"
	severityAlertLabelKey        = "severity"
	partOfAlertLabelKey          = "kubernetes_operator_part_of"
	partOfAlertLabelValue        = "kubevirt"
	componentAlertLabelKey       = "kubernetes_operator_component"
	componentAlertLabelValue     = "ssp-operator"
	PrometheusLabelKey           = "prometheus.ssp.kubevirt.io"
	PrometheusLabelValue         = "true"
	PrometheusClusterRoleName    = "prometheus-k8s-ssp"
	PrometheusServiceAccountName = "prometheus-k8s"
	MetricsPortName              = "metrics"
)

const (
	Total_restored_common_templates_increase_query = "sum(increase(total_restored_common_templates{pod=~'ssp-operator.*'}[1h]))"
	Total_rejected_vms_increase_query              = "sum(increase(total_rejected_vms{pod=~'virt-template-validator.*'}[1h]))"
)

// RecordRulesDesc represent SSP Operator Prometheus Record Rules
type RecordRulesDesc struct {
	Name        string
	Expr        intstr.IntOrString
	Description string
}

// RecordRulesDescList lists all SSP Operator Prometheus Record Rules
var RecordRulesDescList = []RecordRulesDesc{
	{
		Name:        "kubevirt_ssp_operator_up_total",
		Expr:        intstr.FromString("sum(up{pod=~'ssp-operator.*'}) OR on() vector(0)"),
		Description: "The total number of running ssp-operator pods",
	},
	{
		Name:        "kubevirt_ssp_template_validator_up_total",
		Expr:        intstr.FromString("sum(up{pod=~'virt-template-validator.*'}) OR on() vector(0)"),
		Description: "The total number of running virt-template-validator pods",
	},
	{
		Name:        "kubevirt_ssp_num_of_operator_reconciling_properly",
		Expr:        intstr.FromString("sum(ssp_operator_reconciling_properly)"),
		Description: "The total number of ssp-operator pods reconciling with no errors",
	},
	{
		Name:        "kubevirt_ssp_rejected_vms_total",
		Expr:        intstr.FromString(Total_rejected_vms_increase_query + " OR on() vector(0)"),
		Description: "The total number of vms rejected by virt-template-validator",
	},
	{
		Name:        "kubevirt_ssp_total_restored_common_templates",
		Expr:        intstr.FromString(Total_restored_common_templates_increase_query + " OR on() vector(0)"),
		Description: "The total number of common templates restored by the operator back to their original state",
	},
}

var alertRulesList = []promv1.Rule{
	{
		Expr:   intstr.FromString("sum(kubevirt_vmi_phase_count{phase=\"running\"}) by (node,os,workload,flavor)"),
		Record: "cnv:vmi_status_running:count",
	},
	{
		Alert: "SSPDown",
		Expr:  intstr.FromString("kubevirt_ssp_operator_up_total == 0"),
		For:   "5m",
		Annotations: map[string]string{
			"summary":     "All SSP operator pods are down.",
			"runbook_url": runbookURLBasePath + "SSPOperatorDown",
		},
		Labels: map[string]string{
			severityAlertLabelKey:  "critical",
			partOfAlertLabelKey:    partOfAlertLabelValue,
			componentAlertLabelKey: componentAlertLabelValue,
		},
	},
	{
		Alert: "SSPTemplateValidatorDown",
		Expr:  intstr.FromString("kubevirt_ssp_template_validator_up_total == 0"),
		For:   "5m",
		Annotations: map[string]string{
			"summary":     "All Template Validator pods are down.",
			"runbook_url": runbookURLBasePath + "SSPTemplateValidatorDown",
		},
		Labels: map[string]string{
			severityAlertLabelKey:  "critical",
			partOfAlertLabelKey:    partOfAlertLabelValue,
			componentAlertLabelKey: componentAlertLabelValue,
		},
	},
	{
		Alert: "SSPFailingToReconcile",
		Expr:  intstr.FromString("(kubevirt_ssp_num_of_operator_reconciling_properly == 0) and (kubevirt_ssp_operator_up_total > 0)"),
		For:   "5m",
		Annotations: map[string]string{
			"summary":     "The ssp-operator pod is up but failing to reconcile",
			"runbook_url": runbookURLBasePath + "SSPFailingToReconcile",
		},
		Labels: map[string]string{
			severityAlertLabelKey:  "critical",
			partOfAlertLabelKey:    partOfAlertLabelValue,
			componentAlertLabelKey: componentAlertLabelValue,
		},
	},
	{
		Alert: "SSPHighRateRejectedVms",
		Expr:  intstr.FromString("kubevirt_ssp_rejected_vms_total > 5"),
		For:   "5m",
		Annotations: map[string]string{
			"summary":     "High rate of rejected Vms",
			"runbook_url": runbookURLBasePath + "SSPHighRateRejectedVms",
		},
		Labels: map[string]string{
			severityAlertLabelKey:  "warning",
			partOfAlertLabelKey:    partOfAlertLabelValue,
			componentAlertLabelKey: componentAlertLabelValue,
		},
	},
	{
		Alert: "SSPCommonTemplatesModificationReverted",
		Expr:  intstr.FromString("kubevirt_ssp_total_restored_common_templates > 0"),
		For:   "0m",
		Annotations: map[string]string{
			"summary":     "Common Templates manual modifications were reverted by the operator",
			"runbook_url": runbookURLBasePath + "SSPCommonTemplatesModificationReverted",
		},
		Labels: map[string]string{
			severityAlertLabelKey:  "warning",
			partOfAlertLabelKey:    partOfAlertLabelValue,
			componentAlertLabelKey: componentAlertLabelValue,
		},
	},
}

func getRecordRules() []promv1.Rule {
	var recordRules []promv1.Rule

	for _, rrd := range RecordRulesDescList {
		recordRules = append(recordRules, promv1.Rule{Record: rrd.Name, Expr: rrd.Expr})
	}

	return recordRules
}

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
			Name:      PrometheusRuleName,
			Labels:    ServiceMonitorLabels(),
		},
		Spec: promv1.ServiceMonitorSpec{
			NamespaceSelector: v1.NamespaceSelector{
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

func newPrometheusRule(namespace string) *promv1.PrometheusRule {
	return &promv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PrometheusRuleName,
			Namespace: namespace,
			Labels: map[string]string{
				"prometheus":       "k8s",
				"role":             "alert-rules",
				"kubevirt.io":      "prometheus-rules",
				PrometheusLabelKey: PrometheusLabelValue,
			},
		},
		Spec: promv1.PrometheusRuleSpec{
			Groups: []promv1.RuleGroup{
				{
					Name:  "cnv.rules",
					Rules: append(alertRulesList, getRecordRules()...),
				},
			},
		},
	}
}
