#!/bin/bash
set -e

source $(dirname "$0")/versions.sh

NAMESPACE=${1:-kubevirt}

oc apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: ${NAMESPACE}
EOF

# Deploy Tekton
#
# Tekton has changed image repository URL, but did not update old release files, so we need to modify the downloaded file
TEKTON_URL="https://github.com/tektoncd/operator/releases/download/${TEKTON_VERSION}/openshift-release.yaml"
curl --silent --show-error --location "${TEKTON_URL}" \
  | sed 's|gcr.io/tekton-releases|ghcr.io/tektoncd|g' \
  | oc apply -f -

# Deploying kuebvirt
oc apply -n $NAMESPACE -f "https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/kubevirt-operator.yaml"

oc apply -n $NAMESPACE -f "https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/kubevirt-cr.yaml"

# Patch to enable needed functionality
oc patch -n $NAMESPACE kubevirt kubevirt --type='json' -p '[{
  "op": "add",
  "path": "/spec/configuration/developerConfiguration/featureGates/-",
  "value": "DataVolumes",
},{
  "op": "add",
  "path": "/spec/configuration/developerConfiguration/featureGates/-",
  "value": "CPUManager",
},{
  "op": "add",
  "path": "/spec/configuration/developerConfiguration/featureGates/-",
  "value": "KubevirtSeccompProfile",
},{
  "op": "replace",
  "path": "/spec/configuration/seccompConfiguration",
  "value": {
    "virtualMachineInstanceProfile": {
      "customProfile": {
        "localhostProfile": "kubevirt/kubevirt.json"
      }
    }
  },
}]'

echo "Waiting for Kubevirt to be ready..."
oc wait --for=condition=Available --timeout=600s -n $NAMESPACE kv/kubevirt

# Deploying CDI
CDI_NAMESPACE=cdi
oc apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: $CDI_NAMESPACE
EOF

oc apply -n ${CDI_NAMESPACE} -f "https://github.com/kubevirt/containerized-data-importer/releases/download/${CDI_VERSION}/cdi-operator.yaml"
oc apply -n ${CDI_NAMESPACE} -f "https://github.com/kubevirt/containerized-data-importer/releases/download/${CDI_VERSION}/cdi-cr.yaml"

echo "Waiting for CDI to be ready..."

oc wait --for=condition=Available --timeout=600s -n ${CDI_NAMESPACE} cdi/cdi
