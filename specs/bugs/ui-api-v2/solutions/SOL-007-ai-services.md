# SOL-007 — AI Services 503: Fix Startup & Fallback (P1)

**Bugs**: [BUG-007](../BUG-007-ai-triage.md), [BUG-008](../BUG-008-ai-enrichment.md)  
**Service**: `ai-service` (`services/ai-service`, port 9103)  
**Endpoints**: `GET /api/v1/ai/triage/queue`, `GET /api/v1/ai/enrichment`  
**HTTP Error**: `503 Service Unavailable`

**Status**: `✅ Implemented` — via [TASK-007](../../tasks/TASK-007-*.md)

---

## Root Cause Analysis

`503` từ ai-service thường do:
1. **Ollama không sẵn sàng** — ai-service startup dependency chưa đủ
2. **ai-service pod/container crashed** — OOM hoặc startup timeout
3. **LLM provider chain exhausted** — cả Ollama và OpenAI đều fail

---

## Giải pháp

### Bước 1: Kiểm tra trạng thái ai-service

```bash
# Check container status
docker ps | grep ai-service
docker logs ai-service --tail 50

# Test health trực tiếp
curl http://ai-service:9103/health
curl http://ai-service:9103/api/v1/ai/triage/queue

# Check Ollama
docker logs ollama --tail 20
curl http://ollama:11434/api/tags  # Liệt kê models available
```

### Bước 2: Fix ai-service — Defensive Handlers

Endpoints `/api/v1/ai/triage/queue` và `/api/v1/ai/enrichment` phải luôn trả 200 **kể cả khi LLM không sẵn sàng** (vì chúng chỉ là "queue status", không cần gọi LLM thực sự).

```go
// services/ai-service/internal/delivery/http/triage_handler.go

// GET /api/v1/ai/triage/queue
func (h *TriageHandler) GetQueue(w http.ResponseWriter, r *http.Request) {
    // Kiểm tra từ Redis cache
    queueStatus, err := h.redis.Get(r.Context(), "ai:triage:queue").Result()
    if err != nil {
        // Trả empty queue thay vì 503
        respondJSON(w, http.StatusOK, map[string]interface{}{
            "queue_size":    0,
            "processing":   0,
            "completed_today": 0,
            "status":       "ready",     // hoặc "degraded" nếu LLM offline
            "provider":     h.getActiveProvider(),
        })
        return
    }
    
    respondJSON(w, http.StatusOK, queueStatus)
}

// GET /api/v1/ai/enrichment
func (h *EnrichmentHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
    status, err := h.redis.Get(r.Context(), "ai:enrichment:status").Result()
    if err != nil {
        // Trả default status thay vì 503
        respondJSON(w, http.StatusOK, map[string]interface{}{
            "status":           "idle",
            "last_run":         nil,
            "total_enriched":   0,
            "pending_count":    0,
            "provider_status":  h.getProviderStatus(),
        })
        return
    }
    
    respondJSON(w, http.StatusOK, status)
}

func (h *TriageHandler) getActiveProvider() string {
    // Kiểm tra provider hiện tại
    if h.chain != nil && h.chain.ActiveProvider() != nil {
        return h.chain.ActiveProvider().Name()
    }
    return "unavailable"
}
```

### Bước 3: Fix startup dependency trong docker-compose

```yaml
# deploy/dev/docker-compose.server.yaml

ai-service:
  depends_on:
    ollama:
      condition: service_healthy   # Đợi Ollama sẵn sàng
    postgres:
      condition: service_healthy
  healthcheck:
    test: ["CMD", "curl", "-f", "http://localhost:9103/health"]
    interval: 10s
    timeout: 5s
    retries: 5
    start_period: 60s              # Đủ thời gian load model
  environment:
    AI_STARTUP_TIMEOUT: "120s"    # Tăng timeout startup
    OLLAMA_HOST: "http://ollama:11434"
    OLLAMA_MODEL: "llama3"
```

### Bước 4: Health endpoint trong ai-service phải đơn giản

```go
// services/ai-service/internal/delivery/http/ — health handler
// GET /health — KHÔNG gọi LLM, chỉ check service alive

func HandleHealth(w http.ResponseWriter, r *http.Request) {
    respondJSON(w, http.StatusOK, map[string]string{
        "status":  "ok",
        "service": "ai-service",
    })
}
```

### Bước 5: Circuit Breaker trong HealthBFF

Health BFF tại gateway cần không báo 503 khi ai-service đang khởi động:

```go
// apps/osv/internal/gateway/bff/health.go
// checkService đã có timeout 2s — ai-service trả 503 → "degraded" không phải "down"

func (bff *HealthBFF) checkService(ctx context.Context, name, url string) ServiceHealth {
    // ...
    if resp.StatusCode == 503 {
        result.Status = "degraded"    // Không phải "down"
        msg := "service starting up"
        result.Details = &msg
        return result
    }
    // ...
}
```

---

## Response Schema Required

```json
// GET /api/v1/ai/triage/queue
{
  "queue_size": 5,
  "processing": 2,
  "completed_today": 42,
  "status": "ready",
  "provider": "ollama"
}

// GET /api/v1/ai/enrichment
{
  "status": "idle",
  "last_run": "2026-06-20T00:00:00Z",
  "total_enriched": 1234,
  "pending_count": 0,
  "provider_status": {
    "ollama": "healthy",
    "openai": "fallback"
  }
}
```

---

## Verification

```bash
# Sau khi ai-service restart
curl -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v1/ai/triage/queue" | jq '.status'
# Expected: "ready" hoặc "degraded" (không phải 503)

curl -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v1/ai/enrichment" | jq '.status'
# Expected: "idle" hoặc "processing" (không phải 503)
```
