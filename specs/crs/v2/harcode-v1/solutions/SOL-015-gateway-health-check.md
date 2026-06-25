# SOL-015: Gateway Health Check với gRPC Ping — gateway-service

**CR:** CR-HC-015 | **Priority:** 🟡 Medium | **Sprint:** 1  
**Service:** `services/gateway-service`

---

## Implementation Status

**✅ IMPLEMENTED** — 2026-06-24
**Task:** TASK-HC-004
**Note:** Health/ready endpoint ping upstream services thay hardcoded ok
**Build:** ✅ `go build ./...` passes

---

---

## Context phân tích code

**File:** `gateway-service/internal/bff/handlers/handler_v1.go:82-86`

```go
func (h *CVEHandler) handleReadiness(w http.ResponseWriter, r *http.Request) {
    // TODO: ping each gRPC upstream
    respondJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
```

**File:** `gateway-service/internal/bff/handlers/handler_v1.go:62-79` — `Health()` đã aggregate HTTP upstreams ✅

**Khoảng trống:** `/health/ready` chưa kiểm tra gRPC upstreams.

---

## Solution

### Bước 1: Xác định gRPC upstream addresses

```bash
grep -rn "GRPCAddr\|grpc_addr\|gRPCPort\|GRPC_" gateway-service/ --include="*.go" | head -10
```

Từ architecture: `gateway-service` (port 50051-50057) và `ai-service` (50053), `data-service` (50054).

### Bước 2: gRPC Health Check utility

**File mới:** `gateway-service/internal/health/grpc_health.go`

```go
package health

import (
    "context"
    "fmt"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/grpc/health/grpc_health_v1"
)

type GRPCServiceResult struct {
    Service string `json:"service"`
    Target  string `json:"target"`
    Status  string `json:"status"`
    Latency int64  `json:"latency_ms"`
    Error   string `json:"error,omitempty"`
}

// PingGRPC checks health of a gRPC service using the standard grpc_health_v1 protocol.
// Returns result within timeout ms (default 3s).
func PingGRPC(ctx context.Context, name, target string) GRPCServiceResult {
    start := time.Now()
    result := GRPCServiceResult{
        Service: name,
        Target:  target,
    }

    dialCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
    defer cancel()

    conn, err := grpc.DialContext(dialCtx, target,
        grpc.WithTransportCredentials(insecure.NewCredentials()),
        grpc.WithBlock(),
    )
    if err != nil {
        result.Status = "unhealthy"
        result.Error = fmt.Sprintf("dial failed: %v", err)
        result.Latency = time.Since(start).Milliseconds()
        return result
    }
    defer conn.Close()

    hc := grpc_health_v1.NewHealthClient(conn)
    resp, err := hc.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
    result.Latency = time.Since(start).Milliseconds()

    if err != nil {
        result.Status = "unhealthy"
        result.Error = fmt.Sprintf("health check rpc: %v", err)
        return result
    }

    if resp.Status == grpc_health_v1.HealthCheckResponse_SERVING {
        result.Status = "healthy"
    } else {
        result.Status = "unhealthy"
        result.Error = fmt.Sprintf("not serving: %s", resp.Status.String())
    }
    return result
}

// PingGRPCAll checks multiple gRPC upstreams concurrently.
func PingGRPCAll(ctx context.Context, upstreams map[string]string) []GRPCServiceResult {
    type indexedResult struct {
        idx    int
        result GRPCServiceResult
    }

    results := make([]GRPCServiceResult, len(upstreams))
    ch := make(chan indexedResult, len(upstreams))

    i := 0
    for name, target := range upstreams {
        go func(idx int, n, t string) {
            ch <- indexedResult{idx: idx, result: PingGRPC(ctx, n, t)}
        }(i, name, target)
        i++
    }

    for range upstreams {
        r := <-ch
        results[r.idx] = r.result
    }
    return results
}
```

### Bước 3: Implement handleReadiness

**File sửa:** `gateway-service/internal/bff/handlers/handler_v1.go`

