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

BUILD_DIR=$(CURDIR)/build


#####################
## High level targets
######################

.PHONY: help build local

help: help.all
build: 
local: local.kind local.namespace local.kraft local.valkey local.localstack


#########
## Helper
##########

.PHONY: help.all

## help.all: Display this help message
help.all:
	@echo "List of make commands:"
	@grep -hE '^[a-z]+:|^## ' $(MAKEFILE_LIST) | sed 's/## //p' | uniq | \
	awk 'BEGIN {FS = ":";} { \
	if ($$0 ~ /:/) printf("  $(COLOR_BLUE)%-21s$(RESET_FORMAT) %s\n", $$1, $$2); \
	else  printf("\n$(BOLD)%s$(RESET_FORMAT)\n", $$1);    \
	}'


########
## Build
#########

.PHONY: build.vendor build.vendor.tidy build.prepare

## build.vendor: Get dependencies locally
build.vendor:
	@go mod vendor

## build.vendor.tidy: Remove unused project's dependencies
build.vendor.tidy:
	@go mod tidy

## build.prepare: Create build/ folder
build.prepare:
	@mkdir -p $(BUILD_DIR)


########
## Local
#########

-include $(CURDIR)/local/local.mk
