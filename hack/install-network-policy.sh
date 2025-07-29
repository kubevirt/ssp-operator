#!/bin/bash

set -ex

KUBECTL=${KUBECTL:-kubectl}
NAMESPACE=${1:-kubevirt}

cat <<EOF | "${KUBECTL}" -n "${NAMESPACE}" apply -f -
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny-ssp
spec:
  podSelector:
    matchExpressions:
    - key: name
      operator: In
      values:
      - ssp-operator
      - virt-template-validator
      - vm-console-proxy
  policyTypes:
  - Ingress
  - Egress
EOF
