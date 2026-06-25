# TASK-BE-P1-09 — Fix SLA Config Schema (sla-service)

**Phase:** Sprint 3 — P1 Schema Fix  
**Nguồn giải pháp:** [`solutions/SOL-003_fix-findings-sla-500.md — Fix SLA Config`](../solutions/SOL-003_fix-findings-sla-500.md)  
**Ưu tiên:** 🟠 P1 — SLA Configuration page thiếu data  
**Status:** ✅ **DONE** — 2026-06-19
**Phụ thuộc:** Không có

---

## Mục tiêu

`GET /api/v1/sla/config` trả 200 nhưng schema thiếu `global` và `product_overrides`. Response hiện tại là flat object hoặc chỉ có `CriticalDays` v.v. Cần wrap đúng theo spec.

---

## Files cần sửa

### [FIND & MODIFY] `services/sla-service` — config handler

```bash
# Tìm config handler
grep -r "GetConfig\|sla/config\|SLAConfig" \
  services/sla-service/ --include="*.go" -l

# Xem response format hiện tại
grep -r "GetConfig\|respondJSON\|CriticalDays" \
  services/sla-service/internal/ --include="*.go" -A 15
```

```go
// TRƯỚC (trả flat hoặc sai format):
func (h *SLAHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
    config, _ := h.repo.GetConfig(r.Context())
    respondJSON(w, http.StatusOK, config) // ← trả flat SLAConfig struct
}

// SAU — wrap đúng spec { "global": {...}, "product_overrides": [] }:
func (h *SLAHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
    // Lấy global SLA config
    global, err := h.repo.GetGlobalConfig(r.Context())
    if err != nil {
        // Nếu chưa có config, dùng default values từ technical-design.md §5.3
        global = &SLAConfig{
            CriticalDays: 7,
            HighDays:     30,
            MediumDays:   90,
            LowDays:      180,
        }
    }

    // Lấy per-product overrides (có thể là empty)
    overrides, _ := h.repo.GetProductOverrides(r.Context())
    if overrides == nil {
        overrides = []ProductSLAOverride{}
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "global": map[string]interface{}{
            "critical_days": global.CriticalDays,
            "high_days":     global.HighDays,
            "medium_days":   global.MediumDays,
            "low_days":      global.LowDays,
        },
        "product_overrides": overrides,
    })
}
```

Nếu `GetGlobalConfig` chưa tồn tại, thêm vào repository interface:
```go
// domain/repository hoặc infra/postgres
type SLAConfigRepository interface {
    GetGlobalConfig(ctx context.Context) (*SLAConfig, error)
    GetProductOverrides(ctx context.Context) ([]ProductSLAOverride, error)
    // ... existing methods
}
```

Database query:
```go
func (r *PostgresSLARepo) GetGlobalConfig(ctx context.Context) (*SLAConfig, error) {
    var cfg SLAConfig
    err := r.db.QueryRow(ctx, `
        SELECT critical_days, high_days, medium_days, low_days
        FROM sla_configurations
        WHERE product_id IS NULL
        ORDER BY created_at DESC
        LIMIT 1
    `).Scan(&cfg.CriticalDays, &cfg.HighDays, &cfg.MediumDays, &cfg.LowDays)
    if err != nil {
        // Return default values
        return &SLAConfig{CriticalDays: 7, HighDays: 30, MediumDays: 90, LowDays: 180}, nil
    }
    return &cfg, nil
}
```

---

## Acceptance Criteria

- [ ] `GET /api/v1/sla/config` trả HTTP 200 với `global` key
- [ ] `global.critical_days` = 7 (hoặc giá trị đã configure)
- [ ] `global.high_days` = 30
- [ ] `product_overrides` là array (có thể empty `[]`)

## Verification

```bash
curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/sla/config | jq 'keys'
# Expected: ["global", "product_overrides"]

curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/sla/config | jq '.global'
# Expected: {
#   "critical_days": 7,
#   "high_days": 30,
#   "medium_days": 90,
#   "low_days": 180
# }
```

---

# TASK-BE-P2-01 — AI Service: Ollama Config + Graceful Degradation

**Phase:** Sprint 4 — P2  
**Nguồn giải pháp:** [`solutions/SOL-006_fix-ai-service-503.md`](../solutions/SOL-006_fix-ai-service-503.md)  
**Ưu tiên:** 🟡 P2 — AI Center không dùng được  
**Phụ thuộc:** Không có

