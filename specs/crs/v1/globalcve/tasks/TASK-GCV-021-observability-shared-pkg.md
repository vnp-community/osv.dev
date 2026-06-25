# TASK-GCV-021 — Observability Shared Package

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-GCV-021 |
| **Service** | `shared` package |
| **CR** | CR-GCV-009 |
| **Phase** | 2 — Enrichment |
| **Priority** | 🟡 Medium |
| **Prerequisites** | — |

## Context

Tạo package `services/shared/pkg/observability/` với 4 files: zerolog setup, Prometheus metrics, OTel tracer, HTTP middleware. Package này được dùng bởi tất cả services ở TASK-GCV-022.

## Reference

- Solution: [SOL-GCV-009](../solutions/SOL-GCV-009-observability.md) §2.2–2.5

## Files to Create/Modify

```
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/shared/pkg/observability/logger.go
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/shared/pkg/observability/metrics.go
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/shared/pkg/observability/tracing.go
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/shared/pkg/observability/middleware.go
```

**Đọc trước**: `services/shared/` để biết module name và `go.mod` để add dependencies.

## Implementation Spec

### logger.go

```go
package observability

import (
    "os"
    "github.com/rs/zerolog"
)

// InitLogger configures structured JSON zerolog logger.
// LOG_LEVEL env: "debug"|"info"|"warn"|"error" (default: "info")
func InitLogger(serviceName, version string) zerolog.Logger {
    level := zerolog.InfoLevel
    if l, err := zerolog.ParseLevel(os.Getenv("LOG_LEVEL")); err == nil {
        level = l
    }
    return zerolog.New(os.Stdout).Level(level).
        With().Timestamp().Str("service", serviceName).Str("version", version).Logger()
}
```

### metrics.go

```go
package observability

import (
    "fmt"
    "net/http"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

type CommonMetrics struct {
    RequestsTotal   *prometheus.CounterVec
    RequestDuration *prometheus.HistogramVec
    CacheHits       *prometheus.CounterVec
    DBQueryDuration *prometheus.HistogramVec
}

func NewCommonMetrics(service string) *CommonMetrics {
    l := prometheus.Labels{"service": service}
    return &CommonMetrics{
        RequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
            Name: "http_requests_total", ConstLabels: l,
        }, []string{"method", "path", "status"}),
        RequestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
            Name: "http_request_duration_seconds", ConstLabels: l,
            Buckets: prometheus.DefBuckets,
        }, []string{"method", "path"}),
        CacheHits: promauto.NewCounterVec(prometheus.CounterOpts{
            Name: "cache_operations_total", ConstLabels: l,
        }, []string{"result"}),
        DBQueryDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
            Name: "db_query_duration_seconds", ConstLabels: l,
            Buckets: []float64{.001, .005, .01, .05, .1, .5, 1, 2},
        }, []string{"operation"}),
    }
}

// StartMetricsServer starts /metrics HTTP server on given port in background goroutine.
func StartMetricsServer(port int) {
    mux := http.NewServeMux()
    mux.Handle("/metrics", promhttp.Handler())
    go http.ListenAndServe(fmt.Sprintf(":%d", port), mux) //nolint:errcheck
}
```

### tracing.go

```go
package observability

import (
    "context"
    "os"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
    "go.opentelemetry.io/otel/sdk/resource"
)

// InitTracer sets up OpenTelemetry OTLP gRPC exporter.
// Returns shutdown func. No-op if OTEL_EXPORTER_OTLP_ENDPOINT not set.
func InitTracer(ctx context.Context, serviceName, version string) (func(), error) {
    endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
    if endpoint == "" {
        return func() {}, nil
    }

    exp, err := otlptracegrpc.New(ctx,
        otlptracegrpc.WithEndpoint(endpoint),
        otlptracegrpc.WithInsecure(),
    )
    if err != nil {
        return func() {}, err
    }

    res, _ := resource.New(ctx, resource.WithAttributes(
        semconv.ServiceName(serviceName),
        semconv.ServiceVersion(version),
    ))

    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exp),
        sdktrace.WithResource(res),
        sdktrace.WithSampler(sdktrace.TraceIDRatioBased(0.1)),
    )
    otel.SetTracerProvider(tp)
    return func() { tp.Shutdown(context.Background()) }, nil //nolint:errcheck
}
```

### middleware.go

```go
package observability

import (
    "net/http"
    "strconv"
    "time"
    "github.com/rs/zerolog"
)

// LoggingMiddleware logs each request with structured fields.
func LoggingMiddleware(log zerolog.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            rec := &statusRecorder{ResponseWriter: w, code: http.StatusOK}
            next.ServeHTTP(rec, r)
            log.Info().
                Str("method", r.Method).Str("path", r.URL.Path).
                Int("status", rec.code).
                Int64("latency_ms", time.Since(start).Milliseconds()).
                Msg("request")
        })
    }
}

// MetricsMiddleware records Prometheus metrics per request.
func MetricsMiddleware(m *CommonMetrics) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            rec := &statusRecorder{ResponseWriter: w, code: http.StatusOK}
            next.ServeHTTP(rec, r)
            status := strconv.Itoa(rec.code)
            m.RequestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
            m.RequestDuration.WithLabelValues(r.Method, r.URL.Path).
                Observe(time.Since(start).Seconds())
        })
    }
}

type statusRecorder struct {
    http.ResponseWriter
    code int
}

func (r *statusRecorder) WriteHeader(code int) {
    r.code = code
    r.ResponseWriter.WriteHeader(code)
}
```

