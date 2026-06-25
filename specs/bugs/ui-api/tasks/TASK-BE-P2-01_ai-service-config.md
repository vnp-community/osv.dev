# TASK-BE-P2-01 — AI Service: Ollama Config + Graceful Degradation

**Phase:** Sprint 4 — P2  
**Nguồn giải pháp:** [`solutions/SOL-006_fix-ai-service-503.md`](../solutions/SOL-006_fix-ai-service-503.md)  
**Ưu tiên:** 🟡 P2 — AI Center không dùng được  
**Phụ thuộc:** Không có  
**Status:** ✅ **DONE** — 2026-06-19

### Implementation Summary
- **Part A (Config)**: Đổi `deploy/dev/.env` → `AI_BACKEND=ollama`, `AI_MODEL=qwen2.5:1.5b`, `AI_BASE_URL=http://ollama:11434`; thêm `ollama` service vào `docker-compose.server.yml` (network `osv-internal`, volume `ollama_data`)
- **Part B (Code — 4 files)**:
  1. `provider/chain.go`: thêm `HasAvailableProvider()` với 2s timeout
  2. `delivery/http/ai_handler.go`: thêm `chain *provider.Chain` field, `WithChain()`, `isReady()` + guards trong `GetTriageQueue`, `GetEnrichmentStatus`, `TriggerEnrichment`
  3. `cmd/server/embed.go`: **root cause fix** — mount full AI router (`/api/v1/ai/`) thay vì chỉ `/health`; wire Ollama provider chain cho graceful degradation
  4. `cmd/server/main.go`: fix constructor call (thêm `nil` redisClient arg)


---

## Mục tiêu

AI service trả 503 vì `OPENAI_API_KEY` và `OLLAMA_URL` đều trống. Fix bằng 2 bước:
1. Cấu hình Ollama cho staging (không cần API key)
2. Thêm graceful degradation vào AI handlers (trả 200 thay vì 503 khi không có LLM)

---

## Part A — Config Change

### [MODIFY] `deploy/dev/docker-compose.server.yml`

```bash
# Kiểm tra đã có ollama service chưa
grep "ollama" deploy/dev/docker-compose.server.yml
```

Nếu chưa có, thêm service `ollama`:

```yaml
# Thêm vào services section:
  ollama:
    image: ollama/ollama:latest
    container_name: osv-ollama
    volumes:
      - ollama_data:/root/.ollama
    ports:
      - "11434:11434"
    restart: unless-stopped
    networks:
      - osv_network

# Thêm vào volumes section:
volumes:
  ollama_data:
```

### [MODIFY] `deploy/dev/.env`

```bash
# Thay thế hoặc thêm vào deploy/dev/.env:
AI_BACKEND=ollama
OLLAMA_URL=http://ollama:11434
OLLAMA_LLM_MODEL=qwen2.5:1.5b      # model nhẹ, phù hợp staging
OLLAMA_EMBEDDING_MODEL=nomic-embed-text
```

### Deploy steps

```bash
# 1. Start Ollama container
docker compose -f deploy/dev/docker-compose.server.yml up -d ollama

# 2. Wait for Ollama to be ready
sleep 10

# 3. Pull model (chỉ cần chạy 1 lần, sẽ cache vào volume)
docker exec osv-ollama ollama pull qwen2.5:1.5b
# Model size: ~1GB — có thể mất vài phút

# 4. Verify Ollama ready
curl http://localhost:11434/api/tags | jq '.models[].name'
# Expected: "qwen2.5:1.5b"

# 5. Restart ai-service để load config mới
docker compose -f deploy/dev/docker-compose.server.yml restart ai-service
docker logs osv-backend-ai-service-1 --tail 30
# Expected: "AI provider initialized: ollama" hoặc tương tự
```

---

## Part B — Code Change: Graceful Degradation

```bash
# 1. Tìm AI handler files
grep -r "triage\|enrichment\|GetTriageQueue\|503" \
  services/ai-service/ --include="*.go" -l

# 2. Xem provider chain
cat services/ai-service/internal/provider/chain.go
```

### [FIND & MODIFY] AI handler file

Thêm `isReady()` guard vào mọi AI handler:

```go
// services/ai-service/internal/delivery/http/handler.go

// isReady kiểm tra có LLM provider nào available không
func (h *AIHandler) isReady() bool {
    if h.providerChain == nil {
        return false
    }
    return h.providerChain.HasAvailableProvider()
}

// GET /api/v1/ai/triage/queue
func (h *AIHandler) GetTriageQueue(w http.ResponseWriter, r *http.Request) {
    if !h.isReady() {
        respondJSON(w, http.StatusOK, map[string]interface{}{
            "items":  []interface{}{},
            "total":  0,
            "status": "ai_service_initializing",
        })
        return
    }
    // ... existing code
}

// GET /api/v1/ai/enrichment
func (h *AIHandler) GetEnrichmentStatus(w http.ResponseWriter, r *http.Request) {
    if !h.isReady() {
        respondJSON(w, http.StatusOK, map[string]interface{}{
            "status":          "unavailable",
            "provider":        "none",
            "queue_size":      0,
            "processed_today": 0,
            "message":         "AI service not configured",
        })
        return
    }
    // ... existing code
}

// POST /api/v1/ai/enrichment/trigger — nếu AI không ready, return 200 no-op
func (h *AIHandler) TriggerEnrichment(w http.ResponseWriter, r *http.Request) {
    if !h.isReady() {
        respondJSON(w, http.StatusAccepted, map[string]interface{}{
            "status":  "skipped",
            "message": "AI provider not available, enrichment skipped",
        })
        return
    }
    // ... existing trigger code
}
```

### [FIND & MODIFY] `services/ai-service/internal/provider/chain.go`

```go
// Thêm method HasAvailableProvider():
func (c *ProviderChain) HasAvailableProvider() bool {
    if c == nil || len(c.providers) == 0 {
        return false
    }
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()
    for _, p := range c.providers {
        if err := p.Health(ctx); err == nil {
            return true
        }
    }
    return false
}
```

Nếu Provider interface chưa có `Health()`:
```go
type Provider interface {
    Chat(ctx context.Context, prompt string) (string, error)
    Embed(ctx context.Context, text string) ([]float64, error)
    Health(ctx context.Context) error  // THÊM nếu chưa có
}
```

---

## Acceptance Criteria

- [ ] `GET /api/v1/ai/triage/queue` trả HTTP 200 (không còn 503)
- [ ] Khi Ollama running: response có meaningful data
- [ ] Khi Ollama không running: response có `{ "items": [], "status": "ai_service_initializing" }` với HTTP **200**
- [ ] `GET /api/v1/ai/enrichment` trả HTTP 200 trong mọi trường hợp
- [ ] `POST /api/v1/ai/enrichment/trigger` trả HTTP 202 kể cả khi AI unavailable

## Verification

```bash
# Sau khi config Ollama:
docker exec osv-ollama ollama list
# Expected: qwen2.5:1.5b listed

# AI triage queue
curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/ai/triage/queue | jq '.status'
# Expected: "ok" hoặc "ai_service_initializing" — KHÔNG PHẢI 503

# Graceful degradation test (kill ollama temporarily)
docker stop osv-ollama
curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/ai/triage/queue | jq '. | {status, total}'
# Expected: { "status": "ai_service_initializing", "total": 0 }
# NOT: HTTP 503

docker start osv-ollama
```
