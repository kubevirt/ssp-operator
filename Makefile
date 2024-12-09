# Current Operator version
# The value here should be equal to the next version:
# - next minor version on main branch
# - next patch version on release branches
VERSION ?= 0.14.0

#operator-sdk version
OPERATOR_SDK_VERSION ?= v1.25.1

# CHANNELS define the bundle channels used in the bundle.
# Add a new line here if you would like to change its default config. (E.g CHANNELS = "preview,fast,stable")
# To re-generate a bundle for other specific channels without changing the standard setup, you can:
# - use the CHANNELS as arg of the bundle target (e.g make bundle CHANNELS=preview,fast,stable)
# - use environment variables to overwrite this value (e.g export CHANNELS="preview,fast,stable")
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif

# DEFAULT_CHANNEL defines the default channel used in the bundle.
# Add a new line here if you would like to change its default config. (E.g DEFAULT_CHANNEL = "stable")
# To re-generate a bundle for any other default channel without changing the default setup, you can:
# - use the DEFAULT_CHANNEL as arg of the bundle target (e.g make bundle DEFAULT_CHANNEL=stable)
# - use environment variables to overwrite this value (e.g export DEFAULT_CHANNEL="stable")
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# BUNDLE_IMG defines the image:tag used for the bundle.
# You can use it as an arg. (E.g make bundle-build BUNDLE_IMG=<some-registry>/<project-name-bundle>:<tag>)
BUNDLE_IMG ?= controller-bundle:$(VERSION)

# BUNDLE_GEN_FLAGS are the flags passed to the operator-sdk generate bundle command
BUNDLE_GEN_FLAGS ?= -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)

# USE_IMAGE_DIGESTS defines if images are resolved via tags or digests
# You can enable this value if you would like to use SHA Based Digests
# To enable set flag to true
USE_IMAGE_DIGESTS ?= false
ifeq ($(USE_IMAGE_DIGESTS), true)
    BUNDLE_GEN_FLAGS += --use-image-digests
endif

# Image URL to use all building/pushing image targets
IMG_REPOSITORY ?= quay.io/kubevirt/ssp-operator
IMG_TAG ?= latest
IMG ?= ${IMG_REPOSITORY}:${IMG_TAG}

# Image URL variables for template-validator
VALIDATOR_REPOSITORY ?= quay.io/kubevirt/kubevirt-template-validator
VALIDATOR_IMG_TAG ?= latest
VALIDATOR_IMG ?= ${VALIDATOR_REPOSITORY}:${VALIDATOR_IMG_TAG}

CRD_OPTIONS ?= "crd:generateEmbeddedObjectMeta=true"

SRC_PATHS_TESTS = ./internal/... ./hack/... ./webhooks/... ./pkg/...
SRC_PATHS_CONTROLLER_GEN = {./internal/..., ./hack/..., ./webhooks/..., ./pkg/...}
SRC_PATHS_MONITORING_LINTER = ./internal/...  ./pkg/...

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

ifeq (, $(shell which oc))
OC = kubectl
else
OC = oc
endif

ifndef ignore-not-found
  ignore-not-found = false
endif

# Get the current architecture
ARCH := $(shell uname -m)
ifeq ($(ARCH), x86_64)
	ARCH := amd64
endif

# Get the current OS
OS := $(shell uname | tr '[:upper:]' '[:lower:]')

all: manager

.PHONY: unittest
unittest: generate lint fmt vet manifests metrics-rules-test lint-monitoring
	go test -v -coverprofile cover.out $(SRC_PATHS_TESTS)
	cd api && go test -v ./...

.PHONY: build-functests
build-functests:
	go test -c ./tests

GOMOD_PATH ?= ./go.mod
GINKGO_VERSION ?= $(shell grep -E '^\s*github\.com/onsi/ginkgo/v[0-9]+' $(GOMOD_PATH) | awk '{print $$2}')
GINKGO_TIMEOUT ?= 2h
GINKGO_FOCUS ?=

.PHONY: ginkgo
ginkgo: getginkgo vendor

.PHONY: getginkgo
getginkgo:
	go get github.com/onsi/ginkgo/v2@$(GINKGO_VERSION)

.PHONY: functest
functest: ginkgo generate fmt vet manifests
	go run github.com/onsi/ginkgo/v2/ginkgo@$(GINKGO_VERSION) -v -coverprofile cover.out -timeout $(GINKGO_TIMEOUT) --focus="$(GINKGO_FOCUS)" ./tests/...

# Build manager binary
.PHONY: manager
manager: generate lint fmt vet
	go build -o bin/manager \
		-ldflags="-X 'kubevirt.io/ssp-operator/internal/operands/template-validator.defaultTemplateValidatorImage=${VALIDATOR_IMG}'" \
		main.go

