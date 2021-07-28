include variables.mk

# Image URL to use all building/pushing image targets
IMG ?= ${REGISTRY}/csi-baremetal-operator:${TAG}

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Print version
version:
	@printf $(TAG)

all: manager

### Unit tests

coverage:
	go tool cover -html=coverage.out -o coverage.html

test:
	${GO_ENV_VARS} go test `go list ./... | grep pkg` -race -cover -coverprofile=coverage.out -covermode=atomic

# Build manager binary
manager: fmt vet
	go build -o bin/manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: fmt vet resources
	go run ./main.go

# Install CRDs into a cluster
install:
	kustomize build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests
	kustomize build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy:
	cd config/manager && kustomize edit set image controller=${IMG}
	kustomize build config/default | kubectl apply -f -

# Deploy CSI resources from ~/deploy
resources:
	kubectl apply -f config/crd/bases
	kubectl apply -f deploy/rbac
	kubectl apply -f deploy/storageclass
	kubectl apply -f deploy/configmap
	kubectl apply -f deploy/csidriver

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate:
	controller-gen object paths=api/v1/deployment_types.go paths=api/v1/groupversion_info.go output:dir=api/v1
	controller-gen crd:trivialVersions=true paths=api/v1/deployment_types.go paths=api/v1/groupversion_info.go output:crd:dir=config/crd

# Build the docker image
docker-build:
	docker build . -t ${IMG}

# Build the docker image
kind-load:
	kind load docker-image ${IMG}

# Push the docker image
docker-push:
	docker push ${IMG}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.5 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

lint:
	${GO_ENV_VARS} golangci-lint -v run ./...
