# TASK-10 — Docker & Dev Environment

## Mục Tiêu

Setup **docker-compose** với đầy đủ infrastructure (PostgreSQL+pgvector, Redis, NATS, OpenSearch), cấu hình Makefile targets, và `.env.example` hoàn chỉnh để developer có thể `make docker-up && make run` là chạy được.

## Phụ Thuộc

- TASK-09 (App hoàn chỉnh)

## Đầu Ra

- `docker-compose.yml` — infrastructure services
- `.env.example` — hoàn chỉnh
- `Makefile` — đầy đủ targets
- `.air.toml` — hot reload cho development
- `.golangci.yml` — linter config

---

## Checklist

- [x] docker-compose: PostgreSQL với pgvector extension
- [x] docker-compose: Redis
- [x] docker-compose: NATS với JetStream enabled
- [x] docker-compose: OpenSearch
- [x] Makefile: build, run, dev (air), test, lint, migrate, docker
- [x] .air.toml cho hot reload
- [x] .golangci.yml cho linting
- [x] Healthchecks cho tất cả services trong docker-compose

---

## 1. docker-compose.yml

```yaml
version: '3.9'

services:
  # ─── PostgreSQL with pgvector ──────────────────────────────────────────
  postgres:
    image: pgvector/pgvector:pg16
    container_name: globalcve-postgres
    environment:
      POSTGRES_DB: globalcve
      POSTGRES_USER: globalcve
      POSTGRES_PASSWORD: password
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U globalcve -d globalcve"]
      interval: 5s
      timeout: 5s
      retries: 10
    restart: unless-stopped

  # ─── Redis ─────────────────────────────────────────────────────────────
  redis:
    image: redis:7-alpine
    container_name: globalcve-redis
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    command: redis-server --appendonly yes
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 10
    restart: unless-stopped

  # ─── NATS with JetStream ───────────────────────────────────────────────
  nats:
    image: nats:2.10-alpine
    container_name: globalcve-nats
    ports:
      - "4222:4222"   # Client connections
      - "8222:8222"   # HTTP monitoring
    command: ["-js", "-m", "8222"]  # Enable JetStream + monitoring
    volumes:
      - nats_data:/data
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:8222/healthz"]
      interval: 5s
      timeout: 3s
      retries: 10
    restart: unless-stopped

  # ─── OpenSearch ────────────────────────────────────────────────────────
  opensearch:
    image: opensearchproject/opensearch:2.12.0
    container_name: globalcve-opensearch
    environment:
      - discovery.type=single-node
      - OPENSEARCH_INITIAL_ADMIN_PASSWORD=Admin@12345
      - bootstrap.memory_lock=true
      - "OPENSEARCH_JAVA_OPTS=-Xms512m -Xmx512m"
      - plugins.security.disabled=true  # Disable TLS for dev
    ports:
      - "9200:9200"
    volumes:
      - opensearch_data:/usr/share/opensearch/data
    healthcheck:
      test: ["CMD-SHELL", "curl -s http://localhost:9200/_cluster/health | grep -q '\"status\":\"green\"\\|\"status\":\"yellow\"'"]
      interval: 10s
      timeout: 10s
      retries: 20
    ulimits:
      memlock:
        soft: -1
        hard: -1
    restart: unless-stopped

volumes:
  postgres_data:
  redis_data:
  nats_data:
  opensearch_data:

networks:
  default:
    name: globalcve-network
```

---

## 2. .env.example (Hoàn Chỉnh)

```dotenv
# ─── Database ──────────────────────────────────────────────────────────────
DATABASE_URL=postgres://globalcve:password@localhost:5432/globalcve?sslmode=disable

# ─── Redis ─────────────────────────────────────────────────────────────────
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=

# ─── NATS ──────────────────────────────────────────────────────────────────
NATS_URL=nats://localhost:4222

# ─── OpenSearch ────────────────────────────────────────────────────────────
OPENSEARCH_URL=http://localhost:9200
OPENSEARCH_USERNAME=admin
OPENSEARCH_PASSWORD=Admin@12345

# ─── External APIs ─────────────────────────────────────────────────────────
# NVD API Key (optional, tăng rate limit từ 5 → 50 req/30s)
# Đăng ký tại: https://nvd.nist.gov/developers/request-an-api-key
NVD_API_KEY=

# ─── App Config ────────────────────────────────────────────────────────────
APP_PORT=8080
LOG_LEVEL=info

# ─── Admin Auth ────────────────────────────────────────────────────────────
# API key cho admin endpoints (/sync/trigger, /webhooks)
ADMIN_API_KEY=change-me-in-production

# ─── Internal Service Ports ────────────────────────────────────────────────
CVE_SEARCH_PORT=8081
CVE_SYNC_PORT=8082
KEV_SERVICE_PORT=8083
NOTIFICATION_PORT=8084
```

---

## 3. Makefile (Hoàn Chỉnh)

