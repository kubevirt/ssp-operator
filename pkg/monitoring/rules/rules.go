package rules

import (
	"fmt"

	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

const (
	severityAlertLabelKey     = "severity"
	healthImpactAlertLabelKey = "operator_health_impact"
	partOfAlertLabelKey       = "kubernetes_operator_part_of"
	partOfAlertLabelValue     = "kubevirt"
	componentAlertLabelKey    = "kubernetes_operator_component"
	componentAlertLabelValue  = "ssp-operator"
)

const (
	CommonTemplatesRestoredIncreaseQuery   = "sum(increase(kubevirt_ssp_common_templates_restored_total{pod=~'ssp-operator.*'}[1h]))"
	TemplateValidatorRejectedIncreaseQuery = "sum(increase(kubevirt_ssp_template_validator_rejected_total{pod=~'virt-template-validator.*'}[1h]))"
)

// RecordRulesDesc represent SSP Operator Prometheus Record Rules
type RecordRulesDesc struct {
	Name        string
	Expr        intstr.IntOrString
	Description string
	Type        string
}

// recordRulesDescList lists all SSP Operator Prometheus Record Rules
var recordRulesDescList = []RecordRulesDesc{
	{
		Name:        "kubevirt_ssp_operator_up",
		Expr:        intstr.FromString("sum(up{pod=~'ssp-operator.*'}) OR on() vector(0)"),
		Description: "The total number of running ssp-operator pods",
		Type:        "Gauge",
	},
	{
		Name:        "kubevirt_ssp_template_validator_up",
		Expr:        intstr.FromString("sum(up{pod=~'virt-template-validator.*'}) OR on() vector(0)"),
		Description: "The total number of running virt-template-validator pods",
		Type:        "Gauge",
	},
	{
		Name:        "kubevirt_ssp_operator_reconcile_succeeded_aggregated",
		Expr:        intstr.FromString("sum(kubevirt_ssp_operator_reconcile_succeeded)"),
		Description: "The total number of ssp-operator pods reconciling with no errors",
		Type:        "Gauge",
	},
	{
		Name:        "kubevirt_ssp_template_validator_rejected_increase",
		Expr:        intstr.FromString(TemplateValidatorRejectedIncreaseQuery + " OR on() vector(0)"),
		Description: "The increase in the number of rejected template validators, over the last hour",
		Type:        "Gauge",
	},
	{
		Name:        "kubevirt_ssp_common_templates_restored_increase",
		Expr:        intstr.FromString(CommonTemplatesRestoredIncreaseQuery + " OR on() vector(0)"),
		Description: "The increase in the number of common templates restored by the operator back to their original state, over the last hour",
		Type:        "Gauge",
	},
}

func RecordRules() []promv1.Rule {
	result := make([]promv1.Rule, 0, len(recordRulesDescList))
	for _, rrd := range recordRulesDescList {
		result = append(result, promv1.Rule{Record: rrd.Name, Expr: rrd.Expr})
	}
	return result
}

func RecordRulesWithDescriptions() []RecordRulesDesc {
	result := make([]RecordRulesDesc, 0, len(recordRulesDescList))
	for _, rrd := range recordRulesDescList {
		result = append(result, rrd)
	}
	return result
}

func AlertRules(runbookURLTemplate string) []promv1.Rule {
	return []promv1.Rule{
		{
			Expr:   intstr.FromString("sum(kubevirt_vmi_phase_count{phase=\"running\"}) by (node,os,workload,flavor,instance_type,preference)"),
			Record: "cnv:vmi_status_running:count",
		},
		{
			Alert: "SSPDown",
			Expr:  intstr.FromString("kubevirt_ssp_operator_up == 0"),
			For:   ptr.To[promv1.Duration]("5m"),
			Annotations: map[string]string{
				"summary":     "All SSP operator pods are down.",
				"runbook_url": fmt.Sprintf(runbookURLTemplate, "SSPDown"),
			},
			Labels: map[string]string{
				severityAlertLabelKey:     "critical",
				healthImpactAlertLabelKey: "critical",
				partOfAlertLabelKey:       partOfAlertLabelValue,
				componentAlertLabelKey:    componentAlertLabelValue,
			},
		},
		{
			Alert: "SSPTemplateValidatorDown",
			Expr:  intstr.FromString("kubevirt_ssp_template_validator_up == 0"),
			For:   ptr.To[promv1.Duration]("5m"),
			Annotations: map[string]string{
				"summary":     "All Template Validator pods are down.",
				"runbook_url": fmt.Sprintf(runbookURLTemplate, "SSPTemplateValidatorDown"),
			},
			Labels: map[string]string{
				severityAlertLabelKey:     "critical",
				healthImpactAlertLabelKey: "critical",
				partOfAlertLabelKey:       partOfAlertLabelValue,
				componentAlertLabelKey:    componentAlertLabelValue,
			},
		},
		{
			Alert: "SSPFailingToReconcile",
			Expr:  intstr.FromString("(kubevirt_ssp_operator_reconcile_succeeded_aggregated == 0) and (kubevirt_ssp_operator_up > 0)"),
			For:   ptr.To[promv1.Duration]("5m"),
			Annotations: map[string]string{
				"summary":     "The ssp-operator pod is up but failing to reconcile",
				"runbook_url": fmt.Sprintf(runbookURLTemplate, "SSPFailingToReconcile"),
			},
			Labels: map[string]string{
				severityAlertLabelKey:     "critical",
				healthImpactAlertLabelKey: "critical",
				partOfAlertLabelKey:       partOfAlertLabelValue,
				componentAlertLabelKey:    componentAlertLabelValue,
			},
		},
		{
			Alert: "SSPHighRateRejectedVms",
			Expr:  intstr.FromString("kubevirt_ssp_template_validator_rejected_increase > 5"),
			For:   ptr.To[promv1.Duration]("5m"),
			Annotations: map[string]string{
				"summary":     "High rate of rejected Vms",
				"runbook_url": fmt.Sprintf(runbookURLTemplate, "SSPHighRateRejectedVms"),
			},
			Labels: map[string]string{
				severityAlertLabelKey:     "warning",
				healthImpactAlertLabelKey: "warning",
				partOfAlertLabelKey:       partOfAlertLabelValue,
				componentAlertLabelKey:    componentAlertLabelValue,
			},
		},
		{
			Alert: "SSPCommonTemplatesModificationReverted",
			Expr:  intstr.FromString("kubevirt_ssp_common_templates_restored_increase > 0"),
			For:   ptr.To[promv1.Duration]("0m"),
			Annotations: map[string]string{
				"summary":     "Common Templates manual modifications were reverted by the operator",
				"runbook_url": fmt.Sprintf(runbookURLTemplate, "SSPCommonTemplatesModificationReverted"),
			},
			Labels: map[string]string{
				severityAlertLabelKey:     "warning",
				healthImpactAlertLabelKey: "none",
				partOfAlertLabelKey:       partOfAlertLabelValue,
				componentAlertLabelKey:    componentAlertLabelValue,
			},
		},
		{
			Alert: "VMStorageClassWarning",
			Expr:  intstr.FromString("(count(kubevirt_ssp_vm_rbd_block_volume_without_rxbounce > 0) or vector(0)) > 0"),
			Annotations: map[string]string{
				"summary":     "{{ $value }} Virtual Machines may cause reports of bad crc/signature errors due to certain I/O patterns",
				"description": "When running VMs using ODF storage with 'rbd' mounter or 'rbd.csi.ceph.com provisioner', VMs may cause reports of bad crc/signature errors due to certain I/O patterns. Cluster performance can be severely degraded if the number of re-transmissions due to crc errors causes network saturation.",
				"runbook_url": fmt.Sprintf(runbookURLTemplate, "VMStorageClassWarning"),
			},
			Labels: map[string]string{
				severityAlertLabelKey:     "warning",
				healthImpactAlertLabelKey: "none",
				partOfAlertLabelKey:       partOfAlertLabelValue,
				componentAlertLabelKey:    componentAlertLabelValue,
			},
		},
	}
}
