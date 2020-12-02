#!/bin/bash
set -e

NAMESPACE=${1:-kubevirt}

oc apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: ${NAMESPACE}
EOF

# Deploying kuebvirt
LATEST_KUBEVIRT=$(curl -L https://api.github.com/repos/kubevirt/kubevirt/releases | \
            jq '.[] | select(.prerelease==false) | .name' | sort -V | tail -n1 | tr -d '"')

oc apply -n $NAMESPACE -f "https://github.com/kubevirt/kubevirt/releases/download/${LATEST_KUBEVIRT}/kubevirt-operator.yaml"

# Using KubeVirt CR from version v0.35.0
oc apply -n $NAMESPACE -f - <<EOF
apiVersion: kubevirt.io/v1alpha3
kind: KubeVirt
metadata:
  name: kubevirt
  namespace: kubevirt
spec:
  certificateRotateStrategy: {}
  configuration:
    selinuxLauncherType: "virt_launcher.process"
    developerConfiguration:
      featureGates:
        - DataVolumes
        - CPUManager
        - LiveMigration
        # - ExperimentalIgnitionSupport
        # - Sidecar
        # - Snapshot
  customizeComponents: {}
  imagePullPolicy: IfNotPresent
EOF

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

LATEST_CDI=$(curl -L https://api.github.com/repos/kubevirt/containerized-data-importer/releases | \
             jq '.[] | select(.prerelease==false) | .tag_name' | sort -V | tail -n1 | tr -d '"')

oc apply -n ${CDI_NAMESPACE} -f "https://github.com/kubevirt/containerized-data-importer/releases/download/${LATEST_CDI}/cdi-operator.yaml"
oc apply -n ${CDI_NAMESPACE} -f "https://github.com/kubevirt/containerized-data-importer/releases/download/${LATEST_CDI}/cdi-cr.yaml"

echo "Waiting for CDI to be ready..."

oc wait --for=condition=Available --timeout=600s -n ${CDI_NAMESPACE} cdi/cdi
