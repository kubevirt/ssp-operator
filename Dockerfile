# Build the manager binary
FROM golang:1.15 as builder

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

COPY hack/csv-generator.go csv-generator.go

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o manager main.go

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o csv-generator csv-generator.go
RUN chmod +x csv-generator

FROM registry.access.redhat.com/ubi8/ubi-minimal
LABEL org.kubevirt.hco.csv-generator.v1="/csv-generator"

WORKDIR /
COPY --from=builder /workspace/manager .
COPY data/ data/
USER 1000

# Copy csv generator
COPY --from=builder /workspace/csv-generator .
ENTRYPOINT ["/manager"]
