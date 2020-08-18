#!/usr/bin/env bash

set -e

get_image_tag() {
    local current_commit today
    current_commit="$(git rev-parse HEAD)"
    today="$(date +%Y%m%d)"
    echo "v${today}-${current_commit:0:7}"
}

SSP_DIR="$(readlink -f $(dirname $0)/../)"
WORK_DIR="/go/src/github.com/kubevirt/ssp-operator"
TEST_IMAGE_REGISTRY=${TEST_IMAGE_REGISTRY:-quay.io/kubevirt}
TEST_IMAGE_REPOSITORY=${TEST_IMAGE_REPOSITORY:-ssp-builder}
TEST_IMAGE_TAG=$(get_image_tag)

TESTS_BUILD_TAG="${TEST_IMAGE_REGISTRY}/${TEST_IMAGE_REPOSITORY}:${TEST_IMAGE_TAG}"
TESTS_BUILD_DIR=${SSP_DIR}/tests/build
TESTS_DIR=$SSP_DIR/tests
OUT_DIR=$SSP_DIR/_out
TESTS_OUT_DIR=$OUT_DIR/tests
