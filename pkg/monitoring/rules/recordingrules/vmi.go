package recordingrules

import (
	"github.com/machadovilaca/operator-observability/pkg/operatormetrics"
	"github.com/machadovilaca/operator-observability/pkg/operatorrules"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var vmiRecordingRules = []operatorrules.RecordingRule{
	{
		MetricsOpts: operatormetrics.MetricOpts{
			Name: "cnv:vmi_status_running:count",
			Help: "The total number of running VMIs by status",
		},
		MetricType: operatormetrics.GaugeType,
		Expr:       intstr.FromString("sum(kubevirt_vmi_phase_count{phase=\"running\"}) by (node,os,workload,flavor,instance_type,preference)"),
	},
}
