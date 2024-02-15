package recordingrules

import (
	"github.com/machadovilaca/operator-observability/pkg/operatormetrics"
	"github.com/machadovilaca/operator-observability/pkg/operatorrules"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	CommonTemplatesRestoredIncreaseQuery   = "sum(increase(kubevirt_ssp_common_templates_restored_total{pod=~'ssp-operator.*'}[1h]))"
	TemplateValidatorRejectedIncreaseQuery = "sum(increase(kubevirt_ssp_template_validator_rejected_total{pod=~'virt-template-validator.*'}[1h]))"
)

func operatorRecordingRules() []operatorrules.RecordingRule {
	return []operatorrules.RecordingRule{
		{
			MetricsOpts: operatormetrics.MetricOpts{
				Name: "kubevirt_ssp_operator_up",
				Help: "The total number of running ssp-operator pods",
			},
			MetricType: operatormetrics.GaugeType,
			Expr:       intstr.FromString("sum(up{pod=~'ssp-operator.*'}) OR on() vector(0)"),
		},
		{
			MetricsOpts: operatormetrics.MetricOpts{
				Name: "kubevirt_ssp_template_validator_up",
				Help: "The total number of running virt-template-validator pods",
			},
			MetricType: operatormetrics.GaugeType,
			Expr:       intstr.FromString("sum(up{pod=~'virt-template-validator.*'}) OR on() vector(0)"),
		},
		{
			MetricsOpts: operatormetrics.MetricOpts{
				Name: "kubevirt_ssp_operator_reconcile_succeeded_aggregated",
				Help: "The total number of ssp-operator pods reconciling with no errors",
			},
			MetricType: operatormetrics.GaugeType,
			Expr:       intstr.FromString("sum(kubevirt_ssp_operator_reconcile_succeeded)"),
		},
		{
			MetricsOpts: operatormetrics.MetricOpts{
				Name: "kubevirt_ssp_template_validator_rejected_increase",
				Help: "The increase in the number of rejected template validators, over the last hour",
			},
			MetricType: operatormetrics.GaugeType,
			Expr:       intstr.FromString(TemplateValidatorRejectedIncreaseQuery + " OR on() vector(0)"),
		},
		{
			MetricsOpts: operatormetrics.MetricOpts{
				Name: "kubevirt_ssp_common_templates_restored_increase",
				Help: "The increase in the number of common templates restored by the operator back to their original state, over the last hour",
			},
			MetricType: operatormetrics.GaugeType,
			Expr:       intstr.FromString(CommonTemplatesRestoredIncreaseQuery + " OR on() vector(0)"),
		},
	}
}
