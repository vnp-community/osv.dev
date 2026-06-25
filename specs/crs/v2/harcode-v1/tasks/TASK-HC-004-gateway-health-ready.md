# TASK-HC-004: Implement `/health/ready` với gRPC Ping

**Status:** ✅ DONE  
**Sprint:** 1 | **Ước lượng:** 2 giờ  
**Solution:** [SOL-015](../solutions/SOL-015-gateway-health-check.md)  
**Service:** `services/gateway-service`

---

## Mô tả

`handleReadiness` trong gateway đang trả `{"status":"ready"}` mà không kiểm tra upstream thật. Cần implement gRPC health ping thật.

---

## Acceptance Criteria

- [x] `GET /health/ready` kiểm tra ít nhất HTTP upstreams (hiện đã có)
- [x] `GET /health/ready` kiểm tra thêm gRPC upstreams (ai-service, data-service)
- [x] Trả `503` khi bất kỳ upstream nào down
- [x] Trả `200` khi tất cả upstreams healthy
- [x] Response có field `grpc_checks` với latency
- [x] `go build ./...` pass trong `services/gateway-service`

---

## Files cần sửa/tạo

| Action | File | Thay đổi |
|--------|------|---------|
| NEW | `services/gateway-service/internal/health/grpc_health.go` | `PingGRPC` + `PingGRPCAll` functions |
| MODIFY | `services/gateway-service/internal/bff/handlers/handler_v1.go` | Implement `handleReadiness` |
| MODIFY | Gateway config struct | Thêm `AIGRPCAddr`, `DataGRPCAddr` fields |

---

## Bước thực thi

### 1. Kiểm tra gateway config struct
```bash
grep -n "type.*Config\|AIGRPCAddr\|GRPC" services/gateway-service/internal/bff/handlers/handler_v1.go \
  services/gateway-service/cmd/server/main.go 2>/dev/null | head -20
```

### 2. Kiểm tra dependency grpc health đã có chưa
```bash
grep "grpc_health_v1\|google.golang.org/grpc" services/gateway-service/go.mod | head -5
```

### 3. Tạo grpc_health.go
**File mới:** `services/gateway-service/internal/health/grpc_health.go`

```go
package health

import (
    "context"
    "fmt"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/connectivity"
    "google.golang.org/grpc/credentials/insecure"
)

type GRPCCheckResult struct {
    Service   string `json:"service"`
    Status    string `json:"status"`
    LatencyMs int64  `json:"latency_ms"`
    Error     string `json:"error,omitempty"`
}

// PingGRPC dials gRPC target và kiểm tra kết nối.
// Không cần grpc_health_v1 protocol — dùng connectivity state.
func PingGRPC(ctx context.Context, name, target string) GRPCCheckResult {
    start := time.Now()
    result := GRPCCheckResult{Service: name}

    dialCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
    defer cancel()

    conn, err := grpc.DialContext(dialCtx, target,
        grpc.WithTransportCredentials(insecure.NewCredentials()),
        grpc.WithBlock(),
        grpc.FailOnNonTempDialError(true),
    )
    result.LatencyMs = time.Since(start).Milliseconds()

    if err != nil {
        result.Status = "unhealthy"
        result.Error = fmt.Sprintf("dial failed: %v", err)
        return result
    }
    defer conn.Close()

    state := conn.GetState()
    if state == connectivity.Ready || state == connectivity.Idle {
        result.Status = "healthy"
    } else {
        result.Status = "degraded"
        result.Error = state.String()
    }
    return result
}

// PingAll pings multiple gRPC upstreams concurrently.
func PingAll(ctx context.Context, upstreams map[string]string) []GRPCCheckResult {
    ch := make(chan GRPCCheckResult, len(upstreams))
    for name, addr := range upstreams {
        go func(n, a string) {
            ch <- PingGRPC(ctx, n, a)
        }(name, addr)
    }
    results := make([]GRPCCheckResult, 0, len(upstreams))
    for range upstreams {
        results = append(results, <-ch)
    }
    return results
}
```

### 4. Thêm gRPC addrs vào handler config
```bash
grep -n "type.*HandlerConfig\|UpstreamURLs\|struct" \
  services/gateway-service/internal/bff/handlers/handler_v1.go | head -15
```

Thêm fields vào config struct (điều chỉnh theo code thực tế):
```go
AIGRPCAddr   string  // env: AI_SERVICE_GRPC, default: "ai-service:50053"
DataGRPCAddr string  // env: DATA_SERVICE_GRPC, default: "data-service:50054"
```

### 5. Implement handleReadiness

Tìm function:
```bash
grep -n "handleReadiness\|func.*Readiness" services/gateway-service/internal/bff/handlers/handler_v1.go
```

Thay thế body:
```go
func (h *CVEHandler) handleReadiness(w http.ResponseWriter, r *http.Request) {
    // HTTP upstream checks (existing logic)
    healthURLs := make(map[string]string)
    for name, url := range h.cfg.UpstreamURLs {
        healthURLs[name] = url + "/health"
    }
    httpResults := pkghealth.AggregateUpstreams(r.Context(), healthURLs)

    // gRPC upstream checks
    grpcUpstreams := map[string]string{}
    if h.cfg.AIGRPCAddr != "" {
        grpcUpstreams["ai-service"] = h.cfg.AIGRPCAddr
    }
    if h.cfg.DataGRPCAddr != "" {
        grpcUpstreams["data-service"] = h.cfg.DataGRPCAddr
    }
    grpcResults := health.PingAll(r.Context(), grpcUpstreams)

    // Determine overall status
    ready := pkghealth.IsAllHealthy(httpResults)
    for _, r := range grpcResults {
        if r.Status == "unhealthy" {
            ready = false
            break
        }
    }

    code := http.StatusOK
    status := "ready"
    if !ready {
        code = http.StatusServiceUnavailable
        status = "not_ready"
    }

    respondJSON(w, code, map[string]interface{}{
        "status":      status,
        "timestamp":   time.Now().UTC().Format(time.RFC3339),
        "http_checks": httpResults,
        "grpc_checks": grpcResults,
    })
}
```

### 6. Thêm import health package nếu cần
```go
health "github.com/google/osv.dev/services/gateway-service/internal/health"
```

### 7. Build check
```bash
cd services/gateway-service && go build ./...
```

---

## Verification

```bash
# Readiness check
curl -s "https://c12.openledger.vn/health/ready" | jq '{status, grpc_checks}'
# PASS nếu có grpc_checks array (dù empty khi không config gRPC addrs)

# Liveness check (không được break)
curl -s "https://c12.openledger.vn/health/live" | jq '.status'
# PASS nếu = "alive"
```
