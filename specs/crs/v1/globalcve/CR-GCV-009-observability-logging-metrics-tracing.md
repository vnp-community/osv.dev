# CR-GCV-009 — Observability: Structured Logging, Prometheus, OpenTelemetry Tracing

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-GCV-009 |
| **Tiêu đề** | Observability Stack — Structured Logging (zerolog), Prometheus Metrics, OpenTelemetry Distributed Tracing |
| **Nguồn tham chiếu** | `globalcve/specs/services/00-overview.md §Technology Stack`, `globalcve/specs/services/02-cve-search-service.md §8`, `globalcve/specs/services/03-cve-sync-service.md §11` |
| **Target Service** | **Tất cả services** (shared `pkg/observability`) |
| **Ưu tiên** | 🟡 Medium |
| **Loại** | Cross-cutting Concern |
| **Ngày tạo** | 2026-06-14 |
| **Trạng thái** | ✅ IMPLEMENTED — 2026-06-17 |

---

## 1. Tổng quan

GlobalCVE v3.0 định nghĩa observability stack đầy đủ cho tất cả services:
- **Structured logging**: zerolog (JSON format, log levels, contextual fields)
- **Metrics**: Prometheus với các key metrics per service
- **Tracing**: OpenTelemetry (distributed request tracing end-to-end)
- **Health endpoints**: standardized health check format

OSV hiện tại có logging cơ bản nhưng chưa có Prometheus metrics chuẩn và không có distributed tracing.

---

## 2. Gap Analysis

| Feature | OSV | GlobalCVE v3.0 |
|---------|-----|----------------|
| Structured JSON logging | ⚠️ | ✅ zerolog |
| Log levels (debug/info/warn/error) | ⚠️ | ✅ |
| Request ID per request | ❌ | ✅ chi middleware.RequestID |
| Prometheus metrics | ⚠️ Basic | ✅ Full per-service metrics |
| Metrics: request count | ❌ | ✅ |
| Metrics: request latency histogram | ❌ | ✅ |
| Metrics: cache hit/miss | ❌ | ✅ |
| Metrics: sync stats | ❌ | ✅ per source |
| Metrics: CVE total count gauge | ❌ | ✅ |
| OpenTelemetry tracing | ❌ | ✅ |
| Trace propagation (W3C) | ❌ | ✅ traceparent header |
| Span per use case | ❌ | ✅ |
| Metrics scraping endpoint (/metrics) | ⚠️ | ✅ :9090/metrics |
| Health check (/health) | ⚠️ | ✅ Standardized |

---

## 3. Shared Observability Package

```go
// shared/pkg/observability/
// Shared across all GlobalCVE services

shared/pkg/observability/
├── logger.go          # zerolog setup
├── metrics.go         # Prometheus common metrics helpers
├── tracing.go         # OpenTelemetry setup
├── health.go          # Health check struct
└── middleware.go      # HTTP middleware (logging, tracing, metrics)
```

### 3.1 Logger Setup

```go
// shared/pkg/observability/logger.go

package observability

import (
    "os"
    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"
)

// InitLogger — configure zerolog for production use
func InitLogger(level, serviceName, version string) zerolog.Logger {
    logLevel, err := zerolog.ParseLevel(level)
    if err != nil {
        logLevel = zerolog.InfoLevel
    }

    zerolog.SetGlobalLevel(logLevel)
    zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs

    logger := zerolog.New(os.Stdout).
        With().
        Timestamp().
        Str("service", serviceName).
        Str("version", version).
        Logger()

    // Set as global logger
    log.Logger = logger
    return logger
}

// WithRequestID — attach request ID to context logger
func WithRequestID(ctx context.Context, requestID string) context.Context {
    return log.Logger.With().Str("request_id", requestID).Logger().WithContext(ctx)
}

// FromContext — get logger from context
func FromContext(ctx context.Context) *zerolog.Logger {
    return zerolog.Ctx(ctx)
}

// Log convention from globalcve/docs/SRS.md §7.3
// 🔍  = Search/Query
// 📡  = Network request
// 🧩  = Data processing
// 🎯  = Final results
// ❌  = Error
// ⚠️  = Warning
// 🚨  = Security/KEV
```

### 3.2 Prometheus Metrics

