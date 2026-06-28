SPEC_REPO    := spec
SPEC_FILE    := $(SPEC_REPO)/dist/openapi.bundled.yaml
BINARY       := bin/hlctl
OAPI_CODEGEN := go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
SPEC_VERSION := $(shell grep '^  version:' $(SPEC_FILE) | awk '{print $$2}')

.PHONY: help build generate bundle lint test tidy

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

build: ## Build the hlctl binary
	go build -ldflags "-X main.specVersion=$(SPEC_VERSION)" -o $(BINARY) ./cmd/hlctl

generate: bundle ## Generate client code from the bundled spec
	@mkdir -p internal/api/system internal/api/docker internal/api/storage internal/api/network internal/api/meta
	$(OAPI_CODEGEN) --config codegen/system.yaml $(SPEC_FILE)
	$(OAPI_CODEGEN) --config codegen/docker.yaml $(SPEC_FILE)
	$(OAPI_CODEGEN) --config codegen/storage.yaml $(SPEC_FILE)
	$(OAPI_CODEGEN) --config codegen/network.yaml $(SPEC_FILE)
	$(OAPI_CODEGEN) --config codegen/meta.yaml $(SPEC_FILE)

bundle: ## Bundle the OpenAPI spec from the submodule
	$(MAKE) -C $(SPEC_REPO) bundle

lint: ## Run go vet
	go vet ./...

test: ## Run tests
	go test ./...

tidy: ## Tidy go.mod
	go mod tidy
