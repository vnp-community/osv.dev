# Shared Layer — Chi tiết

> Thư mục: `services/shared/`
> Là internal Go module được dùng chung bởi tất cả services.

---

## Cấu trúc

```
services/shared/
├── pkg/     # Utility packages (go.mod: github.com/osv/shared/pkg)
└── proto/   # gRPC proto definitions (go.mod: github.com/osv/shared/proto)
```

---

## shared/pkg

**Module**: `github.com/osv/shared/pkg`

Tất cả services đều declare:
```go
replace github.com/osv/shared/pkg => ../shared/pkg
```

### Packages

| Package | Chức năng |
|---------|-----------|
| `classification/` | Vulnerability classification (CVSS, severity) |
| `clients/` | Reusable HTTP/gRPC client utilities |
| `config/` | Config loading (Viper, env vars) |
| `cpe/` | CPE (Common Platform Enumeration) parsing & matching |
| `cveid/` | CVE ID validation & normalization |
| `cwe/` | CWE (Common Weakness Enumeration) lookup |
| `database/` | DB connection pool helpers (PostgreSQL, MongoDB) |
| `domain/` | Shared domain types (ID types, base entities) |
| `ecosystem/` | Package ecosystem types (npm, pypi, maven, go, etc.) |
| `errors/` | Shared error types & error wrapping |
| `grpcutil/` | gRPC interceptors, error mapping, metadata |
| `health/` | Health check interfaces & HTTP handler |
| `logger/` | Zerolog structured logger wrapper |
| `middleware/` | HTTP/gRPC middleware (auth, logging, tracing) |
| `models/` | Shared data models (DTOs, API models) |
| `nats/` | NATS JetStream client wrapper |
| `observability/` | OpenTelemetry tracing & metrics setup |
| `osvschema/` | OSV JSON schema types & validation |
| `osvutil/` | OSV utility functions |
| `pagination/` | Cursor-based & offset pagination |
| `pgp/` | PGP signature verification |
| `purl/` | Package URL (purl) parsing & generation |
| `resilience/` | Retry, circuit breaker, timeout patterns |
| `search/` | Shared search types (query, filter, sort) |
| `semver/` | Semantic versioning comparison |
| `severity/` | CVSS severity helpers (Critical/High/Medium/Low) |
| `test/` | Test fixtures & helpers |
| `testutil/` | Test utilities (mock builders, assertions) |
| `version/` | Version comparison & range matching |

### Key go.mod Dependencies
```
github.com/google/uuid
github.com/rs/zerolog
go.opentelemetry.io/otel/*
google.golang.org/grpc
google.golang.org/protobuf
github.com/jackc/pgx/v5
go.mongodb.org/mongo-driver
github.com/redis/go-redis/v9
github.com/nats-io/nats.go
github.com/ossf/osv-schema/bindings/go
```

---

## shared/proto

**Module**: `github.com/osv/shared/proto`

Config file: `buf.yaml` + `buf.gen.yaml` (Buf CLI cho proto generation)

### Proto Definitions

| Proto | Service | Chức năng |
|-------|---------|-----------|
| `auth/` | auth-service | Authentication RPC (login, validate token) |
| `identity/` | auth-service | Identity management RPC |
| `cve/` | vulnerability-service | CVE CRUD & query RPC |
| `cvedb/` | vulnerability-service | CVE database operations |
| `datasync/` | ingestion-service | Data sync status & control |
| `finding/` | finding-service | Finding CRUD & status RPC |
| `product/` | product-service | Product & engagement RPC |
| `asset/` | scan-service | Asset management RPC |
| `scan/` | scan-service | Scan job management RPC |
| `scanner/` | scan-service | Scanner agent communication RPC |
| `reporter/` | report-service | Report generation RPC |
| `sbomvex/` | scan-service | SBOM/VEX processing RPC |
| `gen/` | ALL | Generated Go code từ proto files |

### Buf Config
```yaml
# buf.gen.yaml
version: v2
plugins:
  - remote: buf.build/protocolbuffers/go
  - remote: buf.build/grpc/go
```

---

## Quy tắc sử dụng Shared

### 1. Sử dụng go.mod replace directive
Mọi service đều cần:
```go
replace (
    github.com/osv/shared/pkg   => ../shared/pkg
    github.com/osv/shared/proto => ../shared/proto
)
```

### 2. Import theo package path
```go
import (
    "github.com/osv/shared/pkg/logger"
    "github.com/osv/shared/pkg/errors"
    "github.com/osv/shared/pkg/pagination"
    
    pb "github.com/osv/shared/proto/gen/cve/v1"
)
```

### 3. Proto generation
```bash
cd services/shared/proto
buf generate
```

---

## Diagram: Service Dependencies on Shared

```
                    ┌─────────────────┐
                    │   shared/pkg    │
                    │   shared/proto  │
                    └────────┬────────┘
                             │ replaces (go.mod)
              ┌──────────────┼──────────────┐
              │              │              │
    ┌─────────▼──────┐ ┌─────▼──────┐ ┌───▼──────────┐
    │  auth-service  │ │ingestion-  │ │vulnerability-│
    │                │ │service     │ │service       │
    └────────────────┘ └────────────┘ └──────────────┘
              │              │              │
    ┌─────────▼──────┐ ┌─────▼──────┐ ┌───▼──────────┐
    │  ai-service    │ │scan-service│ │finding-      │
    │                │ │            │ │service       │
    └────────────────┘ └────────────┘ └──────────────┘
                    ... và tất cả services khác
```
