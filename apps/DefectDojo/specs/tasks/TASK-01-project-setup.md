# TASK-01: Project Setup

**Phase**: 1 — Foundation  
**Ước tính**: 3 giờ  
**Phụ thuộc**: TASK-00 hoàn thành  
**Output**: Go module hợp lệ, workspace configured, directory structure

---

## Mục tiêu

Tạo scaffolding đầy đủ cho `apps/DefectDojo/` — Go module, workspace configuration, và cấu trúc thư mục.

---

## T-01.1: Tạo Cấu Trúc Thư Mục

**Command**:
```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev

mkdir -p apps/DefectDojo/{cmd/defectdojo,internal/{app/registry,config,gateway,runners,events,transport,migration,metrics,testutil}}
mkdir -p apps/DefectDojo/tests/{integration,e2e}
mkdir -p apps/DefectDojo/scripts
```

**Expected structure**:
```
apps/DefectDojo/
├── cmd/
│   └── defectdojo/
│       └── main.go              # [CREATE in T-01.3]
├── internal/
│   ├── app/
│   │   ├── app.go               # [CREATE in TASK-04]
│   │   └── registry/
│   │       └── registry.go      # [CREATE in TASK-03]
│   ├── config/
│   │   └── config.go            # [CREATE in TASK-02]
│   ├── events/
│   │   ├── publisher.go         # [CREATE in TASK-06]
│   │   ├── consumer.go          # [CREATE in TASK-06]
│   │   └── setup.go             # [CREATE in TASK-06]
│   ├── gateway/
│   │   ├── router.go            # [CREATE in TASK-05]
│   │   ├── middleware.go        # [CREATE in TASK-05]
│   │   └── handlers/            # [CREATE in TASK-05]
│   ├── metrics/
│   │   └── metrics.go           # [CREATE in TASK-02]
│   ├── migration/
│   │   └── runner.go            # [CREATE in TASK-02]
│   ├── runners/
│   │   ├── base.go              # [CREATE in TASK-03]
│   │   ├── auth_runner.go       # [CREATE in TASK-04]
│   │   └── ...                  # [CREATE in TASK-04]
│   ├── transport/
│   │   └── bufconn.go           # [CREATE in TASK-03]
│   └── testutil/
│       └── helpers.go           # [CREATE in TASK-07]
├── tests/
│   ├── integration/
│   └── e2e/
├── scripts/
├── go.mod                       # [CREATE in T-01.2]
├── .env.example                 # [CREATE in T-01.4]
├── Makefile                     # [CREATE in T-01.5]
└── Dockerfile                   # [CREATE in TASK-08]
```

**Checklist**:
- [ ] Tất cả thư mục đã được tạo
- [ ] Cấu trúc khớp với spec

---

## T-01.2: Tạo go.mod

**File**: `apps/DefectDojo/go.mod`

