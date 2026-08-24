# Makefile for data-contract-utils-manager.
#
# Recipes only use `go` plus POSIX-safe constructs so they work with GNU make
# on Linux, macOS and Windows (Git Bash / MSYS). Without make, every target is
# a plain `go ...` command you can run directly — see README.md.

MODULE   := github.com/Thales-OM/data-contract-utils-manager/cmd
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X $(MODULE).version=$(VERSION)

.PHONY: help build test cover vet lint fmt tidy clean

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary into bin/
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/

test: ## Run the test suite
	go test -race ./...

cover: ## Run tests and report coverage
	go test -race -covermode=atomic -coverprofile=coverage.txt ./...
	go tool cover -func=coverage.txt

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint (install: https://golangci-lint.run)
	golangci-lint run

fmt: ## Format all Go sources
	gofmt -s -w .

tidy: ## Sync module dependencies
	go mod tidy

clean: ## Remove build outputs
	$(RM) -r bin coverage.txt
