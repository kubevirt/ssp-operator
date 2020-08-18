FROM fedora:30

MAINTAINER "The KubeVirt Project" <kubevirt-dev@googlegroups.com>

RUN echo "diskspacecheck=0" >> /etc/dnf/dnf.conf && dnf update -y && dnf install -y git gcc findutils && dnf clean all

ENV GIMME_GO_VERSION=1.13.8 \
    ARCH="amd64" \
    GOPATH="/go" \
    PATH=$PATH:${GOPATH}/go/bin \
    GO111MODULE=on

RUN mkdir -p /gimme && \
    curl -sL https://raw.githubusercontent.com/travis-ci/gimme/master/gimme | HOME=/gimme bash >> /etc/profile.d/gimme.sh

RUN \
    mkdir -p ${GOPATH} && \
    source /etc/profile.d/gimme.sh && \
    eval $(go env) && \
    go get github.com/onsi/ginkgo/ginkgo && \
    go get github.com/onsi/gomega

COPY entrypoint.sh /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
