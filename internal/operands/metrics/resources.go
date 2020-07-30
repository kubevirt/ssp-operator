package metrics

import (
	"github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func newPrometheusRule(namespace string) *v1.PrometheusRule {
	return &v1.PrometheusRule{
		ObjectMeta: v12.ObjectMeta{
			Name:      "prometheus-k8s-rules-cnv",
			Namespace: namespace,
			Labels: map[string]string{
				"prometheus":  "k8s",
				"role":        "alert-rules",
				"kubevirt.io": "prometheus-rules",
			},
		},
		Spec: v1.PrometheusRuleSpec{
			Groups: []v1.RuleGroup{{
				Name: "cnv.rules",
				Rules: []v1.Rule{{
					Expr:   intstr.FromString("sum(kubevirt_vmi_phase_count{phase=\"running\"}) by (node)"),
					Record: "cnv:vmi_status_running:count",
				}},
			}},
		},
	}
}
