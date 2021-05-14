# Build the manager binary
FROM registry.access.redhat.com/ubi8/ubi-minimal as builder

RUN microdnf install -y make golang-1.15.* which && microdnf clean all

# Consume required variables so we can work with make
ARG IMG_REPOSITORY
ARG IMG_TAG
ARG IMG
ARG VALIDATOR_REPOSITORY
ARG VALIDATOR_IMG_TAG
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
COPY controllers/ controllers/
COPY internal/ internal/

COPY hack/boilerplate.go.txt hack/boilerplate.go.txt
COPY hack/csv-generator.go hack/csv-generator.go

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on make manager
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on make csv-generator


FROM registry.access.redhat.com/ubi8/ubi-minimal
LABEL org.kubevirt.hco.csv-generator.v1="/csv-generator"

WORKDIR /
COPY --from=builder /workspace/bin/manager .
COPY data/ data/
USER 1000

# Copy csv generator
COPY --from=builder /workspace/bin/csv-generator .
ENTRYPOINT ["/manager"]
