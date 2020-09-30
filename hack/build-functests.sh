#!/usr/bin/env bash

set -euo pipefail

source hack/common.sh

mkdir -p ${TESTS_OUT_DIR}
ginkgo build ${TESTS_DIR}
mv ${TESTS_DIR}/tests.test ${TESTS_OUT_DIR}
