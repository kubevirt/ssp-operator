# Current Operator version
VERSION ?= 0.1.3
# Default bundle image tag
BUNDLE_IMG ?= controller-bundle:$(VERSION)
#operator-sdk version
OPERATOR_SDK_VERSION ?= v1.4.2

# Options for 'bundle-build'
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# Image URL to use all building/pushing image targets
IMG_REPOSITORY ?= quay.io/kubevirt/ssp-operator
IMG_TAG ?= latest
IMG ?= ${IMG_REPOSITORY}:${IMG_TAG}

# Image URL variables for template-validator
VALIDATOR_REPOSITORY ?= quay.io/kubevirt/kubevirt-template-validator
VALIDATOR_IMG_TAG ?= latest
VALIDATOR_IMG ?= ${VALIDATOR_REPOSITORY}:${VALIDATOR_IMG_TAG}

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

ifeq (, $(shell which oc))
OC = kubectl
else
OC = oc
endif

all: manager

# Run tests
unittest: generate fmt vet manifests
	go test -v -coverprofile cover.out ./api/... ./controllers/... ./internal/... ./hack/...

# TODO - skipping build container for functests until OCP CI is ready
#functest: generate fmt vet manifests build-functests run-functest

build-functests:
	go test -c ./tests

functest: generate fmt vet manifests
	go test -v -coverprofile cover.out -timeout 0 ./tests/...

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

# Build csv-generator binary
csv-generator: generate fmt vet
	go build -o csv-generator hack/csv-generator.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./main.go

# Install CRDs into a cluster
install: manifests kustomize
	$(KUSTOMIZE) build config/crd | $(OC) apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests kustomize
	$(KUSTOMIZE) build config/crd | $(OC) delete -f -

manager-envsubst:
	cd config/manager && VALIDATOR_IMG=${VALIDATOR_IMG} envsubst < manager.template.yaml > manager.yaml

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests kustomize manager-envsubst
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | $(OC) apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) crd rbac:roleName=operator-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

operator-sdk:
	curl -JL https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION)/operator-sdk_linux_amd64 -o operator-sdk
	chmod 0755 operator-sdk

# Build the container image
container-build: unittest bundle
	mkdir -p data/olm-catalog
	mkdir -p data/crd
	cp bundle/manifests/ssp-operator.clusterserviceversion.yaml data/olm-catalog/ssp-operator.clusterserviceversion.yaml
	cp bundle/manifests/ssp.kubevirt.io_ssps.yaml data/crd/ssp.kubevirt.io_ssps.yaml
	docker build -t ${IMG} .

# Push the container image
container-push:
	docker push ${IMG}

build-template-validator:
	./hack/build-template-validator.sh ${VERSION}

build-template-validator-container:
	docker build -t ${VALIDATOR_IMG} . -f validator.Dockerfile

push-template-validator-container:
	docker push ${VALIDATOR_IMG}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

kustomize:
ifeq (, $(shell which kustomize))
	@{ \
	set -e ;\
	KUSTOMIZE_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$KUSTOMIZE_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/kustomize/kustomize/v3@v3.5.4 ;\
	rm -rf $$KUSTOMIZE_GEN_TMP_DIR ;\
	}
KUSTOMIZE=$(GOBIN)/kustomize
else
KUSTOMIZE=$(shell which kustomize)
endif

# Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: kustomize operator-sdk manifests csv-generator manager-envsubst
	./operator-sdk generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | ./operator-sdk generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	./csv-generator --csv-version $(VERSION) --namespace kubevirt --operator-image $(IMG) --operator-version $(VERSION) \
			--file bundle/manifests/ssp-operator.clusterserviceversion.yaml \
			--webhook-port 9443 --webhook-remove-certs > bundle/manifests/ssp-operator.clusterserviceversion.yaml.new
	mv bundle/manifests/ssp-operator.clusterserviceversion.yaml.new bundle/manifests/ssp-operator.clusterserviceversion.yaml
	./operator-sdk bundle validate ./bundle
	rm -rf _out
	mkdir -p _out
	cp bundle/manifests/ssp.kubevirt.io_ssps.yaml _out/olm-crds.yaml
	cp bundle/manifests/ssp-operator.clusterserviceversion.yaml _out/olm-ssp-operator.clusterserviceversion.yaml
	$(KUSTOMIZE) build config/default > _out/ssp-operator.yaml

# Build the bundle image.
.PHONY: bundle-build
bundle-build:
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: release
release: container-build container-push build-template-validator-container push-template-validator-container bundle build-functests
	cp ./tests.test _out/tests.test
