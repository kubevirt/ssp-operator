#!/bin/bash
set -e

CDI_NAMESPACE=cdi
oc apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: $CDI_NAMESPACE
EOF

LATEST_CDI=v1.36.0

oc apply -n ${CDI_NAMESPACE} -f "https://github.com/kubevirt/containerized-data-importer/releases/download/${LATEST_CDI}/cdi-operator.yaml"
oc apply -n ${CDI_NAMESPACE} -f "https://github.com/kubevirt/containerized-data-importer/releases/download/${LATEST_CDI}/cdi-cr.yaml"

echo "Waiting for CDI to be ready..."

oc wait --for=condition=Available --timeout=600s -n ${CDI_NAMESPACE} cdi/cdi