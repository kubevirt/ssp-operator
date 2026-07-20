package alerts

import (
	"fmt"

	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

// withVMLabel wraps a PromQL expression with label_replace to add a "vm"
// typed resource label derived from the "name" label. This enables the
// monitoring-plugin to navigate from alerts to the VM resource page.
func withVMLabel(expr string) string {
	return fmt.Sprintf(`label_replace(%s, "vm", "$1", "name", "(.+)")`, expr)
}

const (
	severityAlertLabelKey     = "severity"
	healthImpactAlertLabelKey = "operator_health_impact"
)

func operatorAlerts() []promv1.Rule {
	return []promv1.Rule{
		{
			Alert: "SSPDown",
			Expr:  intstr.FromString("cluster:kubevirt_ssp_operator_up:sum == 0"),
			For:   ptr.To[promv1.Duration]("5m"),
			Annotations: map[string]string{
				"summary": "All SSP operator pods are down.",
			},
			Labels: map[string]string{
				severityAlertLabelKey:     "critical",
				healthImpactAlertLabelKey: "critical",
			},
		},
		{
			Alert: "SSPTemplateValidatorDown",
			Expr:  intstr.FromString("cluster:kubevirt_ssp_template_validator_up:sum == 0"),
			For:   ptr.To[promv1.Duration]("5m"),
			Annotations: map[string]string{
				"summary": "All Template Validator pods are down.",
			},
			Labels: map[string]string{
				severityAlertLabelKey:     "critical",
				healthImpactAlertLabelKey: "critical",
			},
		},
		{
			Alert: "SSPFailingToReconcile",
			Expr:  intstr.FromString("(cluster:kubevirt_ssp_operator_reconcile_succeeded:sum == 0) and (cluster:kubevirt_ssp_operator_up:sum > 0)"),
			For:   ptr.To[promv1.Duration]("5m"),
			Annotations: map[string]string{
				"summary": "The ssp-operator pod is up but failing to reconcile.",
			},
			Labels: map[string]string{
				severityAlertLabelKey:     "critical",
				healthImpactAlertLabelKey: "critical",
			},
		},
		{
			Alert: "SSPHighRateRejectedVms",
			Expr:  intstr.FromString("cluster:kubevirt_ssp_template_validator_rejected:increase1h > 5"),
			For:   ptr.To[promv1.Duration]("5m"),
			Annotations: map[string]string{
				"summary": "High rate of rejected Vms.",
			},
			Labels: map[string]string{
				severityAlertLabelKey:     "warning",
				healthImpactAlertLabelKey: "warning",
			},
		},
		{
			Alert: "SSPCommonTemplatesModificationReverted",
			Expr:  intstr.FromString("cluster:kubevirt_ssp_common_templates_restored:increase1h > 0"),
			For:   ptr.To[promv1.Duration]("0m"),
			Annotations: map[string]string{
				"summary": "Common Templates manual modifications were reverted by the operator.",
			},
			Labels: map[string]string{
				severityAlertLabelKey:     "warning",
				healthImpactAlertLabelKey: "none",
			},
		},
		{
			Alert: "VMStorageClassWarning",
			Expr: intstr.FromString(withVMLabel(
				`kubevirt_ssp_vm_rbd_block_volume_without_rxbounce * on(name, namespace) (kubevirt_vmi_info{guest_os_name="Microsoft Windows"} > 0 or kubevirt_vmi_info{os=~"windows.*"} > 0) > 0`,
			)),
			Annotations: map[string]string{
				"summary":     "Virtual Machine '{{ $labels.name }}' in namespace '{{ $labels.namespace }}' may cause reports of bad crc/signature errors due to certain I/O patterns.",
				"description": "When running Windows VMs using ODF storage with 'rbd' mounter or 'rbd.csi.ceph.com provisioner', the VM '{{ $labels.name }}' may cause reports of bad crc/signature errors due to certain I/O patterns. Cluster performance can be severely degraded if the number of re-transmissions due to crc errors causes network saturation.",
			},
			Labels: map[string]string{
				severityAlertLabelKey:     "warning",
				healthImpactAlertLabelKey: "none",
			},
		},
	}
}