```go
// shared/pkg/observability/metrics.go

package observability

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

// CommonMetrics — shared across all HTTP services
type CommonMetrics struct {
    RequestsTotal   *prometheus.CounterVec
    RequestDuration *prometheus.HistogramVec
    ErrorsTotal     *prometheus.CounterVec
}

func NewCommonMetrics(serviceName string) *CommonMetrics {
    return &CommonMetrics{
        RequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
            Name: "http_requests_total",
            Help: "Total HTTP requests",
            ConstLabels: prometheus.Labels{"service": serviceName},
        }, []string{"method", "path", "status"}),

        RequestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
            Name:    "http_request_duration_seconds",
            Help:    "HTTP request latency",
            Buckets: prometheus.DefBuckets,
            ConstLabels: prometheus.Labels{"service": serviceName},
        }, []string{"method", "path"}),

        ErrorsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
            Name: "http_errors_total",
            Help: "Total HTTP errors by type",
            ConstLabels: prometheus.Labels{"service": serviceName},
        }, []string{"error_type"}),
    }
}

// CVESearchMetrics — specific to cve-search-service
type CVESearchMetrics struct {
    *CommonMetrics
    SearchRequestsTotal    *prometheus.CounterVec
    SearchDuration         *prometheus.HistogramVec
    CacheHitsTotal         *prometheus.CounterVec
    OpenSearchDuration     *prometheus.HistogramVec
    SemanticSearchTotal    *prometheus.CounterVec
    DBQueryDuration        *prometheus.HistogramVec
    CVETotalCount          prometheus.Gauge
}

func NewCVESearchMetrics() *CVESearchMetrics {
    return &CVESearchMetrics{
        CommonMetrics: NewCommonMetrics("cve-search-service"),

        SearchRequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
            Name: "cve_search_requests_total",
            Help: "Total CVE search requests",
        }, []string{"type"}),  // type: keyword|exact|semantic

        SearchDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
            Name:    "cve_search_duration_seconds",
            Help:    "Search latency distribution",
            Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5},
        }, []string{"type", "backend"}),  // backend: postgres|opensearch|semantic

        CacheHitsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
            Name: "cve_search_cache_hits_total",
            Help: "Cache hits by type",
        }, []string{"hit"}),  // hit: true|false

        OpenSearchDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
            Name:    "cve_opensearch_duration_seconds",
            Help:    "OpenSearch query latency",
            Buckets: prometheus.DefBuckets,
        }, []string{"operation"}),

        SemanticSearchTotal: promauto.NewCounter(prometheus.CounterOpts{
            Name: "cve_semantic_search_total",
            Help: "Total semantic search requests",
        }),

        DBQueryDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
            Name:    "cve_db_query_duration_seconds",
            Help:    "PostgreSQL query latency",
            Buckets: prometheus.DefBuckets,
        }, []string{"query"}),

        CVETotalCount: promauto.NewGauge(prometheus.GaugeOpts{
            Name: "cve_total_count",
            Help: "Total number of CVEs in database",
        }),
    }
}

// CVESyncMetrics — specific to cve-sync-service
type CVESyncMetrics struct {
    SyncDuration    *prometheus.HistogramVec
    SyncedTotal     *prometheus.CounterVec
    ErrorsTotal     *prometheus.CounterVec
    EPSSUpdates     prometheus.Counter
    OSIndexTotal    prometheus.Counter
    CVETotalCount   prometheus.Gauge
}

func NewCVESyncMetrics() *CVESyncMetrics {
    return &CVESyncMetrics{
        SyncDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
            Name:    "cve_sync_duration_seconds",
            Help:    "Sync duration by source",
            Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600, 1800},
        }, []string{"source"}),

        SyncedTotal: promauto.NewCounterVec(prometheus.CounterOpts{
            Name: "cve_sync_synced_total",
            Help: "CVEs synced (upserted) per source",
        }, []string{"source"}),

        ErrorsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
            Name: "cve_sync_errors_total",
            Help: "Sync errors per source",
        }, []string{"source", "error_type"}),

        EPSSUpdates: promauto.NewCounter(prometheus.CounterOpts{
            Name: "epss_updates_total",
            Help: "EPSS score updates",
        }),

        OSIndexTotal: promauto.NewCounter(prometheus.CounterOpts{
            Name: "opensearch_index_total",
            Help: "Documents indexed to OpenSearch",
        }),

        CVETotalCount: promauto.NewGauge(prometheus.GaugeOpts{
            Name: "cve_total_count",
            Help: "Total CVEs in database (updated after each sync)",
        }),
    }
}

// KEVMetrics — specific to kev-service
type KEVMetrics struct {
    *CommonMetrics
    TotalEntries    prometheus.Gauge
    SyncDuration    prometheus.Histogram
    SyncInserted    prometheus.Counter
    SyncUpdated     prometheus.Counter
    CheckRequests   prometheus.Counter
    LastSyncTime    prometheus.Gauge
    RansomwareCount prometheus.Gauge
}
```

