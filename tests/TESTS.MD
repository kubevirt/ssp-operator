CI Tests
--------
Tests:

- [tests](http://github.com/omeryahud/ssp-operator/master/blob/tests) is the directory containing all the functional tests.
- The test suite assumes that it already has an OpenShift cluster deployed with the SSP Operator; the tests should not deploy the operator.

Helper tools:

- `tests/build` directory contains `Dockerfile` to build the `ssp-builder` container.
- `hack/build-in-cointainer.sh` script is to build the container, inside which we want to build our tests in a consistent environment. `make build-util-container` should be run after a change to the build definition. A CI Job to keep the image updated in the registry is WIP.
- `hack/in-container.sh` is used as container environment to build tests inside it.
