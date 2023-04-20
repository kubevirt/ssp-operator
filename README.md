# SSP Operator
Operator that manages Scheduling, Scale and Performance addons for [KubeVirt](https://kubevirt.io)

## Functionality

The operator deploys and manages resources needed by these four components:

- [Template Validator](https://github.com/kubevirt/ssp-operator/tree/master/internal/template-validator) (You can read more about it here: [Template Validator old repository](https://github.com/kubevirt/kubevirt-template-validator))
- [Common Templates Bundle](https://github.com/kubevirt/common-templates)
- [VM Console Proxy](https://github.com/kubevirt/vm-console-proxy)
- [KubeVirt Tekton Tasks](https://github.com/kubevirt/kubevirt-tekton-tasks)
- Metrics rules - Currently it is only a single Prometheus rule containing the count of all running VMs.

## Installation

The `ssp-operator` requires an Openshift cluster to run properly.

### Requirements

The following resource types and CRDs are needed by `ssp-operator` when deployed on an OpenShift environment:

| Resource type / CRD                                                     | Needed by                                                                 |
| ------------------------------------------------------------------------| --------------------------------------------------------------------------|
| `dataimportcrons.cdi.kubevirt.io`                                       | `data-sources` operand (Kind `DataImportCron`)                            |
| `datavolumes.cdi.kubevirt.io/v1beta1`                                   | `data-sources` operand (Kind `DataVolume` and `DataVolumeSource`)         |
| `datasources.cdi.kubevirt.io/v1beta1`                                   | `data-sources` operand (Kind `DataSource`)                                |
| `prometheusrules.monitoring.coreos.com`                                 | `metrics` operand (Kind `PrometheusRule`)                                 |
| `securitycontextconstraints.security.openshift.io`                      | `node-labeller` operand (Kind `SecurityContextConstraints`)               |
| `template.openshift.io/v1`                                              | `common-templates` operand (Kind `Template`)                              |
| `virtualmachineclusterinstancetypes.instancetypes.kubevirt.io/v1alpha2` | `common-instancetypes` operand (Kind `VirtualMachineClusterInstancetype`) |
| `virtualmachineclusterpreferences.instancetypes.kubevirt.io/v1alpha2`   | `common-instancetypes` operand (Kind `VirtualMachineClusterPreference`)   |
| `pipelines.tekton.dev`                                                  | `tekton-pipelines` operand (Kind `Pipeline`)                              |
| `tasks.tekton.dev`                                                      | `tekton-tasks` operand (Kind `Task`)                                      |

The following resource types and CRDs are needed by `ssp-operator` when deployed on a vanilla k8s environment:

| Resource type / CRD                                                     | Needed by                                                                 |
| ------------------------------------------------------------------------| --------------------------------------------------------------------------|
| `virtualmachineclusterinstancetypes.instancetypes.kubevirt.io/v1alpha2` | `common-instancetypes` operand (Kind `VirtualMachineClusterInstancetype`) |
| `virtualmachineclusterpreferences.instancetypes.kubevirt.io/v1alpha2`   | `common-instancetypes` operand (Kind `VirtualMachineClusterPreference`)   |
| `pipelines.tekton.dev`                                                  | `tekton-pipelines` operand (Kind `Pipeline`)                              |
| `tasks.tekton.dev`                                                      | `tekton-tasks` operand (Kind `Task`)                                      |

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

The Make will try to install kustomize, however if it is already installed it will not reinstall it.
In case of an error, make sure you are using at least v3 of kustomize, available here: https://kustomize.io/

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
export IMG_REPOSITORY=<registry>/<image_name> # for example: export IMG_REPOSITORY=quay.io/kubevirt/ssp-operator
export IMG_TAG=<image_tag> # for example: export IMG_TAG=latest
```

After the image is pushed to the repository,
manifests and the operator can be deployed using:
```shell
make deploy
```
## Building the Template Validator
Please note that building and deploying the Template Validator requires a separate process

To build the container image run:
```shell
make build-template-validator-container
```

To upload the image to the default repository run:
```shell
make push-template-validator-container
```

The repository and image name and tag can be changed
with these variables:
```shell
export VALIDATOR_REPOSITORY=<registry>/<image_name> # for example: export VALIDATOR_REPOSITORY=quay.io/kubevirt/kubevirt-template-validator
export VALIDATOR_IMG_TAG=<image_tag> # for example: export VALIDATOR_IMG_TAG=latest
```

You should also edit the deployment to pull the image you want, for example you can use this command:
```shell
oc set env deployment/ssp-operator VALIDATOR_IMAGE=$VALIDATOR_IMG
```

## Pausing the operator

The reconciliation can be paused by adding the following
annotation to the `SSP` resource:
```yaml
kubevirt.io/operator.paused: "true"
```
The operator will not react to any changes to the `SSP` resource
or any of the watched resources. If a paused `SSP` resource is deleted,
the operator will still cleanup all the dependent resources.

## Development

See [docs/development.md](docs/development.md)
