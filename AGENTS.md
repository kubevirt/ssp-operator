# SSP Operator - AI Agent Guide

SSP Operator is a Kubernetes operator that deploys and controls additional KubeVirt resources:
- Common Templates Bundle
- VM Console Proxy
- Template Validator (validating webhook for VMs)
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

### Operands Pattern (`internal/operands/`)
The operator's logic is split into independent operands, each handling a specific feature:

- **common-templates** - Bundles common VM templates from `data/common-templates-bundle/`
- **data-sources** - Manages boot source updates via CDI DataSources (configured in SSP CR)
- **metrics** - Installs Prometheus rules for operator metrics
- **template-validator** - Deploys validation webhook for VMs (runs in separate container built from `validator.Dockerfile`)
- **vm-console-proxy** - Bundles VM console proxy from `data/vm-console-proxy-bundle/`
- **vm-delete-protection** - Installs ValidatingAdmissionPolicy to prevent VM deletion when `kubevirt.io/vm-delete-protection: "True"` label is set

All operands implement the `Operand` interface and are invoked during `SspReconciler` reconciliation.

### API Versions (`api/`)
- **v1beta3** - Current primary API version
- **v1beta2** - Legacy API version (still supported)

Both versions have their own module with separate `go.mod` to enable independent versioning.

### Template Validator
The template validator is a separate component that:
- Runs in its own container (built from `validator.Dockerfile`)
- Source in `internal/template-validator/`
- Validates VMs against rules in `vm.kubevirt.io/validations` annotation
- Default image injected at build time via `-ldflags`

### Directory Structure
- `api/` - API definitions (v1beta2, v1beta3)
- `internal/` - Internal packages (controllers, operands, utilities)
- `config/` - Kustomize configurations, CRDs, RBAC
- `data/` - Bundled resources (templates, CRDs, network policies)
- `tests/` - Functional tests (Ginkgo/Gomega)
- `hack/` - Build scripts and utilities
- `webhooks/` - Webhook implementations
- `pkg/monitoring/` - Metrics and monitoring rules

## Testing Strategy

### Unit Tests
- Located alongside code in `*_test.go` files
- Use standard Go testing + Gomega matchers
- Run with `make unittest`

### Functional Tests
- In `tests/` directory
- Use Ginkgo v2 framework
- Test against real cluster (can use existing SSP CR with env vars)
- Environment variables:
  - `TEST_EXISTING_CR_NAME`, `TEST_EXISTING_CR_NAMESPACE` - Use existing SSP CR
  - `SKIP_UPDATE_SSP_TESTS` - Skip tests that modify/delete CR
  - `SKIP_CLEANUP_AFTER_TESTS` - Leave resources after tests
  - `TIMEOUT_MINUTES`, `SHORT_TIMEOUT_MINUTES` - Adjust timeouts

## Development Workflow

1. Make code changes
2. If API changed: `make vendor && make generate && make manifests`
3. Run tests: `make unittest` (includes linting)
4. Deploy to cluster: Build and deploy with `make deploy`

## Linting and Code Quality

- **golangci-lint** - Configured in `.golangci.yaml`, run with `make lint`
- **Monitoring linter** - Custom linter for Prometheus metrics/rules: `make lint-monitoring`
- **Metrics linter** - Validates metric names: `make lint-metrics`

## Image Building

The project builds multiple container images:
- Main operator image (from `Dockerfile`)
- Template validator image (from `validator.Dockerfile`)

Images support multi-arch (amd64, s390x) via podman manifests.

## Build and Run Commands

### Local Development
```bash
make install                    # Install CRDs to cluster
make run ENABLE_WEBHOOKS=false  # Run operator locally (webhooks disabled)
make uninstall                  # Remove CRDs from cluster
```

### Building
```bash
make manager                    # Build the operator binary
make container-build            # Build container images (uses podman)
make build-template-validator   # Build template validator binary
```

### Testing
```bash
make unittest                   # Run unit tests (includes lint, fmt, vet, manifests)
make functest                   # Run functional tests with Ginkgo
make lint                       # Run golangci-lint
```

To run a single test file or focus on specific tests:
```bash
# Using Ginkgo focus
make functest GINKGO_FOCUS="<test description pattern>"

# Or run ginkgo directly
go run github.com/onsi/ginkgo/v2/ginkgo -v --focus="<pattern>" ./tests/...
```

### Code Generation
After modifying APIs in `api/` directory:
```bash
make vendor                     # Update vendored dependencies
make generate                   # Generate deepcopy, runtime.Object implementations
make manifests                  # Generate CRDs, RBAC, webhooks
```

### Deployment
```bash
make deploy                     # Deploy to cluster using kustomize
make undeploy                   # Remove from cluster
```

## Important Notes

- When running locally, webhooks are disabled by default (use `ENABLE_WEBHOOKS=false`)
- The operator uses `oc` if available, otherwise falls back to `kubectl`
- CRD generation uses `crd:generateEmbeddedObjectMeta=true` option
- The project uses Kubebuilder v3 layout - see [Kubebuilder Book](https://book.kubebuilder.io/) for guidance
- Bundles (common-templates, vm-console-proxy) are auto-updated via PRs when upstream releases
