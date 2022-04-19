#!/usr/bin/env bash

# Select an available container runtime
if [ -z ${KUBEVIRT_CRI} ]; then
    if podman ps >/dev/null; then
        KUBEVIRT_CRI=podman
        echo "selecting podman as container runtime"
    else
        echo "no working container runtime found. Podman seems not to work."
        exit 1
    fi
fi
