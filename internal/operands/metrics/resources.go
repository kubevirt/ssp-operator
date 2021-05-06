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
			Groups: []promv1.RuleGroup{{
				Name: "cnv.rules",
				Rules: []promv1.Rule{{
					Expr:   intstr.FromString("sum(kubevirt_vmi_phase_count{phase=\"running\"}) by (node,os,workload,flavor)"),
					Record: "cnv:vmi_status_running:count",
				}},
			}},
		},
	}
}
