# BUG-006 — Hardcoded Service Version "1.0.0" Across Multiple Services

## Metadata
- **ID**: BUG-006
- **Services**: `gateway-service`, `notification-service`, `ai-service` (embed.go)
- **Severity**: Low
- **Category**: Hardcode / Versioning
- **Status**: Open

## Mô tả

Version string `"1.0.0"` được hardcode trực tiếp trong `main()` của nhiều services:

```go
// gateway-service/cmd/server/main.go
log := observability.InitLogger("gateway-service", "1.0.0")
shutdown, err := observability.InitTracer(ctx, "gateway-service", "1.0.0")
healthUseCase := health.NewAggregateUseCase(upstreams, "1.0.0")

// gateway-service/internal/health/info_handler.go:40
Version:   "1.0.0",

// notification-service/cmd/server/main.go
log := observability.InitLogger("notification-service", "1.0.0")
shutdown, err := observability.InitTracer(ctx, "notification-service", "1.0.0")

// product-service/internal/delivery/http/handlers.go:237
if req.Version == "" { req.Version = "1.0.0" }
```

## Tác động

1. Logs và traces trong Datadog/GCP sẽ luôn hiển thị version `1.0.0` — không thể
   phân biệt deployment versions khi debugging production issues.
2. Health endpoint `/info` trả về version sai khi deploy version mới.
3. Product version mặc định `"1.0.0"` có thể gây confused cho vulnerability matching.

## Fix Proposal

### Inject version qua build flags (Go ldflags)

```go
// Trong mỗi service main package:
var (
    Version   = "dev"     // overridden by -ldflags
    BuildTime = "unknown"
)

func main() {
    log := observability.InitLogger("gateway-service", Version)
    ...
}
```

```makefile
# Trong Makefile hoặc build script:
LDFLAGS=-ldflags "-X main.Version=$(git describe --tags --always) \
                  -X main.BuildTime=$(date -u +%Y%m%dT%H%M%SZ)"
go build $(LDFLAGS) ./cmd/server/
```

### Hoặc đọc từ env var

```go
version := os.Getenv("SERVICE_VERSION")
if version == "" {
    version = "dev"
}
```

## Files Affected

| File | Line | Hardcode |
|------|------|----------|
| [gateway-service/cmd/server/main.go](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/gateway-service/cmd/server/main.go) | 23, 31, 55 | `"1.0.0"` |
| [gateway-service/internal/health/info_handler.go](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/gateway-service/internal/health/info_handler.go) | 40 | `"1.0.0"` |
| [notification-service/cmd/server/main.go](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service/cmd/server/main.go) | 31, 40 | `"1.0.0"` |
| [product-service/internal/delivery/http/handlers.go](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/product-service/internal/delivery/http/handlers.go) | 237 | `"1.0.0"` |
