package recordingrules

import (
	"github.com/rhobs/operator-observability-toolkit/pkg/operatormetrics"
	"github.com/rhobs/operator-observability-toolkit/pkg/operatorrules"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var vmiRecordingRules = []operatorrules.RecordingRule{
	{
		MetricsOpts: operatormetrics.MetricOpts{
			Name: "cnv:vmi_status_running:count",
			Help: "The total number of running VMIs, labeled with node, instance type, preference and guest OS information",
		},
		MetricType: operatormetrics.GaugeType,
		Expr: intstr.FromString("sum(kubevirt_vmi_phase_count{phase=\"running\"}) by " +
			"(node,os,workload,flavor,instance_type,preference,guest_os_kernel_release,guest_os_machine,guest_os_name,guest_os_version_id)"),
	},
}
