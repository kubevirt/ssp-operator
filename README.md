# SSP Operator
Kubevirt SSP Operator

This operator is currently Work In Progress, the current SSP operator can be found here: https://github.com/kubevirt/kubevirt-ssp-operator

### Building
To build the binary and container image run:
```shell script
make operator-build
```

To upload the image to the default repository run:
```shell script
make operator-push
```

The repository and image name and tag can be changed 
with these variables:
```shell script
export IMAGE_REGISTRY=<registry>
export OPERATOR_IMAGE=<image_name>
export IMAGE_TAG=<image_tag>
```

### Changing API
When the API definition in `pkg/apis/ssp/v1` is changed,
the generated code and CRDs need to be regenerated:
```shell script
make generate
```