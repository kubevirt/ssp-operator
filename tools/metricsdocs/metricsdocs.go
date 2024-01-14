package main

import (
	"fmt"

	"github.com/machadovilaca/operator-observability/pkg/docs"

	sspMetrics "kubevirt.io/ssp-operator/pkg/monitoring/metrics/ssp-operator"
	validatorMetrics "kubevirt.io/ssp-operator/pkg/monitoring/metrics/template-validator"
	"kubevirt.io/ssp-operator/pkg/monitoring/rules"
)

const tpl = `# SSP Operator metrics

{{- range . }}

{{ $deprecatedVersion := "" -}}
{{- with index .ExtraFields "DeprecatedVersion" -}}
    {{- $deprecatedVersion = printf " in %s" . -}}
{{- end -}}

{{- $stabilityLevel := "" -}}
{{- if and (.ExtraFields.StabilityLevel) (ne .ExtraFields.StabilityLevel "STABLE") -}}
	{{- $stabilityLevel = printf "[%s%s] " .ExtraFields.StabilityLevel $deprecatedVersion -}}
{{- end -}}

### {{ .Name }}
{{ print $stabilityLevel }}{{ .Help }}. Type: {{ .Type -}}.

{{- end }}

## Developing new metrics

All metrics documented here are auto-generated and reflect exactly what is being
exposed. After developing new metrics or changing old ones please regenerate
this document.
`

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

	docsString := docs.BuildMetricsDocsWithCustomTemplate(sspMetrics.ListMetrics(), rules.ListRecordingRules(), tpl)

	fmt.Print(docsString)
}
