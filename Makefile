.PHONY: test build lint fmt vet clean install help

# Variables
GOBASE=$(shell pwd)
GOBIN=$(GOBASE)/bin
GOFILES=$(wildcard *.go)

# Colors for output
COLOR_RESET=\033[0m
COLOR_BOLD=\033[1m
COLOR_GREEN=\033[32m
COLOR_YELLOW=\033[33m

help: ## Display this help screen
	@echo "$(COLOR_BOLD)purefunccore - Functional bindings for Go$(COLOR_RESET)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(COLOR_GREEN)%-15s$(COLOR_RESET) %s\n", $$1, $$2}'

test: ## Run all tests
	@echo "$(COLOR_BOLD)Running tests...$(COLOR_RESET)"
	go test -v -race -coverprofile=coverage.out ./...
	@echo "$(COLOR_GREEN)✓ Tests complete$(COLOR_RESET)"

test-coverage: test ## Run tests with coverage report
	@echo "$(COLOR_BOLD)Generating coverage report...$(COLOR_RESET)"
	go tool cover -html=coverage.out -o coverage.html
	@echo "$(COLOR_GREEN)✓ Coverage report generated: coverage.html$(COLOR_RESET)"

bench: ## Run benchmarks
	@echo "$(COLOR_BOLD)Running benchmarks...$(COLOR_RESET)"
	go test -bench=. -benchmem ./...

lint: ## Run golangci-lint
	@echo "$(COLOR_BOLD)Running linter...$(COLOR_RESET)"
	golangci-lint run ./...
	@echo "$(COLOR_GREEN)✓ Lint complete$(COLOR_RESET)"

fmt: ## Format code
	@echo "$(COLOR_BOLD)Formatting code...$(COLOR_RESET)"
	gofmt -s -w $(GOFILES)
	@echo "$(COLOR_GREEN)✓ Format complete$(COLOR_RESET)"

vet: ## Run go vet
	@echo "$(COLOR_BOLD)Running go vet...$(COLOR_RESET)"
	go vet ./...
	@echo "$(COLOR_GREEN)✓ Vet complete$(COLOR_RESET)"

build: ## Build the library
	@echo "$(COLOR_BOLD)Building...$(COLOR_RESET)"
	go build -v ./...
	@echo "$(COLOR_GREEN)✓ Build complete$(COLOR_RESET)"

clean: ## Clean build artifacts
	@echo "$(COLOR_BOLD)Cleaning...$(COLOR_RESET)"
	rm -f coverage.out coverage.html
	go clean
	@echo "$(COLOR_GREEN)✓ Clean complete$(COLOR_RESET)"

install: ## Install dependencies
	@echo "$(COLOR_BOLD)Installing dependencies...$(COLOR_RESET)"
	go mod download
	go mod tidy
	@echo "$(COLOR_GREEN)✓ Dependencies installed$(COLOR_RESET)"

doc: ## Generate and serve documentation
	@echo "$(COLOR_BOLD)Starting godoc server...$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Open http://localhost:6060/pkg/github.com/Pure-Company/purefunccore/$(COLOR_RESET)"
	godoc -http=:6060

check: fmt vet lint test ## Run all checks (fmt, vet, lint, test)
	@echo "$(COLOR_GREEN)✓ All checks passed$(COLOR_RESET)"

.DEFAULT_GOAL := help

