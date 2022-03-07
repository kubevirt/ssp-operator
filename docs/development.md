# Development

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

There are two controllers found in `controllers/`. The `CrdController` ensures
that all required CRDs are available before the `SspReconciler` is started.
Both controllers are setup by `controllers/setup.go` (deviating from the
default kubebuilder structure, because of `SspReconciler`'s dependency on
`CrdController`). If all required CRDs are already available during setup,
`SspReconciler` is started immediately.

The logic of `ssp-operator` is split into separate operands, which can be found
under `internal/operands`. Each operand deals with a designated task, all
operands are called during the reconciliation of `SspReconciler`.

### Operands

Each operand handles a designated task.

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

#### `node-labeller` operand

Node labeling was moved to the [kubevirt](https://github.com/kubevirt/kubevirt)
core. The operand removes remaining resources created by older versions of
`ssp-operator`.

#### `template-validator` operand

The main `template-validator` runs in at least one separate container than the
`ssp-operator`. Is is built with `validator.Dockerfile`.

This operand deploys the `template-validator` found in
`internal/template-validator` and installs an admission webhook, so the
`template-validator` can evaluate validations found in the
`vm.kubevirt.io/validations` annotation of `VirtualMachine` resources.

See the old `template-validator` [repository](https://github.com/kubevirt/kubevirt-template-validator)
for more docs.