---

## Mục tiêu

AI service trả 503 vì `OPENAI_API_KEY` và `OLLAMA_URL` đều trống. Fix bằng 2 bước: (1) cấu hình Ollama cho staging, (2) thêm graceful degradation vào AI handlers.

---

## Bước 1 — Cấu hình Ollama (config change)

```bash
# 1. Kiểm tra docker compose có service ollama không
grep -A 20 "ollama" deploy/dev/docker-compose.server.yml

# 2. Nếu chưa có, thêm vào deploy/dev/docker-compose.server.yml:
cat >> deploy/dev/docker-compose.server.yml << 'EOF'
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
EOF

# 3. Cập nhật deploy/dev/.env:
AI_BACKEND=ollama
OLLAMA_URL=http://ollama:11434
OLLAMA_LLM_MODEL=qwen2.5:1.5b
OLLAMA_EMBEDDING_MODEL=nomic-embed-text

# 4. Start Ollama và pull model
docker compose -f deploy/dev/docker-compose.server.yml up -d ollama
sleep 5
docker exec osv-ollama ollama pull qwen2.5:1.5b

# 5. Restart ai-service
docker compose -f deploy/dev/docker-compose.server.yml restart ai-service
docker logs osv-backend-ai-service-1 --tail 30
```

## Bước 2 — Graceful Degradation (code change)

```bash
# Tìm AI handlers
grep -r "triage/queue\|enrichment\|503\|ServiceUnavailable" \
  services/ai-service/ --include="*.go" -l
```

```go
// services/ai-service/internal/delivery/http/handler.go
// Thêm isReady check vào tất cả handlers:

type AIHandler struct {
    providerChain ProviderChain
    triageUC      TriageUseCase
    enrichUC      EnrichUseCase
    log           zerolog.Logger
}

// isReady returns true nếu có ít nhất 1 AI provider available
func (h *AIHandler) isReady() bool {
    return h.providerChain != nil && h.providerChain.HasAvailableProvider()
}

// GET /api/v1/ai/triage/queue
func (h *AIHandler) GetTriageQueue(w http.ResponseWriter, r *http.Request) {
    if !h.isReady() {
        // Graceful degradation: empty queue thay vì 503
        respondJSON(w, http.StatusOK, map[string]interface{}{
            "items":  []interface{}{},
            "total":  0,
            "status": "ai_service_initializing",
        })
        return
    }
    // ... normal queue processing
}

// GET /api/v1/ai/enrichment
func (h *AIHandler) GetEnrichmentStatus(w http.ResponseWriter, r *http.Request) {
    if !h.isReady() {
        respondJSON(w, http.StatusOK, map[string]interface{}{
            "status":           "unavailable",
            "provider":         "none",
            "queue_size":       0,
            "processed_today":  0,
            "message":          "AI service not configured. Set OPENAI_API_KEY or OLLAMA_URL.",
        })
        return
    }
    // ... normal status
}

// GET /api/v1/ai/enrichment/{cveId}
func (h *AIHandler) GetEnrichmentByCVE(w http.ResponseWriter, r *http.Request) {
    if !h.isReady() {
        respondJSON(w, http.StatusOK, map[string]interface{}{
            "cve_id":  chi.URLParam(r, "cveId"),
            "status":  "not_enriched",
            "message": "AI enrichment not available",
        })
        return
    }
    // ... normal enrichment lookup
}
```

Thêm `HasAvailableProvider()` vào provider chain:
```bash
# Tìm chain.go
cat services/ai-service/internal/provider/chain.go
```

```go
// services/ai-service/internal/provider/chain.go — THÊM method:
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

---

## Acceptance Criteria

- [ ] `GET /api/v1/ai/triage/queue` trả HTTP 200 (không còn 503)
- [ ] Khi Ollama chạy: response có `status: "ok"`, `items: []`
- [ ] Khi Ollama không chạy: response có `status: "ai_service_initializing"` với HTTP 200 (không phải 503)
- [ ] `GET /api/v1/ai/enrichment` trả HTTP 200 trong mọi trường hợp

## Verification

```bash
# After config:
curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/ai/triage/queue | jq '.status'
# Expected: "ok" hoặc "ai_service_initializing" (không phải 503)

# Enrichment status
curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/ai/enrichment | jq 'type'
# Expected: "object" (không phải 503 error)
```
