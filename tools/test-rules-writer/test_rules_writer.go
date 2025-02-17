package main

import (
	"encoding/json"
	"fmt"
	"os"

	"kubevirt.io/ssp-operator/pkg/monitoring/rules"
)

func main() {
	if err := rules.SetupRules(); err != nil {
		panic(err)
	}

	pr, err := rules.BuildPrometheusRule("testnamespace")
	if err != nil {
		panic(err)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(pr.Spec); err != nil {
		// Ignoring returned error: no reasonable way to handle it.
		_, _ = fmt.Fprintf(os.Stderr, "Error encoding prometheus spec: %v", err)
		os.Exit(1)
	}
}