```go
// [FIX CR-HC-015] Implement real readiness check with gRPC ping
func (h *CVEHandler) handleReadiness(w http.ResponseWriter, r *http.Request) {
    // 1. HTTP health checks (existing)
    healthURLs := make(map[string]string)
    for name, url := range h.cfg.UpstreamURLs {
        healthURLs[name] = url + "/health"
    }
    httpResults := pkghealth.AggregateUpstreams(r.Context(), healthURLs)

    // 2. gRPC health checks (new)
    grpcUpstreams := map[string]string{}
    if h.cfg.AIGRPCAddr != "" {
        grpcUpstreams["ai-service-grpc"] = h.cfg.AIGRPCAddr
    }
    if h.cfg.DataGRPCAddr != "" {
        grpcUpstreams["data-service-grpc"] = h.cfg.DataGRPCAddr
    }
    if h.cfg.GatewayGRPCAddr != "" {
        grpcUpstreams["gateway-grpc"] = h.cfg.GatewayGRPCAddr
    }

    var grpcResults []health.GRPCServiceResult
    if len(grpcUpstreams) > 0 {
        grpcResults = health.PingGRPCAll(r.Context(), grpcUpstreams)
    }

    // 3. Determine overall readiness
    ready := true
    for _, r := range httpResults {
        if s, ok := r.(map[string]interface{}); ok {
            if s["status"] != "healthy" {
                ready = false
                break
            }
        }
    }
    for _, r := range grpcResults {
        if r.Status != "healthy" {
            ready = false
            break
        }
    }

    statusCode := http.StatusOK
    statusStr := "ready"
    if !ready {
        statusCode = http.StatusServiceUnavailable
        statusStr = "not_ready"
    }

    respondJSON(w, statusCode, map[string]interface{}{
        "status":      statusStr,
        "service":     "api-gateway",
        "timestamp":   time.Now().UTC().Format(time.RFC3339),
        "http_checks": httpResults,
        "grpc_checks": grpcResults,
    })
}
```

### Bước 4: Thêm gRPC addresses vào config

**File sửa:** `gateway-service` config struct:

```go
type Config struct {
    // existing...
    UpstreamURLs    map[string]string
    
    // [FIX CR-HC-015] gRPC upstream addresses
    AIGRPCAddr      string `env:"AI_SERVICE_GRPC"       envDefault:"ai-service:50053"`
    DataGRPCAddr    string `env:"DATA_SERVICE_GRPC"     envDefault:"data-service:50054"`
    GatewayGRPCAddr string `env:"GATEWAY_SERVICE_GRPC"  envDefault:""`
}
```

### Bước 5: Cải thiện `/health` response với structured format

Thêm chuẩn health check response (từ SOL architecture standard §11.4):

```go
// Health handles GET /health — detailed health status
func (h *CVEHandler) Health(w http.ResponseWriter, r *http.Request) {
    // ... existing aggregate logic ...
    
    // [FIX CR-HC-015] Add structured format
    overallStatus := "healthy"
    if !pkghealth.IsAllHealthy(results) {
        overallStatus = "degraded"
    }
    
    respondJSON(w, status, map[string]interface{}{
        "status":    overallStatus,
        "service":   "api-gateway",
        "version":   h.cfg.Version,            // from Config
        "timestamp": time.Now().UTC().Format(time.RFC3339),
        "upstreams": results,
    })
}
```

---

## Files cần tạo/sửa

| Action | File |
|--------|------|
| NEW | `gateway-service/internal/health/grpc_health.go` |
| MODIFY | `gateway-service/internal/bff/handlers/handler_v1.go` — implement handleReadiness |
| MODIFY | `gateway-service` config struct — thêm gRPC addresses |

---

## Verification

```bash
cd services/gateway-service && go build ./...

# Test readiness endpoint
curl "https://c12.openledger.vn/health/ready"
# Expect:
# {
#   "status": "ready",
#   "service": "api-gateway",
#   "timestamp": "2026-06-23T15:00:00Z",
#   "http_checks": {...},
#   "grpc_checks": [
#     {"service":"ai-service-grpc","status":"healthy","latency_ms":5},
#     {"service":"data-service-grpc","status":"healthy","latency_ms":3}
#   ]
# }

# Test liveness (should always return 200)
curl "https://c12.openledger.vn/health/live"
# Expect: {"status":"alive"}

# Kubernetes probe simulation
curl -f "https://c12.openledger.vn/health/ready"
echo "Exit code: $?"  # 0 = ready, non-zero = not ready
```

## Dependency

Cần thêm dependency nếu chưa có:
```bash
cd services/gateway-service
go get google.golang.org/grpc/health/grpc_health_v1
```
