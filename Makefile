SPEC_REPO    := spec
SPEC_FILE    := $(SPEC_REPO)/dist/openapi.bundled.yaml
BINARY       := bin/hlctl
OAPI_CODEGEN := go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

.PHONY: help build generate bundle lint test tidy

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

build: ## Build the hlctl binary
	go build -o $(BINARY) ./cmd/hlctl

generate: bundle ## Generate client code from the bundled spec
	@mkdir -p internal/system internal/docker internal/storage internal/network
	$(OAPI_CODEGEN) --config oapi-codegen-system.yaml $(SPEC_FILE)
	$(OAPI_CODEGEN) --config oapi-codegen-docker.yaml $(SPEC_FILE)
	$(OAPI_CODEGEN) --config oapi-codegen-storage.yaml $(SPEC_FILE)
	$(OAPI_CODEGEN) --config oapi-codegen-network.yaml $(SPEC_FILE)

bundle: ## Bundle the OpenAPI spec from the submodule
	$(MAKE) -C $(SPEC_REPO) bundle

lint: ## Run go vet
	go vet ./...

test: ## Run tests
	go test ./...

tidy: ## Tidy go.mod
	go mod tidy
