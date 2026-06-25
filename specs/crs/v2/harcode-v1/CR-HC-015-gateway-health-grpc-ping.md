# CR-HC-015: gateway-service — Health Check không ping gRPC Upstreams

## Trạng thái: 🟡 Medium

## Vấn đề
File: `services/gateway-service/internal/bff/handlers/handler_v1.go:82`

```go
// TODO: ping each gRPC upstream
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
    writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
```

Health check chỉ trả `{"status": "ok"}` mà không kiểm tra các upstream services.
Đây là vấn đề nghiêm trọng trong production:
- Load balancer/k8s sẽ nghĩ service healthy khi thực ra upstreams đã down
- Không có observability về trạng thái hệ thống
- Không phân biệt được `healthy` vs `degraded` vs `unhealthy`

## Giải pháp

### 1. Chuẩn health check enterprise (RFC 9457 / Spring Actuator style)

```json
{
  "status": "healthy",     // "healthy" | "degraded" | "unhealthy"
  "version": "1.2.3",
  "timestamp": "2026-06-23T14:00:00Z",
  "services": {
    "identity-service":    {"status": "healthy",   "latency_ms": 12},
    "finding-service":     {"status": "healthy",   "latency_ms": 8},
    "scan-service":        {"status": "healthy",   "latency_ms": 15},
    "data-service":        {"status": "healthy",   "latency_ms": 45},
    "search-service":      {"status": "healthy",   "latency_ms": 22},
    "notification-service":{"status": "degraded",  "latency_ms": 350, "error": "slow response"},
    "ai-service":          {"status": "unhealthy", "error": "connection refused"},
    "postgresql":          {"status": "healthy",   "latency_ms": 3},
    "redis":               {"status": "healthy",   "latency_ms": 1},
    "nats":                {"status": "healthy",   "latency_ms": 2}
  }
}
```

HTTP status code:
- `200` nếu tất cả `healthy`
- `207 Multi-Status` nếu có service `degraded`
- `503 Service Unavailable` nếu có service critical `unhealthy`

### 2. HealthChecker interface
```go
type ServiceHealthChecker interface {
    Name() string
    Check(ctx context.Context) HealthStatus
}

type HealthStatus struct {
    Status    string        // "healthy" | "degraded" | "unhealthy"
    LatencyMs int64
    Error     string
}
```

### 3. Implementation cho từng service
```go
// HTTP-based health check (cho embedded services)
type HTTPHealthChecker struct {
    name   string
    url    string
    client *http.Client
}

func (c *HTTPHealthChecker) Check(ctx context.Context) HealthStatus {
    start := time.Now()
    resp, err := c.client.Get(c.url)
    latency := time.Since(start).Milliseconds()
    
    if err != nil {
        return HealthStatus{Status: "unhealthy", Error: err.Error()}
    }
    defer resp.Body.Close()
    
    if resp.StatusCode >= 500 {
        return HealthStatus{Status: "unhealthy", LatencyMs: latency, Error: fmt.Sprintf("HTTP %d", resp.StatusCode)}
    }
    if latency > 500 {
        return HealthStatus{Status: "degraded", LatencyMs: latency}
    }
    return HealthStatus{Status: "healthy", LatencyMs: latency}
}
```

### 4. Health Handler
```go
func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()
    
    results := make(map[string]HealthStatus)
    var mu sync.Mutex
    var wg sync.WaitGroup
    
    for _, checker := range h.checkers {
        wg.Add(1)
        go func(c ServiceHealthChecker) {
            defer wg.Done()
            status := c.Check(ctx)
            mu.Lock()
            results[c.Name()] = status
            mu.Unlock()
        }(checker)
    }
    wg.Wait()
    
    overallStatus, httpCode := computeOverall(results)
    writeJSON(w, httpCode, HealthResponse{
        Status:    overallStatus,
        Version:   h.version,
        Timestamp: time.Now().UTC().Format(time.RFC3339),
        Services:  results,
    })
}
```

### 5. Wire checkers trong main.go / embedded.go
```go
checkers := []ServiceHealthChecker{
    health.NewHTTPChecker("postgresql", postgresHealthURL),
    health.NewHTTPChecker("redis", redisHealthURL),
    health.NewHTTPChecker("nats", natsHealthURL),
    health.NewHTTPChecker("identity-service", identityHealthURL),
    health.NewHTTPChecker("finding-service", findingHealthURL),
    // ...
}
healthHandler := health.NewHandler(checkers, version)
```

## Files cần thay đổi
- `services/gateway-service/internal/bff/handlers/handler_v1.go` — implement health
- `services/gateway-service/internal/health/checker.go` [NEW]
- `services/gateway-service/internal/health/http_checker.go` [NEW]
- `services/gateway-service/internal/health/postgres_checker.go` [NEW]

## Acceptance Criteria
- [ ] `GET /health` trả trạng thái thực của tất cả upstreams
- [ ] Khi identity-service down → `GET /health` trả 503
- [ ] Response bao gồm latency cho từng service
- [ ] Timeout 5 giây — không block quá lâu
