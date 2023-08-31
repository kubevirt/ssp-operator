# Configuration

Operator configuration can be modified by editing the SSP Custom Resource (CR).
See [SSP Specification](https://github.com/kubevirt/ssp-operator/blob/main/api/v1beta2/ssp_types.go#L74)
that defines the desired state of SSP.

## Table Of Contents

- [SSP Custom Resource (CR)](#ssp-custom-resource-cr)
- [Annotations](#annotations)
- [Common Templates](#common-templates)
- [Template Validator](#template-validator)
- [Tekton Pipelines](#tekton-pipelines)
- [Tekton Tasks](#tekton-tasks)
- [Feature Gates](#feature-gates)

## SSP Custom Resource (CR)

To activate the operator, create the SSP Custom Resource (CR):
```
apiVersion: ssp.kubevirt.io/v1beta2
kind: SSP
metadata:
  name: ssp-sample
  namespace: kubevirt
spec:
  commonTemplates:
    namespace: kubevirt
  templateValidator:
    replicas: 2
  featureGates:
    deployTektonTaskResources: true
    deployVmConsoleProxy: true
  tektonPipelines:
    namespace: kubevirt
  tektonTasks:
    namespace: kubevirt
```

## Annotations

### VM Console Proxy

This annotation is used by VM console proxy operand.

```
apiVersion: ssp.kubevirt.io/v1beta1
kind: SSP
metadata:
  name: ssp-sample
  namespace: kubevirt
spec: {}
```

### Pause Operator

This annotation will pause operator reconciliation.

```
apiVersion: ssp.kubevirt.io/v1beta1
kind: SSP
metadata:
  annotations:
    kubevirt.io/operator.paused: "false" # If not set, then by default is false
  name: ssp-sample
  namespace: kubevirt
spec: {}
```

The operator will not react to any changes to the `SSP` resource
or any of the watched resources. If a paused `SSP` resource is deleted,
the operator will still cleanup all the dependent resources.

## Common Templates

A set of common templates to create KubeVirt Virtual Machines (VMs).

```
apiVersion: ssp.kubevirt.io/v1beta1
kind: SSP
metadata:
  name: ssp-sample
  namespace: kubevirt
spec:
  commonTemplates:
    namespace: kubevirt
```

## Template Validator

Template Validator is designed to inspect virtual machines (VMs) and detect any violations of the rules defined in VM's annotations.

```
apiVersion: ssp.kubevirt.io/v1beta1
kind: SSP
metadata:
  name: ssp-sample
  namespace: kubevirt
spec:
  templateValidator:
    replicas: 2 # Customize the number of replicas for the validator deployment
```

## Tekton Pipelines

Specify the deployment namespace for Tekton Pipelines.

```
apiVersion: ssp.kubevirt.io/v1beta2
kind: SSP
metadata:
  name: ssp-sample
  namespace: kubevirt
spec:
  tektonPipelines:
    namespace: kubevirt
```

## Tekton Tasks

Specify the deployment namespace for Tekton Tasks.

```
apiVersion: ssp.kubevirt.io/v1beta2
kind: SSP
metadata:
  name: ssp-sample
  namespace: kubevirt
spec:
  tektonTasks:
    namespace: kubevirt
```

## Feature Gates

The `featureGates` field is an optional set of optional boolean feature enabler.
The features in the list are experimental features that are not enabled by default.

To enable a feature, add its name to the `featureGates` list and set it to true.
Missing or false feature gates disables the feature.

### `deployTektonTaskResources`

Set the `deployTektonTaskResources` feature gate to true to allow the operator
to deploy Tekton resources.

Example pipelines and tasks will be deployed, enabling Tekton to work with
Virtual Machines (VMs), disks, and common templates.

```
apiVersion: ssp.kubevirt.io/v1beta2
kind: SSP
metadata:
  name: ssp-sample
  namespace: kubevirt
spec:
  featureGates:
    deployTektonTaskResources: true
```

### `deployVmConsoleProxy`

Set the `deployVmConsoleProxy` feature gate to true to allow the operator
to deploy VM console proxy resources.

Resources will be deployed that provide access to the VNC console of a KubeVirt VM,
enabling users to access VMs without requiring access to the cluster's API.

```
apiVersion: ssp.kubevirt.io/v1beta2
kind: SSP
metadata:
  name: ssp-sample
  namespace: kubevirt
spec:
  featureGates:
    deployVmConsoleProxy: true
```
