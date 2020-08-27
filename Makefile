OPERATOR_SDK_VERSION ?= v0.18.2
IMAGE_REGISTRY ?= quay.io/oyahud
OPERATOR_IMAGE ?= kubevirt-ssp-operator
IMAGE_TAG ?= latest
OPERATOR_BUILD_ARGS ?= "-v -mod=vendor"

operator-sdk:
	curl -JL https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION)/operator-sdk-$(OPERATOR_SDK_VERSION)-x86_64-linux-gnu -o operator-sdk
	chmod 0755 ./operator-sdk

operator-build: operator-sdk
	go mod tidy
	go mod vendor
	./operator-sdk build $(IMAGE_REGISTRY)/$(OPERATOR_IMAGE):$(IMAGE_TAG) --go-build-args $(OPERATOR_BUILD_ARGS)

operator-push:
	docker push $(IMAGE_REGISTRY)/$(OPERATOR_IMAGE):$(IMAGE_TAG)

generate: operator-sdk
	./operator-sdk generate k8s
	./operator-sdk generate crds

unittest:
	go test -count=1 ./cmd/... ./internal/... ./pkg/...

functest:
	go test -count=1 ./tests/...

clean:
	-rm -f operator-sdk
	-rm -rf build/_output


.PHONY: operator-build operator-push generate unittest functest clean
