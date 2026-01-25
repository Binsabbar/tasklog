.PHONY: help build test lint fmt clean docker-build up-deps-services up-app up-all down-app down-volume down logs setup release release-snapshot go-build go-build-release go-test go-test-coverage go-lint go-vulncheck go-fmt go-fmt-check docker-up-deps-services docker-up-app docker-up-all docker-status docker-down-app docker-down docker-down-volume docker-deps-logs docker-app-logs

# Variables
BINARY_NAME=tasklog
DOCKER_IMAGE=tasklog
DOCKER_TAG=latest
MAIN_PATH=./main.go

# Docker variables
DOCKER_REGISTRY ?= ghcr.io
DOCKER_IMAGE_NAME ?= binsabbar/tasklog
VERSION ?= latest

# Build variables (for go-build-release)
BUILD_VERSION ?= dev
BUILD_COMMIT ?= $(shell git rev-parse --short HEAD)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
BUILD_BY ?= manual

export GITHUB_REPOSITORY_OWNER ?= binsabbar

# Colors for output
RED=\033[0;31m
GREEN=\033[0;32m
YELLOW=\033[1;33m
BLUE=\033[0;34m
NC=\033[0m # No Color


help:
	@echo "$(BLUE)Tasklog - Makefile Commands$(NC)"
	@echo ""
	@echo "$(YELLOW)Development:$(NC)"
	@echo "  make go-build              - Build the binary to bin/$(BINARY_NAME)"
	@echo "  make go-build-release      - Build with version info (mimics GoReleaser)"
	@echo "  make go-test               - Run all tests (silent mode)"
	@echo "  make go-test-verbose       - Run all tests with verbose output"
	@echo "  make go-test-coverage      - Run all tests with race detector and coverage"
	@echo "  make go-lint               - Run golangci-lint checks"
	@echo "  make go-vulncheck          - Run govulncheck for security vulnerabilities"
	@echo "  make go-fmt                - Format code with gofmt"
	@echo "  make go-fmt-check          - Check if code is properly formatted"
	@echo ""
	@echo "$(YELLOW)Docker:$(NC)"
	@echo "  make docker-build              - Build Docker image with tag $(VERSION)"
	@echo "  make docker-push               - Push Docker image to $(DOCKER_REGISTRY)"
	@echo "  make docker-build-and-push     - Build and push Docker image"
	@echo ""
	@echo "$(YELLOW)Maintenance:$(NC)"
	@echo "  make clean                     - Clean build artifacts (bin/, dist/, cache)"
	@echo ""
	@echo "$(YELLOW)Release:$(NC)"
	@echo "  make release                   - Create a tagged release with GoReleaser (requires git tag)"
	@echo "  make release-snapshot          - Build snapshot release locally (no tag required)"
	@echo ""
	@echo "$(YELLOW)Examples:$(NC)"
	@echo "  make docker-build VERSION=v1.0.0"
	@echo "  make go-build-release BUILD_VERSION=1.1.0-alpha.1"
	@echo "  make release-snapshot"
	@echo "  git tag v0.1.0 && make release"
	@echo ""
	@echo "$(YELLOW)Variables:$(NC)"
	@echo "  VERSION                - Version tag for Docker image (default: latest)"
	@echo "  BUILD_VERSION          - Version for go-build-release (default: dev)"
	@echo "  BUILD_COMMIT           - Git commit for build (default: current short SHA)"
	@echo "  BUILD_DATE             - Build date (default: current UTC time)"
	@echo "  BUILD_BY               - Builder name (default: manual)"
	@echo "  DOCKER_REGISTRY        - Docker registry URL (default: ghcr.io)"
	@echo "  DOCKER_IMAGE_NAME      - Docker image name (default: binsabbar/tasklog)"
	@echo "  GITHUB_REPOSITORY_OWNER - GitHub owner for release (default: binsabbar)"

## build: Build the binary
go-build:
	@echo "$(BLUE)Building $(BINARY_NAME)...$(NC)"
	@go build -o bin/$(BINARY_NAME) $(MAIN_PATH)
	@echo "$(GREEN)✓ Build complete: bin/$(BINARY_NAME)$(NC)"

## build-release: Build with version info (mimics GoReleaser)
go-build-release:
	@echo "$(BLUE)Building $(BINARY_NAME) with version info...$(NC)"
	@mkdir -p dist
	@go build \
		-ldflags="-X main.version=$(BUILD_VERSION) -X main.commit=$(BUILD_COMMIT) -X main.date=$(BUILD_DATE) -X main.builtBy=$(BUILD_BY)" \
		-o dist/$(BINARY_NAME) $(MAIN_PATH)
	@echo "$(GREEN)✓ Build complete: dist/$(BINARY_NAME)$(NC)"
	@echo "$(YELLOW)Version: $(BUILD_VERSION)$(NC)"
	@echo "$(YELLOW)Commit:  $(BUILD_COMMIT)$(NC)"
	@echo "$(YELLOW)Date:    $(BUILD_DATE)$(NC)"
	@echo "$(YELLOW)Built by: $(BUILD_BY)$(NC)"

## test: Run all tests
## test: Run tests silently  
go-test:
	@echo "$(BLUE)Clearing test cache...$(NC)"
	@go clean -testcache
	@echo "$(BLUE)Running all tests...$(NC)"
	@TEST_SILENT=1 go test ./...
	@echo "$(GREEN)✓ Tests completed$(NC)"

## test-verbose: Run tests with logs
go-test-verbose:
	@echo "$(BLUE)Clearing test cache...$(NC)"
	@go clean -testcache
	@echo "$(BLUE)Running all tests (verbose)...$(NC)"
	@go test -v ./...
	@echo "$(GREEN)✓ Tests completed$(NC)"

go-test-coverage:
	@echo "$(BLUE)Clearing test cache...$(NC)"
	@go clean -testcache
	@echo "$(BLUE)Running all tests with coverage...$(NC)"
	@TEST_SILENT=1 go test -race -timeout 5m -cover ./...
	@echo "$(GREEN)✓ Tests completed$(NC)"


## lint: Run golangci-lint
go-lint:
	@echo "$(BLUE)Running golangci-lint (production code only)...$(NC)"
	@golangci-lint run --config .golangci.yml --timeout 5m
	@echo "$(GREEN)✓ Linting complete$(NC)"

## vulncheck: Run govulncheck for security vulnerabilities
go-vulncheck:
	@echo "$(BLUE)Running govulncheck for security vulnerabilities...$(NC)"
	@which govulncheck > /dev/null || (echo "$(RED)Error: govulncheck not installed. Run: go install golang.org/x/vuln/cmd/govulncheck@latest$(NC)" && exit 1)
	@govulncheck ./...
	@echo "$(GREEN)✓ Vulnerability check complete$(NC)"

## fmt: Format code with gofmt
go-fmt:
	@echo "$(BLUE)Formatting code...$(NC)"
	@gofmt -s -w .
	@echo "$(GREEN)✓ Code formatted$(NC)"

## fmt-check: Check if code is properly formatted
go-fmt-check:
	@echo "$(BLUE)Checking code formatting...$(NC)"
	@OUTPUT=$$(gofmt -l .); \
	if [ -n "$$OUTPUT" ]; then \
		echo "$(RED)Go files are not formatted. Please run 'make go-fmt':$(NC)"; \
		echo "$$OUTPUT"; \
		exit 1; \
	fi
	@echo "$(GREEN)✓ Code is properly formatted$(NC)"

## docker-build: Build Docker image
docker-build:
	@echo "$(BLUE)Building Docker image...$(NC)"
	docker build -t $(DOCKER_REGISTRY)/$(DOCKER_IMAGE_NAME):$(VERSION) .
	docker tag $(DOCKER_REGISTRY)/$(DOCKER_IMAGE_NAME):$(VERSION) $(DOCKER_REGISTRY)/$(DOCKER_IMAGE_NAME):latest
	@echo "$(GREEN)✓ Docker image built$(NC)"

## docker-push: Push Docker image to registry
docker-push:
	@echo "$(BLUE)Pushing Docker image...$(NC)"
	docker push $(DOCKER_REGISTRY)/$(DOCKER_IMAGE_NAME):$(VERSION)
	docker push $(DOCKER_REGISTRY)/$(DOCKER_IMAGE_NAME):latest
	@echo "$(GREEN)✓ Docker image pushed$(NC)"

## docker-build-and-push: Build and push Docker image
docker-build-and-push: docker-build docker-push

## release: Create a new release with GoReleaser
release:
	@echo "$(BLUE)Creating release with GoReleaser...$(NC)"
	@which goreleaser > /dev/null || (echo "$(RED)Error: goreleaser not installed. Run: brew install goreleaser$(NC)" && exit 1)
	@goreleaser release --clean
	@echo "$(GREEN)✓ Release complete$(NC)"

## release-snapshot: Build snapshot release (no publish)
release-snapshot:
	@echo "$(BLUE)Building snapshot release...$(NC)"
	@which goreleaser > /dev/null || (echo "$(RED)Error: goreleaser not installed. Run: brew install goreleaser$(NC)" && exit 1)
	@goreleaser release --snapshot --clean
	@echo "$(GREEN)✓ Snapshot built in dist/$(NC)"

## clean: Clean build artifacts
clean:
	@echo "$(BLUE)Cleaning build artifacts...$(NC)"
	@rm -rf bin/
	@rm -rf dist/
	@go clean -cache
	@echo "$(GREEN)✓ Clean complete$(NC)"

# Default target
.DEFAULT_GOAL := help