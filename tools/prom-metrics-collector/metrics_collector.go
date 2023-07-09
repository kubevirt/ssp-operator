package main

import (
	parser "github.com/kubevirt/monitoring/pkg/metrics/parser"
	"kubevirt.io/ssp-operator/internal/operands/metrics"

	dto "github.com/prometheus/client_model/go"
)

// excludedMetrics defines the metrics to ignore.
// open bug: https://bugzilla.redhat.com/show_bug.cgi?id=2219763
// Do not add metrics to this list!
var excludedMetrics = map[string]struct{}{
	"kubevirt_ssp_operator_up_total":           {},
	"kubevirt_ssp_template_validator_up_total": {},
	"ssp_operator_reconciling_properly":        {},
	"total_rejected_vms":                       {},
	"total_restored_common_templates":          {},
}

func readMetrics() []*dto.MetricFamily {
	var metricFamilies []*dto.MetricFamily
	sspMetrics := metrics.RecordRulesDescList

	for _, metric := range sspMetrics {
		if _, isExcludedMetric := excludedMetrics[metric.Name]; !isExcludedMetric {
			mf := parser.CreateMetricFamily(parser.Metric{
				Name: metric.Name,
				Help: metric.Description,
				Type: metric.Type,
			})
			metricFamilies = append(metricFamilies, mf)
		}
	}

	return metricFamilies
}
