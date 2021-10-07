package metrics

import (
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const PrometheusRuleName = "prometheus-k8s-rules-cnv"

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
								"runbook_url": "https://kubevirt.io/monitoring/runbooks/SSPOperatorDown",
							},
							Labels: map[string]string{
								"severity": "critical",
							},
						},
						{
							Alert: "SSPTemplateValidatorDown",
							Expr:  intstr.FromString("kubevirt_ssp_template_validator_up_total == 0"),
							For:   "5m",
							Annotations: map[string]string{
								"summary":     "All Template Validator pods are down.",
								"runbook_url": "https://kubevirt.io/monitoring/runbooks/SSPTemplateValidatorDown",
							},
							Labels: map[string]string{
								"severity": "critical",
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
								"runbook_url": "https://kubevirt.io/monitoring/runbooks/SSPHighRateRejectedVms",
							},
							Labels: map[string]string{
								"severity": "warning",
							},
						},
					},
				},
			},
		},
	}
}
