#!/bin/bash
set -e

# This file runs the tests.
# It is run from the root of the repository.

# These evn variables are defined by the CI:
# CI_OPERATOR_IMG - path of the operator image in the local repository accessible on the CI
# CI_VALIDATOR_IMG - path of the validator image in the local repository accessible on the CI

export VALIDATOR_IMG=${CI_VALIDATOR_IMG}
export IMG=${CI_OPERATOR_IMG}
export SKIP_CLEANUP_AFTER_TESTS=true

make deploy functest
