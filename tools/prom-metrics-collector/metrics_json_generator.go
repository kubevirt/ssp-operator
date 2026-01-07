package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kubevirt/monitoring/pkg/metrics/parser"
	dto "github.com/prometheus/client_model/go"

	sspMetrics "kubevirt.io/ssp-operator/pkg/monitoring/metrics/ssp-operator"
	validatorMetrics "kubevirt.io/ssp-operator/pkg/monitoring/metrics/template-validator"
	"kubevirt.io/ssp-operator/pkg/monitoring/rules"
)

// This should be used only for very rare cases where the naming conventions that are explained in the best practices:
// https://sdk.operatorframework.io/docs/best-practices/observability-best-practices/#metrics-guidelines
// should be ignored.
var excludedMetrics = map[string]struct{}{}

type RecordingRule struct {
	Record string `json:"record,omitempty"`
	Expr   string `json:"expr,omitempty"`
	Type   string `json:"type,omitempty"`
}

type Output struct {
	MetricFamilies []*dto.MetricFamily `json:"metricFamilies,omitempty"`
	RecordingRules []RecordingRule     `json:"recordingRules,omitempty"`
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

	var metricFamilies []*dto.MetricFamily

	metricsList := sspMetrics.ListMetrics()
	for _, m := range metricsList {
		if _, isExcludedMetric := excludedMetrics[m.GetOpts().Name]; !isExcludedMetric {
			pm := parser.Metric{
				Name: m.GetOpts().Name,
				Help: m.GetOpts().Help,
				Type: strings.ToUpper(string(m.GetBaseType())),
			}
			metricFamilies = append(metricFamilies, parser.CreateMetricFamily(pm))
		}
	}

	recNames := make(map[string]struct{})
	var recRules []RecordingRule
	rulesList := rules.ListRecordingRules()
	for _, r := range rulesList {
		name := r.GetOpts().Name
		if _, isExcludedMetric := excludedMetrics[name]; isExcludedMetric {
			continue
		}
		recNames[name] = struct{}{}
		recRules = append(recRules, RecordingRule{
			Record: name,
			Expr:   r.Expr.String(),
			Type:   strings.ToUpper(string(r.GetType())),
		})
	}

	// Filter out metric families that are also recording rules
	var filteredFamilies []*dto.MetricFamily
	for _, mf := range metricFamilies {
		if mf == nil || mf.Name == nil {
			continue
		}
		if _, isRec := recNames[*mf.Name]; isRec {
			continue
		}
		filteredFamilies = append(filteredFamilies, mf)
	}

	out := Output{MetricFamilies: filteredFamilies, RecordingRules: recRules}
	if jsonBytes, err := json.Marshal(out); err != nil {
		panic(err)
	} else {
		fmt.Println(string(jsonBytes))
	}
}
