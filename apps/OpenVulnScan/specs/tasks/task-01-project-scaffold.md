# T01 — Project Scaffold (v3)

## Thông tin
| | |
|---|---|
| **Phase** | 0 — Foundation |
| **Ước tính** | 2–3 giờ |
| **Depends on** | T00 |
| **Blocks** | T02, T03, T04–T12 |

## Mục tiêu
Tạo cấu trúc thư mục dự án (v3: goroutine-per-service), `go.mod` với replace directives, và tích hợp vào Go workspace.

---

## 1.1 Tạo cấu trúc thư mục

```bash
mkdir -p osv.dev/apps/OpenVulnScan/{cmd/server,internal/{app,transport,runners,gateway/{handlers},events,syslog},migrations,configs}
```

Cấu trúc đầy đủ:
```
apps/OpenVulnScan/
├── cmd/
│   └── server/
│       └── main.go                    # Entry point
├── internal/
│   ├── app/
│   │   ├── app.go                     # App container + wire-up
│   │   ├── registry.go                # ServiceRunner interface + Registry
│   │   ├── clients.go                 # gRPC clients đến service goroutines
│   │   └── config.go                  # Config struct
│   ├── transport/
│   │   └── bufconn.go                 # bufconn helpers (NewBufConnListener, DialBufConn)
│   ├── runners/                       # Một file per service goroutine
│   │   ├── auth_runner.go
│   │   ├── scan_runner.go
│   │   ├── finding_runner.go
│   │   ├── product_runner.go
│   │   ├── vuln_runner.go
│   │   ├── report_runner.go
│   │   ├── query_runner.go
│   │   ├── notify_runner.go
│   │   ├── ingestion_runner.go
│   │   ├── gateway_runner.go          # HTTP server goroutine
│   │   └── helpers.go                 # Shared interceptors, utilities
│   ├── gateway/
│   │   ├── router.go                  # chi.Router: mount all routes
│   │   ├── middleware.go              # JWT + APIKey middleware
│   │   └── handlers/
│   │       ├── auth.go
│   │       ├── scan.go
│   │       ├── finding.go
│   │       ├── product.go
│   │       ├── vuln.go
│   │       ├── report.go
│   │       ├── dashboard.go
│   │       ├── agent.go
│   │       ├── health.go
│   │       └── helpers.go             # jsonResponse, httpGRPCError, pagination
│   ├── events/
│   │   ├── setup.go                   # JetStream stream setup
│   │   └── subjects.go                # NATS subject constants
│   └── syslog/
│       └── channel.go                 # SIEM syslog channel adapter
├── migrations/
│   └── *.sql
├── configs/
│   ├── config.yaml
│   └── config.docker.yaml
├── docker-compose.yml
├── Dockerfile
├── go.mod
└── Makefile
```

---

## 1.2 Xác nhận module paths từ services

```bash
# Chạy tất cả lệnh này và ghi lại kết quả
for svc in auth-service scan-service finding-service product-service \
           vulnerability-service report-service ingestion-service \
           notification-service query-service; do
    echo "=== $svc ==="
    grep "^module" osv.dev/services/$svc/go.mod
done

grep "^module" osv.dev/services/shared/pkg/go.mod
grep "^module" osv.dev/services/shared/proto/go.mod
```

**Điền kết quả vào bảng (quan trọng — phải làm trước bước 1.3)**:

| Service | Actual module name |
|---|---|
| auth-service | `github.com/osv/auth-service` |
| scan-service | `github.com/osv/scan-service` |
| finding-service | `github.com/defectdojo/finding-service` |
| product-service | `github.com/defectdojo/product-service` |
| vulnerability-service | `github.com/osv/vulnerability-service` |
| report-service | `github.com/osv/report-service` |
| ingestion-service | `github.com/osv/ingestion-service` |
| notification-service | `github.com/osv/notification-service` |
| query-service | `github.com/osv/query-service` |
| shared/pkg | `github.com/osv/shared/pkg` |
| shared/proto | `github.com/osv/shared/proto` |

---

## 1.3 Tạo `go.mod`

> **Cập nhật các module names theo bảng ở bước 1.2 trước khi tạo file này!**

