SHELL := /bin/bash

.DEFAULT_GOAL := help

GO ?= go
GOFMT ?= gofmt
BIN_DIR ?= bin
BIN_NAME ?= mtest
PACKAGE ?= ./cmd/mtest
VERBOSE ?= 0
INTEGRATION_PACKAGE ?= ./test/dummy
INTEGRATION_FLAGS ?= --once --verbose=$(VERBOSE)
COVERAGE_FILE ?= coverage.out

BIN_EXT :=
ifeq ($(OS),Windows_NT)
	BIN_EXT := .exe
endif
BIN := $(BIN_DIR)/$(BIN_NAME)$(BIN_EXT)

.PHONY: help
help: ## Show available targets
	@printf "Available targets:\n"
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z0-9_.-]+:.*##/ {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: build
build: ## Build the CLI binary into $(BIN_DIR)
	@mkdir -p "$(BIN_DIR)"
	$(GO) build -trimpath -o "$(BIN)" "$(PACKAGE)"

.PHONY: install
install: ## Install the CLI into GOPATH/bin
	$(GO) install "$(PACKAGE)"

.PHONY: test
test: ## Run unit tests
	$(GO) test ./...

.PHONY: npm-test
npm-test: ## Run npm wrapper tests
	cd npm && npm test

.PHONY: coverage
coverage: ## Generate coverage profile at $(COVERAGE_FILE)
	$(GO) test -coverprofile="$(COVERAGE_FILE)" ./...

.PHONY: fmt
fmt: ## Format source code in place
	$(GO) fmt ./...

.PHONY: fmt-check
fmt-check: ## Check that source code is gofmt formatted
	@FMT_OUT="$$($(GOFMT) -l .)"; \
	if [ -n "$$FMT_OUT" ]; then \
		printf 'Go files need formatting:\n%s\nRun "make fmt" to format them.\n' "$$FMT_OUT"; \
		exit 1; \
	fi

.PHONY: vet
vet: ## Run go vet static analysis
	$(GO) vet ./...

.PHONY: lint
lint: fmt-check vet ## Run formatting and vet checks

.PHONY: tidy
tidy: ## Sync go.mod and go.sum with imports
	$(GO) mod tidy

.PHONY: deps
deps: ## Download module dependencies
	$(GO) mod download

.PHONY: integration
integration: build ## Run the functional test using the dummy Meteor package
	./$(BIN) --package "$(INTEGRATION_PACKAGE)" $(INTEGRATION_FLAGS)

.PHONY: ci
ci: lint test npm-test ## Run the default CI suite

.PHONY: clean
clean: ## Remove build artifacts and cached data
	$(GO) clean -cache -testcache
	rm -rf "$(BIN_DIR)" "$(COVERAGE_FILE)"
