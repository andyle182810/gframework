.PHONY:pre-lint
pre-lint:
	go install mvdan.cc/gofumpt@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

.PHONY:lint
lint: gci pre-lint
	go mod tidy
	gofumpt -l -w .
	go vet ./...
	golangci-lint cache clean && golangci-lint run ./...

.PHONY:pre-gci
pre-gci:
	go install github.com/daixiang0/gci@latest

.PHONY:gci
gci: pre-gci
	gci write --skip-generated -s standard -s default .

.PHONY: test
test:
	go test ./... -v -coverprofile=coverage.out

.PHONY:pre-benchmark
pre-benchmark:
	go install golang.org/x/perf/cmd/benchstat@latest
