package metrics

import (
	"github.com/machadovilaca/operator-observability/pkg/operatormetrics"
)

func SetupMetrics() error {
	return operatormetrics.RegisterMetrics(
		templateMetrics,
	)
}

// ListMetrics registered prometheus metrics
func ListMetrics() []operatormetrics.Metric {
	return operatormetrics.ListMetrics()
}
