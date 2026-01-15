package main

import (
	"fmt"

	"github.com/rhobs/operator-observability-toolkit/pkg/docs"

	sspMetrics "kubevirt.io/ssp-operator/pkg/monitoring/metrics/ssp-operator"
	validatorMetrics "kubevirt.io/ssp-operator/pkg/monitoring/metrics/template-validator"
	"kubevirt.io/ssp-operator/pkg/monitoring/rules"
)

const title = `SSP Operator metrics`

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

	docsString := docs.BuildMetricsDocs(title, sspMetrics.ListMetrics(), rules.ListRecordingRules())

	fmt.Print(docsString)
}
