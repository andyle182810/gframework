.PHONY: help docker-up docker-down docker-logs run clean deps test pre-lint lint pre-gci gci

.DEFAULT_GOAL := help

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

## Docker commands
docker-up: ## Start docker containers (postgres + valkey)
	docker-compose up -d
	@echo "Waiting for services to be ready..."
	@sleep 5
	@echo "Services are ready!"

docker-down: ## Stop docker containers
	docker-compose down

docker-logs: ## Show docker logs
	docker-compose logs -f

## Application commands
run: ## Run the application
	go run main.go

## Testing commands
test: ## Run all tests
	go test -v ./...

test-short: ## Run only unit tests (skip integration tests)
	go test -v -short ./...

test-integration: ## Run only integration tests
	go test -v -run Integration ./...

test-service: ## Run service layer tests
	go test -v ./internal/service/...

test-repo: ## Run repository layer tests
	go test -v ./internal/repo/...

test-coverage: ## Run tests with coverage report
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-watch: ## Run tests in watch mode (requires watchexec or similar)
	@command -v watchexec >/dev/null 2>&1 || (echo "watchexec not found. Install with: brew install watchexec" && exit 1)
	watchexec -e go -c -r -- make test-short

test-clean: ## Clean test cache and coverage reports
	go clean -testcache
	rm -f coverage.out coverage.html

## Maintenance commands
clean: ## Clean up docker volumes and data
	docker-compose down -v
	rm -rf data/

deps: ## Install dependencies
	go mod download
	go mod tidy

## Linting commands
pre-gci: ## Install gci tool
	go install github.com/daixiang0/gci@latest

gci: pre-gci ## Format imports using gci
	gci write --skip-generated -s standard -s default .

pre-lint: ## Install linting tools
	go install mvdan.cc/gofumpt@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

lint: gci pre-lint ## Run all linters and formatters
	go mod tidy
	gofumpt -l -w .
	go vet ./...
	golangci-lint run

lint-fix: gci pre-lint ## Run all linters with auto-fix
	go mod tidy
	gofumpt -l -w .
	go vet ./...
	golangci-lint run --fix