```makefile
.PHONY: build run dev test lint migrate-up migrate-down \
        docker-up docker-down docker-logs \
        tidy generate setup

# ─── Variables ───────────────────────────────────────────────────────────
BINARY      := globalcve
MAIN        := ./cmd/main.go
GIT_COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME  := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS     := -ldflags="-X main.version=$(GIT_COMMIT) -X main.buildTime=$(BUILD_TIME)"

# ─── Build ───────────────────────────────────────────────────────────────
build:
	@echo "Building $(BINARY)..."
	go build $(LDFLAGS) -o bin/$(BINARY) $(MAIN)

# ─── Run ─────────────────────────────────────────────────────────────────
run:
	go run $(LDFLAGS) $(MAIN)

dev:
	@which air > /dev/null || go install github.com/cosmtrek/air@latest
	air -c .air.toml

# ─── Test ────────────────────────────────────────────────────────────────
test:
	go test ./... -v -race -count=1

test-short:
	go test ./... -short -count=1

test-coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

# ─── Lint ────────────────────────────────────────────────────────────────
lint:
	@which golangci-lint > /dev/null || \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin
	golangci-lint run ./...

# ─── Database ────────────────────────────────────────────────────────────
migrate-up:
	goose -dir migrations postgres "$(DATABASE_URL)" up

migrate-down:
	goose -dir migrations postgres "$(DATABASE_URL)" down

migrate-status:
	goose -dir migrations postgres "$(DATABASE_URL)" status

migrate-reset:
	goose -dir migrations postgres "$(DATABASE_URL)" reset

# ─── Docker ──────────────────────────────────────────────────────────────
docker-up:
	docker compose up -d
	@echo "Waiting for services to be healthy..."
	@sleep 5
	@docker compose ps

docker-down:
	docker compose down

docker-destroy:
	docker compose down -v  # Remove volumes

docker-logs:
	docker compose logs -f

docker-ps:
	docker compose ps

# ─── Setup (first time) ──────────────────────────────────────────────────
setup:
	@cp -n .env.example .env || true
	@echo "✓ .env created from .env.example"
	$(MAKE) docker-up
	@echo "Waiting for PostgreSQL..."
	@sleep 8
	$(MAKE) migrate-up
	@echo "✓ Setup complete! Run 'make run' to start"

# ─── Go Tools ────────────────────────────────────────────────────────────
tidy:
	go mod tidy

generate:
	go generate ./...

# ─── Help ────────────────────────────────────────────────────────────────
help:
	@echo "GlobalCVE v3.0 Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make setup          First-time setup (copy .env, start docker, migrate)"
	@echo "  make run            Run the application"
	@echo "  make dev            Run with hot reload (air)"
	@echo "  make build          Build binary"
	@echo "  make test           Run all tests"
	@echo "  make lint           Run linter"
	@echo "  make migrate-up     Apply migrations"
	@echo "  make docker-up      Start infrastructure"
	@echo "  make docker-down    Stop infrastructure"
```

---

## 4. .air.toml (Hot Reload)

```toml
root = "."
tmp_dir = "tmp"

[build]
  cmd = "go build -o ./tmp/main ./cmd/main.go"
  bin = "tmp/main"
  full_bin = "./tmp/main"
  include_ext = ["go", "yaml", "env"]
  exclude_dir = ["tmp", "vendor", "testdata", "migrations"]
  delay = 1000    # ms
  kill_delay = 500 # ms

[log]
  time = true

[color]
  main = "magenta"
  watcher = "cyan"
  build = "yellow"
  runner = "green"

[misc]
  clean_on_exit = true
```

---

## 5. .golangci.yml

```yaml
linters:
  enable:
    - errcheck
    - govet
    - staticcheck
    - gosimple
    - ineffassign
    - unused
    - gofmt
    - goimports
    - misspell
    - revive
    - nolintlint

linters-settings:
  goimports:
    local-prefixes: github.com/binhnt/globalcve
  revive:
    rules:
      - name: exported
        severity: warning

run:
  timeout: 5m
  tests: true

issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - errcheck
```

---

## 6. Quick Start Guide

```bash
# 1. Clone và setup
git clone <repo>
cd osv.dev/apps/globalcve

# 2. First time setup (copy .env, start docker, run migrations)
make setup

# 3. Chỉnh sửa .env nếu cần (đặc biệt NVD_API_KEY)
vi .env

# 4. Chạy app
make run

# 5. Verify
curl http://localhost:8080/health
# → {"status":"healthy","services":[...]}

curl "http://localhost:8080/api/v2/cves?query=log4j&limit=5"
# → {"total":..., "cves":[...]}
```

---

## 7. Infrastructure URLs (Development)

| Service | URL | Purpose |
|---------|-----|---------|
| GlobalCVE API | `http://localhost:8080` | Public API |
| NATS Monitoring | `http://localhost:8222` | JetStream dashboard |
| OpenSearch | `http://localhost:9200` | Search engine |
| PostgreSQL | `localhost:5432` | Database |
| Redis | `localhost:6379` | Cache |

---

## Định Nghĩa Hoàn Thành

- [x] `make setup` chạy thành công (docker up + migrate)
- [x] `make run` khởi động app, log "API Gateway starting"
- [x] `curl localhost:8080/health` → `{"status":"ok",...}`
- [x] `make dev` (air) hot reload khi thay đổi code
- [x] `make test` tất cả tests pass
- [x] `make lint` không có errors
- [x] `make migrate-status` hiển thị 4 migrations OK
- [x] `make infra-down && make infra-up` → app vẫn chạy đúng

---

*TASK-10 | Docker & Dev Environment | GlobalCVE v3.0*