# Build csv-generator binary
.PHONY: csv-generator
csv-generator: generate lint fmt vet
	go build -o bin/csv-generator hack/csv-generator.go

# Run against the configured Kubernetes cluster in ~/.kube/config
.PHONY: run
run: generate lint fmt vet manifests
	go run ./main.go

# Install CRDs into a cluster
.PHONY: install
install: manifests kustomize
	$(KUSTOMIZE) build config/crd | $(OC) apply -f -

# Uninstall CRDs from a cluster
.PHONY: uninstall
uninstall: manifests kustomize
	$(KUSTOMIZE) build config/crd | $(OC) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: manager-envsubst
manager-envsubst:
	cd config/manager && VALIDATOR_IMG=${VALIDATOR_IMG} envsubst < manager.template.yaml > manager.yaml

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
.PHONY: deploy
deploy: manifests kustomize manager-envsubst
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | $(OC) apply -f -

# UnDeploy controller from the configured Kubernetes cluster in ~/.kube/config
.PHONY: undeploy
undeploy:
	$(KUSTOMIZE) build config/default | $(OC) delete --ignore-not-found=$(ignore-not-found) -f -

# Generate manifests e.g. CRD, RBAC etc.
.PHONY: manifests
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=operator-role webhook "paths=$(SRC_PATHS_CONTROLLER_GEN)" output:crd:artifacts:config=config/crd/bases
	cd api && $(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=operator-role webhook "paths=./..." output:crd:artifacts:config=../config/crd/bases

# Run go fmt against code
.PHONY: fmt
fmt:
	go fmt ./...

# Run go vet against code
.PHONY: vet
vet:
	go vet ./...

# Update vendor modules
.PHONY: vendor
vendor:
	cd api && go mod tidy && go mod vendor
	go mod tidy
	go mod vendor

# Validate that this repository does not contain offensive language
.PHONY: validate-no-offensive-lang
validate-no-offensive-lang:
	./hack/validate-no-offensive-lang.sh

# Generate code
.PHONY: generate
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" "paths=$(SRC_PATHS_CONTROLLER_GEN)"
	cd api && $(CONTROLLER_GEN) object:headerFile="../hack/boilerplate.go.txt" "paths=./..."

# Build the container image
.PHONY: container-build
container-build: unittest bundle
	mkdir -p data/olm-catalog
	mkdir -p data/crd
	cp bundle/manifests/ssp-operator.clusterserviceversion.yaml data/olm-catalog/ssp-operator.clusterserviceversion.yaml
	cp bundle/manifests/ssp.kubevirt.io_ssps.yaml data/crd/ssp.kubevirt.io_ssps.yaml
	podman manifest rm ${IMG} || true
	podman build --build-arg TARGET_ARCH=amd64 --build-arg VALIDATOR_IMG=${VALIDATOR_IMG} --manifest=${IMG} . && \
    podman build --build-arg TARGET_ARCH=s390x --build-arg VALIDATOR_IMG=${VALIDATOR_IMG} --manifest=${IMG} .

# Push the container image
.PHONY: container-push
container-push:
	podman manifest push ${IMG} ${IMG}

.PHONY: build-template-validator
build-template-validator:
	./hack/build-template-validator.sh ${VERSION}

.PHONY: build-template-validator-container
build-template-validator-container:
	podman manifest rm ${VALIDATOR_IMG} || true && \
	podman build --build-arg TARGET_ARCH=amd64 --manifest=${VALIDATOR_IMG} . -f validator.Dockerfile && \
	podman build --build-arg TARGET_ARCH=s390x --manifest=${VALIDATOR_IMG} . -f validator.Dockerfile

.PHONY: push-template-validator-container
push-template-validator-container:
	podman manifest push ${VALIDATOR_IMG} ${VALIDATOR_IMG}


##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
OPERATOR_SDK ?= $(LOCALBIN)/operator-sdk
MONITORING_LINTER ?= $(LOCALBIN)/monitoringlinter

## Tool Versions
KUSTOMIZE_VERSION ?= v4.5.7
CONTROLLER_TOOLS_VERSION ?= v0.14.0
MONITORING_LINTER_REVISION ?= e2be790

KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	test -s $(LOCALBIN)/kustomize || curl -s $(KUSTOMIZE_INSTALL_SCRIPT) | bash -s -- $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN)

# The final line allows for downloading and copying into the LOCALBIN folder when cross-compiling, as GOBIN is not compatible with setting a different GOARCH
.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen || \
	GOBIN=$(LOCALBIN) GOARCH=$(ARCH) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

