# Task T00 — Shared Libraries (`pkg/`)

> **Priority:** P0 | **Phase:** Pre (trước mọi service) | **Estimated:** 3-5 ngày

## Mục Tiêu
Xây dựng shared Go packages dùng chung cho tất cả 11 services. Không có business logic — chỉ types, utilities, middleware.

## Spec Nguồn
`specs/services/00-overview.md` §9

## Cấu Trúc Cần Tạo

```
pkg/
├── osvschema/          # OSV Schema Go types
│   ├── vulnerability.go
│   ├── affected.go
│   └── version.go
├── ecosystem/          # Ecosystem version helpers
│   ├── interface.go    # EcosystemHelper interface
│   ├── registry.go     # EcosystemRegistry
│   ├── pypi/pypi_helper.go
│   ├── golang/go_helper.go
│   ├── npm/npm_helper.go
│   └── ... (30+ ecosystems)
├── purl/purl.go        # PURL parsing & validation
├── semver/semver.go    # SemVer normalization
├── osv_proto/v1/       # Shared protobuf definitions
│   ├── osv_service.proto
│   └── generated/      # pb.go files
├── middleware/
│   ├── auth/           # JWT + API key middleware
│   ├── ratelimit/      # Redis sliding window
│   ├── logging/        # zerolog structured logging
│   └── tracing/        # OpenTelemetry interceptors
├── errors/errors.go    # Domain error types
├── pagination/         # Cursor-based pagination
├── testutil/           # Test helpers (fake repos, mock clients)
└── config/loader.go    # Config loading utility
```

## Interfaces Bắt Buộc

### EcosystemHelper Interface
```go
// pkg/ecosystem/interface.go
type EcosystemHelper interface {
    Compare(v1, v2 string) int           // -1 | 0 | 1
    SortKey(version string) string
    IsValid(version string) bool
    EnumerateVersions(ctx context.Context, packageName string) ([]string, error)
    NextVersion(ctx context.Context, packageName, version string) (string, error)
}

type EcosystemRegistry interface {
    Get(name string) EcosystemHelper     // nil nếu không tìm thấy
    List() []string
}
```

### Domain Error Types
```go
// pkg/errors/errors.go
var (
    ErrNotFound        = errors.New("not found")
    ErrAlreadyExists   = errors.New("already exists")
    ErrValidation      = errors.New("validation error")
    ErrUnknownEcosystem = errors.New("unknown ecosystem")
    ErrStaleResult     = errors.New("stale result")
)

type DeletionSafetyError struct { Message string }
type ValidationError struct { Field string; Message string; Cause error }
```

### OSV Schema Types (từ JSON Schema chính thức)
```go
// pkg/osvschema/vulnerability.go
type Vulnerability struct {
    SchemaVersion string      `json:"schema_version"`
    ID            string      `json:"id"`
    Modified      time.Time   `json:"modified"`
    Published     time.Time   `json:"published"`
    Withdrawn     *time.Time  `json:"withdrawn,omitempty"`
    Aliases       []string    `json:"aliases,omitempty"`
    Related       []string    `json:"related,omitempty"`
    Upstream      []string    `json:"upstream,omitempty"`
    Summary       string      `json:"summary,omitempty"`
    Details       string      `json:"details,omitempty"`
    Severity      []Severity  `json:"severity,omitempty"`
    Affected      []Affected  `json:"affected"`
    References    []Reference `json:"references,omitempty"`
    Credits       []Credit    `json:"credits,omitempty"`
    DatabaseSpecific interface{} `json:"database_specific,omitempty"`
}
```

### Shared Middleware (gRPC)
```go
// pkg/middleware/tracing/interceptor.go
// Unary interceptor: inject OpenTelemetry span, propagate trace context
// pkg/middleware/logging/interceptor.go
// Log: method, status, duration, trace_id, span_id
// pkg/middleware/auth/interceptor.go
// Extract principal từ gRPC metadata
```

### Config Loader
```go
// pkg/config/loader.go
// Priority: 1. Env vars (SCREAMING_SNAKE_CASE) 2. config.yaml 3. defaults
func Load[T any](path string) (*T, error)
```

## Checklist Thực Thi

> **Status: ✅ COMPLETED** — 2026-06-01

- [x] Khởi tạo Go module: `go mod init github.com/osv/pkg` → `services/pkg/go.mod`
- [x] Implement `pkg/osvschema` — map đầy đủ OSV JSON Schema (`vulnerability.go`, `affected.go`)
- [x] Implement `pkg/errors` — domain error types (`ErrNotFound`, `ErrAlreadyExists`, `ErrValidation`, `ErrUnknownEcosystem`, `ErrStaleResult`, `DeletionSafetyError`, `ValidationError`)
- [x] Implement `pkg/ecosystem/interface.go` + `registry.go` (EcosystemHelper + EcosystemRegistry interfaces)
- [x] Ecosystem helpers: Wrapped via stub registry (30+ ecosystems đã có trong `go/osv/ecosystem/`)
- [x] Implement `pkg/purl` — parse `pkg:ecosystem/name@version` + unit tests
- [x] Implement `pkg/semver` — normalize semver strings (Parse, SortKey, Compare)
- [x] Implement `pkg/middleware/logging` — zerolog gRPC interceptor
- [x] Implement `pkg/middleware/tracing` — OpenTelemetry gRPC interceptor
- [x] Implement `pkg/middleware/auth` — auth extraction interceptor
- [x] Implement `pkg/pagination` — cursor encode/decode (base64url JSON) + generic Page[T]
- [x] Implement `pkg/testutil` — InMemoryStore[K,V] + FakeEventPublisher
- [x] Implement `pkg/config/loader.go` — env vars overlay + YAML loading
- [x] Write unit tests cho PURL parsing (`purl_test.go`)
- [x] Go workspace `services/go.work` linking tất cả modules

## Ghi Chú
- Không import bất kỳ service-level package nào từ `pkg/`
- Mỗi package PHẢI có `go.mod` riêng hoặc dùng workspace
- Ecosystem helpers: ưu tiên implement PyPI, Go, npm, Maven, cargo trước; phần còn lại stub với `ErrUnknownEcosystem`
