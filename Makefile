# Makefile for gllm

# Variables
BINARY_NAME=gllm
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DIR=dist
GOBASE=$(shell pwd)
GOBIN=$(GOBASE)/$(BUILD_DIR)

# Flags
LDFLAGS=-ldflags "-X github.com/activebook/gllm/cmd.version=$(VERSION)"

.PHONY: all build run test clean install lint release help deps

all: build

build: ## Build the binary
	@echo "  >  Building binary..."
	@mkdir -p $(GOBIN)
	@go build $(LDFLAGS) -o $(GOBIN)/$(BINARY_NAME) main.go

run: ## Run the application
	@echo "  >  Running application..."
	@go run $(LDFLAGS) main.go

test: ## Run tests
	@echo "  >  Running tests..."
	@go test ./... -v

clean: ## Clean build artifacts
	@echo "  >  Cleaning build cache..."
	@go clean
	@rm -rf $(BUILD_DIR)

install: ## Install the binary to $GOPATH/bin
	@echo "  >  Installing binary..."
	@go install $(LDFLAGS) main.go

lint: ## Run linter (golangci-lint if available)
	@echo "  >  Linting..."
	@if command -v golangci-lint >/dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, falling back to go vet"
		go vet ./...; \
	fi

deps: ## Download and tidy dependencies
	@echo "  >  Downloading dependencies..."
	@go mod download
	@go mod tidy

update: ## Update all dependencies to latest
	@echo "  >  Updating dependencies..."
	@go get -u ./...
	@go mod tidy

create-pr: ## Create a pull request
	@echo "  >  Creating pull request..."
	@./build/create-pr.sh

close-pr: ## Close a pull request
	@echo "  >  Closing pull request..."
	@./build/close-pr.sh

merge-pr: ## Merge a pull request
	@echo "  >  Merging pull request..."
	@./build/merge-pr.sh

release: ## Run the release script
	@echo "  >  Releasing..."
	@./build/release.sh

help: ## Show this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
