# project name
PROJECT          := csi-baremetal-operator

# controller-gen related vars
CSI_OPERATOR_CHART_PATH=charts/csi-baremetal-operator
CSI_DEPLOYMENT_CHART_PATH=charts/csi-baremetal-deployment
CSI_CHART_CRDS_PATH=charts/csi-baremetal-operator/crds
CONTROLLER_GEN_BIN=./bin/controller-gen
CRD_OPTIONS ?= "crd:trivialVersions=true"

### version
MAJOR            := 1
MINOR            := 1
PATCH            := 0
PRODUCT_VERSION  ?= ${MAJOR}.${MINOR}.${PATCH}
BUILD_REL_A      := $(shell git rev-list HEAD |wc -l)
BUILD_REL_B      := $(shell git rev-parse --short HEAD)
BLD_CNT          := $(shell echo ${BUILD_REL_A})
BLD_SHA          := $(shell echo ${BUILD_REL_B})
RELEASE_STR      := ${BLD_CNT}.${BLD_SHA}
FULL_VERSION     := ${PRODUCT_VERSION}-${RELEASE_STR}
TAG              := ${FULL_VERSION}
BRANCH           := $(shell git rev-parse --abbrev-ref HEAD)

### go env vars
GO_ENV_VARS     := GO111MODULE=on ${GOPRIVATE_PART} ${GOPROXY_PART}

### custom variables that could be ommited
GOPRIVATE_PART  :=
GOPROXY_PART    := GOPROXY=https://proxy.golang.org,direct

# override some of variables, optional file
-include variables.override.mk
