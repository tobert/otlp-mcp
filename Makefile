# Ports (override with: make run MCP_PORT=9999 OTLP_PORT=5555)
MCP_PORT ?= 9912
OTLP_PORT ?= 4317

# Docker Desktop for Mac: host.docker.internal resolves to both IPv4 and IPv6,
# but IPv6 is unreachable. Force IPv4 via --add-host override.
HOST_ADDRESS := host.docker.internal
HOST_IPV4 := 192.168.65.254

# Images
OTEL_IMAGE := otel/opentelemetry-collector:0.146.1
IMAGE_NAME := otlp-mcp

.PHONY: help build-local test fmt vet build run run-bg serve proxy

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Variables:"
	@echo "  \033[33mMCP_PORT\033[0m     MCP HTTP port          (default: $(MCP_PORT))"
	@echo "  \033[33mOTLP_PORT\033[0m    OTLP gRPC port         (default: $(OTLP_PORT))"
	@echo "  \033[33mSTATELESS\033[0m    Run otlp-mcp stateless (default: off, set to 1 to enable)"

## Go development

build-local: ## Build otlp-mcp binary locally
	go build -o otlp-mcp ./cmd/otlp-mcp

test: ## Run all tests
	go test ./...

fmt: ## Format Go source files
	go fmt ./...

vet: ## Run Go vet linter
	go vet ./...

## Docker

build: ## Build all-in-one Docker image (proxy + otlp-mcp)
	docker build -t $(IMAGE_NAME) .

run: ## Run all-in-one container (proxy + otlp-mcp)
	@echo "Starting all-in-one container..."
	@echo "OTLP gRPC: localhost:$(OTLP_PORT)"
	@echo "OTel HTTP:  http://localhost:4318"
	@echo "MCP HTTP:   http://localhost:$(MCP_PORT)"
	docker run --rm \
		--name $(IMAGE_NAME) \
		-p $(OTLP_PORT):$(OTLP_PORT) \
		-p 4318:4318 \
		-p $(MCP_PORT):$(MCP_PORT) \
		-e MCP_PORT=$(MCP_PORT) \
		-e OTLP_PORT=$(OTLP_PORT) \
		$(if $(STATELESS),-e STATELESS=1,) \
		$(IMAGE_NAME)

run-bg: ## Run all-in-one container in background
	@echo "Starting all-in-one container..."
	@echo "OTLP gRPC: localhost:$(OTLP_PORT)"
	@echo "OTel HTTP:  http://localhost:4318"
	@echo "MCP HTTP:   http://localhost:$(MCP_PORT)"
	docker run --rm -d \
		--name $(IMAGE_NAME) \
		-p $(OTLP_PORT):$(OTLP_PORT) \
		-p 4318:4318 \
		-p $(MCP_PORT):$(MCP_PORT) \
		-e MCP_PORT=$(MCP_PORT) \
		-e OTLP_PORT=$(OTLP_PORT) \
		$(if $(STATELESS),-e STATELESS=1,) \
		$(IMAGE_NAME)

serve: ## Start otlp-mcp server (host, no Docker)
	@echo "Starting otlp-mcp..."
	@echo "MCP HTTP: http://localhost:$(MCP_PORT)"
	@echo "OTLP gRPC: localhost:$(OTLP_PORT)"
	otlp-mcp serve --transport http --http-port $(MCP_PORT) --otlp-port $(OTLP_PORT) --verbose $(if $(STATELESS),--stateless,)

proxy: ## Start HTTP-to-gRPC proxy only (Docker)
	@echo "Starting OTel Proxy..."
	@echo "Listening on: http://localhost:4318 (HTTP)"
	@echo "Forwarding to: $(HOST_ADDRESS):$(OTLP_PORT) (gRPC)"
	docker run --rm \
		--name otel-proxy \
		-p 4318:4318 \
		--add-host=$(HOST_ADDRESS):$(HOST_IPV4) \
		-v "$(PWD)/otel-config.yaml":/tmp/config.yaml \
		-e OTLP_MCP_ENDPOINT=$(HOST_ADDRESS):$(OTLP_PORT) \
		$(OTEL_IMAGE) \
		--config /tmp/config.yaml
