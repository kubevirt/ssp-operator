package metrics

import (
	"github.com/machadovilaca/operator-observability/pkg/operatormetrics"
)

var (
	templateMetrics = []operatormetrics.Metric{
		templateValidatorRejected,
	}

	templateValidatorRejected = operatormetrics.NewCounter(
		operatormetrics.MetricOpts{
			Name: "kubevirt_ssp_template_validator_rejected_total",
			Help: "The total number of rejected template validators",
		},
	)
)

func IncTemplateValidatorRejected() {
	templateValidatorRejected.Inc()
}
