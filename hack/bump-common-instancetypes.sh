#!/bin/bash
#
# Copyright 2024 Red Hat, Inc.
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

target_branch=${1:-"main"}

_common_instancetypes_base_url="https://github.com/kubevirt/common-instancetypes/releases/download"
_cluster_instancetypes_path="data/common-instancetypes-bundle/common-clusterinstancetypes-bundle.yaml"
_cluster_preferences_path="data/common-instancetypes-bundle/common-clusterpreferences-bundle.yaml"

function latest_version() {
    if [[ $target_branch == "main" ]]; then
        curl --fail -s "https://api.github.com/repos/kubevirt/common-instancetypes/releases/latest" |
            jq -r '.tag_name'
    else
        curl --fail -s https://api.github.com/repos/kubevirt/common-instancetypes/releases?per_page=100 |
            jq -r '.[] | select(.target_commitish == '\""${target_branch}"\"') | .tag_name' | head -n1
    fi
}

function checksum() {
    local version="$1"
    local file="$2"

    curl -L "${_common_instancetypes_base_url}/${version}/CHECKSUMS.sha256" |
        grep "${file}" | cut -d " " -f 1
}

version=$(latest_version)
instancetypes_checksum=$(checksum "${version}" "common-clusterinstancetypes-bundle-${version}.yaml")
preferences_checksum=$(checksum "${version}" "common-clusterpreferences-bundle-${version}.yaml")

curl \
    -L "${_common_instancetypes_base_url}/${version}/common-clusterinstancetypes-bundle-${version}.yaml" \
    -o "${_cluster_instancetypes_path}"
echo "${instancetypes_checksum} ${_cluster_instancetypes_path}" | sha256sum --check --strict

curl \
    -L "${_common_instancetypes_base_url}/${version}/common-clusterpreferences-bundle-${version}.yaml" \
    -o "${_cluster_preferences_path}"
echo "${preferences_checksum} ${_cluster_preferences_path}" | sha256sum --check --strict
