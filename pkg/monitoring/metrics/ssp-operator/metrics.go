package metrics

import (
	"github.com/machadovilaca/operator-observability/pkg/operatormetrics"
	runtimemetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

func SetupMetrics() {
	operatormetrics.Register = runtimemetrics.Registry.Register

	if err := operatormetrics.RegisterMetrics(
		operatorMetrics,
		rbdMetrics,
		templateMetrics,
	); err != nil {
		panic(err)
	}
}
