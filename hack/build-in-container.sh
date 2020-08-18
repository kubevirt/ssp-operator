#!/usr/bin/env bash

set -e

source hack/config.sh
source hack/common.sh

main() {
  # Build the encapsulated compile and test container
  (cd "${TESTS_BUILD_DIR}" && $KUBEVIRT_CRI build --tag "${TESTS_BUILD_TAG}" .)
  $KUBEVIRT_CRI push "${TESTS_BUILD_TAG}"
  echo "Successfully created and pushed new test utils image: ${TESTS_BUILD_TAG}"
}

main "$@"
