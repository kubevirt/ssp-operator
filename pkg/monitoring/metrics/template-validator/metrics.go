package metrics

import (
	"github.com/rhobs/operator-observability-toolkit/pkg/operatormetrics"
)

func SetupMetrics() error {
	return operatormetrics.RegisterMetrics(
		templateMetrics,
	)
}
