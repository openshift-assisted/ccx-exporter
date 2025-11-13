.DEFAULT_GOAL := help

SHELL := /bin/bash

##################
# Common variables
###################

# Colors used in this Makefile
escape       := $(shell printf '\033')
RESET_FORMAT := $(escape)[0m
COLOR_RED    := $(escape)[91m
COLOR_YELLOW := $(escape)[38;5;220m
COLOR_GREEN  := $(escape)[0;32m
COLOR_BLUE   := $(escape)[94m
BOLD         := $(escape)[1m

# build and deploy variables
GIT_COMMIT := $(shell git rev-list -1 HEAD --abbrev-commit)

IMAGE_NAME := ccx-exporter
IMAGE_TAG  := $(GIT_COMMIT)
IMAGE_FULL ?= $(IMAGE_NAME):$(IMAGE_TAG)

BUILD_DIR := $(CURDIR)/build


#####################
## High level targets
######################

.PHONY: help format build generate check test logs local

help:     help.all
format:   format.imports format.code
build:    build.docker
generate: generate.mocks
check:    check.licenses check.imports check.fmt check.lint check.mocks
test:     test.coverage
logs:     logs.kube
local:    local.kind local.kubeconfig local.namespace local.kraft local.valkey local.localstack local.processing.secret


#########
## Helper
##########

.PHONY: help.all

## help.all: Display this help message
help.all:
	@echo "List of make commands:"
	@grep -hE '^[a-z]+:|^## ' $(MAKEFILE_LIST) | sed 's/## //p' | uniq | \
	awk 'BEGIN {FS = ":";} { \
	if ($$0 ~ /:/) printf("  $(COLOR_BLUE)%-23s$(RESET_FORMAT) %s\n", $$1, $$2); \
	else  printf("\n$(BOLD)%s$(RESET_FORMAT)\n", $$1);    \
	}'


#################
## Format targets
##################

GO_MODULE  := $(shell head -n 1 go.mod | cut -d ' ' -f 2)
FILES_LIST := cmd/ internal/ pkg/ test/

.PHONY: format.imports format.code

## format.imports: Format go imports
format.imports:
	@goimports -w -local $(GO_MODULE) $(FILES_LIST)

## format.code: Format go code
format.code:
	@gofumpt -w $(FILES_LIST)


########
## Build
#########

BUILD_ENV := CGO_ENABLED=0

.PHONY: build.prepare build.local build.docker

## build.prepare: Create build/ folder
build.prepare:
	@mkdir -p $(BUILD_DIR)

## build.local: Build binary app
build.local: build.prepare
	$(BUILD_ENV) go build \
		-mod readonly     \
		-tags=viper_bind_struct \
		-ldflags "-s -w -extldflags -static \
		  -X github.com/openshift-assisted/ccx-exporter/internal/version.Revision=$(GIT_COMMIT)                    \
		  -X github.com/openshift-assisted/ccx-exporter/internal/version.Branch=$(shell git branch --show-current) \
		  -X 'github.com/prometheus/common/version.BuildUser=$(shell whoami)' \
		  -X 'github.com/prometheus/common/version.BuildDate=$(shell date)'   \
		" \
		$(BUILD_ARGS) \
		-o $(BUILD_DIR)/ccx-exporter \
		$(CURDIR)/cmd/ccx-exporter/main.go

## build.docker: Build image and tag
build.docker:
	docker build --build-arg version=$(GIT_COMMIT) --build-arg BUILD_ARGS="$(BUILD_ARGS)" -t $(IMAGE_FULL) .


###########
## Generate
############

.PHONY: generate.mocks

## generate.mocks: Generate mocks with mockgen
generate.mocks:
	@find $(FILES_LIST) -type d -name mock -exec rm -rv {} +
	@go generate ./pkg/... ./internal/...


################
## Check targets
#################

.PHONY: check.licenses check.imports check.fmt check.lint check.mocks

## check.licenses: Check thirdparties' licences (allow-list in .wwhrd.yml)
check.licenses:
	@wwhrd check -q

## check.imports: Check if imports are well formated: builtin -> external -> rome -> repo
check.imports:
	@goimports -l -local $(GO_MODULE) $(FILES_LIST) | wc -l | grep -q 0

## check.fmt: Check if code is formated according gofumpt rules
check.fmt:
	@gofumpt -l $(FILES_LIST) | wc -l | grep -q 0

## check.lint: Run Go linter across the code base without fixing issues
check.lint:
	@golangci-lint run --timeout 10m

## check.mocks: Ensure mocks have been regenerated
check.mocks:
	@git diff --name-only | grep mock | wc -l | grep -q 0


#######
## Test
########

COVERAGE_DIR=$(BUILD_DIR)/coverdata

.PHONY: test.prepare test.e2e.skip-build test.e2e test.unit test.coverage

## test.prepare: Create build/coverdata/ folder
test.prepare:
	@mkdir -p $(COVERAGE_DIR)
	@rm -fr $(COVERAGE_DIR)/*
	@chmod 777 $(COVERAGE_DIR)

## test.e2e.skip-build: Run e2e test without rebuilding and reimporting the processing image
test.e2e.skip-build:
	@ginkgo -p test ./test/e2e/... -v -kubeconfig $(LOCAL_KUBE_CONFIG)

## test.e2e: Run e2e test
test.e2e: build.docker local.import test.e2e.skip-build

## test.unit: Run unit tests
test.unit:
	@go test ./pkg/... ./cmd/... ./internal/...

## test.coverage: Run all tests and compute code coverage
test.coverage: test.prepare
	@$(MAKE) -s build.docker local.import BUILD_ARGS="-cover"
	@go test -timeout 30m ./... -cover -test.gocoverdir $(CURDIR)/build/coverdata
	@go tool covdata textfmt -i=$(COVERAGE_DIR) -o=$(COVERAGE_DIR)/coverage.out.tmp
	@cat $(COVERAGE_DIR)/coverage.out.tmp | grep -vE "mock_|$(GO_MODULE)/test/|/build/cmd/ccx-exporter/main.go" > $(COVERAGE_DIR)/coverage.out
	@go tool cover -func $(COVERAGE_DIR)/coverage.out


#######
## Logs
########

.PHONY: logs.kube

DEPLOYMENT=ccx-exporter

ANYTHING_BETWEEN_QUOTES=\"\([^\"]*\)\"

define COLORIZE
sed -u -e "\
s/caller=$(ANYTHING_BETWEEN_QUOTES)/caller=\"$(COLOR_BLUE)\1$(RESET_FORMAT)\"/g; \
s/error=$(ANYTHING_BETWEEN_QUOTES)/error=\"$(COLOR_RED)\1$(RESET_FORMAT)\"/g;    \
s/msg=$(ANYTHING_BETWEEN_QUOTES)/msg=\"$(COLOR_YELLOW)\1$(RESET_FORMAT)\"/g;     \
s/level=error/level=$(COLOR_RED)error$(RESET_FORMAT)/g;                          \
s/level=info/level=$(COLOR_GREEN)info$(RESET_FORMAT)/g;                          \
s/level=debug/level=$(COLOR_GREEN)info$(RESET_FORMAT)/g;                         \
s/level=trace/level=$(COLOR_GREEN)info$(RESET_FORMAT)/g;                         \
s/level=unknown/level=$(COLOR_GREEN)info$(RESET_FORMAT)/g                        \
"
endef

## logs.kube: Display processing log with color and unify level
logs.kube:
	@kubectl logs -f deployment/$(DEPLOYMENT) | $(COLORIZE)


########
## Local
#########

-include $(CURDIR)/local/local.mk
