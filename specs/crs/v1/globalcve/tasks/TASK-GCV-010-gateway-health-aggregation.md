# TASK-GCV-010 — Health Aggregation Endpoint (gateway-service)

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-GCV-010 |
| **Service** | `gateway-service` |
| **CR** | CR-GCV-008 |
| **Phase** | 1 — Core Pipeline |
| **Priority** | 🔴 High |
| **Prerequisites** | — |

## Context

Nâng cấp `GET /health` trong `gateway-service` từ local health check thành **aggregate health check** — parallel call tới tất cả upstream services trong 3 giây, trả về trạng thái tổng hợp `healthy | degraded`. `apps/osv` expose `/health` này ra ngoài.

## Reference

- Solution: [SOL-GCV-008](../solutions/SOL-GCV-008-api-gateway-enhancement.md) §2.3
- CR: [CR-GCV-008](../CR-GCV-008-api-gateway-enhancement.md) §4

## Files to Create/Modify

```
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/gateway-service/internal/health/
        (đọc cấu trúc health package hiện có, update usecase + handler)
```

**Đọc trước**: `gateway-service/internal/health/` để hiểu cấu trúc hiện tại.

## Implementation Spec

### health/usecase.go — MODIFY hoặc CREATE

```go
// Package health — aggregate health check usecase.
package health

import (
    "context"
    "net/http"
    "sync"
    "time"
)

// UpstreamConfig holds URL and name of an upstream service.
type UpstreamConfig struct {
    Name string
    URL  string // Base URL, e.g. "http://data-service:8082"
}

// ServiceHealth is the health status of one upstream.
type ServiceHealth struct {
    Name      string `json:"name"`
    Status    string `json:"status"`    // "healthy" | "unhealthy"
    LatencyMs int64  `json:"latency_ms"`
    Error     string `json:"error,omitempty"`
}

// SystemHealth is the aggregated system health.
type SystemHealth struct {
    Status    string                    `json:"status"`     // "healthy" | "degraded"
    CheckedAt time.Time                 `json:"checked_at"`
    Services  map[string]*ServiceHealth `json:"services"`
    Version   string                    `json:"version"`
}

// UseCase performs parallel health checks against all upstreams.
type UseCase struct {
    upstreams []UpstreamConfig
    client    *http.Client // short timeout: 3s
    version   string
}

// NewUseCase creates a HealthUseCase.
func NewUseCase(upstreams []UpstreamConfig, version string) *UseCase {
    return &UseCase{
        upstreams: upstreams,
        client:    &http.Client{Timeout: 3 * time.Second},
        version:   version,
    }
}

// Check performs parallel health checks and aggregates results.
// Returns "degraded" if any upstream is unhealthy (not "unhealthy" globally).
func (uc *UseCase) Check(ctx context.Context) *SystemHealth {
    results := make([]*ServiceHealth, len(uc.upstreams))
    var wg sync.WaitGroup

    for i, u := range uc.upstreams {
        wg.Add(1)
        go func(idx int, upstream UpstreamConfig) {
            defer wg.Done()
            start := time.Now()
            svc := &ServiceHealth{Name: upstream.Name}

            resp, err := uc.client.Get(upstream.URL + "/health")
            svc.LatencyMs = time.Since(start).Milliseconds()

            if err != nil {
                svc.Status = "unhealthy"
                svc.Error = err.Error()
            } else {
                resp.Body.Close()
                if resp.StatusCode >= 200 && resp.StatusCode < 300 {
                    svc.Status = "healthy"
                } else {
                    svc.Status = "unhealthy"
                    svc.Error = fmt.Sprintf("status %d", resp.StatusCode)
                }
            }
            results[idx] = svc
        }(i, u)
    }

    wg.Wait()

    health := &SystemHealth{
        Status:    "healthy",
        CheckedAt: time.Now().UTC(),
        Services:  make(map[string]*ServiceHealth, len(results)),
        Version:   uc.version,
    }

    for _, svc := range results {
        if svc == nil {
            continue
        }
        health.Services[svc.Name] = svc
        if svc.Status == "unhealthy" {
            health.Status = "degraded" // degraded, not unhealthy (partial availability)
        }
    }

    return health
}
```

### health/handler.go — ADD Health handler

```go
// Handler exposes /health as an HTTP handler.
type Handler struct {
    uc *UseCase
}

func NewHandler(uc *UseCase) *Handler {
    return &Handler{uc: uc}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
    health := h.uc.Check(r.Context())

    statusCode := http.StatusOK
    if health.Status == "degraded" {
        statusCode = http.StatusOK // still 200 — degraded is not an error
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)
    json.NewEncoder(w).Encode(health) //nolint:errcheck
}
```

### Wire upstreams (trong config hoặc main.go)

Danh sách upstreams phải bao gồm:

```go
upstreams := []health.UpstreamConfig{
    {Name: "data-service",         URL: cfg.DataServiceHTTP},      // e.g. "http://data-service:8082"
    {Name: "search-service",       URL: cfg.SearchServiceHTTP},    // "http://search-service:8081"
    {Name: "notification-service", URL: cfg.NotificationServiceHTTP}, // "http://notification-service:8084"
    // gateway tự check local không cần include
}
```

### Sample Response

```json
GET /health → 200 OK
{
  "status": "healthy",
  "checked_at": "2026-06-14T07:00:00Z",
  "version": "3.0.0",
  "services": {
    "data-service": { "name": "data-service", "status": "healthy", "latency_ms": 12 },
    "search-service": { "name": "search-service", "status": "healthy", "latency_ms": 8 },
    "notification-service": { "name": "notification-service", "status": "unhealthy", "latency_ms": 3001, "error": "context deadline exceeded" }
  }
}
→ "status": "degraded" khi bất kỳ service nào unhealthy
```

## Acceptance Criteria

- [x] `GET /health` → 200 JSON với `status`, `checked_at`, `version`, `services`
- [x] Tất cả upstream checks chạy **parallel** (không sequential)
- [x] Timeout 3s per upstream (không chặn tổng response quá 3s)
- [x] Một service down → `"status":"degraded"` (không phải `"unhealthy"`)
- [x] Tất cả service down → `"status":"degraded"` (vẫn 200, không 503)
- [x] `services` map có key là service name, value có `latency_ms`
- [x] `go build ./...` pass không lỗi
