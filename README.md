# SSP Operator
Kubevirt SSP Operator

This operator is currently Work In Progress, the current SSP operator can be found here: https://github.com/kubevirt/kubevirt-ssp-operator

### Building
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

The binary without a container can be build using:
```shell
make manager
```

### Changing API
When the API definition in `api/v1beta1` is changed,
the generated code and CRDs need to be regenerated:
```shell
make generate
make manifests
```

### Testing
To run unittests, use this command:
```shell
make unittest
```

The functional tests can be run on a cluster
without deploying the operator to the cluster. The `KUBECONFIG`
environment variable has to be set to access the cluster.
These are the steps:
```shell
make install                    # Install CRDs to the cluster
make run ENABLE_WEBHOOKS=false  # Start the operator locally
make functest                   # Execute functional tests
make uninstall                  # Remove CRDs from the cluster
```