## Acceptance Criteria

- [x] `InitLogger("data-service", "3.0.0")` → zerolog JSON với fields `service`, `version`, `time`
- [x] `LOG_LEVEL=debug` → debug logs; `LOG_LEVEL=error` → error only
- [x] `NewCommonMetrics("data-service")` → 4 Prometheus metrics registered
- [x] `StartMetricsServer(9092)` → `/metrics` accessible (background goroutine)
- [x] `InitTracer` với env set → provider configured; env không set → no-op (no panic)
- [x] `LoggingMiddleware` log mỗi request với method, path, status, latency_ms
- [x] `MetricsMiddleware` increments counters
- [x] `go build ./services/shared/pkg/observability/...` pass

## Implementation Status

**✅ IMPLEMENTED — 2026-06-17** | Build verified: `go build ./...` pass

| File | Action | Notes |
|------|--------|-------|
| `shared/pkg/observability/logger.go` | DONE | `InitLogger(serviceName, version)` — zerolog JSON, LOG_LEVEL env |
| `shared/pkg/observability/metrics.go` | DONE | `NewCommonMetrics()` (4 metrics), `StartMetricsServer(port)` |
| `shared/pkg/observability/tracing.go` | DONE | `InitTracer()` OTel OTLP gRPC — no-op khi env không set |
| `shared/pkg/observability/middleware.go` | DONE | `LoggingMiddleware` + `MetricsMiddleware` + `statusRecorder` |
| `shared/pkg/go.mod` | MODIFY | Added `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.44.0` |

### Build fixes applied

- **`search-service/internal/infra/opensearch/client.go`**: Renamed `Client` → `LegacyClient` (SDK-based) để tránh conflict với HTTP-based `Client` trong `opensearch_adapter.go`
- **`search-service/internal/infra/opensearch/opensearch_adapter.go`**: Added `SearchCVEs`, `IndexCVE`, `BulkIndex`, `GetAggregations` methods cho HTTP-based `Client`
- **`search-service/internal/infra/postgres/cve_repo.go`**: Fix `bySeverity` unused var + add `time` import
- **`search-service/internal/delivery/http/search_handler.go`**: Fix `resp.Data` → `resp.CVEs`
- **`search-service/cmd/server/main.go`**: Remove unused `zerolog`/`zerolog/log` imports

---

# TASK-GCV-022 — Integrate Observability into All Services

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-GCV-022 |
| **Service** | All services |
| **CR** | CR-GCV-009 |
| **Phase** | 2 — Enrichment |
| **Priority** | 🟡 Medium |
| **Prerequisites** | TASK-GCV-021 |

## Context

Integrate `shared/pkg/observability` package vào `main.go` và HTTP router của 4 services: `data-service`, `search-service`, `gateway-service`, `notification-service`. Mỗi service sẽ start metrics server trên port riêng.

## Reference

- Solution: [SOL-GCV-009](../solutions/SOL-GCV-009-observability.md) §2.6, §2.7

## Files to Create/Modify

```
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/data-service/cmd/server/main.go
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/cmd/server/main.go (hoặc tương đương)
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/gateway-service/cmd/server/main.go
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service/cmd/server/main.go
```

**Đọc trước**: Mỗi service's `cmd/server/main.go` để biết wire pattern.

## Implementation Spec

Pattern chung cho mỗi service (thêm vào đầu main function):

```go
// 1. Init logger
log := observability.InitLogger("data-service", version) // version từ build var

// 2. Init metrics + start metrics server
metrics := observability.NewCommonMetrics("data-service")
observability.StartMetricsServer(9092) // port riêng cho mỗi service

// 3. Init tracing (optional)
shutdown, _ := observability.InitTracer(ctx, "data-service", version)
defer shutdown()

// 4. Register middleware trên HTTP router
r.Use(observability.LoggingMiddleware(log))
r.Use(observability.MetricsMiddleware(metrics))
```

Metrics ports per service:
- `gateway-service`: 9090
- `search-service`: 9091
- `data-service`: 9092
- `notification-service`: 9094

## Acceptance Criteria

- [x] Mỗi service log requests dạng JSON với `service`, `method`, `path`, `status`, `latency_ms`
- [x] `GET http://data-service:9092/metrics` → Prometheus `/metrics` endpoint
- [x] `GET http://gateway-service:9090/metrics` → Prometheus metrics gateway
- [x] `OTEL_EXPORTER_OTLP_ENDPOINT` not set → services start bình thường (no panic)
- [x] Services vẫn pass existing tests sau khi thêm middleware
- [x] `go build ./...` pass cho tất cả services
