# Development

## golangci-lint

A [golangci-lint](https://golangci-lint.run/) [config](../.golangci.yaml) and [Makefile](../Makefile) target are provided to keep the codebase aligned to certain best practices and styles.

The target installs golangci-lint if it is not already present. To run use the following command:

```shell
make lint
```

The target is also used when running unit tests locally.

## pre-commit

A [pre-commit](https://pre-commit.com/) [config](../.pre-commit-config.yaml) is provided to help developers catch any simple mistakes prior to committing their changes.

To install and use the tool please run the following commands:

```shell
pip install --user pre-commit
pre-commit install
git commit -s
```

## Running locally

The operator can run locally on the developer's machine.
It will watch the cluster configured in a file pointed to by the `$KUBECONFIG`.
```shell
make install                    # Install CRDs to the cluster
make run ENABLE_WEBHOOKS=false  # Start the operator locally
```

When running locally, the validating webhooks that check the SSP CR
are disabled. It is up to the developer to use correct SSP CRs.

The CRDs can be removed using:
```shell
make uninstall
```

Alternatively developers can also deploy `kubevirtci` based clusters before
building and deploying the operator. For convenience the following makefile
targets are provided that automate this flow either directly through the
`kubevirtci` project or indirectly using the main `kubevirt` project to
launch an environment. The latter being useful when testing changes that
rely on core changes in `kubevirt` itself:

```shell
make cluster-up   # Deploys the latest released version of KubeVirt
make cluster-sync # Builds and deploys the SSP operator from source
make cluster-down # Destroys the environment
```

```shell
make kubevirt-up   # Deploys KubeVirt from source
make kubevirt-sync # Builds and deploys the SSP operator from source
make kubevirt-down # Destroys the environment
```

## Testing

To run unit tests, use this command:
```shell
make unittest
```

The functional tests can be run using command:
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

## Changing API

When the API definition in `api/v1beta1` is changed,
the generated code and CRDs need to be regenerated,
and API submodule has to be updated.
```shell
make vendor generate manifests
```

## Development hints

The `ssp-operator` project is based on Kubebuilder v3, so a good starting point
for development is the [Kubebuilder Book](https://book.kubebuilder.io/).

### Controllers

There are two controllers found in `controllers/`.
- `ServiceReconciler` adds and reconciles a Service that should exist
independently of the SSP CR for Prometheus monitoring to work with SSP.
- `SspReconciler` ensures that all required CRDs are available and reconciles
  the ssp CRD itself and its operands .

The logic of `ssp-operator` is split into separate operands, which can be found
under `internal/operands`. Each operand deals with a designated task, all
operands are called during the reconciliation of `SspReconciler`.

### Operands

Each operand handles a designated task.

#### `common-instancetypes` operand

Installs the bundled common instance types found in `data/common-instancetypes-bundle`.

#### `common-templates` operand

Installs the bundled common templates found in `data/common-templates-bundle`.
A PR for updating the bundle is automatically created when a new version of the
[common-templates](https://github.com/kubevirt/common-templates) is released.

#### `data-sources` operand

Manages automatic updates of boot sources for templates in cooperation with
[CDI](https://github.com/kubevirt/containerized-data-importer).

#### `metrics` operand

Installs prometheus monitoring rules for `ssp-operator` metrics. The available
metrics can be found in `docs/metrics.md`. The installed rules can be found in
`internal/operands/metrics/resources.go`.

#### `template-validator` operand

The main `template-validator` runs in at least one separate container than the
`ssp-operator`. Is is built with `validator.Dockerfile`.

This operand deploys the `template-validator` found in
`internal/template-validator` and installs an admission webhook, so the
`template-validator` can evaluate validations found in the
`vm.kubevirt.io/validations` annotation of `VirtualMachine` resources.

See the old `template-validator` [repository](https://github.com/kubevirt/kubevirt-template-validator)
for more docs.

#### `vm-console-proxy` operand

Installs the VM console proxy found in `data/vm-console-proxy-bundle`. A PR for updating the bundle is automatically created when a new version of the [vm-console-proxy](https://github.com/kubevirt/vm-console-proxy) is released.

#### `tekton-pipelines` operand

Installs the Tekton Pipelines found in `data/tekton-pipelines`. A PR for updating the bundle is automatically created when a new version of the [kubevirt-tekton-tasks](https://github.com/kubevirt/kubevirt-tekton-tasks) is released.

#### `tekton-tasks` operand

Installs the Tekton Tasks found in `data/tekton-tasks`. A PR for updating the bundle is automatically created when a new version of the [kubevirt-tekton-tasks](https://github.com/kubevirt/kubevirt-tekton-tasks) is released.
