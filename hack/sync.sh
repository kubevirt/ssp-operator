#!/bin/bash
#
# Copyright 2022 Red Hat, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

source ./hack/common.sh

# Please note that the validation webhooks are not currently deployed so take care when using a custom SSP CR
export KUBEVIRT_SSP=${KUBEVIRT_SSP:-./config/samples/ssp_v1beta2_ssp.yaml}

if [ -z "${KUBECTL}" ] || [ -z "${KUBESSH}" ] || [ -z "${KUSTOMIZE}" ] || [ -z "${KUBEVIRTCI_REGISTERY}" ]; then
    echo "${BASH_SOURCE[0]} expects the following env variables to be provided: KUBECTL, KUBESSH, KUSTOMIZE and KUBEVIRTCI_REGISTERY."
    exit 1
fi

# The ssps.ssp.kubevirt.io CRD might not be present if this is the first sync so check before attempting to delete any actual objects
if "${KUBECTL}" get crd/ssps.ssp.kubevirt.io; then
    "${KUBECTL}" delete --ignore-not-found=true ssps -n kubevirt --all
    _found_ssps=$("${KUBECTL}" get ssps -n kubevirt -oname)
    if [[ -n "${_found_ssps}" ]]; then
        echo "SSP resources ${_found_ssps} still active, please delete manually and attempt to sync again."
        exit 1
    fi
fi
# The remaining CRDs should be present already from the k8s and KubeVirt installs so just ignore object not found failures
"${KUBECTL}" delete --ignore-not-found=true deployment/ssp-operator -n kubevirt

nodes=()
for i in $(seq 1 "${KUBEVIRT_NUM_NODES}"); do
    nodes+=("node$(printf "%02d" "${i}")")
done

IMG_REPOSITORY=${KUBEVIRTCI_REGISTERY}/kubevirt/ssp-operator make container-build container-push

for node in "${nodes[@]}"; do
    "${KUBESSH}" "${node}" "sudo docker pull registry:5000/kubevirt/ssp-operator"
done

# TODO - clean this up, move manifest generation out of the actual tree etc.
cd config/manager && ${KUSTOMIZE} edit set image controller=registry:5000/kubevirt/ssp-operator && cd - || exit
"${KUSTOMIZE}" build config/kubevirtci | "${KUBECTL}" apply -f -
"${KUBECTL}" wait deployment/ssp-operator -n kubevirt --for=condition=Available --timeout="540s"
"${KUBECTL}" apply -n kubevirt -f "${KUBEVIRT_SSP}"

