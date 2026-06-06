# OSV.dev Microservices — Root Makefile
# Usage: make help

SHELL := /bin/bash
.DEFAULT_GOAL := help

SERVICES := api-gateway vulnerability-query ingestion search ai-enrichment \
            impact-analysis version-index alias-relations notification source-sync web-bff
PKG      := pkg

BUF_VERSION  := v1.47.2
GO_VERSION   := 1.22
DOCKER_REPO  := ghcr.io/osv-dev
VERSION      := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT       := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE   := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS      := -ldflags="-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)"

# Colors
CYAN  := \033[36m
GREEN := \033[32m
RESET := \033[0m

##@ Help
.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\n$(CYAN)OSV.dev Microservices$(RESET)\nUsage: make [target]\n\n"} \
	/^[a-zA-Z_-]+:.*?##/ { printf "  $(GREEN)%-28s$(RESET) %s\n", $$1, $$2 } \
	/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)

##@ Development
.PHONY: env-setup
env-setup: ## Copy .env.example → .env (first-time setup)
	@if [ -f .env ]; then \
		echo "$(GREEN).env already exists — skipping. Edit it manually if needed.$(RESET)"; \
	else \
		cp .env.example .env; \
		echo "$(GREEN).env created from .env.example. Fill in secrets before running.$(RESET)"; \
	fi

.PHONY: infra
infra: ## Start local infrastructure (NATS, Redis, OpenSearch, Firestore, Ollama)
	@[ -f .env ] || (echo "$(CYAN)Tip: run 'make env-setup' to create your .env file first.$(RESET)")
	docker-compose --profile infra up -d
	@echo "$(GREEN)Infrastructure started.$(RESET)"

.PHONY: up
up: ## Start all services
	docker-compose --profile all up -d

.PHONY: down
down: ## Stop all services
	docker-compose --profile all down

.PHONY: logs
logs: ## Follow logs for all services (or SVC=service-name for one)
	docker-compose logs -f $(SVC)

##@ Build
.PHONY: build
build: ## Build all service binaries
	@for svc in $(SERVICES); do \
		echo "$(CYAN)Building $$svc...$(RESET)"; \
		(cd services/$$svc && go build $(LDFLAGS) ./cmd/...) || exit 1; \
	done
	@echo "$(GREEN)All services built$(RESET)"

.PHONY: docker-build
docker-build: ## Build Docker images for all services
	@for svc in $(SERVICES); do \
		docker build -t $(DOCKER_REPO)/$$svc:$(VERSION) -t $(DOCKER_REPO)/$$svc:latest services/$$svc/ || exit 1; \
	done

##@ Proto
.PHONY: proto
proto: ## Generate gRPC code from all proto files (requires buf)
	buf generate && echo "$(GREEN)Proto generation complete$(RESET)"

##@ Testing
.PHONY: test
test: ## Run all unit tests with race detector
	@for svc in pkg $(SERVICES); do \
		echo "$(CYAN)Testing services/$$svc...$(RESET)"; \
		(cd services/$$svc && go test -race -count=1 -timeout=60s ./...) || exit 1; \
	done
	@echo "$(GREEN)All tests passed$(RESET)"

.PHONY: test-svc
test-svc: ## Run tests for a specific service (SVC=search)
	cd services/$(SVC) && go test -race -count=1 -v -timeout=60s ./...

.PHONY: integration-test
integration-test: infra ## Run integration tests (requires Docker)
	@sleep 15
	@for svc in $(SERVICES); do \
		[ -d "services/$$svc/test/integration" ] && \
		(cd services/$$svc && go test -tags=integration -race -timeout=300s ./test/integration/...); \
	done

##@ Quality
.PHONY: lint
lint: ## Run golangci-lint on all services
	@for svc in pkg $(SERVICES); do \
		echo "$(CYAN)Linting services/$$svc...$(RESET)"; \
		(cd services/$$svc && golangci-lint run --timeout=5m ./...) || exit 1; \
	done
	@echo "$(GREEN)All lint checks passed$(RESET)"

.PHONY: vet
vet: ## Run go vet on all services
	@for svc in pkg $(SERVICES); do \
		(cd services/$$svc && go vet ./...) || exit 1; \
	done

.PHONY: tidy
tidy: ## Run go mod tidy on all services
	@for svc in pkg $(SERVICES); do \
		(cd services/$$svc && go mod tidy) || exit 1; \
	done

.PHONY: vuln-check
vuln-check: ## Run govulncheck on all services
	@for svc in $(SERVICES); do \
		(cd services/$$svc && govulncheck ./...) || true; \
	done

##@ Smoke Tests
.PHONY: smoke
smoke: ## Run smoke tests against local stack
	go run ./tools/smoke-test/main.go \
		--api-gateway http://localhost:8080 \
		--query http://localhost:8081 \
		--search http://localhost:8082

##@ Infrastructure
.PHONY: tf-plan
tf-plan: ## Plan Terraform changes (ENV=dev|staging|prod)
	cd infrastructure/terraform/environments/$(or $(ENV),dev) && terraform plan

.PHONY: tf-apply
tf-apply: ## Apply Terraform (ENV=dev|staging|prod)
	cd infrastructure/terraform/environments/$(or $(ENV),dev) && terraform apply

##@ Utilities
.PHONY: fmt
fmt: ## Format all Go code with gofumpt
	find services -name "*.go" -exec gofumpt -w {} \;

.PHONY: clean
clean: ## Remove build artifacts and volumes
	find services -name "*.test" -delete
	docker-compose --profile all down -v --remove-orphans 2>/dev/null || true
