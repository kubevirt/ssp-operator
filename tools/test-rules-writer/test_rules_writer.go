package main

import (
	"encoding/json"
	"fmt"
	"os"

	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	"kubevirt.io/ssp-operator/pkg/monitoring/rules"
)

func main() {
	const runbookTemplate = "test-runbook:%s"

	allRules := append(rules.RecordRules(), rules.AlertRules(runbookTemplate)...)

	spec := promv1.PrometheusRuleSpec{
		Groups: []promv1.RuleGroup{{
			Name:  "test.rules",
			Rules: allRules,
		}},
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(spec); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding prometheus spec: %v", err)
		os.Exit(1)
	}
}
