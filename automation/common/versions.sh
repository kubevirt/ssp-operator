#!/bin/bash

set -e
set -o pipefail

function latest_patch_version() {
  local repo="$1"
  local minor_version="$2"

  # The loop is necessary, because GitHub API call cannot return more than 100 items
  local latest_version=""
  local page=1
  while true ; do
    # Declared separately to not mask return value
    local versions_in_page
    versions_in_page=$(
      curl --fail -s "https://api.github.com/repos/kubevirt/${repo}/releases?per_page=100&page=${page}" |
      jq '.[] | select(.prerelease==false) | .tag_name' |
      tr -d '"'
    )
    if [ $? -ne 0 ]; then
      return 1
    fi

    if [ -z "${versions_in_page}" ]; then
      break
    fi

    latest_version=$(
      echo "${versions_in_page} ${latest_version}" |
      tr " " "\n" |
      grep "^${minor_version}\\." |
      sort --version-sort |
      tail -n1
    )

    ((++page))
  done

  echo "${latest_version}"
}

# The version is fixed for release branch, because newer Kubevirt versions may be incompatible
KUBEVIRT_VERSION="$(latest_patch_version "kubevirt" "v0.49")"

# The version is fixed for release branch, because newer CDI versions may be incompatible
CDI_VERSION="$(latest_patch_version "containerized-data-importer" "v1.46")"
