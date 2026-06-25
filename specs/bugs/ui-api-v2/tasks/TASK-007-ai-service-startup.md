# TASK-007 — ai-service: Fix Startup + Defensive Handlers cho 503

**Bug**: [BUG-007](../BUG-007-ai-triage.md), [BUG-008](../BUG-008-ai-enrichment.md)  
**Solution**: [SOL-007](../solutions/SOL-007-ai-services.md)  
**Priority**: 🟠 P1  
**Effort**: ~20 phút  
**Status**: `[x] DONE`

---

## Mô tả

`GET /api/v1/ai/triage/queue` và `GET /api/v1/ai/enrichment` trả `503 Service Unavailable`. Các endpoints này chỉ đọc **queue status** — không cần LLM hoạt động. Cần: (1) handler trả safe defaults khi LLM down, (2) docker startup dependency.

---

## File cần sửa

**File 1**: Tìm triage handler trong ai-service:

```bash
find /Users/binhnt/Lab/sec/cve/osv.dev/services/ai-service/internal/delivery/http \
  -name "*.go" | head -10
```

**File 2** (docker): `deploy/dev/docker-compose.server.yaml`

---

## Thay đổi 1 — ai-service: Triage Queue Handler

**Tìm** handler cho `GET /api/v1/ai/triage/queue`.

**Nếu chưa có**, tạo trong handler file phù hợp:

```go
// GET /api/v1/ai/triage/queue
func (h *TriageHandler) GetQueue(w http.ResponseWriter, r *http.Request) {
    // Thử đọc từ Redis cache — không fail nếu không có
    ctx := r.Context()

    queueSize := 0
    processing := 0

    // Optional: đọc từ Redis nếu có
    if h.redis != nil {
        if val, err := h.redis.Get(ctx, "ai:triage:queue:size").Int(); err == nil {
            queueSize = val
        }
        if val, err := h.redis.Get(ctx, "ai:triage:queue:processing").Int(); err == nil {
            processing = val
        }
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "queue_size":      queueSize,
        "processing":      processing,
        "completed_today": 0,
        "status":          h.getProviderStatus(),  // "ready"|"degraded"|"down"
        "provider":        h.getActiveProvider(),
    })
}

func (h *TriageHandler) getProviderStatus() string {
    // Kiểm tra provider chain — trả "degraded" nếu down, không trả 503
    if h.chain != nil && h.chain.IsHealthy() {
        return "ready"
    }
    return "degraded"
}

func (h *TriageHandler) getActiveProvider() string {
    if h.chain != nil && h.chain.ActiveProvider() != nil {
        return h.chain.ActiveProvider().Name()
    }
    return "unavailable"
}
```

---

## Thay đổi 2 — ai-service: Enrichment Status Handler

**Tìm** handler cho `GET /api/v1/ai/enrichment`.

```go
// GET /api/v1/ai/enrichment
func (h *EnrichmentHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    var lastRun interface{} = nil
    totalEnriched := 0

    if h.redis != nil {
        if val, err := h.redis.Get(ctx, "ai:enrichment:last_run").Result(); err == nil {
            lastRun = val
        }
        if val, err := h.redis.Get(ctx, "ai:enrichment:total").Int(); err == nil {
            totalEnriched = val
        }
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "status":         "idle",
        "last_run":       lastRun,
        "total_enriched": totalEnriched,
        "pending_count":  0,
        "provider_status": map[string]string{
            "ollama": h.getOllamaStatus(),
            "openai": "fallback",
        },
    })
}
```

---

## Thay đổi 3 — Health endpoint trong ai-service

**Tìm** health handler. Đảm bảo trả `200` ngay mà không cần LLM:

```go
// GET /health
func HandleHealth(w http.ResponseWriter, r *http.Request) {
    respondJSON(w, http.StatusOK, map[string]string{
        "status":  "ok",
        "service": "ai-service",
    })
}
```

---

## Thay đổi 4 — docker-compose.server.yaml

**Tìm** service `ai-service` trong file deploy:

```bash
find /Users/binhnt/Lab/sec/cve/osv.dev/deploy -name "docker-compose*.yaml" | head -5
```

**Thêm** hoặc **cập nhật** `depends_on` và `start_period`:

```yaml
ai-service:
  depends_on:
    ollama:
      condition: service_started    # chỉ cần started, không cần healthy
    postgres:
      condition: service_healthy
  healthcheck:
    test: ["CMD", "curl", "-f", "http://localhost:9103/health"]
    interval: 15s
    timeout: 5s
    retries: 5
    start_period: 90s               # đủ thời gian load LLM model
```

---

## Acceptance Criteria

- [ ] `GET /api/v1/ai/triage/queue` trả `200` kể cả khi Ollama chưa sẵn sàng
- [ ] `GET /api/v1/ai/enrichment` trả `200` với `status: "idle"`
- [ ] `GET /health` trả `200` ngay lập tức (không timeout)
- [ ] `go build ./...` trong ai-service không có lỗi

---

## Verify

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev/services/ai-service
go build ./...

# Test (có thể test trực tiếp service hoặc qua gateway)
curl -s -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v1/ai/triage/queue" | jq '.status'
# Expected: "ready" hoặc "degraded" (không phải HTTP 503)

curl -s -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v1/ai/enrichment" | jq '.status'
# Expected: "idle"
```
