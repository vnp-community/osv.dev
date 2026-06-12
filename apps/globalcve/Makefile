BINARY      := globalcve-mono
GO          := $(shell ls /Users/binhnt/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.23.0.darwin-arm64/bin/go 2>/dev/null || echo /usr/local/go/bin/go)
BUILD_DIR   := ./build
CONFIG      := config/config.yaml
GIT_COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME  := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS     := -ldflags="-X main.version=$(GIT_COMMIT) -X main.buildTime=$(BUILD_TIME)"

.PHONY: all build run dev tidy test test-short test-coverage \
        lint migrate migrate-down migrate-status migrate-reset \
        infra-up infra-down infra-destroy infra-logs \
        setup clean help

all: build

# ─── Build ───────────────────────────────────────────────────────────────────
## Build the monolithic binary
build:
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(LDFLAGS) -trimpath -o $(BUILD_DIR)/$(BINARY) ./cmd/main.go
	@echo "Built: $(BUILD_DIR)/$(BINARY)"

# ─── Run ─────────────────────────────────────────────────────────────────────
## Run in production mode
run:
	$(GO) run $(LDFLAGS) ./cmd/main.go --config $(CONFIG)

## Run with hot reload (requires air)
dev:
	@which air > /dev/null || $(GO) install github.com/air-verse/air@latest
	air -c .air.toml

# ─── Test ────────────────────────────────────────────────────────────────────
## Run all tests with race detector
test:
	$(GO) test -race -cover ./...

## Run tests without integration tests
test-short:
	$(GO) test -short ./...

## Generate coverage report
test-coverage:
	$(GO) test ./... -coverprofile=coverage.out
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# ─── Lint ────────────────────────────────────────────────────────────────────
## Run linter
lint:
	@which golangci-lint > /dev/null || \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$($(GO) env GOPATH)/bin
	golangci-lint run ./...

# ─── Database ────────────────────────────────────────────────────────────────
## Run database migrations (up)
migrate:
	$(GO) run ./cmd/main.go migrate --config $(CONFIG)

## Rollback last migration
migrate-down:
	GOOSE_DRIVER=pgx GOOSE_DBSTRING="$${DATABASE_URL}" \
		$(GO) run github.com/pressly/goose/v3/cmd/goose@v3.24.3 -dir migrations down

## Show migration status
migrate-status:
	GOOSE_DRIVER=pgx GOOSE_DBSTRING="$${DATABASE_URL}" \
		$(GO) run github.com/pressly/goose/v3/cmd/goose@v3.24.3 -dir migrations status

## Reset all migrations
migrate-reset:
	GOOSE_DRIVER=pgx GOOSE_DBSTRING="$${DATABASE_URL}" \
		$(GO) run github.com/pressly/goose/v3/cmd/goose@v3.24.3 -dir migrations reset

# ─── Infrastructure ───────────────────────────────────────────────────────────
## Start all infrastructure services (PostgreSQL, Redis, NATS, OpenSearch)
infra-up:
	docker compose up -d
	@echo "Waiting for services to be healthy..."
	@sleep 10
	@docker compose ps

## Stop infrastructure services
infra-down:
	docker compose down

## Stop and remove all volumes
infra-destroy:
	docker compose down -v

## Follow infrastructure logs
infra-logs:
	docker compose logs -f

# ─── Setup (First Time) ──────────────────────────────────────────────────────
## First-time setup: copy .env, start infra, run migrations
setup:
	@cp -n .env.example .env 2>/dev/null && echo "✓ .env created" || echo "• .env already exists"
	$(MAKE) infra-up
	@echo "Waiting for PostgreSQL to be ready..."
	@sleep 8
	$(MAKE) migrate
	@echo "✓ Setup complete! Run 'make run' to start"

# ─── Go Tools ─────────────────────────────────────────────────────────────────
## Tidy dependencies
tidy:
	$(GO) mod tidy

## Generate code
generate:
	$(GO) generate ./...

## Clean build artifacts
clean:
	rm -rf $(BUILD_DIR) tmp coverage.out coverage.html

# ─── Help ────────────────────────────────────────────────────────────────────
## Show this help
help:
	@echo "GlobalCVE v3.0 Makefile"
	@echo ""
	@echo "Usage: make <target>"
	@echo ""
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
