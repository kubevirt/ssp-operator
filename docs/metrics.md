# SSP Operator metrics

### cnv:vmi_status_running:count
The total number of running VMIs, labeled with node, instance type, preference and guest OS information. Type: Gauge.

### kubevirt_ssp_common_templates_restored_increase
The increase in the number of common templates restored by the operator back to their original state, over the last hour. Type: Gauge.

### kubevirt_ssp_common_templates_restored_total
The total number of common templates restored by the operator back to their original state. Type: Counter.

### kubevirt_ssp_operator_reconcile_succeeded
Set to 1 if the reconcile process of all operands completes with no errors, and to 0 otherwise. Type: Gauge.

### kubevirt_ssp_operator_reconcile_succeeded_aggregated
The total number of ssp-operator pods reconciling with no errors. Type: Gauge.

### kubevirt_ssp_operator_up
The total number of running ssp-operator pods. Type: Gauge.

### kubevirt_ssp_template_validator_rejected_increase
The increase in the number of rejected template validators, over the last hour. Type: Gauge.

### kubevirt_ssp_template_validator_rejected_total
The total number of rejected template validators. Type: Counter.

### kubevirt_ssp_template_validator_up
The total number of running virt-template-validator pods. Type: Gauge.

### kubevirt_ssp_vm_rbd_block_volume_without_rxbounce
[ALPHA] VM with RBD mounted Block volume (without rxbounce option set). Type: Gauge.

## Developing new metrics

All metrics documented here are auto-generated and reflect exactly what is being
exposed. After developing new metrics or changing old ones please regenerate
this document.