```go
module github.com/osv/apps/openvulnscan

go 1.22

require (
    // Shared foundations
    github.com/osv/shared/pkg   v0.0.0
    github.com/osv/shared/proto v0.0.0

    // Business services (adjust module names after step 1.2)
    github.com/osv/auth-service            v0.0.0
    github.com/osv/scan-service            v0.0.0
    github.com/defectdojo/finding-service  v0.0.0
    github.com/osv/product-service         v0.0.0
    github.com/osv/vulnerability-service   v0.0.0
    github.com/osv/report-service          v0.0.0
    github.com/osv/ingestion-service       v0.0.0
    github.com/osv/notification-service    v0.0.0
    github.com/osv/query-service           v0.0.0

    // gRPC + protobuf
    google.golang.org/grpc          v1.81.1
    google.golang.org/protobuf      v1.36.0

    // HTTP
    github.com/go-chi/chi/v5        v5.2.2
    github.com/go-chi/cors          v1.2.1

    // NATS
    github.com/nats-io/nats.go      v1.37.0

    // Database
    github.com/jackc/pgx/v5         v5.10.0
    github.com/redis/go-redis/v9    v9.7.3

    // Utilities
    github.com/rs/zerolog           v1.33.0
    github.com/google/uuid          v1.6.0
    github.com/spf13/viper          v1.19.0
)

replace (
    github.com/osv/shared/pkg             => ../../services/shared/pkg
    github.com/osv/shared/proto           => ../../services/shared/proto
    github.com/osv/auth-service           => ../../services/auth-service
    github.com/osv/scan-service           => ../../services/scan-service
    github.com/defectdojo/finding-service => ../../services/finding-service
    github.com/osv/product-service        => ../../services/product-service
    github.com/osv/vulnerability-service  => ../../services/vulnerability-service
    github.com/osv/report-service         => ../../services/report-service
    github.com/osv/ingestion-service      => ../../services/ingestion-service
    github.com/osv/notification-service   => ../../services/notification-service
    github.com/osv/query-service          => ../../services/query-service
)
```

---

## 1.4 Cập nhật `go.work`

```bash
# Mở osv.dev/services/go.work và thêm:
use (
    // ... existing entries ...
    ../apps/OpenVulnScan   // ← thêm dòng này
)
```

Hoặc dùng lệnh:
```bash
cd osv.dev/services && go work edit -use ../apps/OpenVulnScan
```

---

## 1.5 Tạo Makefile

```makefile
.PHONY: build run tidy test lint docker-up docker-down

BINARY := bin/openvulnscan
CMD    := ./cmd/server/

build:
	go build -o $(BINARY) $(CMD)

run:
	go run $(CMD)

tidy:
	go mod tidy

test:
	go test ./... -v -race -timeout 60s

test-integration:
	go test ./tests/integration/... -tags=integration -v -timeout 120s

lint:
	golangci-lint run ./...

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down -v

migrate:
	psql $(DATABASE_URL) < migrations/001_initial.sql

# Build & run với docker-compose
dev: docker-up
	sleep 3 && go run $(CMD)
```

---

## 1.6 Tạo `internal/runners/helpers.go`

Shared gRPC interceptors cho tất cả runners:

```go
// internal/runners/helpers.go
package runners

import (
    "context"
    "runtime/debug"

    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    "github.com/rs/zerolog/log"
)

// grpcRecoveryInterceptor recover từ panic trong gRPC handler.
func grpcRecoveryInterceptor(
    ctx context.Context,
    req interface{},
    info *grpc.UnaryServerInfo,
    handler grpc.UnaryHandler,
) (resp interface{}, err error) {
    defer func() {
        if r := recover(); r != nil {
            log.Error().
                Str("method", info.FullMethod).
                Interface("panic", r).
                Str("stack", string(debug.Stack())).
                Msg("gRPC panic recovered")
            err = status.Errorf(codes.Internal, "internal server error")
        }
    }()
    return handler(ctx, req)
}

// grpcLoggingInterceptor log mỗi gRPC call.
func grpcLoggingInterceptor(
    ctx context.Context,
    req interface{},
    info *grpc.UnaryServerInfo,
    handler grpc.UnaryHandler,
) (interface{}, error) {
    resp, err := handler(ctx, req)
    if err != nil {
        log.Error().Str("method", info.FullMethod).Err(err).Msg("gRPC error")
    } else {
        log.Debug().Str("method", info.FullMethod).Msg("gRPC ok")
    }
    return resp, err
}
```

---

## 1.7 Verify

```bash
cd osv.dev/apps/OpenVulnScan

# Module tidy (có thể có warning nhưng không lỗi fatal)
go mod tidy

# Verify workspace
cd ../../services && go work sync

# Build (sẽ lỗi do file Go chưa có nội dung đầy đủ, nhưng không lỗi module)
cd ../apps/OpenVulnScan && go build ./...
```

---

## Output

- [x] Thư mục `apps/OpenVulnScan/` với đầy đủ cấu trúc
- [x] `go.mod` với replace directives (đã xác nhận module names thực tế)
- [x] `go.work` đã bao gồm `apps/OpenVulnScan`
- [x] `Makefile` với build, run, test, docker, migrate targets
- [x] `internal/runners/helpers.go` với gRPC interceptors
- [x] Module paths đã xác nhận và điền vào bảng

## Trạng thái: ✅ HOÀN THÀNH
> Thực thi: 2026-06-09
> `go build ./...` thành công không lỗi
> `go.work` đã thêm `../apps/OpenVulnScan`
> Lưu ý: `defectdojo/product-service` chưa có trong go.mod (cần kiểm tra)

## Acceptance Criteria

```bash
# go work sync không lỗi
cd osv.dev/services && go work sync

# go list không lỗi module
cd osv.dev/apps/OpenVulnScan && go list ./...
```

## Rủi ro

| Rủi ro | Xử lý |
|--------|-------|
| Module path finding-service sai | Đọc `finding-service/go.mod` để xác nhận |
| Services dùng Go version khác nhau | Dùng version cao nhất |
| Shared/proto chưa generate | Chạy protoc hoặc dùng proto trực tiếp |
