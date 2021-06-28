#!/bin/bash
set -e

# Deploy KubeVirt and CDI
./hack/deploy-kubevirt-and-cdi.sh

# Deploy latest released SSP operator
NAMESPACE=${1:-kubevirt}

LATEST_SSP_VERSION=$(curl 'https://api.github.com/repos/kubevirt/ssp-operator/releases/latest' | jq '.name' | tr -d '"')
oc apply -n $NAMESPACE -f "https://github.com/kubevirt/ssp-operator/releases/download/${LATEST_SSP_VERSION}/ssp-operator.yaml"

# Wait for deployment to be available, otherwise the validating webhook would reject the SSP CR.
oc wait --for=condition=Available --timeout=600s -n ${NAMESPACE} deployments/ssp-operator

SSP_NAME="ssp-test"
SSP_NAMESPACE="ssp-operator-functests"
SSP_TEMPLATES_NAMESPACE="ssp-operator-functests-templates"

oc apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: ${SSP_NAMESPACE}
EOF

oc apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: ${SSP_TEMPLATES_NAMESPACE}
EOF

# TODO - in a future release, this script should use the CR template from the latest released version
sed -e "s/%%_SSP_NAME_%%/${SSP_NAME}/g" \
    -e "s/%%_SSP_NAMESPACE_%%/${SSP_NAMESPACE}/g" \
    -e "s/%%_COMMON_TEMPLATES_NAMESPACE_%%/${SSP_TEMPLATES_NAMESPACE}/g" \
    ./automation/ssp-cr-template.yaml | oc apply -f -

oc wait --for=condition=Available --timeout=600s -n ${SSP_NAMESPACE} ssp/${SSP_NAME}

# $IMG variable contains the image built by OCP CI
export GOFLAGS=""
export SKIP_CLEANUP_AFTER_TESTS="true"
export TEST_EXISTING_CR_NAME="${SSP_NAME}"
export TEST_EXISTING_CR_NAMESPACE="${SSP_NAMESPACE}"
make deploy functest
