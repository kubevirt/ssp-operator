package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kubevirt/monitoring/pkg/metrics/parser"

	sspMetrics "kubevirt.io/ssp-operator/pkg/monitoring/metrics/ssp-operator"
	validatorMetrics "kubevirt.io/ssp-operator/pkg/monitoring/metrics/template-validator"
	"kubevirt.io/ssp-operator/pkg/monitoring/rules"
)

// This should be used only for very rare cases where the naming conventions that are explained in the best practices:
// https://sdk.operatorframework.io/docs/best-practices/observability-best-practices/#metrics-guidelines
// should be ignored.
var excludedMetrics = map[string]struct{}{
	"cnv:vmi_status_running:count": {},
}

func main() {
	if err := sspMetrics.SetupMetrics(); err != nil {
		panic(err)
	}

	if err := validatorMetrics.SetupMetrics(); err != nil {
		panic(err)
	}

	if err := rules.SetupRules(); err != nil {
		panic(err)
	}

	var metricFamilies []parser.Metric

	metricsList := sspMetrics.ListMetrics()
	for _, m := range metricsList {
		if _, isExcludedMetric := excludedMetrics[m.GetOpts().Name]; !isExcludedMetric {
			metricFamilies = append(metricFamilies, parser.Metric{
				Name: m.GetOpts().Name,
				Help: m.GetOpts().Help,
				Type: strings.ToUpper(string(m.GetBaseType())),
			})
		}
	}

	rulesList := rules.ListRecordingRules()
	for _, r := range rulesList {
		if _, isExcludedMetric := excludedMetrics[r.GetOpts().Name]; !isExcludedMetric {
			metricFamilies = append(metricFamilies, parser.Metric{
				Name: r.GetOpts().Name,
				Help: r.GetOpts().Help,
				Type: strings.ToUpper(string(r.GetType())),
			})
		}
	}

	if jsonBytes, err := json.Marshal(metricFamilies); err != nil {
		panic(err)
	} else {
		fmt.Println(string(jsonBytes))
	}
}
