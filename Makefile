BINARY  := cc-status
PKG     := ./cmd/cc-status
BINDIR  := bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.DEFAULT_GOAL := build

.PHONY: build
build: ## Build the binary into ./bin
	@mkdir -p $(BINDIR)
	go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/$(BINARY) $(PKG)

.PHONY: install
install: ## Install the binary into $GOBIN / $GOPATH/bin
	go install -ldflags "$(LDFLAGS)" $(PKG)

.PHONY: run
run: ## Render testdata/sample.json with the example config
	go run $(PKG) --config testdata/config.yaml < testdata/sample.json

.PHONY: test
test: ## Run all tests
	go test ./...

.PHONY: cover
cover: ## Run tests with a coverage summary
	go test -cover ./...

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: fmt
fmt: ## Format all Go sources
	gofmt -w .

.PHONY: fmt-check
fmt-check: ## Fail if any file is not gofmt-clean
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "unformatted files:"; echo "$$out"; exit 1; fi

.PHONY: tidy
tidy: ## Tidy go.mod / go.sum
	go mod tidy

.PHONY: check
check: fmt-check vet test ## Run fmt-check, vet and tests

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf $(BINDIR)

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'
