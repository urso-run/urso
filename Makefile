BINARY_NAME=urso
BIN_DIR=./bin
GOBIN ?= $(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN = $(shell go env GOPATH)/bin
endif

ZSH_COMPLETION_DIR ?= $(HOME)/.zsh/completions

# Build information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || echo "unknown")

# Go build flags
# -s -w strips debug info for a smaller binary
LDFLAGS = -ldflags="-s -w \
	-X 'main.version=$(VERSION)' \
	-X 'main.commit=$(COMMIT)' \
	-X 'main.date=$(DATE)'"

.DEFAULT_GOAL := help

.PHONY: help
help: ## Display this help screen
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

.PHONY: completions
completions: ## Generate shell completions
	@sh ./scripts/completions.sh

.PHONY: build
build: ## Build the binary
	@mkdir -p $(BIN_DIR)
	go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/urso

.PHONY: install
install: build ## Install the binary and zsh completions
	@echo "Installing binary to $(GOBIN)..."
	@install -d $(GOBIN)
	@install $(BIN_DIR)/$(BINARY_NAME) $(GOBIN)/$(BINARY_NAME)
	@echo "Installing zsh completions to $(ZSH_COMPLETION_DIR)..."
	@mkdir -p $(ZSH_COMPLETION_DIR)
	@$(BIN_DIR)/$(BINARY_NAME) completion zsh > $(ZSH_COMPLETION_DIR)/_urso
	@echo "Done!"

# @echo ""
# @echo "To enable completions, ensure $(ZSH_COMPLETION_DIR) is in your fpath."
# @echo "Add the following to your .zshrc if not already present:"
# @echo '  fpath=($(ZSH_COMPLETION_DIR) $$fpath)'
# @echo '  autoload -Uz compinit && compinit'

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf $(BIN_DIR)
	rm -rf completions
	rm -f coverage.out coverage.html

.PHONY: test
test: ## Run tests
	go test -v -race -count=1 -timeout=30s ./...

.PHONY: coverage
coverage: ## Run tests and generate coverage report
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated at coverage.html"

.PHONY: tidy
tidy: ## Tidy and verify go modules
	go mod tidy -v
	go mod verify

.PHONY: fmt
fmt: ## Format source code
	go fmt ./...

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run --fix

.PHONY: all
all: tidy fmt vet test completions build install ## Run tidy, fmt, vet, test, completions, build and install