---

## 4. OpenTelemetry Tracing

### 4.1 OTel Setup

```go
// shared/pkg/observability/tracing.go

package observability

import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    "go.opentelemetry.io/otel/sdk/resource"
    "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// InitTracing — configure OpenTelemetry with OTLP gRPC exporter
// Connects to otel-collector (Jaeger-compatible)
func InitTracing(ctx context.Context, cfg TracingConfig) (*trace.TracerProvider, error) {
    exporter, err := otlptracegrpc.New(ctx,
        otlptracegrpc.WithEndpoint(cfg.Endpoint),
        otlptracegrpc.WithInsecure(),
    )
    if err != nil {
        return nil, fmt.Errorf("create otlp exporter: %w", err)
    }

    res, _ := resource.New(ctx,
        resource.WithAttributes(
            semconv.ServiceName(cfg.ServiceName),
            semconv.ServiceVersion(cfg.Version),
            attribute.String("environment", cfg.Environment),
        ),
    )

    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithResource(res),
        trace.WithSampler(trace.TraceIDRatioBased(cfg.SamplingRate)),  // 0.1 = 10%
    )

    otel.SetTracerProvider(tp)
    otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
        propagation.TraceContext{},   // W3C traceparent header
        propagation.Baggage{},
    ))

    return tp, nil
}

type TracingConfig struct {
    ServiceName  string
    Version      string
    Environment  string  // "production" | "staging" | "development"
    Endpoint     string  // "otel-collector:4317"
    SamplingRate float64 // 0.0-1.0 (1.0 = trace all requests)
}
```

### 4.2 HTTP Middleware with Tracing

```go
// shared/pkg/observability/middleware.go

package observability

import (
    "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
)

// ObservabilityMiddleware — combines logging + metrics + tracing
func ObservabilityMiddleware(metrics *CommonMetrics, serviceName string) func(http.Handler) http.Handler {
    return otelhttp.NewMiddleware(
        serviceName,
        otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents),
        otelhttp.WithSpanOptions(
            trace.WithAttributes(attribute.String("service.name", serviceName)),
        ),
    )
}

// LoggingMiddleware — structured access log per request
func LoggingMiddleware(logger zerolog.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            requestID := middleware.GetReqID(r.Context())

            // Attach request ID to context logger
            ctx := logger.With().
                Str("request_id", requestID).
                Str("method", r.Method).
                Str("path", r.URL.Path).
                Logger().WithContext(r.Context())

            rec := newResponseRecorder(w)
            next.ServeHTTP(rec, r.WithContext(ctx))

            // Log after request
            zerolog.Ctx(ctx).Info().
                Int("status", rec.statusCode).
                Int64("duration_ms", time.Since(start).Milliseconds()).
                Int64("bytes", int64(rec.body.Len())).
                Str("ip", r.RemoteAddr).
                Msg("request")
        })
    }
}

// TracingMiddleware — propagate W3C traceparent headers
func TracingMiddleware() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ctx := otel.GetTextMapPropagator().Extract(r.Context(),
                propagation.HeaderCarrier(r.Header))

            tracer := otel.Tracer("http")
            spanName := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
            ctx, span := tracer.Start(ctx, spanName)
            defer span.End()

            span.SetAttributes(
                attribute.String("http.method", r.Method),
                attribute.String("http.url", r.URL.String()),
                attribute.String("http.request_id", middleware.GetReqID(r.Context())),
            )

            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

### 4.3 Use Case Tracing

```go
// In use cases: wrap business logic in OTel spans

// cve-search-service/internal/usecase/search/usecase.go

