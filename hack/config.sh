#!/usr/bin/env bash

# Select an available container runtime
if [ -z ${KUBEVIRT_CRI} ]; then
    if podman ps >/dev/null; then
        KUBEVIRT_CRI=podman
        echo "selecting podman as container runtime"
    elif docker ps >/dev/null; then
        KUBEVIRT_CRI=docker
        echo "selecting docker as container runtime"
    else
        echo "no working container runtime found. Neither docker nor podman seems to work."
        exit 1
    fi
fi
