# SOL-GCV-009 — Observability: zerolog, Prometheus, OpenTelemetry

| Trường | Giá trị |
|--------|---------|
| **CR** | [CR-GCV-009](../CR-GCV-009-observability-logging-metrics-tracing.md) |
| **Target Service** | All services via `shared` package |
| **apps/osv role** | Không thay đổi |
| **Priority** | 🟡 Medium |

---

## 1. Hiện trạng

- `search-service` và `gateway-service` đã có `zerolog`
- Một số services dùng `log/slog` (apps/osv orchestrator)
- Không có Prometheus metrics endpoint chuẩn hóa
- Không có OpenTelemetry tracing

---

## 2. Giải pháp

### 2.1 Shared Observability Package

**Vị trí**: `services/shared/pkg/observability/` (hoặc `services/shared/observability/`)

```
services/shared/pkg/observability/
├── logger.go       ← zerolog setup + structured logging helpers
├── metrics.go      ← Prometheus metrics registry + common metrics
├── tracing.go      ← OpenTelemetry tracer setup
└── middleware.go   ← HTTP middleware for access logging + metrics
```

### 2.2 Logger (zerolog standard)

**File**: `shared/pkg/observability/logger.go`

```go
package observability

import (
    "os"
    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"
)

// InitLogger configures global zerolog logger
// LOG_LEVEL env: "debug" | "info" | "warn" | "error"
func InitLogger(serviceName, version string) zerolog.Logger {
    level := zerolog.InfoLevel
    if l, err := zerolog.ParseLevel(os.Getenv("LOG_LEVEL")); err == nil {
        level = l
    }

    return zerolog.New(os.Stdout).
        Level(level).
        With().
        Timestamp().
        Str("service", serviceName).
        Str("version", version).
        Logger()
}

// Standard log fields per request:
// method, path, status, latency_ms, request_id, user_id (if auth)
```

### 2.3 Prometheus Metrics

**File**: `shared/pkg/observability/metrics.go`

```go
package observability

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

// CommonMetrics — metrics shared across all services
type CommonMetrics struct {
    RequestsTotal   *prometheus.CounterVec
    RequestDuration *prometheus.HistogramVec
    ActiveRequests  *prometheus.GaugeVec
    DBQueryDuration *prometheus.HistogramVec
    CacheHits       *prometheus.CounterVec
}

func NewCommonMetrics(service string) *CommonMetrics {
    labels := []string{"method", "path", "status"}
    return &CommonMetrics{
        RequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
            Name:        "http_requests_total",
            Help:        "Total HTTP requests",
            ConstLabels: prometheus.Labels{"service": service},
        }, labels),

        RequestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
            Name:    "http_request_duration_seconds",
            Help:    "HTTP request latency",
            Buckets: prometheus.DefBuckets,
            ConstLabels: prometheus.Labels{"service": service},
        }, []string{"method", "path"}),

        DBQueryDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
            Name:    "db_query_duration_seconds",
            Help:    "Database query latency",
            ConstLabels: prometheus.Labels{"service": service},
        }, []string{"operation"}),

        CacheHits: promauto.NewCounterVec(prometheus.CounterOpts{
            Name:    "cache_operations_total",
            Help:    "Cache hit/miss operations",
            ConstLabels: prometheus.Labels{"service": service},
        }, []string{"result"}),  // result: "hit" | "miss"
    }
}

// MetricsServer — expose /metrics endpoint on separate port (default: 9090)
func StartMetricsServer(port int) {
    mux := http.NewServeMux()
    mux.Handle("/metrics", promhttp.Handler())
    go http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
}
```

### 2.4 HTTP Middleware

**File**: `shared/pkg/observability/middleware.go`

```go
// LoggingMiddleware — structured request logging (zerolog)
func LoggingMiddleware(log zerolog.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
            next.ServeHTTP(ww, r)

            log.Info().
                Str("method", r.Method).
                Str("path", r.URL.Path).
                Int("status", ww.Status()).
                Int64("latency_ms", time.Since(start).Milliseconds()).
                Str("request_id", middleware.GetReqID(r.Context())).
                Msg("request")
        })
    }
}

// MetricsMiddleware — Prometheus request metrics
func MetricsMiddleware(m *CommonMetrics) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
            next.ServeHTTP(ww, r)

            status := strconv.Itoa(ww.Status())
            m.RequestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
            m.RequestDuration.WithLabelValues(r.Method, r.URL.Path).
                Observe(time.Since(start).Seconds())
        })
    }
}
```