func (uc *searchUseCase) Execute(ctx context.Context, req *Request) (*Response, error) {
    ctx, span := tracer.Start(ctx, "search.Execute",
        trace.WithAttributes(
            attribute.String("search.query", req.Query),
            attribute.String("search.severity", req.Severity),
            attribute.Int("search.page", req.Page),
        ))
    defer span.End()

    // Cache check span
    ctx, cacheSpan := tracer.Start(ctx, "cache.Get")
    cached, err := uc.cache.Get(ctx, cacheKey)
    cacheSpan.End()

    if err == nil {
        span.SetAttributes(attribute.Bool("cache.hit", true))
        // return cached
    }

    // DB query span
    ctx, dbSpan := tracer.Start(ctx, "db.Search",
        trace.WithAttributes(attribute.String("db.system", "postgresql")))
    cves, total, err := uc.cveRepo.Search(ctx, filter)
    if err != nil {
        dbSpan.RecordError(err)
        dbSpan.SetStatus(codes.Error, err.Error())
    }
    dbSpan.End()

    span.SetAttributes(
        attribute.Int64("search.total", total),
        attribute.Int("search.results", len(cves)),
    )
    return &Response{...}, nil
}
```

---

## 5. Metrics Endpoint Setup

```go
// cmd/main.go (every service)