```go
module github.com/defectdojo/apps/defectdojo

go 1.26.3

toolchain go1.26.3

require (
    // Shared packages từ services/
    github.com/osv/shared/pkg   v0.0.0
    github.com/osv/shared/proto v0.0.0

    // Service modules (resolved qua go.work)
    github.com/osv/auth-service              v0.0.0
    github.com/defectdojo/finding-service    v0.0.0
    github.com/defectdojo/product-service    v0.0.0
    github.com/defectdojo/scan-service       v0.0.0
    github.com/defectdojo/notification-service v0.0.0
    github.com/defectdojo/report-service     v0.0.0
    github.com/defectdojo/integration-service v0.0.0
    github.com/defectdojo/vulnerability-service v0.0.0
    github.com/defectdojo/search-service     v0.0.0
    github.com/defectdojo/ingestion-service  v0.0.0
    github.com/defectdojo/ai-service         v0.0.0
    github.com/defectdojo/impact-service     v0.0.0

    // HTTP
    github.com/go-chi/chi/v5  v5.2.2
    github.com/go-chi/cors    v1.2.1

    // gRPC & proto
    google.golang.org/grpc     v1.81.1
    google.golang.org/protobuf v1.36.12-0.20260120151049-f2248ac996af

    // Database
    github.com/jackc/pgx/v5 v5.10.0

    // NATS
    github.com/nats-io/nats.go v1.37.0

    // Redis
    github.com/redis/go-redis/v9 v9.7.3

    // Logging
    github.com/rs/zerolog v1.33.0

    // Config
    github.com/spf13/viper v1.19.0

    // Metrics
    github.com/prometheus/client_golang v1.21.1

    // UUID
    github.com/google/uuid v1.6.0

    // Testing
    github.com/stretchr/testify v1.10.0
)

// Tất cả replace directives trỏ vào local services/
replace (
    github.com/osv/shared/pkg                  => ../../services/shared/pkg
    github.com/osv/shared/proto                => ../../services/shared/proto
    github.com/osv/auth-service                => ../../services/auth-service
    github.com/defectdojo/finding-service      => ../../services/finding-service
    github.com/defectdojo/product-service      => ../../services/product-service
    github.com/defectdojo/scan-service         => ../../services/scan-service
    github.com/defectdojo/notification-service => ../../services/notification-service
    github.com/defectdojo/report-service       => ../../services/report-service
    github.com/defectdojo/integration-service  => ../../services/integration-service
    github.com/defectdojo/vulnerability-service => ../../services/vulnerability-service
    github.com/defectdojo/search-service       => ../../services/search-service
    github.com/defectdojo/ingestion-service    => ../../services/ingestion-service
    github.com/defectdojo/ai-service           => ../../services/ai-service
    github.com/defectdojo/impact-service       => ../../services/impact-service
)
```

> **Lưu ý**: Module names phải khớp với kết quả từ T-00.4. Chỉnh sửa nếu cần.

**Sau khi tạo**:
```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev/apps/DefectDojo
go mod tidy
```

**Checklist**:
- [ ] go.mod tạo thành công
- [ ] `go mod tidy` chạy không lỗi
- [ ] Replace directives đúng paths

---

## T-01.3: Cập nhật go.work

**File**: `services/go.work` (chỉnh sửa để thêm apps/DefectDojo)

```go
// Thêm vào phần use (...)
use (
    // ... existing entries ...
    ../apps/DefectDojo    // ADD THIS LINE
)
```

**Verify**:
```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev/services
go work sync
# Không có lỗi
```

**Checklist**:
- [ ] `apps/DefectDojo` đã thêm vào go.work
- [ ] `go work sync` thành công

---

## T-01.4: Tạo main.go (skeleton)

**File**: `apps/DefectDojo/cmd/defectdojo/main.go`

```go
// Package main is the entry point for the DefectDojo Go monolith.
//
// Tất cả chức năng của DefectDojo Django được tái implement bằng Go,
// sử dụng code base tại services/ mà không thay đổi source code.
package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"
)

func main() {
    // Configure logging
    zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
    if os.Getenv("LOG_FORMAT") == "console" {
        log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
    }

    log.Info().Msg("DefectDojo Go starting...")

    // TODO: Load config (TASK-02)
    // TODO: Create app (TASK-04)
    // TODO: Start app (TASK-04)

    // Signal handling
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

    sig := <-quit
    log.Info().Str("signal", sig.String()).Msg("received shutdown signal")

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    _ = ctx

    // TODO: app.Shutdown(ctx) (TASK-04)

    log.Info().Msg("DefectDojo Go stopped")
}
```

**Verify**:
```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev/apps/DefectDojo
go build ./cmd/defectdojo/
# Phải compile thành công (dù chưa có logic)
```

**Checklist**:
- [ ] main.go tạo thành công
- [ ] `go build` thành công

---

## T-01.5: Tạo .env.example

**File**: `apps/DefectDojo/.env.example`