# Download operator-sdk locally if necessary
$(OPERATOR_SDK): $(LOCALBIN)
	curl --create-dirs -JL https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION)/operator-sdk_linux_$(ARCH) -o $(OPERATOR_SDK)
	chmod 0755 $(OPERATOR_SDK)

.PHONY: operator-sdk
operator-sdk: $(OPERATOR_SDK)

# Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: operator-sdk manifests kustomize csv-generator manager-envsubst
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | $(OPERATOR_SDK) generate bundle $(BUNDLE_GEN_FLAGS)
	./bin/csv-generator --csv-version $(VERSION) --namespace kubevirt --operator-image $(IMG) --operator-version $(VERSION) \
			--file bundle/manifests/ssp-operator.clusterserviceversion.yaml \
			--webhook-port 9443 --webhook-remove-certs > bundle/manifests/ssp-operator.clusterserviceversion.yaml.new
	mv bundle/manifests/ssp-operator.clusterserviceversion.yaml.new bundle/manifests/ssp-operator.clusterserviceversion.yaml
	$(OPERATOR_SDK) bundle validate ./bundle
	rm -rf _out
	mkdir -p _out
	cp bundle/manifests/ssp.kubevirt.io_ssps.yaml _out/olm-crds.yaml
	cp bundle/manifests/ssp-operator.clusterserviceversion.yaml _out/olm-ssp-operator.clusterserviceversion.yaml
	$(KUSTOMIZE) build config/default > _out/ssp-operator.yaml

# Build the bundle image.
.PHONY: bundle-build
bundle-build:
	podman build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: release
release: container-build container-push build-template-validator-container push-template-validator-container bundle build-functests
	cp ./tests.test _out/tests.test

.PHONY: generate-doc
generate-doc: build-docgen
	_out/metricsdocs > docs/metrics.md

.PHONY: build-docgen
build-docgen:
	go build -ldflags="-s -w" -o _out/metricsdocs ./tools/metricsdocs

.PHONY: cluster-up
cluster-up:
	./hack/kubevirtci.sh up

.PHONY: cluster-down
cluster-down:
	./hack/kubevirtci.sh down

.PHONY: cluster-sync
cluster-sync:
	KUSTOMIZE=$(KUSTOMIZE) ./hack/kubevirtci.sh sync

.PHONY: kubevirt-up
kubevirt-up:
	./hack/kubevirt.sh up

.PHONY: kubevirt-down
kubevirt-down:
	./hack/kubevirt.sh down

.PHONY: kubevirt-sync
kubevirt-sync:
	KUSTOMIZE=$(KUSTOMIZE) ./hack/kubevirt.sh sync

GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint
GOLANGCI_LINT_VERSION ?= v1.55.2

.PHONY: lint
lint:
	test -s $(GOLANGCI_LINT) || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(LOCALBIN) $(GOLANGCI_LINT_VERSION)
	$(GOLANGCI_LINT) run --timeout 5m

.PHONY: lint-metrics
lint-metrics:
	./hack/prom_metric_linter.sh  --operator-name="kubevirt" --sub-operator-name="ssp"

.PHONY: lint-monitoring
lint-monitoring: $(LOCALBIN)
	test -s $(LOCALBIN)/monitoringlinter || GOBIN=$(LOCALBIN) go install github.com/kubevirt/monitoring/monitoringlinter/cmd/monitoringlinter@$(MONITORING_LINTER_REVISION)
	$(MONITORING_LINTER) $(SRC_PATHS_MONITORING_LINTER)

PROMTOOL ?= $(LOCALBIN)/promtool
PROMTOOL_VERSION ?= 2.44.0

.PHONY: promtool
promtool: $(PROMTOOL)
$(PROMTOOL): $(LOCALBIN)
	test -s $(PROMTOOL) || curl -sSfL "https://github.com/prometheus/prometheus/releases/download/v$(PROMTOOL_VERSION)/prometheus-$(PROMTOOL_VERSION).${OS}-$(ARCH).tar.gz" | \
		tar xvzf - --directory=$(LOCALBIN) "prometheus-$(PROMTOOL_VERSION).${OS}-$(ARCH)"/promtool --strip-components=1

METRIC_RULES_WRITER ?= $(LOCALBIN)/metrics-rules-writer

.PHONY: build-metric-rules-writer
build-metric-rules-writer: $(LOCALBIN)
	go build -o $(METRIC_RULES_WRITER) tools/test-rules-writer/test_rules_writer.go

.PHONY: metrics-rules-test
metrics-rules-test: build-metric-rules-writer promtool
	./hack/metrics-rules-test.sh $(METRIC_RULES_WRITER) "./pkg/monitoring/rules/rules-tests.yaml"
