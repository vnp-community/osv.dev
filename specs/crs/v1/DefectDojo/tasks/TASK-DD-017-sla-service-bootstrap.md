# ✅ COMPLETED — TASK-DD-017 — SLA Service Bootstrap (New Service)

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-DD-017 |
| **Service** | `sla-service` (NEW) |
| **CR** | CR-DD-006 |
| **Phase** | 2 — Security Management |
| **Priority** | 🔴 High |
| **Prerequisites** | — (độc lập) |
| **Estimated effort** | 1 ngày |

## Context

Tạo mới `sla-service` từ đầu với đầy đủ scaffolding: Go module, Dockerfile, migrations, server bootstrap theo chuẩn kiến trúc của project (xem `services/notification-service` hoặc `services/scan-service` làm template).

## Reference

- Solution: [`sol-sla-service.md`](../solutions/sol-sla-service.md)

## Working Directory

```
/Users/binhnt/Lab/sec/cve/osv.dev/services/sla-service/
```

## Files to Create

```
services/sla-service/
├── go.mod
├── go.sum
├── Dockerfile
├── .env.example
├── cmd/
│   └── server/
│       └── main.go
├── migrations/
│   ├── 001_sla_configurations.sql
│   └── 002_sla_computation_log.sql
└── internal/
    ├── config/
    │   └── config.go
    ├── domain/
    │   ├── config/
    │   │   ├── entity.go
    │   │   └── repository.go
    │   └── computation/
    │       └── entity.go
    ├── infra/
    │   ├── postgres/
    │   │   └── db.go
    │   └── nats/
    │       └── client.go
    └── delivery/
        ├── http/
        │   └── server.go
        └── grpc/
            └── server.go
```

## Implementation Spec

### `go.mod`

```
module github.com/osv/services/sla-service

go 1.22

require (
    github.com/go-chi/chi/v5 v5.1.0
    github.com/nats-io/nats.go v1.35.0
    github.com/lib/pq v1.10.9
    google.golang.org/grpc v1.64.0
    google.golang.org/protobuf v1.34.0
    github.com/google/uuid v1.6.0
)
```

### `internal/config/config.go`

```go
package config

import "os"

type Config struct {
    HTTPPort    int
    GRPCPort    int
    DatabaseURL string
    NATSAddress string
    FindingGRPC string
}

func Load() *Config {
    return &Config{
        HTTPPort:    envInt("SLA_HTTP_PORT", 8086),
        GRPCPort:    envInt("SLA_GRPC_PORT", 9006),
        DatabaseURL: os.Getenv("SLA_DATABASE_URL"),
        NATSAddress: os.Getenv("NATS_ADDRESS"),
        FindingGRPC: os.Getenv("FINDING_GRPC_ADDR"),
    }
}
```

### `internal/domain/config/entity.go`

```go
package config

import "time"

// SLAConfiguration defines days-to-remediate per severity
type SLAConfiguration struct {
    ID          string
    Name        string
    Description string
    Critical    int  // days to fix Critical severity findings
    High        int  // days to fix High severity findings
    Medium      int  // days to fix Medium severity findings
    Low         int  // days to fix Low severity findings
    
    IsDefault   bool  // product gets this config if none explicitly assigned
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

// SLAProductAssignment links a product to an SLA config
type SLAProductAssignment struct {
    ProductID         string
    SLAConfigurationID string
    AssignedAt        time.Time
    AssignedBy        string
}
```

### `migrations/001_sla_configurations.sql`

```sql
CREATE TABLE IF NOT EXISTS sla_configurations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(200) NOT NULL,
    description TEXT,
    critical_days INTEGER NOT NULL DEFAULT 7,
    high_days INTEGER NOT NULL DEFAULT 30,
    medium_days INTEGER NOT NULL DEFAULT 90,
    low_days INTEGER NOT NULL DEFAULT 365,
    is_default BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_default UNIQUE (is_default)
        DEFERRABLE INITIALLY DEFERRED  -- only one default at a time
);

CREATE TABLE IF NOT EXISTS sla_product_assignments (
    product_id UUID PRIMARY KEY,
    sla_configuration_id UUID NOT NULL REFERENCES sla_configurations(id),
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    assigned_by UUID
);

-- Seed default SLA configuration (CVSS baseline)
INSERT INTO sla_configurations (id, name, description, critical_days, high_days, medium_days, low_days, is_default)
VALUES (
    gen_random_uuid(),
    'Default CVSS-based SLA',
    'Based on CVSS severity levels — industry standard remediation timeframes',
    7, 30, 90, 365, TRUE
) ON CONFLICT DO NOTHING;
```

### `migrations/002_sla_computation_log.sql`

```sql
CREATE TABLE IF NOT EXISTS sla_computation_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    finding_id UUID NOT NULL,
    product_id UUID NOT NULL,
    sla_configuration_id UUID NOT NULL,
    severity VARCHAR(20) NOT NULL,
    sla_days INTEGER NOT NULL,
    found_date DATE NOT NULL,
    computed_expiry DATE NOT NULL,
    previous_expiry DATE,
    computed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_sla_log_finding ON sla_computation_log(finding_id, computed_at DESC);
```

### `cmd/server/main.go`

```go
package main

import (
    "context"
    "log/slog"
    "os"
    "os/signal"
    "syscall"
    "golang.org/x/sync/errgroup"

    "github.com/osv/services/sla-service/internal/config"
)

func main() {
    cfg := config.Load()
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer cancel()

    // Wire dependencies
    db := mustConnectDB(cfg.DatabaseURL)
    nc := mustConnectNATS(cfg.NATSAddress)

    // Wire use cases, repos, handlers
    // ...

    g, gCtx := errgroup.WithContext(ctx)
    g.Go(func() error { return startHTTPServer(gCtx, cfg.HTTPPort) })
    g.Go(func() error { return startGRPCServer(gCtx, cfg.GRPCPort) })
    g.Go(func() error { return startEventSubscribers(gCtx, nc) })
    g.Go(func() error { return startScheduler(gCtx) })

    if err := g.Wait(); err != nil {
        slog.Error("sla-service error", "error", err)
        os.Exit(1)
    }
}
```

### `Dockerfile`

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o sla-service ./cmd/server

FROM gcr.io/distroless/static
WORKDIR /
COPY --from=builder /app/sla-service .
EXPOSE 8086 9006
ENTRYPOINT ["/sla-service"]
```

## Acceptance Criteria

- [x] `go build ./...` thành công
- [x] `go mod tidy` không có unused deps
- [x] `docker build -t sla-service:test .` thành công
- [x] Service starts và lắng nghe trên port 8086 (HTTP) và 9006 (gRPC)
- [x] Migrations chạy thành công trên empty PostgreSQL database
- [x] Default SLA configuration seeded sau migration
- [x] Health endpoint `GET /health` → 200
- [x] Service graceful shutdown khi nhận SIGTERM

## Implementation Status: ✅ DONE

> `services/sla-service/` — full service scaffold
> `go.mod` — module github.com/osv/sla-service, go 1.22
> `Dockerfile` — multi-stage build, distroless base, expose 8086/9006
> `cmd/server/main.go` — errgroup: HTTP + gRPC + NATS subscribers + scheduler
> `migrations/001_sla_configurations.sql` — tables + default SLA seed
> `migrations/002_sla_computation_log.sql` — computation log table
> `internal/config/config.go` — SLA_HTTP_PORT, SLA_GRPC_PORT, SLA_DATABASE_URL, NATS_ADDRESS
