.PHONY: build build-fast run tidy test test-short test-integration lint docker-infra docker-down docker-logs docker-build docker-run migrate migrate-check migrate-prod secrets-dir gen-jwt-key dev help vet

BINARY    := bin/openvulnscan
CMD       := ./cmd/server/
GOFLAGS   := -race
DATABASE_URL ?= postgres://openvulnscan:secret@localhost:5432/openvulnscan?sslmode=disable
VERSION   ?= $(shell git describe --tags --always 2>/dev/null || echo "dev")
IMAGE_NAME ?= openvulnscan

# ── Build & Run ───────────────────────────────────────────────────────────────

build:
	go build -o $(BINARY) $(CMD)

build-fast:
	go build -o $(BINARY) $(CMD)

run:
	go run $(CMD)

tidy:
	go mod tidy

vet:
	go vet ./...

# ── Testing ───────────────────────────────────────────────────────────────────

test:
	go test ./... -v $(GOFLAGS) -timeout 60s

test-short:
	go test ./... -short -timeout 30s

## test-integration: Run integration tests (requires docker-compose.test.yml up)
test-integration:
	docker-compose -f docker-compose.test.yml up -d
	@echo "Waiting for infra..."
	@sleep 5
	go test -tags integration ./integration/... -v -timeout 120s

## test-coverage: Run tests with HTML coverage report
test-coverage:
	go test ./... -short -coverprofile=coverage.out -timeout 30s
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# ── Infrastructure ────────────────────────────────────────────────────────────

docker-infra:
	docker-compose up -d postgres redis nats minio

docker-down:
	docker-compose down -v

docker-logs:
	docker-compose logs -f

# ── Docker Production (T18) ───────────────────────────────────────────────────

## docker-build: Build production Docker image
docker-build:
	@echo "Building $(IMAGE_NAME):$(VERSION)..."
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ") \
		-t $(IMAGE_NAME):$(VERSION) \
		-t $(IMAGE_NAME):latest \
		-f Dockerfile ../..
	@echo "Image size: $$(docker image inspect $(IMAGE_NAME):$(VERSION) --format='{{.Size}}' | numfmt --to=iec 2>/dev/null || echo 'N/A')"

## docker-run: Start production stack (requires .env)
docker-run:
	@test -f .env || (cp .env.example .env && echo "⚠ Edit .env before running" && exit 1)
	docker-compose -f docker-compose.prod.yml --env-file .env up -d

## docker-stop: Stop production stack
docker-stop:
	docker-compose -f docker-compose.prod.yml down

## migrate-prod: Run migrations in production container
migrate-prod:
	docker-compose -f docker-compose.prod.yml exec openvulnscan \
		/usr/local/bin/openvulnscan migrate --config /app/configs/config.docker.yaml

## logs: Tail production logs
logs:
	docker-compose -f docker-compose.prod.yml logs -f openvulnscan

# ── Migrations ────────────────────────────────────────────────────────────────

migrate:
	@echo "Running migrations against $(DATABASE_URL)..."
	@for f in migrations/*.sql; do \
		echo "  Applying $$f..."; \
		psql "$(DATABASE_URL)" -f $$f || exit 1; \
	done
	@echo "Migrations complete."

migrate-check:
	psql "$(DATABASE_URL)" -c "\dt" | grep -E "Table|schema"

# ── JWT Key Generation ────────────────────────────────────────────────────────

secrets-dir:
	mkdir -p secrets

gen-jwt-key: secrets-dir
	@echo "Generating RSA-4096 JWT private key..."
	openssl genrsa -out secrets/jwt_private.pem 4096
	openssl rsa -in secrets/jwt_private.pem -pubout -out secrets/jwt_public.pem
	@echo "Keys generated: secrets/jwt_private.pem, secrets/jwt_public.pem"
	@echo "IMPORTANT: Add secrets/ to .gitignore!"

# ── Development ───────────────────────────────────────────────────────────────

dev: docker-infra gen-jwt-key migrate
	@echo "Infrastructure ready. Starting server..."
	go run $(CMD)

lint:
	golangci-lint run ./... --timeout 5m

# ── Help ──────────────────────────────────────────────────────────────────────

help:
	@echo "OpenVulnScan Makefile targets:"
	@echo "  build              Build binary to bin/openvulnscan"
	@echo "  run                Run server (dev mode)"
	@echo "  tidy               go mod tidy"
	@echo "  vet                go vet ./..."
	@echo "  test               Run all tests with race detector"
	@echo "  test-integration   Run integration tests (needs docker-compose.test.yml)"
	@echo "  test-coverage      Run tests with HTML coverage"
	@echo "  docker-infra       Start dev infrastructure (postgres, redis, nats)"
	@echo "  docker-down        Stop and remove containers"
	@echo "  docker-build       Build production Docker image"
	@echo "  docker-run         Start production stack (needs .env)"
	@echo "  docker-stop        Stop production stack"
	@echo "  migrate            Apply SQL migrations (dev)"
	@echo "  migrate-prod       Apply migrations in production container"
	@echo "  logs               Tail production logs"
	@echo "  gen-jwt-key        Generate RSA-4096 JWT key pair"
	@echo "  dev                Full dev setup: infra + migrations + run"
