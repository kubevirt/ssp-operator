FROM registry.access.redhat.com/ubi8/ubi-minimal as builder

RUN microdnf install -y tar gzip && microdnf clean all

RUN curl -L https://go.dev/dl/go1.16.15.linux-amd64.tar.gz | tar -C /usr/local -xzf -
ENV PATH=$PATH:/usr/local/go/bin

ARG VERSION=latest
ARG COMPONENT="kubevirt-template-validator"
ARG BRANCH=master
ARG REVISION=master

WORKDIR /workspace

# Copy the Go Modules manifests and vendor directory
COPY go.mod go.mod
COPY go.sum go.sum
COPY vendor/ vendor/

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY internal/ internal/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -ldflags="-X 'kubevirt.io/ssp-operator/internal/template-validator/version.COMPONENT=$COMPONENT'\
-X 'kubevirt.io/ssp-operator/internal/template-validator/version.VERSION=$VERSION'\
-X 'kubevirt.io/ssp-operator/internal/template-validator/version.BRANCH=$BRANCH'\
-X 'kubevirt.io/ssp-operator/internal/template-validator/version.REVISION=$REVISION'" -o kubevirt-template-validator internal/template-validator/main.go

FROM registry.access.redhat.com/ubi8/ubi-minimal
RUN mkdir -p /etc/webhook/certs

WORKDIR /
COPY --from=builder /workspace/kubevirt-template-validator /usr/sbin/kubevirt-template-validator
USER 1000

ENTRYPOINT [ "/usr/sbin/kubevirt-template-validator" ]
