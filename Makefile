# Current Operator version
# The value here should be equal to the next version:
# - next minor version on main branch
# - next patch version on release branches
VERSION ?= 0.14.0

#operator-sdk version
OPERATOR_SDK_VERSION ?= v1.17.0


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

# Image URL to use all building/pushing image targets
IMG_REPOSITORY ?= quay.io/kubevirt/ssp-operator
IMG_TAG ?= latest
IMG ?= ${IMG_REPOSITORY}:${IMG_TAG}

# Image URL variables for template-validator
VALIDATOR_REPOSITORY ?= quay.io/kubevirt/kubevirt-template-validator
VALIDATOR_IMG_TAG ?= latest
VALIDATOR_IMG ?= ${VALIDATOR_REPOSITORY}:${VALIDATOR_IMG_TAG}

CRD_OPTIONS ?= "crd:generateEmbeddedObjectMeta=true"

SRC_PATHS_TESTS = ./controllers/... ./internal/... ./hack/... ./webhooks/...
SRC_PATHS_CONTROLLER_GEN = {./controllers/..., ./internal/..., ./hack/..., ./webhooks/...}

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

# Default to podman
SSP_BUILD_RUNTIME ?= podman

ifndef ignore-not-found
  ignore-not-found = false
endif

all: manager

.PHONY: unittest
unittest: generate fmt vet manifests
	go test -v -coverprofile cover.out $(SRC_PATHS_TESTS)
	cd api && go test -v ./...

.PHONY: build-functests
build-functests:
	go test -c ./tests

.PHONY: functest
functest: generate fmt vet manifests
	go test -v -coverprofile cover.out -timeout 0 ./tests/...

# Build manager binary
.PHONY: manager
manager: generate fmt vet
	go build -o bin/manager \
		-ldflags="-X 'kubevirt.io/ssp-operator/internal/operands/template-validator.defaultTemplateValidatorImage=${VALIDATOR_IMG}'" \
		main.go

# Build csv-generator binary
.PHONY: csv-generator
csv-generator: generate fmt vet
	go build -o bin/csv-generator hack/csv-generator.go

# Run against the configured Kubernetes cluster in ~/.kube/config
.PHONY: run
run: generate fmt vet manifests
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
	go mod vendor
	go mod tidy

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
	${SSP_BUILD_RUNTIME} build -t ${IMG} \
		--build-arg IMG_REPOSITORY=${IMG_REPOSITORY} \
		--build-arg IMG_TAG=${IMG_TAG} \
		--build-arg IMG=${IMG} \
		--build-arg VALIDATOR_REPOSITORY=${VALIDATOR_REPOSITORY} \
		--build-arg VALIDATOR_IMG_TAG=${VALIDATOR_IMG_TAG} \
		--build-arg VALIDATOR_IMG=${VALIDATOR_IMG} \
		.

# Push the container image
.PHONY: container-push
container-push:
	${SSP_BUILD_RUNTIME} push ${IMG}

.PHONY: build-template-validator
build-template-validator:
	./hack/build-template-validator.sh ${VERSION}

.PHONY: build-template-validator-container
build-template-validator-container:
	${SSP_BUILD_RUNTIME} build -t ${VALIDATOR_IMG} \
		--build-arg IMG_REPOSITORY=${IMG_REPOSITORY} \
		--build-arg IMG_TAG=${IMG_TAG} \
		--build-arg IMG=${IMG} \
		--build-arg VALIDATOR_REPOSITORY=${VALIDATOR_REPOSITORY} \
		--build-arg VALIDATOR_IMG_TAG=${VALIDATOR_IMG_TAG} \
		--build-arg VALIDATOR_IMG=${VALIDATOR_IMG} \
		. -f validator.Dockerfile

.PHONY: push-template-validator-container
push-template-validator-container:
	${SSP_BUILD_RUNTIME} push ${VALIDATOR_IMG}

# Download controller-gen locally if necessary
CONTROLLER_GEN = $(shell pwd)/bin/controller-gen
.PHONY: controller-gen
controller-gen:
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.10.0)

# Download kustomize locally if necessary
KUSTOMIZE = $(shell pwd)/bin/kustomize
.PHONY: kustomize
kustomize:
	$(call go-get-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v4@v4.5.7)


# Download operator-sdk locally if necessary
OPERATOR_SDK = $(shell pwd)/bin/operator-sdk
$(OPERATOR_SDK):
	curl --create-dirs -JL https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION)/operator-sdk_linux_amd64 -o $(OPERATOR_SDK)
	chmod 0755 $(OPERATOR_SDK)

.PHONY: operator-sdk
operator-sdk: $(OPERATOR_SDK)

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOFLAGS='' GOBIN=$(PROJECT_DIR)/bin go install $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef

# Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: operator-sdk manifests kustomize csv-generator manager-envsubst
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | $(OPERATOR_SDK) generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
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
	${SSP_BUILD_RUNTIME} build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: release
release: container-build container-push build-template-validator-container push-template-validator-container bundle build-functests
	cp ./tests.test _out/tests.test

.PHONY: generate-doc
generate-doc: build-docgen
	_out/metricsdocs > docs/metrics.md

.PHONY: build-docgen
build-docgen:
	go build -ldflags="-s -w" -o _out/metricsdocs ./tools/metricsdocs

cluster-up:
	./hack/kubevirtci.sh up

cluster-down:
	./hack/kubevirtci.sh down

cluster-sync:
	KUSTOMIZE=$(KUSTOMIZE) ./hack/kubevirtci.sh sync

kubevirt-up:
	./hack/kubevirt.sh up

kubevirt-down:
	./hack/kubevirt.sh down

kubevirt-sync:
	KUSTOMIZE=$(KUSTOMIZE) ./hack/kubevirt.sh sync