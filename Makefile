.PHONY: help build clean test test-verbose test-coverage test-watch fmt lint vet install run deps tidy build-linux build-darwin build-windows

# Variables
GO := go
BINARY_NAME := ezcode
MAIN_PACKAGE := ./cmd/ezcode
OUTPUT_DIR := bin
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Default target
.DEFAULT_GOAL := help

# Colors for output
CYAN := \033[36m
GREEN := \033[32m
YELLOW := \033[33m
NC := \033[0m # No Color

help: ## Display this help message
	@echo "$(CYAN)Ezcode Makefile$(NC)"
	@echo "$(CYAN)=====================$(NC)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "$(GREEN)%-20s$(NC) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(YELLOW)Examples:$(NC)"
	@echo "  make build              # Build the binary"
	@echo "  make test               # Run all tests"
	@echo "  make test-coverage      # Run tests with coverage report"
	@echo "  make run                # Run the application"
	@echo "  make clean              # Remove build artifacts"
	@echo ""

# ============================================================================
# BUILD TARGETS
# ============================================================================

build: fmt vet ## Build the binary for current OS
	@echo "$(GREEN)[BUILD]$(NC) Building $(BINARY_NAME)..."
	@mkdir -p $(OUTPUT_DIR)
	@$(GO) build -o $(OUTPUT_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)
	@echo "$(GREEN)[BUILD]$(NC) Binary created: $(OUTPUT_DIR)/$(BINARY_NAME)"

build-with-ldflags: fmt vet ## Build with version info embedded
	@echo "$(GREEN)[BUILD]$(NC) Building $(BINARY_NAME) with version info..."
	@mkdir -p $(OUTPUT_DIR)
	@$(GO) build \
		-ldflags="-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)" \
		-o $(OUTPUT_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)
	@echo "$(GREEN)[BUILD]$(NC) Binary created: $(OUTPUT_DIR)/$(BINARY_NAME)"

build-linux: fmt vet ## Build for Linux (amd64)
	@echo "$(GREEN)[BUILD]$(NC) Building for Linux (amd64)..."
	@mkdir -p $(OUTPUT_DIR)
	@GOOS=linux GOARCH=amd64 $(GO) build -o $(OUTPUT_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PACKAGE)
	@echo "$(GREEN)[BUILD]$(NC) Binary created: $(OUTPUT_DIR)/$(BINARY_NAME)-linux-amd64"

build-linux-arm64: fmt vet ## Build for Linux (arm64)
	@echo "$(GREEN)[BUILD]$(NC) Building for Linux (arm64)..."
	@mkdir -p $(OUTPUT_DIR)
	@GOOS=linux GOARCH=arm64 $(GO) build -o $(OUTPUT_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PACKAGE)
	@echo "$(GREEN)[BUILD]$(NC) Binary created: $(OUTPUT_DIR)/$(BINARY_NAME)-linux-arm64"

build-darwin: fmt vet ## Build for macOS (amd64)
	@echo "$(GREEN)[BUILD]$(NC) Building for macOS (amd64)..."
	@mkdir -p $(OUTPUT_DIR)
	@GOOS=darwin GOARCH=amd64 $(GO) build -o $(OUTPUT_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PACKAGE)
	@echo "$(GREEN)[BUILD]$(NC) Binary created: $(OUTPUT_DIR)/$(BINARY_NAME)-darwin-amd64"

build-darwin-arm64: fmt vet ## Build for macOS (arm64)
	@echo "$(GREEN)[BUILD]$(NC) Building for macOS (arm64)..."
	@mkdir -p $(OUTPUT_DIR)
	@GOOS=darwin GOARCH=arm64 $(GO) build -o $(OUTPUT_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PACKAGE)
	@echo "$(GREEN)[BUILD]$(NC) Binary created: $(OUTPUT_DIR)/$(BINARY_NAME)-darwin-arm64"

build-windows: fmt vet ## Build for Windows (amd64)
	@echo "$(GREEN)[BUILD]$(NC) Building for Windows (amd64)..."
	@mkdir -p $(OUTPUT_DIR)
	@GOOS=windows GOARCH=amd64 $(GO) build -o $(OUTPUT_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PACKAGE)
	@echo "$(GREEN)[BUILD]$(NC) Binary created: $(OUTPUT_DIR)/$(BINARY_NAME)-windows-amd64.exe"

build-all: build-linux build-linux-arm64 build-darwin build-darwin-arm64 build-windows ## Build for all platforms

# ============================================================================
# TEST TARGETS
# ============================================================================

test: ## Run all tests
	@echo "$(GREEN)[TEST]$(NC) Running tests..."
	@$(GO) test ./...
	@echo "$(GREEN)[TEST]$(NC) All tests passed!"

test-verbose: ## Run all tests with verbose output
	@echo "$(GREEN)[TEST]$(NC) Running tests (verbose)..."
	@$(GO) test -v ./...

test-watch: ## Run tests in watch mode (requires entr or similar)
	@echo "$(GREEN)[TEST]$(NC) Running tests in watch mode..."
	@find . -name "*.go" -type f | entr -c make test

test-coverage: ## Run tests with coverage report
	@echo "$(GREEN)[TEST]$(NC) Running tests with coverage..."
	@$(GO) test -coverprofile=coverage.out ./...
	@$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)[TEST]$(NC) Coverage report: coverage.html"

test-coverage-report: ## Display coverage in terminal
	@echo "$(GREEN)[TEST]$(NC) Generating coverage report..."
	@$(GO) test -cover ./...

test-unit: ## Run unit tests only
	@echo "$(GREEN)[TEST]$(NC) Running unit tests..."
	@$(GO) test -short ./...

test-integration: ## Run integration tests
	@echo "$(GREEN)[TEST]$(NC) Running integration tests..."
	@$(GO) test -tags=integration ./...

# Run specific package tests
test-config: ## Run tests for config package
	@echo "$(GREEN)[TEST]$(NC) Running config package tests..."
	@$(GO) test -v ./internal/config

test-docker: ## Run tests for docker package
	@echo "$(GREEN)[TEST]$(NC) Running docker package tests..."
	@$(GO) test -v ./internal/docker

test-components: ## Run tests for ui/components package
	@echo "$(GREEN)[TEST]$(NC) Running ui/components tests..."
	@$(GO) test -v ./ui/components

test-mcp: ## Run tests for mcp package
	@echo "$(GREEN)[TEST]$(NC) Running mcp package tests..."
	@$(GO) test -v ./internal/mcp

# Run specific test by name (e.g., make test-run test=TestAppInitializationFirstRun)
test-run: ## Run specific test - usage: make test-run test=TestName
	@if [ -z "$(test)" ]; then \
		echo "$(YELLOW)[ERROR]$(NC) Please specify test name: make test-run test=TestName"; \
		exit 1; \
	fi
	@echo "$(GREEN)[TEST]$(NC) Running $(test)..."
	@$(GO) test -v -run $(test) ./...

# ============================================================================
# CODE QUALITY TARGETS
# ============================================================================

fmt: ## Format all Go files
	@echo "$(GREEN)[FMT]$(NC) Formatting Go files..."
	@$(GO) fmt ./...
	@echo "$(GREEN)[FMT]$(NC) Formatting complete!"

fmt-check: ## Check if Go files are properly formatted
	@echo "$(GREEN)[FMT]$(NC) Checking Go formatting..."
	@if [ -n "$$(gofmt -l ./...)" ]; then \
		echo "$(YELLOW)[WARNING]$(NC) Files needing formatting:"; \
		gofmt -l ./...; \
		exit 1; \
	fi
	@echo "$(GREEN)[FMT]$(NC) All files properly formatted!"

vet: ## Run go vet to check for potential issues
	@echo "$(GREEN)[VET]$(NC) Running go vet..."
	@$(GO) vet ./...
	@echo "$(GREEN)[VET]$(NC) No issues found!"

lint: fmt vet ## Run formatters and vet (shorthand)
	@echo "$(GREEN)[LINT]$(NC) Linting complete!"

# ============================================================================
# DEPENDENCY MANAGEMENT
# ============================================================================

deps: ## Download and verify dependencies
	@echo "$(GREEN)[DEPS]$(NC) Downloading dependencies..."
	@$(GO) mod download
	@echo "$(GREEN)[DEPS]$(NC) Dependencies downloaded!"

tidy: ## Clean up go.mod and go.sum
	@echo "$(GREEN)[TIDY]$(NC) Tidying go.mod and go.sum..."
	@$(GO) mod tidy
	@echo "$(GREEN)[TIDY]$(NC) Done!"

vendor: ## Vendor dependencies
	@echo "$(GREEN)[VENDOR]$(NC) Vendoring dependencies..."
	@$(GO) mod vendor
	@echo "$(GREEN)[VENDOR]$(NC) Dependencies vendored!"

update-deps: ## Update all dependencies
	@echo "$(GREEN)[UPDATE]$(NC) Updating dependencies..."
	@$(GO) get -u ./...
	@$(GO) mod tidy
	@echo "$(GREEN)[UPDATE]$(NC) Dependencies updated!"

# ============================================================================
# RUNTIME TARGETS
# ============================================================================

run: build ## Build and run the application
	@echo "$(GREEN)[RUN]$(NC) Starting $(BINARY_NAME)..."
	@./$(OUTPUT_DIR)/$(BINARY_NAME)

install: build ## Build and install the binary to $GOPATH/bin
	@echo "$(GREEN)[INSTALL]$(NC) Installing $(BINARY_NAME)..."
	@$(GO) install $(MAIN_PACKAGE)
	@echo "$(GREEN)[INSTALL]$(NC) Installed to $GOPATH/bin/$(BINARY_NAME)"

# ============================================================================
# CLEANUP TARGETS
# ============================================================================

clean: ## Remove build artifacts and temporary files
	@echo "$(GREEN)[CLEAN]$(NC) Removing build artifacts..."
	@rm -rf $(OUTPUT_DIR)
	@rm -f coverage.out coverage.html
	@echo "$(GREEN)[CLEAN]$(NC) Cleanup complete!"

clean-cache: ## Clear Go build cache
	@echo "$(GREEN)[CLEAN]$(NC) Clearing Go build cache..."
	@$(GO) clean -cache
	@echo "$(GREEN)[CLEAN]$(NC) Cache cleared!"

distclean: clean clean-cache ## Remove all generated files and caches

# ============================================================================
# DEVELOPMENT TARGETS
# ============================================================================

dev: fmt vet test build ## Run fmt, vet, test, and build (development workflow)
	@echo "$(GREEN)[DEV]$(NC) Development workflow complete!"

check: fmt-check vet test ## Run format check, vet, and tests
	@echo "$(GREEN)[CHECK]$(NC) All checks passed!"

# ============================================================================
# UTILITY TARGETS
# ============================================================================

version: ## Display version information
	@echo "$(CYAN)Ezcode Version Info$(NC)"
	@echo "Version:   $(VERSION)"
	@echo "Commit:    $(GIT_COMMIT)"
	@echo "Build:     $(BUILD_TIME)"
	@echo "Go:        $$($(GO) version)"

info: ## Display project information
	@echo "$(CYAN)Ezcode Project Info$(NC)"
	@echo "Module:    $$(grep '^module' go.mod | awk '{print $$2}')"
	@echo "Go:        $$(grep '^go ' go.mod | awk '{print $$2}')"
	@echo "Binary:    $(BINARY_NAME)"
	@echo "Main:      $(MAIN_PACKAGE)"
	@echo "Output:    $(OUTPUT_DIR)"

list-packages: ## List all packages in the project
	@echo "$(CYAN)Project Packages$(NC)"
	@$(GO) list ./...

list-tests: ## List all test functions
	@echo "$(CYAN)Test Functions$(NC)"
	@grep -r "^func Test" ./... --include="*.go" | sed 's/:func /: /' | sed 's/(.*)//'
