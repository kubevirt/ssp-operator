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

func SetupRules() error {
	if err := recordingrules.Register(); err != nil {
		return err
	}

	if err := alerts.Register(); err != nil {
		return err
	}

	return nil
}

func BuildPrometheusRule(namespace string) (*promv1.PrometheusRule, error) {
	return operatorrules.BuildPrometheusRule(
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
	return operatorrules.ListAlerts()
}

func ListRecordingRules() []operatorrules.RecordingRule {
	return operatorrules.ListRecordingRules()
}