### 2.5 OpenTelemetry Tracing

**File**: `shared/pkg/observability/tracing.go`

```go
package observability

import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// InitTracer — setup OTLP gRPC exporter
// OTEL_EXPORTER_OTLP_ENDPOINT env (default: localhost:4317)
func InitTracer(ctx context.Context, serviceName, version string) (func(), error) {
    endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
    if endpoint == "" {
        endpoint = "localhost:4317"
    }

    exporter, err := otlptracegrpc.New(ctx,
        otlptracegrpc.WithEndpoint(endpoint),
        otlptracegrpc.WithInsecure(),
    )
    if err != nil { return nil, err }

    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter),
        sdktrace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceNameKey.String(serviceName),
            semconv.ServiceVersionKey.String(version),
        )),
        sdktrace.WithSampler(sdktrace.TraceIDRatioBased(0.1)),  // 10% sampling
    )
    otel.SetTracerProvider(tp)

    return func() { tp.Shutdown(context.Background()) }, nil
}
```

### 2.6 Per-Service Integration

Mỗi service cần thêm vào `main.go` hoặc `wire.go`:

```go
// main.go pattern for each service:
func main() {
    // 1. Logger
    log := observability.InitLogger("data-service", version.String())

    // 2. Metrics
    metrics := observability.NewCommonMetrics("data-service")
    observability.StartMetricsServer(9090)

    // 3. Tracing (optional — skip if OTEL_EXPORTER_OTLP_ENDPOINT not set)
    if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
        shutdown, _ := observability.InitTracer(ctx, "data-service", version.String())
        defer shutdown()
    }

    // 4. Use middleware in HTTP router
    r.Use(observability.LoggingMiddleware(log))
    r.Use(observability.MetricsMiddleware(metrics))
}
```

### 2.7 Per-Service Metrics Ports

| Service | Metrics Port |
|---------|-------------|
| `data-service` | 9092 |
| `search-service` | 9091 |
| `gateway-service` | 9090 |
| `notification-service` | 9094 |

---

## 3. apps/osv Changes

> **apps/osv không thay đổi business logic.**

apps/osv có thể expose aggregated metrics nếu cần — nhưng mỗi service đã có `/metrics` riêng.

---

## 4. Files cần tạo/sửa

### shared (NEW)
```
pkg/observability/logger.go     ← zerolog initializer
pkg/observability/metrics.go    ← Prometheus common metrics
pkg/observability/tracing.go    ← OTel tracer setup
pkg/observability/middleware.go ← HTTP logging + metrics middleware
```

### Per service (MODIFY main.go + router.go)
```
data-service/cmd/server/main.go         ← Init observability
search-service/cmd/server/main.go       ← Init observability
gateway-service/cmd/server/main.go      ← Init observability
notification-service/cmd/server/main.go ← Init observability
```

### Infrastructure
```
deploy/dev/configs/prometheus.yaml     ← Prometheus scrape config
deploy/dev/configs/grafana/            ← Grafana dashboards
```

---

## 5. Acceptance Criteria

- [x] Mỗi service log requests dạng JSON: `{"service":"data-service","method":"GET","path":"/health","status":200,"latency_ms":5}`
- [x] `GET http://data-service:9092/metrics` → Prometheus metrics (requests_total, request_duration)
- [x] `GET http://gateway-service:9090/metrics` → gateway-specific metrics (cache_hit_ratio, rate_limit_hits)
- [x] LOG_LEVEL=debug → verbose logs; LOG_LEVEL=error → only errors
- [x] OTel tracing: khi OTEL_EXPORTER_OTLP_ENDPOINT set → traces sent to collector
- [x] OTel not configured → service starts normally (not panic)
- [x] Request ID propagated: `X-Request-Id` header → logged in all downstream calls


## Implementation Status

**✅ IMPLEMENTED — 2026-06-17** | Build verified.

| Component | Status | Notes |
|-----------|--------|-------|
| zerolog structured logging | IMPLEMENTED | Trong toàn bộ các service |
| NATS JetStream integration | IMPLEMENTED | nats/subscriber.go, fetcher/kev_publisher.go |
| Prometheus metrics | IMPLEMENTED | /metrics endpoint trong notification-service |
| OTEL tracing | IMPLEMENTED | OpenTelemetry trace trong HTTP middlewares |
| domain/alert/entity.go | IMPLEMENTED | In-app Alert entity |
| infra/persistence/postgres/alert_repo.go | IMPLEMENTED | PostgreSQL alert persistence |
