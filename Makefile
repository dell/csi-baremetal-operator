include variables.mk

# Image URL to use all building/pushing image targets
IMG ?= ${REGISTRY}/csi-baremetal-operator:${TAG}

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"
CSI_BAREMETAL_DRIVER_DIR=../csi-baremetal
CSI_CHART_CRDS_PATH=charts/csi-baremetal-operator/crds
CONTROLLER_GEN_BIN=./bin/controller-gen

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
	$(CONTROLLER_GEN_BIN) $(CRD_OPTIONS) paths=$(CSI_BAREMETAL_DRIVER_DIR)/api/v1/availablecapacitycrd/availablecapacity_types.go paths=$(CSI_BAREMETAL_DRIVER_DIR)/api/v1/availablecapacitycrd/groupversion_info.go output:crd:dir=$(CSI_CHART_CRDS_PATH)
	$(CONTROLLER_GEN_BIN) $(CRD_OPTIONS) paths=$(CSI_BAREMETAL_DRIVER_DIR)/api/v1/acreservationcrd/availablecapacityreservation_types.go paths=$(CSI_BAREMETAL_DRIVER_DIR)/api/v1/acreservationcrd/groupversion_info.go output:crd:dir=$(CSI_CHART_CRDS_PATH)
	$(CONTROLLER_GEN_BIN) $(CRD_OPTIONS) paths=$(CSI_BAREMETAL_DRIVER_DIR)/api/v1/volumecrd/volume_types.go paths=$(CSI_BAREMETAL_DRIVER_DIR)/api/v1/volumecrd/groupversion_info.go output:crd:dir=$(CSI_CHART_CRDS_PATH)
	$(CONTROLLER_GEN_BIN) $(CRD_OPTIONS) paths=$(CSI_BAREMETAL_DRIVER_DIR)/api/v1/drivecrd/drive_types.go paths=$(CSI_BAREMETAL_DRIVER_DIR)/api/v1/drivecrd/groupversion_info.go output:crd:dir=$(CSI_CHART_CRDS_PATH)
	$(CONTROLLER_GEN_BIN) $(CRD_OPTIONS) paths=$(CSI_BAREMETAL_DRIVER_DIR)/api/v1/lvgcrd/logicalvolumegroup_types.go paths=$(CSI_BAREMETAL_DRIVER_DIR)/api/v1/lvgcrd/groupversion_info.go output:crd:dir=$(CSI_CHART_CRDS_PATH)
	$(CONTROLLER_GEN_BIN) $(CRD_OPTIONS) paths=$(CSI_BAREMETAL_DRIVER_DIR)/api/v1/nodecrd/node_types.go paths=$(CSI_BAREMETAL_DRIVER_DIR)/api/v1/nodecrd/groupversion_info.go output:crd:dir=$(CSI_CHART_CRDS_PATH)
	$(CONTROLLER_GEN_BIN) $(CRD_OPTIONS) paths=api/v1/deployment_types.go paths=api/v1/groupversion_info.go output:crd:dir=$(CSI_CHART_CRDS_PATH)

# Build the docker image
docker-build:
	docker build . -t ${IMG}

# Build the docker image
kind-load:
	kind load docker-image ${IMG}

# Push the docker image
docker-push:
	docker push ${IMG}

# build controller-gen executable
controller-gen:
	${GO_ENV_VARS} go build -mod='' -o ./bin/ sigs.k8s.io/controller-tools/cmd/controller-gen

lint:
	${GO_ENV_VARS} golangci-lint -v run ./...
