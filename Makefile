BINARY  := plumbline
BIN_DIR := bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

# Paths passed to `make run`, e.g. make run ARGS="--format github testdata"
ARGS ?= testdata/sample-contract

.PHONY: help build test lint cover run corpus corpus-check clean

help: ## Show this help
	@grep -hE '^[a-z-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-8s\033[0m %s\n", $$1, $$2}'

build: ## Build the plumbline binary into bin/
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) ./cmd/plumbline

test: ## Run the test suite with the race detector
	go test -race ./...

lint: ## Check formatting, vet, and run golangci-lint
	@unformatted=$$(gofmt -l . 2>/dev/null); \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needs to run on:"; echo "$$unformatted"; exit 1; \
	fi
	go vet ./...
	golangci-lint run

cover: ## Report test coverage per function
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

run: build ## Lint the sample contract (override with ARGS=...)
	./$(BIN_DIR)/$(BINARY) $(ARGS)

corpus: build ## Lint the pinned third-party corpus and report per-rule counts
	@./corpus/run.sh

corpus-check: build ## Lint the corpus and fail if the counts left the baseline
	@./corpus/run.sh --check

clean: ## Remove build output
	rm -rf $(BIN_DIR) coverage.out
