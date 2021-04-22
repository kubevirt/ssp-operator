#!/bin/sh

set -ex

if [ -z "$1" ]; then
	echo "usage: $0 <tag>"
	exit 1
fi

TAG="$1"  #TODO: validate tag is vX.Y.Z
COMPONENT="kubevirt-template-validator"
LDFLAGS="\
-X 'kubevirt.io/ssp-operator/internal/template-validator/version.COMPONENT=$COMPONENT' \
-X 'kubevirt.io/ssp-operator/internal/template-validator/version.VERSION=$TAG' "
if git rev-parse &>/dev/null; then
    BRANCH=$( git rev-parse --abbrev-ref HEAD )
    REVISION=$( git rev-parse --short HEAD )
    LDFLAGS="${LDFLAGS}\
-X 'kubevirt.io/ssp-operator/internal/template-validator/version.BRANCH=$BRANCH' \
-X 'kubevirt.io/ssp-operator/internal/template-validator/version.REVISION=$REVISION' "
fi

mkdir -p internal/template-validator/_out

export GO111MODULE=on
export GOPROXY=off
export GOFLAGS=-mod=vendor

go build -v -ldflags="$LDFLAGS" -o internal/template-validator/_out/kubevirt-template-validator internal/template-validator/main.go
