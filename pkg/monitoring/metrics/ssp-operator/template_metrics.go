package metrics

import (
	"github.com/machadovilaca/operator-observability/pkg/operatormetrics"
	ioprometheusclient "github.com/prometheus/client_model/go"
)

var (
	templateMetrics = []operatormetrics.Metric{
		commonTemplatesRestored,
	}

	commonTemplatesRestored = operatormetrics.NewCounter(
		operatormetrics.MetricOpts{
			Name: "kubevirt_ssp_common_templates_restored_total",
			Help: "The total number of common templates restored by the operator back to their original state",
		},
	)
)

func IncCommonTemplatesRestored() {
	commonTemplatesRestored.Inc()
}

func GetCommonTemplatesRestored() (float64, error) {
	dto := &ioprometheusclient.Metric{}
	err := commonTemplatesRestored.Write(dto)
	if err != nil {
		return 0, err
	}
	return dto.Counter.GetValue(), nil
}
