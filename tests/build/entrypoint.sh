#!/usr/bin/env bash

set -eo pipefail

source /etc/profile.d/gimme.sh

export PATH=${GOPATH}/bin:$PATH

eval "$@"
