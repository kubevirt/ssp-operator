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

set -ex

export KUBEVIRTCI_TAG=${KUBEVIRTCI_TAG:-$(curl -sfL https://storage.googleapis.com/kubevirt-prow/release/kubevirt/kubevirtci/latest)}
export KUBEVIRT_DEPLOY_CDI="true"

_base_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
_cluster_up_dir="${_base_dir}/_cluster_up"
_kubectl="${_cluster_up_dir}/cluster-up/kubectl.sh"
_kubessh="${_cluster_up_dir}/cluster-up/ssh.sh"
_kubevirtcicli="${_cluster_up_dir}/cluster-up/cli.sh"
_action=$1
shift

function kubevirtci::fetch_kubevirtci() {
	if [[ ! -d ${_cluster_up_dir} ]]; then
    git clone --depth 1 --branch "${KUBEVIRTCI_TAG}" https://github.com/kubevirt/kubevirtci.git "${_cluster_up_dir}"
  fi
}

function kubevirtci::up() {
  make cluster-up -C "${_cluster_up_dir}"
  KUBECONFIG=$(kubevirtci::kubeconfig)
  export KUBECONFIG
  echo "adding kubevirtci registry to cdi-insecure-registries"
  ${_kubectl} patch configmap cdi-insecure-registries -n cdi --type merge -p '{"data":{"kubevirtci": "registry:5000"}}'
  echo "installing kubevirt..."
  LATEST=$(curl -L https://storage.googleapis.com/kubevirt-prow/devel/release/kubevirt/kubevirt/stable.txt)
  ${_kubectl} apply -f "https://github.com/kubevirt/kubevirt/releases/download/${LATEST}/kubevirt-operator.yaml"
  ${_kubectl} apply -f "https://github.com/kubevirt/kubevirt/releases/download/${LATEST}/kubevirt-cr.yaml"
  echo "waiting for kubevirt to become ready, this can take a few minutes..."
  ${_kubectl} -n kubevirt wait kv kubevirt --for condition=Available --timeout=15m
}

function kubevirtci::down() {
  make cluster-down -C "${_cluster_up_dir}"
}

function kubevirtci::registry() {
  port=$(${_kubevirtcicli} ports registry 2>/dev/null)
  echo "localhost:${port}"
}

function kubevirtci::sync() {
  KUBECTL=${_kubectl} KUBESSH=${_kubessh} KUBEVIRTCI_REGISTERY=$(kubevirtci::registry) "${_base_dir}/hack/sync.sh"
}

function kubevirtci::kubeconfig() {
  "${_cluster_up_dir}/cluster-up/kubeconfig.sh"
}

kubevirtci::fetch_kubevirtci

case ${_action} in
  "up")
    kubevirtci::up
    ;;
  "down")
    kubevirtci::down
    ;;
  "sync")
    kubevirtci::sync
    ;;
  "ssh")
    ${_kubessh} "$@"
    ;;
  "kubeconfig")
    kubevirtci::kubeconfig
    ;;
  "registry")
    kubevirtci::registry
    ;;
  "kubectl")
    ${_kubectl} "$@"
    ;;
  *)
    echo "No command provided, known commands are 'up', 'down', 'sync', 'ssh', 'kubeconfig', 'registry', 'kubectl'"
    exit 1
    ;;
esac

