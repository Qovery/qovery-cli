.PHONY: help build test test-verbose test-coverage lint clean install ci-local

help: ## Show this help
	@echo "Available targets:"
	@echo "  build          - Build the CLI"
	@echo "  test           - Run tests"
	@echo "  test-verbose   - Run tests with verbose output"
	@echo "  test-coverage  - Run tests with coverage"
	@echo "  lint           - Run linter"
	@echo "  clean          - Clean build artifacts"
	@echo "  install        - Install CLI locally"
	@echo "  ci-local       - Run CI checks locally"

build: ## Build the CLI
	@echo "Building qovery CLI..."
	go build -ldflags "-X github.com/qovery/qovery-cli/utils.Version=$$(git describe --tags --always)" -o qovery .

test: ## Run tests
	@echo "Running tests..."
	go test -tags=testing ./...

test-verbose: ## Run tests with verbose output
	@echo "Running tests (verbose)..."
	go test -v -tags=testing ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	go test -tags=testing -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

lint: ## Run linter
	@echo "Running linter..."
	golangci-lint run ./...

clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	rm -rf dist/ coverage.out coverage.html qovery

install: build ## Install CLI locally
	@echo "Installing qovery CLI..."
	@if [ -z "$(GOPATH)" ]; then \
		echo "Error: GOPATH is not set"; \
		exit 1; \
	fi
	cp qovery $(GOPATH)/bin/
	@echo "Installed to $(GOPATH)/bin/qovery"

ci-local: lint test build ## Run CI checks locally
	@echo "✅ All CI checks passed!"

.DEFAULT_GOAL := help
