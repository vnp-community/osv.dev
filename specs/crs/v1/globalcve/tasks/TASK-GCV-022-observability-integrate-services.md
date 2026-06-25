# TASK-GCV-022 — Integrate Observability into All Services

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-GCV-022 |
| **Service** | `data-service`, `search-service`, `gateway-service`, `notification-service` |
| **CR** | CR-GCV-009 |
| **Phase** | 2 — Enrichment |
| **Priority** | 🟡 Medium |
| **Prerequisites** | TASK-GCV-021 |

## Context

Integrate `shared/pkg/observability` vào `main.go` và HTTP router của 4 services. Mỗi service start metrics server trên port riêng và use logging/metrics middleware.

## Reference

- Solution: [SOL-GCV-009](../solutions/SOL-GCV-009-observability.md) §2.6, §2.7

## Files to Create/Modify

```
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/data-service/cmd/server/main.go
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/cmd/server/main.go (tìm entry point)
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/gateway-service/cmd/server/main.go
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service/cmd/server/main.go
```

## Implementation Spec

Pattern cho mỗi service — thêm vào đầu `main()` hoặc vào `SetupServer()`:

```go
import "github.com/osv/shared/pkg/observability"

func main() {
    // Existing setup...

    // 1. Init logger (replace existing logger if any)
    log := observability.InitLogger("data-service", buildVersion)

    // 2. Start Prometheus metrics server
    metrics := observability.NewCommonMetrics("data-service")
    observability.StartMetricsServer(9092)  // non-blocking goroutine

    // 3. OTel tracing (optional, graceful if env not set)
    shutdown, err := observability.InitTracer(ctx, "data-service", buildVersion)
    if err != nil {
        log.Warn().Err(err).Msg("tracing init failed, continuing without tracing")
    }
    defer shutdown()

    // 4. Add middleware to HTTP router
    router.Use(observability.LoggingMiddleware(log))
    router.Use(observability.MetricsMiddleware(metrics))

    // Existing server start...
}
```

**Per-service metrics ports**:

| Service | Metrics Port |
|---------|-------------|
| `gateway-service` | 9090 |
| `search-service` | 9091 |
| `data-service` | 9092 |
| `notification-service` | 9094 |

**go.mod update**: Mỗi service phải add dependency vào `shared` module:
```
require github.com/osv/shared vX.X.X
```
(hoặc dùng `replace` directive nếu dùng local module)

## Acceptance Criteria

- [x] `data-service` log requests as JSON với fields: `service`, `method`, `path`, `status`, `latency_ms`
- [x] `search-service` log requests dạng JSON
- [x] `gateway-service` log requests dạng JSON
- [x] `notification-service` log requests dạng JSON
- [x] `GET http://localhost:9092/metrics` → Prometheus metrics cho data-service
- [x] `GET http://localhost:9091/metrics` → Prometheus metrics cho search-service
- [x] `GET http://localhost:9090/metrics` → Prometheus metrics cho gateway-service
- [x] `OTEL_EXPORTER_OTLP_ENDPOINT` không set → services start bình thường (no panic, no error)
- [x] Existing service functionality không bị ảnh hưởng
- [x] `go build ./...` pass tất cả services
