# Makefile for Go library project

# Variables
PKGS := $(shell go list ./... 2>/dev/null || true)
GOLANGCI_LINT_VERSION ?= v1.59.1

# Default task
.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage: make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z0-9_\-]+:.*?##/ { printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: deps
deps: ## Install/upgrade dev tools locally (golangci-lint).
	@echo "Installing dev tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: tidy
tidy: ## Ensure go.mod/go.sum are in sync.
	@go mod tidy

.PHONY: fmt
fmt: ## Format code with gofmt.
	@gofmt -s -w .

.PHONY: fmt-check
fmt-check: ## Check formatting (fails if unformatted files exist).
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "The following files are not formatted:"; \
		echo "$$unformatted"; \
		echo "Run 'make fmt' to format."; \
		exit 1; \
	fi

.PHONY: vet
vet: ## Run go vet on all packages.
	@if [ -n "$(PKGS)" ]; then \
		go vet ./...; \
	else \
		echo "No packages to vet."; \
	fi

.PHONY: lint
lint: ## Run golangci-lint on all packages.
	@if [ -n "$(PKGS)" ]; then \
		if ! command -v golangci-lint >/dev/null 2>&1; then \
			$(MAKE) deps; \
		fi; \
		golangci-lint run; \
	else \
		echo "No packages to lint."; \
	fi

.PHONY: build
build: ## Build all packages.
	@if [ -n "$(PKGS)" ]; then \
		go build ./...; \
	else \
		echo "No packages to build."; \
	fi

.PHONY: test
test: ## Run unit tests with race detector and coverage.
	@if [ -n "$(PKGS)" ]; then \
		go test ./... -race -coverprofile=coverage.out -covermode=atomic; \
	else \
		echo "No packages to test."; \
	fi

.PHONY: test-short
test-short: ## Run short unit tests.
	@if [ -n "$(PKGS)" ]; then \
		go test ./... -short; \
	else \
		echo "No packages to test."; \
	fi

.PHONY: coverage
coverage: ## Show coverage summary (requires coverage.out).
	@if [ -f coverage.out ]; then \
		go tool cover -func=coverage.out; \
	else \
		echo "coverage.out not found. Run 'make test' first."; \
	fi

.PHONY: clean
clean: ## Clean generated files.
	@rm -f coverage.out coverage.html

.PHONY: ci
ci: fmt-check vet lint test ## Run checks used in CI.
