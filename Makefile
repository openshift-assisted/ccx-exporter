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

.PHONY: help format build test local

help: help.all
format: format.imports format.code
build: build.docker
test: test.e2e
local: local.kind local.namespace local.kraft local.valkey local.localstack local.processing.secret


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
FILES_LIST := cmd/ internal/ test/

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
		  -X github.com/prometheus/common/version.Revision=$(GIT_COMMIT)                    \
		  -X github.com/prometheus/common/version.Branch=$(shell git branch --show-current) \
		  -X 'github.com/prometheus/common/version.BuildUser=$(shell whoami)'               \
		  -X 'github.com/prometheus/common/version.BuildDate=$(shell date)'                 \
		" \
		-o $(BUILD_DIR)/ccx-exporter \
		$(CURDIR)/cmd/ccx-exporter/main.go

## build.docker: Build image and tag
build.docker:
	docker build --build-arg version=$(GIT_COMMIT) -t $(IMAGE_FULL) .


#######
## Test
########

.PHONY: test.e2e

## test.e2e: Run e2e test
test.e2e: build.docker local.import
	@go test ./test/e2e/...


########
## Local
#########

-include $(CURDIR)/local/local.mk
