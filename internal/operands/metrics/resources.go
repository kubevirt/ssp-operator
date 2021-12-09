package metrics

import (
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	PrometheusRuleName       = "prometheus-k8s-rules-cnv"
	runbookURLBasePath       = "http://kubevirt.io/monitoring/runbooks/"
	severityAlertLabelKey    = "severity"
	partOfAlertLabelKey      = "kubernetes_operator_part_of"
	partOfAlertLabelValue    = "kubevirt"
	componentAlertLabelKey   = "kubernetes_operator_component"
	componentAlertLabelValue = "ssp-operator"
)

func newPrometheusRule(namespace string) *promv1.PrometheusRule {
	return &promv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PrometheusRuleName,
			Namespace: namespace,
			Labels: map[string]string{
				"prometheus":  "k8s",
				"role":        "alert-rules",
				"kubevirt.io": "prometheus-rules",
			},
		},
		Spec: promv1.PrometheusRuleSpec{
			Groups: []promv1.RuleGroup{
				{
					Name: "cnv.rules",
					Rules: []promv1.Rule{
						{
							Expr:   intstr.FromString("sum(kubevirt_vmi_phase_count{phase=\"running\"}) by (node,os,workload,flavor)"),
							Record: "cnv:vmi_status_running:count",
						},
						{
							Record: "kubevirt_ssp_operator_up_total",
							Expr:   intstr.FromString("sum(up{pod=~'ssp-operator.*'}) OR on() vector(0)"),
						},
						{
							Record: "kubevirt_ssp_template_validator_up_total",
							Expr:   intstr.FromString("sum(up{pod=~'virt-template-validator.*'}) OR on() vector(0)"),
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
							Record: "kubevirt_ssp_num_of_operator_reconciling_properly",
							Expr:   intstr.FromString("sum(ssp_operator_reconciling_properly)"),
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
							Record: "kubevirt_ssp_rejected_vms_total",
							Expr:   intstr.FromString("sum(increase(total_rejected_vms{pod=~'virt-template-validator.*'}[1h]))"),
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
							Record: "kubevirt_ssp_total_restored_common_templates",
							Expr:   intstr.FromString("sum(increase(total_restored_common_templates{pod=~'ssp-operator.*'}[1h])) OR on() vector(0)"),
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
					},
				},
			},
		},
	}
}
