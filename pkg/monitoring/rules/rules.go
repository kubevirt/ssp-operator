package rules

import (
	"github.com/machadovilaca/operator-observability/pkg/operatorrules"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	"kubevirt.io/ssp-operator/pkg/monitoring/rules/alerts"
	"kubevirt.io/ssp-operator/pkg/monitoring/rules/recordingrules"
)

const (
	RuleName             = "prometheus-k8s-rules-cnv"
	PrometheusKey        = "prometheus"
	PrometheusValue      = "k8s"
	RoleLabelKey         = "role"
	RoleLabelValue       = "alert-rules"
	KubevirtLabelKey     = "kubevirt.io"
	KubevirtLabelValue   = "prometheus-rules"
	PrometheusLabelKey   = "prometheus.ssp.kubevirt.io"
	PrometheusLabelValue = "true"
)

var operatorRegistry = operatorrules.NewRegistry()

func SetupRules() error {
	if err := recordingrules.Register(operatorRegistry); err != nil {
		return err
	}

	if err := alerts.Register(operatorRegistry); err != nil {
		return err
	}

	return nil
}

func BuildPrometheusRule(namespace string) (*promv1.PrometheusRule, error) {
	return operatorRegistry.BuildPrometheusRule(
		RuleName,
		namespace,
		map[string]string{
			PrometheusKey:      PrometheusValue,
			RoleLabelKey:       RoleLabelValue,
			KubevirtLabelKey:   KubevirtLabelValue,
			PrometheusLabelKey: PrometheusLabelValue,
		},
	)
}

func ListAlerts() []promv1.Rule {
	return operatorRegistry.ListAlerts()
}

func ListRecordingRules() []operatorrules.RecordingRule {
	return operatorRegistry.ListRecordingRules()
}
