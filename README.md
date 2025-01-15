# SSP Operator

Operator that deploying and controlling additional [KubeVirt](https://kubevirt.io) resources:

- [Common Templates Bundle](https://github.com/kubevirt/common-templates)
- [VM Console Proxy](https://github.com/kubevirt/vm-console-proxy)
- [Template Validator](https://github.com/kubevirt/ssp-operator/tree/main/internal/template-validator)
- Metrics Rules (Currently there is just a single Prometheus rule that counts the number of running Virtual Machines.)
- Virtual Machine (VM) Delete Protection (deploys ValidatingAdmissionPolicy (VAP) that prevents VM from being deleted
  when protection is enabled.)

## Installation

See [Installation Guide](docs/installation.md) to explore our installation options.

## Configuration

See [Configuration Guide](docs/configuration.md) to configure SSP Custom Resource (CR).

## Development

See [Development Guide](docs/development.md) to setup development environment locally.
