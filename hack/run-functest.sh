#!/usr/bin/env bash

set -eo pipefail

source hack/common.sh

${TESTS_OUT_DIR}/tests.test -ginkgo.v -test.v
