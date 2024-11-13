# Configuration

Operator configuration can be modified by editing the SSP Custom Resource (CR).
See [SSP Specification](https://github.com/kubevirt/ssp-operator/blob/main/api/v1beta3/ssp_types.go#L51)
that defines the desired state of SSP.

## Table Of Contents

- [SSP Custom Resource (CR)](#ssp-custom-resource-cr)
- [Annotations](#annotations)
- [Common Templates](#common-templates)
- [Template Validator](#template-validator)
- [VNC Token Generation Service](#vnc-token-generation-service)

## SSP Custom Resource (CR)

To activate the operator, create the SSP Custom Resource (CR):
```yaml
apiVersion: ssp.kubevirt.io/v1beta3
kind: SSP
metadata:
  name: ssp-sample
  namespace: kubevirt
spec:
  commonTemplates:
    namespace: kubevirt
  templateValidator:
    replicas: 2
```

## Annotations

### Pause Operator

This annotation will pause operator reconciliation.

```yaml
apiVersion: ssp.kubevirt.io/v1beta3
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

```yaml
apiVersion: ssp.kubevirt.io/v1beta3
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

```yaml
apiVersion: ssp.kubevirt.io/v1beta3
kind: SSP
metadata:
  name: ssp-sample
  namespace: kubevirt
spec:
  templateValidator:
    replicas: 2 # Customize the number of replicas for the validator deployment
```

## VNC Token Generation Service

The  [VM Console Proxy](https://github.com/kubevirt/vm-console-proxy)
can be deployed by SSP operator when it is enabled in the `.spec`.
It is a service that exposes an API for generating VNC access tokens for VMs.

```yaml
apiVersion: ssp.kubevirt.io/v1beta3
kind: SSP
metadata:
  name: ssp-sample
  namespace: kubevirt
spec:
  tokenGenerationService:
    enabled: true
```
