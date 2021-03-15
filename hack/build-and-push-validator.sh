#!/bin/bash

set -e

VERSION="${1:-devel}"

if [ -z "$QUAY_BOT_PASS" ] || [ -z "$QUAY_BOT_USER" ]; then
	echo "missing QUAY_BOT_{USER,PASS} env vars"
	exit 1
fi

echo "$QUAY_BOT_PASS" | docker login -u="$QUAY_BOT_USER" --password-stdin quay.io
docker build -t quay.io/kubevirt/kubevirt-template-validator:$VERSION ./internal/template-validator/
docker push quay.io/kubevirt/kubevirt-template-validator:$VERSION
docker logout quay.io
