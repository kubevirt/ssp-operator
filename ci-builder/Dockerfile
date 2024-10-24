# This Dockerfile is used in openshift CI
FROM quay.io/fedora/fedora:42

RUN curl -L https://go.dev/dl/go1.22.4.linux-amd64.tar.gz | tar -C /usr/local -xzf -
ENV PATH=$PATH:/usr/local/go/bin

# Install dependencies and tools
RUN dnf install -y jq git make findutils which podman gcc gettext && dnf clean all

# Download latest stable oc client binary
RUN curl -L https://mirror.openshift.com/pub/openshift-v4/x86_64/clients/ocp/stable/openshift-client-linux.tar.gz | tar -C /usr/local/bin -xzf - oc && \
    chmod +x /usr/local/bin/oc