```bash
# ── Required ──────────────────────────────────────────────────────────────────
POSTGRES_URL=postgres://defectdojo:changeme@localhost:5432/defectdojo?sslmode=disable
JWT_SECRET=change-me-to-at-least-32-characters-long-secret

# ── Infrastructure ────────────────────────────────────────────────────────────
NATS_URL=nats://localhost:4222
REDIS_URL=redis://localhost:6379
OPENSEARCH_URL=http://localhost:9200

# ── HTTP/gRPC Ports ───────────────────────────────────────────────────────────
HTTP_PORT=8080
GRPC_PORT=9090

# ── Auth ──────────────────────────────────────────────────────────────────────
JWT_EXPIRY=24h
REFRESH_EXPIRY=168h

# ── AI (optional) ─────────────────────────────────────────────────────────────
AI_BACKEND=ollama
AI_MODEL=llama3
AI_BASE_URL=http://localhost:11434
# AI_API_KEY=sk-...  (for OpenAI/Azure)

# ── Integrations (optional) ───────────────────────────────────────────────────
JIRA_ENCRYPTION_KEY=32-bytes-encryption-key-for-jira

# ── Email (optional) ──────────────────────────────────────────────────────────
# SMTP_HOST=smtp.gmail.com
# SMTP_PORT=587
# SMTP_USER=noreply@company.com
# SMTP_PASSWORD=app-specific-password

# ── Slack (optional) ──────────────────────────────────────────────────────────
# SLACK_TOKEN=xoxb-...

# ── Logging ───────────────────────────────────────────────────────────────────
LOG_LEVEL=info
LOG_FORMAT=json
```

---

## T-01.6: Tạo Makefile

**File**: `apps/DefectDojo/Makefile`

```makefile
.PHONY: build run test test-integration lint clean docker-build docker-up

BINARY=bin/defectdojo
SVC_DIR=/Users/binhnt/Lab/sec/cve/osv.dev/services

# ── Build ─────────────────────────────────────────────────────────────────────
build:
	go build -ldflags="-s -w" -o $(BINARY) ./cmd/defectdojo/

run:
	go run ./cmd/defectdojo/

# ── Test ──────────────────────────────────────────────────────────────────────
test:
	go test ./... -v -timeout 60s

test-integration:
	go test ./tests/integration/... -tags=integration -v -timeout 300s

test-e2e:
	go test ./tests/e2e/... -tags=e2e -v -timeout 600s

# ── Code quality ──────────────────────────────────────────────────────────────
lint:
	golangci-lint run ./...

vet:
	go vet ./...

# ── Database ──────────────────────────────────────────────────────────────────
migrate:
	go run ./cmd/defectdojo/ --migrate-only

# ── Docker ────────────────────────────────────────────────────────────────────
docker-build:
	docker build -f Dockerfile \
	  --build-arg SVC_DIR=$(SVC_DIR) \
	  -t defectdojo-go:dev ../..

docker-up:
	docker-compose -f docker-compose.yml up -d

docker-down:
	docker-compose -f docker-compose.yml down -v

# ── Clean ─────────────────────────────────────────────────────────────────────
clean:
	rm -f $(BINARY)
	go clean ./...

# ── Dependencies ──────────────────────────────────────────────────────────────
tidy:
	go mod tidy

# ── Proto (nếu cần generate thêm) ────────────────────────────────────────────
proto:
	cd $(SVC_DIR)/shared/proto && buf generate
```

---

## T-01.7: Verify Full Project Build

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev/apps/DefectDojo

# Verify module
go mod verify

# Build
go build ./...

# Vet
go vet ./...
```

**Checklist**:
- [ ] `go mod verify` pass
- [ ] `go build ./...` không lỗi
- [ ] `go vet ./...` không warning

---

## Definition of Done

- [ ] Directory structure đầy đủ
- [ ] go.mod với đúng module names và replace directives
- [ ] go.work đã update
- [ ] main.go skeleton compile được
- [ ] .env.example đầy đủ
- [ ] Makefile hoạt động
- [ ] `go build` thành công
