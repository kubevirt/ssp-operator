#!/usr/bin/env bash

set -ex

source hack/config.sh
source hack/common.sh

# Execute the build
[ -t 1 ] && USE_TTY="-it"
$KUBEVIRT_CRI run ${USE_TTY} \
    --rm \
    -v ${SSP_DIR}:${WORK_DIR}:rw,Z \
    -e RUN_UID=$(id -u) \
    -e RUN_GID=$(id -g) \
    -e GOCACHE=/gocache \
    -w ${WORK_DIR} \
    ${TESTS_BUILD_TAG} "$1"
