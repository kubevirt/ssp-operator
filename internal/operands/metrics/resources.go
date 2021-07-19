package metrics

import (
	promv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
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
							Record: "num_of_running_ssp_operators",
							Expr:   intstr.FromString("sum(up{pod=~'ssp-operator.*'}) OR on() vector(0)"),
						},
						{
							Record: "num_of_running_template_validator_pods",
							Expr:   intstr.FromString("sum(up{pod=~'virt-template-validator.*'}) OR on() vector(0)"),
						},
						{
							Alert: "SSPDown",
							Expr:  intstr.FromString("num_of_running_ssp_operators == 0"),
							For:   "5m",
							Annotations: map[string]string{
								"summary": "All SSP operator pods are down.",
							},
							Labels: map[string]string{
								"severity": "Critical",
							},
						},
						{
							Alert: "TemplateValidatorDown",
							Expr:  intstr.FromString("num_of_running_template_validator_pods == 0"),
							For:   "5m",
							Annotations: map[string]string{
								"summary": "All Template Validator pods are down.",
							},
							Labels: map[string]string{
								"severity": "Critical",
							},
						},
					},
				},
			},
		},
	}
}
