# Development

## Table Of Contents

- [Running Locally](#running-locally)
- [Testing](#testing)
- [API Changes](#api-changes)
- [Development Hints](#development-hints)
- [Controllers](#controllers)
- [Golang CI Linter](#golang-ci-linter)
- [Pre-commit Hooks](#pre-commit-hooks)

## Running Locally

The operator can run locally on the developer's machine.
It will monitor the cluster specified in a file indicated by the `$KUBECONFIG`
environment variable:
```shell
make install                    # Install CRDs to the cluster
make run ENABLE_WEBHOOKS=false  # Start the operator locally
```

### Webhooks

When running locally, the validating webhooks responsible for verifying the SSP Custom Resource (CR)
are deactivated, placing the responsibility on the developer to ensure the usage of the accurate
SSP CR.

The Custom Resource Definitions (CRDs) can be removed using:
```shell
make uninstall
```

### KubeVirt CI

Alternatively developers can also deploy `kubevirtci` based clusters before
building and deploying the operator.

For convenience the following Makefile
targets are provided that automate this flow either directly through the
`kubevirtci` project or indirectly using the main `kubevirt` project to
launch an environment.

Deploy latest released version of KubeVirt:
```shell
make cluster-up   # Deploys the latest released version of KubeVirt
make cluster-sync # Builds and deploys the SSP operator from source
make cluster-down # Destroys the environment
```

Deploy KubeVirt from source:
```shell
make kubevirt-up   # Deploys KubeVirt from source
make kubevirt-sync # Builds and deploys the SSP operator from source
make kubevirt-down # Destroys the environment
```

## Testing

### Unit Tests

To run unit tests, use this command:
```shell
make unittest
```

### Functional Tests

To run functional tests, use this command:
```shell
make functest
```

The following environment variables control how functional tests are run:

- `TEST_EXISTING_CR_NAME` and `TEST_EXISTING_CR_NAMESPACE` - Can be used
  to set an existing SSP CR to be used during the tests.
  The CR will be modified, deleted and recreated during testing.
- `SKIP_UPDATE_SSP_TESTS` - Skips tests that need to modify or delete
  the SSP CR. This is useful if the CR is owned by another operator.
- `SKIP_CLEANUP_AFTER_TESTS` - Do not remove created resources when
  the tests are finished.
- `TIMEOUT_MINUTES` and `SHORT_TIMEOUT_MINUTES` - Can be used to increase the timeouts used.

## API Changes

When the API definition in `api` is changed,
the generated code and CRDs need to be regenerated,
and API submodule has to be updated.

Use these commands:
```shell
make vendor
make generate
make manifests
```

## Development Hints

To dive into development for the `ssp-operator` project, an ideal resource to get started
is the [Kubebuilder Book](https://book.kubebuilder.io/). With its foundation on Kubebuilder v3,
this book offers valuable insights and guidance for seamless development.

## Controllers

Controllers that can be found in `controllers`:

- `ServiceReconciler` - Adds and reconciles a Service that should exist
independently of the SSP CR for Prometheus monitoring to work with SSP.
- `SspReconciler` - Ensures that all required CRDs are available and reconciles
  the SSP CRD itself and its operands.

The logic of `ssp-operator` is split into separate operands, which can be found
under `internal/operands`. Each operand deals with a designated task, all
operands are called during the reconciliation of `SspReconciler`.

### SSP Operands

#### `common-instancetypes`

Installs the bundled common instance types found in `data/common-instancetypes-bundle`.
A pull request (PR) for updating the bundle is automatically created when a new version
of the [common-instancetypes](https://github.com/kubevirt/common-instancetypes) is released.

#### `common-templates`

Installs the bundled common templates found in `data/common-templates-bundle`.
A pull request (PR) for updating the bundle is automatically created when a new version
of the [common-templates](https://github.com/kubevirt/common-templates) is released.

#### `data-sources`

Manages automatic updates of boot sources for templates in cooperation with
[Containerized Data Importer (CDI)](https://github.com/kubevirt/containerized-data-importer).
Data sources to deploy are defined in the SSP CR.

#### `metrics`

Installs prometheus monitoring rules for `ssp-operator` metrics. The available
metrics can be found in `docs/metrics.md`. The installed rules can be found in
`internal/operands/metrics/resources.go`.

#### `template-validator`

The main `template-validator` runs in at least one separate container than the
`ssp-operator`. Is is built with `validator.Dockerfile`.

This operand deploys the `template-validator` found in
`internal/template-validator` and installs an admission webhook, so the
`template-validator` can evaluate validations found in the
`vm.kubevirt.io/validations` annotation of `VirtualMachine` resources.

See the old `template-validator` [repository](https://github.com/kubevirt/kubevirt-template-validator)
for more docs.

#### `vm-console-proxy`

Installs the VM console proxy found in `data/vm-console-proxy-bundle`.
A pull request (PR) for updating the bundle is automatically created when a new version
of the [vm-console-proxy](https://github.com/kubevirt/vm-console-proxy) is released.

#### `tekton-pipelines`

Installs the Tekton Pipelines found in `data/tekton-pipelines`.

Pipelines eliminate the need for manual updates when new tasks are added.
They simply refer to tasks by their names. So, pipelines only require changes if a task's
name is modified or if there are functional adjustments, but otherwise,
they don't need to be updated.

#### `tekton-tasks`

Installs the Tekton Tasks found in `data/tekton-tasks`.
A pull request (PR) for updating the bundle is automatically created when a new version
of the [kubevirt-tekton-tasks](https://github.com/kubevirt/kubevirt-tekton-tasks) is released.

## Golang CI Linter

A [golangci-lint](https://golangci-lint.run/) [config](../.golangci.yaml) and [Makefile](../Makefile)
target are provided to keep the codebase aligned to certain best practices and styles.

The target installs golangci-lint if it is not already present. To run use the following command:
```shell
make lint
```

The target is also used when running unit tests locally.

## Pre-commit Hooks

A [pre-commit](https://pre-commit.com/) [config](../.pre-commit-config.yaml) is provided to help
developers catch any simple mistakes prior to committing their changes.

To install and use the tool please run the following commands:
```shell
pip install --user pre-commit
pre-commit install
git commit -s
```