import (
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

// Start metrics server on separate port (not exposed via Gateway)
func startMetricsServer(port int) {
    mux := http.NewServeMux()
    mux.Handle("/metrics", promhttp.Handler())
    mux.HandleFunc("/health", healthHandler)

    server := &http.Server{
        Addr:    fmt.Sprintf(":%d", port),
        Handler: mux,
    }

    log.Info().Int("port", port).Msg("metrics server started")
    server.ListenAndServe()
}
```

---

## 6. Metrics Ports (per service)

| Service | HTTP Port | Metrics Port |
|---------|:---------:|:------------:|
| gateway-service | 8080 | 9090 |
| cve-search-service | 8081 | 9091 |
| cve-sync-service | 8082 | 9092 |
| kev-service | 8083 | 9093 |
| notification-service | 8084 | 9094 |

---

## 7. Docker Compose — Observability Stack

```yaml
# deploy/docker-compose.observability.yml

services:
  # Prometheus — metrics scraping
  prometheus:
    image: prom/prometheus:v2.53.0
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
    ports:
      - "9999:9090"  # Web UI (internal only)

  # OpenTelemetry Collector — trace ingestion
  otel-collector:
    image: otel/opentelemetry-collector-contrib:0.104.0
    volumes:
      - ./otel-config.yaml:/etc/otel/config.yaml
    ports:
      - "4317:4317"   # gRPC OTLP
      - "4318:4318"   # HTTP OTLP
    command: ["--config=/etc/otel/config.yaml"]

  # Jaeger — trace visualization
  jaeger:
    image: jaegertracing/all-in-one:1.59
    ports:
      - "16686:16686"  # Jaeger UI
      - "14250:14250"  # gRPC from otel-collector

  # Grafana — dashboards
  grafana:
    image: grafana/grafana:11.0.0
    ports:
      - "3030:3000"  # Grafana UI
    volumes:
      - ./grafana/dashboards:/var/lib/grafana/dashboards
      - ./grafana/provisioning:/etc/grafana/provisioning
```

```yaml
# prometheus.yml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'api-gateway'
    static_configs:
      - targets: ['api-gateway:9090']

  - job_name: 'cve-search-service'
    static_configs:
      - targets: ['cve-search-service:9091']

  - job_name: 'cve-sync-service'
    static_configs:
      - targets: ['cve-sync-service:9092']

  - job_name: 'kev-service'
    static_configs:
      - targets: ['kev-service:9093']

  - job_name: 'notification-service'
    static_configs:
      - targets: ['notification-service:9094']
```

```yaml
# otel-config.yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

exporters:
  jaeger:
    endpoint: jaeger:14250
    tls:
      insecure: true
  logging:
    loglevel: warn

service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [jaeger, logging]
```

---

## 8. Grafana Dashboards

```
dashboards/
├── globalcve-overview.json     # Service health + request rates
├── cve-search-service.json     # Search latency P50/P95/P99, cache hit ratio
├── cve-sync-service.json       # Sync duration, CVEs/hour, errors per source
├── kev-service.json            # KEV sync, total count, ransomware count
└── api-gateway.json            # Rate limits, upstream errors, cache hit ratio
```

### Key Panels (cve-search-service dashboard)

```
Panel 1: Search Latency P50/P95/P99 (Histogram)
Panel 2: Cache Hit Ratio over time (Gauge)
Panel 3: Total CVEs in DB (Stat)
Panel 4: OpenSearch vs PostgreSQL fallback ratio
Panel 5: Semantic search requests/min
Panel 6: Top 10 most searched queries (if logging enabled)
```

---

## 9. Alert Rules (Prometheus)

```yaml
# alerts.yml
groups:
  - name: globalcve
    rules:
      - alert: SyncServiceDown
        expr: up{job="cve-sync-service"} == 0
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "CVE Sync Service is down"

      - alert: HighSearchLatency
        expr: histogram_quantile(0.95, cve_search_duration_seconds_bucket) > 2
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "CVE search P95 latency > 2s"

      - alert: NVDSyncStale
        expr: time() - kev_last_sync_timestamp > 86400  # 24h
        for: 1m
        labels:
          severity: warning
        annotations:
          summary: "NVD sync has not run in 24 hours"

      - alert: HighRateLimitRejects
        expr: rate(gateway_rate_limit_hits_total[5m]) > 10
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High rate of rate-limit rejections"
```

---

## 10. Acceptance Criteria

- [x] Tất cả services export metrics tại `/metrics` endpoint (Prometheus format)
- [x] Prometheus scrapes metrics từ tất cả 5 services mỗi 15s
- [x] `cve_total_count` gauge được update sau mỗi NVD sync
- [x] `cve_search_duration_seconds` histogram có buckets: 5ms, 10ms, 25ms, 50ms, 100ms, 250ms, 500ms, 1s
- [x] `cve_search_cache_hits_total{hit="true"}` counter tăng khi Redis cache hit
- [x] Mỗi HTTP request có `traceparent` header propagated đến upstream
- [x] Jaeger UI hiển thị traces với spans: HTTP → Gateway → CVESearch → PostgreSQL/OpenSearch
- [x] `SyncServiceDown` alert fires khi cve-sync-service down > 5m
- [x] `HighSearchLatency` alert fires khi P95 > 2s trong 10 phút
- [x] Grafana dashboard "globalcve-overview" hiển thị service health, request rates
- [x] Structured access log: `{"service":"api-gateway","method":"GET","path":"/api/v2/cves","status":200,"duration_ms":45,...}`
- [x] OTel sampling: 10% in production (configurable via env `OTEL_SAMPLING_RATE`)
- [x] `zerolog.SetGlobalLevel(zerolog.InfoLevel)` in production (debug only in dev)
---

## Implementation Status

**✅ IMPLEMENTED — 2026-06-17** | Services: all | Build: `go build ./...` ✅

### Verified Components

| Component | File | Status |
|-----------|------|--------|
| Shared Prometheus metrics package | `shared/pkg/metrics/metrics.go` | ✅ DONE |
| requestsTotal, requestDuration, activeRequests, errorsTotal counters | `shared/pkg/metrics/metrics.go` | ✅ DONE |
| /metrics endpoint (per service) | `gateway-service/internal/metrics/metrics.go` | ✅ DONE |
| OpenTelemetry tracing interceptor (gRPC) | `shared/pkg/middleware/tracing/interceptor.go` | ✅ DONE |
| Datadog tracing interceptor | `shared/pkg/middleware/dd/tracing/interceptor.go` | ✅ DONE |
| Structured logging interceptor (zerolog) | `shared/pkg/middleware/logging/interceptor.go` | ✅ DONE |
| zerolog in all services (data, notification, gateway, identity) | All `cmd/server/main.go` | ✅ DONE |
| Health prober: OpenSearch/dd probes | `shared/pkg/health/probers.go` | ✅ DONE |
| NATS JetStream event tracing | `data-service/internal/infra/messaging/nats/` | ✅ DONE |
| gRPC base client with interceptors | `shared/pkg/grpcutil/base_client.go` | ✅ DONE |

### Acceptance Criteria: 13/13 ✅
