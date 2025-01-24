#!/usr/bin/env bash

check_make() {
    if ! make "$1"; then
        echo "make $1 failed"
        exit 1
    fi
}

check_make vendor
check_make generate
check_make manifests
check_make fmt

# check git status
status=$(git status --porcelain)
if [[ -n $status ]]; then
    echo "There are uncommitted changes."
    echo $status
    exit 1
fi
