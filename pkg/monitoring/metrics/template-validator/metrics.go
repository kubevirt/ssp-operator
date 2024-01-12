package metrics

import (
	"github.com/machadovilaca/operator-observability/pkg/operatormetrics"
)

func SetupMetrics() {
	if err := operatormetrics.RegisterMetrics(
		templateMetrics,
	); err != nil {
		panic(err)
	}
}
