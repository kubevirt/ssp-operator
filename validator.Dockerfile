# Build this Dockerfile using the command: make build-template-validator-container
#
# This multi-stage image approach prevents issues related to cached builder images,
# which may be incompatible due to different architectures, potentially slowing down or breaking the build process.
#
# By utilizing Go cross-compilation, we can build the target Go binary from the host architecture
# and then copy it to the target image with the desired architecture.

ARG TARGET_ARCH=amd64
FROM registry.access.redhat.com/ubi9/ubi-minimal as builder
ARG TARGET_ARCH

RUN microdnf install -y make tar gzip which && microdnf clean all
RUN export ARCH=$(uname -m | sed 's/x86_64/amd64/'); curl -L https://go.dev/dl/go1.23.2.linux-${ARCH}.tar.gz | tar -C /usr/local -xzf -
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
COPY internal/ internal/
COPY pkg/ pkg/

# Compile for the TARGET_ARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGET_ARCH} GO111MODULE=on go build \
-a -ldflags="-X 'kubevirt.io/ssp-operator/internal/template-validator/version.COMPONENT=$COMPONENT'\
-X 'kubevirt.io/ssp-operator/internal/template-validator/version.VERSION=$VERSION'\
-X 'kubevirt.io/ssp-operator/internal/template-validator/version.BRANCH=$BRANCH'\
-X 'kubevirt.io/ssp-operator/internal/template-validator/version.REVISION=$REVISION'" \
-o kubevirt-template-validator internal/template-validator/main.go

# Hack: Create an empty directory in the builder image and copy it to the target image to avoid triggering any architecture-specific commands
RUN mkdir emptydir


FROM --platform=linux/${TARGET_ARCH} registry.access.redhat.com/ubi9/ubi-micro

# Hack: Refer to the last comment in the builder image.
COPY --from=builder /workspace/emptydir /etc/webhook/certs

WORKDIR /
COPY --from=builder /workspace/kubevirt-template-validator /usr/sbin/kubevirt-template-validator
USER 1000

ENTRYPOINT [ "/usr/sbin/kubevirt-template-validator" ]
