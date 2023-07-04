# Branch name change:

## Development has been moved from the master branch to the main branch.

# SSP Operator

Operator that deploying and controlling additional [KubeVirt](https://kubevirt.io) resources:

- [Common Instancetypes and Preferences Bundle](https://github.com/kubevirt/common-instancetypes/)
- [Common Templates Bundle](https://github.com/kubevirt/common-templates)
- [VM Console Proxy](https://github.com/kubevirt/vm-console-proxy)
- [KubeVirt Tekton Tasks](https://github.com/kubevirt/kubevirt-tekton-tasks)
- [Template Validator](https://github.com/kubevirt/ssp-operator/tree/main/internal/template-validator)
- Metrics Rules (Currently there is just a single Prometheus rule that counts the number of running Virtual Machines.)

## Installation

See [Installation Guide](docs/installation.md) to explore our installation options.

## Configuration

See [Configuration Guide](docs/configuration.md) to configure SSP Custom Resource (CR).

## Development

See [Development Guide](docs/development.md) to setup development environment locally.
