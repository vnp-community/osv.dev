# SOL-006 — Fix AI Service 503 (BUG-BE-011)

| Trường | Giá trị |
|---|---|
| **Bug** | [BUG-BE-011](../BUG-BE-011_ai-service-503.md) |
| **Service** | `services/ai-service` (port :9103) |
| **Priority** | P2 |
| **Estimated effort** | 1–2h (config) hoặc 8–16h (Ollama integration) |
| **Kiến trúc** | [architecture.md §3.11](../../../01-architecture.md) — AI-Service LLM Provider Chain |

---

## Root Cause

Từ `services/ai-service/internal/provider/`:
- `chain.go` — Failover chain: Ollama → OpenAI
- `ollama/` — Ollama local LLM (primary)
- `openai/` — OpenAI API (fallback)

Từ `deploy/dev/.env`:
```
AI_BACKEND=openai
OPENAI_API_KEY=      ← TRỐNG
OLLAMA_URL=          ← TRỐNG
```

Cả hai providers đều không configured → ai-service khởi động nhưng không thể serve bất kỳ request nào.

---

## Option A — Cấu Hình Ollama (Recommended cho staging)

Ollama là local LLM, không cần API key, phù hợp cho staging.

### A1. Thêm Ollama service vào docker-compose

```yaml
# deploy/dev/docker-compose.server.yml — THÊM service ollama
  ollama:
    image: ollama/ollama:latest
    container_name: osv-ollama
    volumes:
      - ollama_data:/root/.ollama
    ports:
      - "11434:11434"
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: all
              capabilities: [gpu]
    restart: unless-stopped
    networks:
      - osv_network

volumes:
  ollama_data:
```

### A2. Pull model

```bash
docker exec osv-ollama ollama pull llama3.2
# Hoặc model nhẹ hơn cho staging:
docker exec osv-ollama ollama pull qwen2.5:1.5b
```

### A3. Cập nhật `.env`

```bash
# deploy/dev/.env
AI_BACKEND=ollama
OLLAMA_URL=http://ollama:11434
OLLAMA_LLM_MODEL=llama3.2
OLLAMA_EMBEDDING_MODEL=nomic-embed-text
```

### A4. Restart ai-service

```bash
docker compose -f docker-compose.server.yml restart ai-service
docker logs osv-backend-ai-service-1 --tail 50
```

---

## Option B — Cấu Hình OpenAI (cho production)

```bash
# deploy/dev/.env
AI_BACKEND=openai
OPENAI_API_KEY=sk-proj-xxxxxxxxxxxxx
OPENAI_MODEL=gpt-4o-mini         # cheaper
OPENAI_EMBEDDING_MODEL=text-embedding-3-small
```

---

## Option C — Graceful Degradation (code fix)

Dù Option A hay B, ai-service nên trả **empty state** thay vì 503 khi không có LLM provider:

```go
// services/ai-service/internal/delivery/http/handler.go

// GET /api/v1/ai/triage/queue — khi AI unavailable, trả empty queue
func (h *AIHandler) GetTriageQueue(w http.ResponseWriter, r *http.Request) {
    if !h.isReady() {
        // Graceful degradation: trả empty queue thay vì 503
        respondJSON(w, http.StatusOK, map[string]interface{}{
            "items":  []interface{}{},
            "total":  0,
            "status": "ai_service_initializing",
        })
        return
    }
    // ... normal processing
}

// GET /api/v1/ai/enrichment — khi AI unavailable
func (h *AIHandler) GetEnrichmentStatus(w http.ResponseWriter, r *http.Request) {
    if !h.isReady() {
        respondJSON(w, http.StatusOK, map[string]interface{}{
            "status":      "unavailable",
            "provider":    "none",
            "queue_size":  0,
            "processed_today": 0,
        })
        return
    }
    // ... normal processing
}

// isReady checks if LLM provider is configured and accessible
func (h *AIHandler) isReady() bool {
    return h.providerChain != nil && h.providerChain.HasAvailableProvider()
}
```

Thêm `HasAvailableProvider()` vào provider chain:
```go
// services/ai-service/internal/provider/chain.go
func (c *ProviderChain) HasAvailableProvider() bool {
    for _, p := range c.providers {
        if p.IsAvailable() {
            return true
        }
    }
    return false
}
```

---

## Xác Nhận Fix

```bash
# Sau khi cấu hình Ollama:
docker logs osv-backend-ai-service-1 --tail 20
# Expected: "AI provider initialized: ollama"

curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/ai/triage/queue
# Expected: HTTP 200 (không còn 503)

# Test enrichment:
curl -X POST -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"cve_ids": ["CVE-2021-44228"]}' \
  https://c12.openledger.vn/api/v1/ai/enrichment/trigger
# Expected: HTTP 202 Accepted
```
