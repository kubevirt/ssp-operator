package alerts

import (
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

const (
	severityAlertLabelKey     = "severity"
	healthImpactAlertLabelKey = "operator_health_impact"
)

func operatorAlerts() []promv1.Rule {
	return []promv1.Rule{
		{
			Alert: "SSPDown",
			Expr:  intstr.FromString("kubevirt_ssp_operator_up == 0"),
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
			Expr:  intstr.FromString("kubevirt_ssp_template_validator_up == 0"),
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
			Expr:  intstr.FromString("(kubevirt_ssp_operator_reconcile_succeeded_aggregated == 0) and (kubevirt_ssp_operator_up > 0)"),
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
			Expr:  intstr.FromString("kubevirt_ssp_template_validator_rejected_increase > 5"),
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
			Expr:  intstr.FromString("kubevirt_ssp_common_templates_restored_increase > 0"),
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
			Expr:  intstr.FromString("(count(kubevirt_ssp_vm_rbd_block_volume_without_rxbounce > 0) or vector(0)) > 0"),
			Annotations: map[string]string{
				"summary":     "{{ $value }} Virtual Machines may cause reports of bad crc/signature errors due to certain I/O patterns.",
				"description": "When running VMs using ODF storage with 'rbd' mounter or 'rbd.csi.ceph.com provisioner', VMs may cause reports of bad crc/signature errors due to certain I/O patterns. Cluster performance can be severely degraded if the number of re-transmissions due to crc errors causes network saturation.",
			},
			Labels: map[string]string{
				severityAlertLabelKey:     "warning",
				healthImpactAlertLabelKey: "none",
			},
		},
	}
}
