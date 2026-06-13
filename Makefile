.PHONY: help test test-coverage test-clean pre-lint fmt lint lint-fix pre-benchmark benchmark deps

.DEFAULT_GOAL := help

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

## Testing commands
test: ## Run all tests (no cache, 30s timeout)
	go test -v -count=1 ./...

test-coverage: ## Run tests with coverage report
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-clean: ## Clean test cache and coverage reports
	go clean -testcache
	rm -f coverage.out coverage.html

## Linting commands
pre-lint: ## Install golangci-lint v2
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

fmt: pre-lint ## Format code (gci/gofmt/gofumpt/goimports via golangci-lint v2)
	golangci-lint fmt

lint: pre-lint ## Run all formatters and linters
	go mod tidy
	go vet ./...
	golangci-lint fmt
	golangci-lint cache clean && golangci-lint run ./...

lint-fix: pre-lint ## Run all formatters and linters with auto-fix
	go mod tidy
	go vet ./...
	golangci-lint fmt
	golangci-lint cache clean && golangci-lint run --fix ./...

## Benchmark commands
pre-benchmark: ## Install benchstat tool
	go install golang.org/x/perf/cmd/benchstat@latest

benchmark: pre-benchmark ## Run benchmarks
	go test -bench=. -benchmem ./...

## Maintenance commands
deps: ## Install and tidy dependencies
	go mod download
	go mod tidy
