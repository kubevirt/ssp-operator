package metrics

import "github.com/rhobs/operator-observability-toolkit/pkg/operatormetrics"

var (
	operatorMetrics = []operatormetrics.Metric{
		sspOperatorReconcileSucceeded,
	}

	sspOperatorReconcileSucceeded = operatormetrics.NewGauge(
		operatormetrics.MetricOpts{
			Name: "kubevirt_ssp_operator_reconcile_succeeded",
			Help: "Set to 1 if the reconcile process of all operands completes with no errors, and to 0 otherwise",
		},
	)
)

func SetSspOperatorReconcileSucceeded(isSucceeded bool) {
	value := 0.0
	if isSucceeded {
		value = 1.0
	}
	sspOperatorReconcileSucceeded.Set(value)
}
