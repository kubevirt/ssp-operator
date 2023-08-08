package metrics

import (
	"errors"
	"fmt"
	"os"
	"strings"

	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	v1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	PrometheusRuleName           = "prometheus-k8s-rules-cnv"
	MonitorNamespace             = "openshift-monitoring"
	defaultRunbookURLTemplate    = "https://kubevirt.io/monitoring/runbooks/%s"
	runbookURLTemplateEnv        = "RUNBOOK_URL_TEMPLATE"
	severityAlertLabelKey        = "severity"
	healthImpactAlertLabelKey    = "operator_health_impact"
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
	CommonTemplatesRestoredIncreaseQuery   = "sum(increase(kubevirt_ssp_common_templates_restored_total{pod=~'ssp-operator.*'}[1h]))"
	TemplateValidatorRejectedIncreaseQuery = "sum(increase(kubevirt_ssp_template_validator_rejected_total{pod=~'virt-template-validator.*'}[1h]))"
)

// RecordRulesDesc represent SSP Operator Prometheus Record Rules
type RecordRulesDesc struct {
	Name        string
	Expr        intstr.IntOrString
	Description string
	Type        string
}

// RecordRulesDescList lists all SSP Operator Prometheus Record Rules
var RecordRulesDescList = []RecordRulesDesc{
	{
		Name:        "kubevirt_ssp_operator_up",
		Expr:        intstr.FromString("sum(up{pod=~'ssp-operator.*'}) OR on() vector(0)"),
		Description: "The total number of running ssp-operator pods",
		Type:        "Gauge",
	},
	{
		Name:        "kubevirt_ssp_template_validator_up",
		Expr:        intstr.FromString("sum(up{pod=~'virt-template-validator.*'}) OR on() vector(0)"),
		Description: "The total number of running virt-template-validator pods",
		Type:        "Gauge",
	},
	{
		Name:        "kubevirt_ssp_operator_reconcile_succeeded_aggregated",
		Expr:        intstr.FromString("sum(kubevirt_ssp_operator_reconcile_succeeded)"),
		Description: "The total number of ssp-operator pods reconciling with no errors",
		Type:        "Gauge",
	},
	{
		Name:        "kubevirt_ssp_template_validator_rejected_increase",
		Expr:        intstr.FromString(TemplateValidatorRejectedIncreaseQuery + " OR on() vector(0)"),
		Description: "The increase in the number of rejected template validators, over the last hour",
		Type:        "Gauge",
	},
	{
		Name:        "kubevirt_ssp_common_templates_restored_increase",
		Expr:        intstr.FromString(CommonTemplatesRestoredIncreaseQuery + " OR on() vector(0)"),
		Description: "The increase in the number of common templates restored by the operator back to their original state, over the last hour",
		Type:        "Gauge",
	},
}

func getAlertRules() ([]promv1.Rule, error) {
	runbookURLTemplate, err := getRunbookURLTemplate()
	if err != nil {
		return nil, err
	}

	return []promv1.Rule{
		{
			Expr:   intstr.FromString("sum(kubevirt_vmi_phase_count{phase=\"running\"}) by (node,os,workload,flavor)"),
			Record: "cnv:vmi_status_running:count",
		},
		{
			Alert: "SSPDown",
			Expr:  intstr.FromString("kubevirt_ssp_operator_up == 0"),
			For:   "5m",
			Annotations: map[string]string{
				"summary":     "All SSP operator pods are down.",
				"runbook_url": fmt.Sprintf(runbookURLTemplate, "SSPDown"),
			},
			Labels: map[string]string{
				severityAlertLabelKey:     "critical",
				healthImpactAlertLabelKey: "critical",
				partOfAlertLabelKey:       partOfAlertLabelValue,
				componentAlertLabelKey:    componentAlertLabelValue,
			},
		},
		{
			Alert: "SSPTemplateValidatorDown",
			Expr:  intstr.FromString("kubevirt_ssp_template_validator_up == 0"),
			For:   "5m",
			Annotations: map[string]string{
				"summary":     "All Template Validator pods are down.",
				"runbook_url": fmt.Sprintf(runbookURLTemplate, "SSPTemplateValidatorDown"),
			},
			Labels: map[string]string{
				severityAlertLabelKey:     "critical",
				healthImpactAlertLabelKey: "critical",
				partOfAlertLabelKey:       partOfAlertLabelValue,
				componentAlertLabelKey:    componentAlertLabelValue,
			},
		},
		{
			Alert: "SSPFailingToReconcile",
			Expr:  intstr.FromString("(kubevirt_ssp_operator_reconcile_succeeded_aggregated == 0) and (kubevirt_ssp_operator_up > 0)"),
			For:   "5m",
			Annotations: map[string]string{
				"summary":     "The ssp-operator pod is up but failing to reconcile",
				"runbook_url": fmt.Sprintf(runbookURLTemplate, "SSPFailingToReconcile"),
			},
			Labels: map[string]string{
				severityAlertLabelKey:     "critical",
				healthImpactAlertLabelKey: "critical",
				partOfAlertLabelKey:       partOfAlertLabelValue,
				componentAlertLabelKey:    componentAlertLabelValue,
			},
		},
		{
			Alert: "SSPHighRateRejectedVms",
			Expr:  intstr.FromString("kubevirt_ssp_template_validator_rejected_increase > 5"),
			For:   "5m",
			Annotations: map[string]string{
				"summary":     "High rate of rejected Vms",
				"runbook_url": fmt.Sprintf(runbookURLTemplate, "SSPHighRateRejectedVms"),
			},
			Labels: map[string]string{
				severityAlertLabelKey:     "warning",
				healthImpactAlertLabelKey: "warning",
				partOfAlertLabelKey:       partOfAlertLabelValue,
				componentAlertLabelKey:    componentAlertLabelValue,
			},
		},
		{
			Alert: "SSPCommonTemplatesModificationReverted",
			Expr:  intstr.FromString("kubevirt_ssp_common_templates_restored_increase > 0"),
			For:   "0m",
			Annotations: map[string]string{
				"summary":     "Common Templates manual modifications were reverted by the operator",
				"runbook_url": fmt.Sprintf(runbookURLTemplate, "SSPCommonTemplatesModificationReverted"),
			},
			Labels: map[string]string{
				severityAlertLabelKey:     "warning",
				healthImpactAlertLabelKey: "none",
				partOfAlertLabelKey:       partOfAlertLabelValue,
				componentAlertLabelKey:    componentAlertLabelValue,
			},
		},
	}, nil
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

func newPrometheusRule(namespace string) (*promv1.PrometheusRule, error) {
	alertRules, err := getAlertRules()
	if err != nil {
		return nil, err
	}

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
					Rules: append(alertRules, getRecordRules()...),
				},
			},
		},
	}, nil
}

func getRunbookURLTemplate() (string, error) {
	runbookURLTemplate, exists := os.LookupEnv(runbookURLTemplateEnv)
	if !exists {
		runbookURLTemplate = defaultRunbookURLTemplate
	}

	if strings.Count(runbookURLTemplate, "%s") != 1 || strings.Count(runbookURLTemplate, "%") != 1 {
		return "", errors.New("runbook URL template must have exactly 1 %s substring")
	}

	return runbookURLTemplate, nil
}
