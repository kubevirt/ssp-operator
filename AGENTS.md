# SSP Operator - AI Agent Guide

SSP Operator is a Kubernetes operator that deploys and controls additional KubeVirt resources:
- Common Templates Bundle
- VM Console Proxy
- Template Validator (validating webhook for VMs created from common templates)
- Metrics Rules (Prometheus rules for VM monitoring)
- VM Delete Protection (ValidatingAdmissionPolicy)

The operator is part of the KubeVirt ecosystem and designed to run on Kubernetes/OpenShift clusters.

## Documentation

- [configuration.md](docs/configuration.md) - How to configure the SSP CR, including annotations, common templates, template validator, and VNC token generation service
- [development.md](docs/development.md) - Development guide covering running locally, testing, API changes, controllers, operands, linting, and pre-commit hooks
- [installation.md](docs/installation.md) - Installation requirements for OpenShift/Kubernetes, HCO and manual installation methods, building and pushing images
- [metrics.md](docs/metrics.md) - Complete reference of all SSP operator metrics and Prometheus recording rules
- [monitoring-guidelines.md](docs/monitoring-guidelines.md) - Observability compatibility policy and guidelines for metrics, recording rules, and alerting rules

## Architecture

### Controllers (`internal/controllers/`)
- **SspReconciler** - Main controller that reconciles the SSP CR and delegates to operands
- **ServiceReconciler** - Manages the metrics Service independently of SSP CR for Prometheus monitoring
- **VmReconciler** - Watches VMs and creates metrics for VMs with RBD volumes
- **WebhookReconciler** - Manages webhook configurations

### Operands (`internal/operands/`)
All operands implement the `Operand` interface and are invoked during `SspReconciler` reconciliation. See [development.md](docs/development.md) for details on each operand.

- **common-templates** - templates from `data/common-templates-bundle/`
- **data-sources** - boot source updates, configured via SSP CR
- **metrics** - Prometheus rules
- **template-validator** - validation webhook; runs in a separate container (`validator.Dockerfile`), source in `internal/template-validator/`
- **vm-console-proxy** - from `data/vm-console-proxy-bundle/`
- **vm-delete-protection** - ValidatingAdmissionPolicy

### API Versions (`api/`)
- **v1beta3** - Current primary API version
- **v1beta2** - Legacy API version (still supported)

Both versions have their own module with separate `go.mod` to enable independent versioning.

### Directory Structure
- `api/` - API definitions (v1beta2, v1beta3)
- `internal/` - Internal packages (controllers, operands, utilities)
- `config/` - Kustomize configurations, CRDs, RBAC
- `data/` - Bundled resources (templates, CRDs, network policies)
- `tests/` - Functional tests (Ginkgo/Gomega)
- `hack/` - Build scripts and utilities
- `webhooks/` - Webhook implementations
- `pkg/monitoring/` - Metrics and monitoring rules

## Linting

See [development.md](docs/development.md) for golangci-lint usage. Additional linters not covered there:
- **Monitoring linter** - Custom linter for Prometheus metrics/rules: `make lint-monitoring`
- **Metrics linter** - Validates metric names: `make lint-metrics`
