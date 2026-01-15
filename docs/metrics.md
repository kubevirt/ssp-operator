# SSP Operator metrics

| Name | Kind | Type | Description |
|------|------|------|-------------|
| kubevirt_ssp_common_templates_restored_total | Metric | Counter | The total number of common templates restored by the operator back to their original state |
| kubevirt_ssp_operator_reconcile_succeeded | Metric | Gauge | Set to 1 if the reconcile process of all operands completes with no errors, and to 0 otherwise |
| kubevirt_ssp_template_validator_rejected_total | Metric | Counter | The total number of rejected template validators |
| kubevirt_ssp_vm_rbd_block_volume_without_rxbounce | Metric | Gauge | [ALPHA] VM with RBD mounted Block volume (without rxbounce option set) |
| cnv:vmi_status_running:count | Recording rule | Gauge | The total number of running VMIs, labeled with node, instance type, preference and guest OS information |
| kubevirt_ssp_common_templates_restored_increase | Recording rule | Gauge | The increase in the number of common templates restored by the operator back to their original state, over the last hour |
| kubevirt_ssp_operator_reconcile_succeeded_aggregated | Recording rule | Gauge | The total number of ssp-operator pods reconciling with no errors |
| kubevirt_ssp_operator_up | Recording rule | Gauge | The total number of running ssp-operator pods |
| kubevirt_ssp_template_validator_rejected_increase | Recording rule | Gauge | The increase in the number of rejected template validators, over the last hour |
| kubevirt_ssp_template_validator_up | Recording rule | Gauge | The total number of running virt-template-validator pods |

## Developing new metrics

All metrics documented here are auto-generated and reflect exactly what is being
exposed. After developing new metrics or changing old ones please regenerate
this document.
