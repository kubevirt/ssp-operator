#!/bin/bash
set -e

NAMESPACE=${1:-kubevirt}

# Deploying the operator
LATEST_SSP_OPERATOR=$(curl -L https://api.github.com/repos/kubevirt/kubevirt-ssp-operator/releases | \
            jq '.[] | select(.prerelease==false) | .name' | sort -V | tail -n1 | tr -d '"')

oc apply -n $NAMESPACE -f "https://github.com/kubevirt/kubevirt-ssp-operator/releases/download/${LATEST_SSP_OPERATOR}/kubevirt-ssp-operator-crd.yaml"
oc apply -n $NAMESPACE -f "https://github.com/kubevirt/kubevirt-ssp-operator/releases/download/${LATEST_SSP_OPERATOR}/kubevirt-ssp-operator.yaml"
oc apply -n $NAMESPACE -f "https://github.com/kubevirt/kubevirt-ssp-operator/releases/download/${LATEST_SSP_OPERATOR}/kubevirt-ssp-operator-cr.yaml"

# Wait for the operator deployment to be ready
echo "Waiting for Kubevirt SSP Operator to be ready..."
oc wait --for=condition=Available --timeout=600s -n $NAMESPACE deployment/kubevirt-ssp-operator

# Wait for the operator operands to be ready
oc wait --for=condition=Available --timeout=600s -n $NAMESPACE KubevirtCommonTemplatesBundle/kubevirt-common-template-bundle
oc wait --for=condition=Available --timeout=600s -n $NAMESPACE KubevirtMetricsAggregation/kubevirt-metrics-aggregation
oc wait --for=condition=Available --timeout=600s -n $NAMESPACE KubevirtNodeLabellerBundle/kubevirt-node-labeller-bundle
oc wait --for=condition=Available --timeout=600s -n $NAMESPACE KubevirtTemplateValidator/kubevirt-template-validator

