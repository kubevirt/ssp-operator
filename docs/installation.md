# Installation

## Table Of Contents

- [Requirements](#requirements)
- [Hyperconverged Cluster Operator (HCO) Installation](#hyperconverged-cluster-operator-hco-installation)
- [Manually Installation](#manually-installation)
- [Building](#building)

## Requirements

SSP operator requires an OpenShift cluster in order to deliver complete functionality.
However, running on Kubernetes cluster is still possible.

Also it requires resource types and Custom Resource Definitions (CRDs) 
for successful deployment across different Kubernetes distributions.

### OpenShift

| Resource Type / CRD                                                     | Needed By                                                                 |
| ------------------------------------------------------------------------| --------------------------------------------------------------------------|
| `dataimportcrons.cdi.kubevirt.io`                                       | `data-sources` operand (Kind `DataImportCron`)                            |
| `datavolumes.cdi.kubevirt.io/v1beta1`                                   | `data-sources` operand (Kind `DataVolume` and `DataVolumeSource`)         |
| `datasources.cdi.kubevirt.io/v1beta1`                                   | `data-sources` operand (Kind `DataSource`)                                |
| `prometheusrules.monitoring.coreos.com`                                 | `metrics` operand (Kind `PrometheusRule`)                                 |
| `template.openshift.io/v1`                                              | `common-templates` operand (Kind `Template`)                              |
| `virtualMachine.kubevirt.io`                                            | `vm-controller` operand (Kind `VirtualMachine`)                           |

### Kubernetes

| Resource Type / CRD                                                     | Needed By                                                                 |
| ------------------------------------------------------------------------| --------------------------------------------------------------------------|
| `virtualMachine.kubevirt.io`                                            | `vm-controller` operand (Kind `VirtualMachine`)                           |

## Hyperconverged Cluster Operator (HCO) Installation

The SSP operator is typically deployed and controlled by
[Hyperconverged Cluster Operator](https://github.com/kubevirt/hyperconverged-cluster-operator)
automatically.

## Manually Installation

To install the latest released version, the following commands can be used:
```shell
export SSP_VERSION=$(curl https://api.github.com/repos/kubevirt/ssp-operator/releases/latest | jq '.name' | tr -d '"')
oc apply -f https://github.com/kubevirt/ssp-operator/releases/download/${SSP_VERSION}/ssp-operator.yaml
```

To activate the SSP operator, you will need to create a Custom Resource (CR).
An example CR can be found in the file [ssp_v1beta2_ssp.yaml](config/samples/ssp_v1beta2_ssp.yaml).
Please refer to [Configuration Guide](./configuration.md) to see how to configure and
create the necessary Custom Resource to enable the operator.

## Building

### SSP Operator

The Makefile will attempt to install kustomize if it is not already installed.
However, if kustomize is already present, it will skip the installation process.

In the event of an error, please ensure that you are using at least v3 of kustomize,
which can be obtained from the official [kustomize](https://kustomize.io) website.

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
manifests and the operator can be deployed by using this command:
```shell
make deploy
```

### Template Validator

Please note that building and deploying the Template Validator is a separate process.

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
export VALIDATOR_REPOSITORY=<registry>/<image_name> # For example: export VALIDATOR_REPOSITORY=quay.io/kubevirt/kubevirt-template-validator
export VALIDATOR_IMG_TAG=<image_tag> # For example: export VALIDATOR_IMG_TAG=latest
```

Edit the deployment to pull the image you want, by using this command:
```shell
oc set env deployment/ssp-operator VALIDATOR_IMAGE=$VALIDATOR_IMG
```
