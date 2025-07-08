#!/bin/bash
set -e

# This file runs the tests.
# It is run from the root of the repository.

# These evn variables are defined by the CI:
# CI_BRANCH - name of the git branch that CI is testing
# CI_OPERATOR_IMG - path of the operator image in the local repository accessible on the CI
# CI_VALIDATOR_IMG - path of the validator image in the local repository accessible on the CI

SOURCE_DIR=$(dirname "$0")

# Deploy KubeVirt and CDI
./automation/common/deploy-kubevirt-and-cdi.sh

# Deploy latest released SSP operator
NAMESPACE=${1:-kubevirt}

RELEASE_BRANCH=${CI_BRANCH}
if [[ -z ${RELEASE_BRANCH} ]] || [[ ${RELEASE_BRANCH} == "main" ]]
then
  PAGE=1
  BRANCHES="[]"
  while true; do
    DATA=$(curl -s "https://api.github.com/repos/kubevirt/ssp-operator/branches?page=$PAGE&per_page=100")
    if [[ $(echo "$DATA" | jq 'length') -eq 0 ]]; then
      break
    fi
    BRANCHES=$(jq -s 'add' <(echo "$BRANCHES") <(echo "$DATA"))
    ((PAGE++))
  done

  RELEASE_BRANCH=$(echo "$BRANCHES" | jq -r '[.[].name | select(startswith("release-v"))] | max_by(ltrimstr("release-v") | split(".") | map(tonumber))')
  if [[ "${RELEASE_BRANCH}" == "null" ]]
  then
    echo "No branch with prefix release-v found" >&2
    exit 1
  fi
fi

# GitHub API returns releases sorted by creation time. Latest release is the first.
LATEST_RELEASED_VERSION=$(curl -s 'https://api.github.com/repos/kubevirt/ssp-operator/releases' |
  jq -r --arg BRANCH "${RELEASE_BRANCH}" '[.[] | select(.target_commitish == $BRANCH) | .name] | .[0]')

oc apply -n $NAMESPACE -f "https://github.com/kubevirt/ssp-operator/releases/download/${LATEST_RELEASED_VERSION}/ssp-operator.yaml"

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
  labels:
    openshift.io/cluster-monitoring: "true"
EOF

oc apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: ${SSP_TEMPLATES_NAMESPACE}
EOF

sed -e "s/%%_SSP_NAME_%%/${SSP_NAME}/g" \
    -e "s/%%_SSP_NAMESPACE_%%/${SSP_NAMESPACE}/g" \
    -e "s/%%_COMMON_TEMPLATES_NAMESPACE_%%/${SSP_TEMPLATES_NAMESPACE}/g" \
    ${SOURCE_DIR}/ssp-cr-template.yaml | oc apply -f -

oc wait --for=condition=Available --timeout=600s -n ${SSP_NAMESPACE} ssp/${SSP_NAME}


export VALIDATOR_IMG=${CI_VALIDATOR_IMG}
export IMG=${CI_OPERATOR_IMG}
export SKIP_CLEANUP_AFTER_TESTS="true"
export TEST_EXISTING_CR_NAME="${SSP_NAME}"
export TEST_EXISTING_CR_NAMESPACE="${SSP_NAMESPACE}"
export IS_UPGRADE_LANE="true"

make deploy functest
