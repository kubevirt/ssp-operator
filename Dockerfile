# Build this Dockerfile using the command: make container-build
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

RUN ARCH=$(uname -m | sed 's/x86_64/amd64/') && \
    curl -L https://go.dev/dl/go1.23.2.linux-${ARCH}.tar.gz -o go.tar.gz && \
    tar -C /usr/local -xzf go.tar.gz && \
    rm go.tar.gz

ENV PATH=$PATH:/usr/local/go/bin

# Consume required variables so we can work with make
ARG VALIDATOR_IMG

WORKDIR /workspace
# Copy the Go Modules manifests and vendor directory
COPY go.mod go.mod
COPY go.sum go.sum
COPY vendor/ vendor/

# Copy the go source
COPY Makefile Makefile
COPY main.go main.go
COPY api/ api/
COPY internal/ internal/
COPY pkg/ pkg/
COPY webhooks/ webhooks/

COPY hack/boilerplate.go.txt hack/boilerplate.go.txt
COPY hack/csv-generator.go hack/csv-generator.go

# Copy .golangci.yaml so we can run lint as part of the build process
COPY .golangci.yaml .golangci.yaml

# Build the manager and csv-generator binaries
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGET_ARCH} GO111MODULE=on make manager csv-generator


FROM --platform=linux/${TARGET_ARCH} registry.access.redhat.com/ubi9/ubi-micro
LABEL org.kubevirt.hco.csv-generator.v1="/csv-generator"

WORKDIR /
COPY --from=builder /workspace/bin/manager .
COPY data/ data/
USER 1000

# Copy csv generator
COPY --from=builder /workspace/bin/csv-generator .
ENTRYPOINT ["/manager"]
