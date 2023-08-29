package metrics

import (
	"github.com/machadovilaca/operator-observability/pkg/operatormetrics"
	runtimemetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

const metricPrefix = "kubevirt_ssp_"

var (
	metrics = [][]operatormetrics.Metric{
		rbdMetrics,
	}
)

func SetupMetrics() {
	operatormetrics.Register = runtimemetrics.Registry.Register

	if err := operatormetrics.RegisterMetrics(metrics...); err != nil {
		panic(err)
	}
}

func ListMetrics() []operatormetrics.Metric {
	return operatormetrics.ListMetrics()
}
