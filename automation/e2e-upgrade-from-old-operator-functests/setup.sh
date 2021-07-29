#!/bin/bash
set -e

# This file is run before tests. It should be used to install other required components on the cluster.
# It is run from the root of the repository.

./hack/deploy-kubevirt-and-cdi.sh
./hack/deploy-old-ssp-operator.sh