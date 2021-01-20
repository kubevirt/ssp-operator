# SSP Operator
Operator that manages Scheduling, Scale and Performance addons for [KubeVirt](https://kubevirt.io)

## Functionality

The operator deploys and manages resources needed by these four components:

- [Template Validator](https://github.com/kubevirt/kubevirt-template-validator)
- [Node Labeller](https://github.com/kubevirt/node-labeller)
- [Common Templates Bundle](https://github.com/kubevirt/common-templates)
- Metrics rules - Currently it is only a single Prometheus rule containing the count of all running VMs.

## Installation

The `ssp-operator` requires an Openshift cluster to run properly.

### Using HCO

The [Hyperconverged Cluster Operator](https://github.com/kubevirt/hyperconverged-cluster-operator) automatically installs the SSP operator when deploying.

### Manual installation

The operator can be installed manually by applying the file `ssp-operator.yaml` from GitHub releases:
```shell
oc apply -f ssp-operator.yaml
```

To install the latest released version, the following commands can be used:
```shell
export SSP_VERSION=$(curl https://api.github.com/repos/kubevirt/ssp-operator/releases/latest | jq '.name' | tr -d '"')
oc apply -f https://github.com/kubevirt/ssp-operator/releases/download/${SSP_VERSION}/ssp-operator.yaml
```

To activate the operator, a CR needs to be created.
An example is in [config/samples/ssp_v1beta1_ssp.yaml](config/samples/ssp_v1beta1_ssp.yaml).

## Building

To build the container image run:
```shell
make container-build
```

To upload the image to the default repository run:
```shell
make container-push
```

The repository and image name and tag can be changed 
with these variables:
```shell
export IMAGE_REGISTRY=<registry>
export OPERATOR_IMAGE=<image_name>
export IMAGE_TAG=<image_tag>
```

After the image is pushed to the repository,
manifests and the operator can be deployed using:
```shell
make deploy
```

## Development

### Running locally

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

### Testing

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

### Changing API

When the API definition in `api/v1beta1` is changed,
the generated code and CRDs need to be regenerated:
```shell
make generate manifests
```

### Pausing the operator

The reconciliation can be paused by adding the following 
annotation to the `SSP` reosurce:
```yaml
kubevirt.io/operator.paused: "true"
```
The operator will not react to any changes to the `SSP` resource
or any of the watched resources. If a paused `SSP` resource is deleted, 
the operator will still cleanup all the dependent resources.
