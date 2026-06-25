# TASK-006: AI-Service — Wire Triage Queue & Enrichment Handlers

> **Bug**: BUG-011  
> **Solution**: SOL-006  
> **Service**: `services/ai-service`  
> **File chính**: `cmd/server/main.go`, `internal/delivery/http/ai_handler.go`  
> **Priority**: 🟡 MEDIUM  
> **Status**: `[x] DONE`

### Thay Đổi Thực Tế
- **Root cause**: `NewAIHTTPHandler(nil, nil, nil, nil, nil, ...)` — `redis=nil` + `chain=nil` → `isReady()` luôn `false` → tất cả endpoints trả về graceful degradation rỗng; queue/enrichment status không được persist
- **Wire Redis**: `cmd/server/main.go` — init `redis.NewClient` từ `REDIS_URL`, ping để verify, pass vào handler
- **Wire Provider Chain**: Build `provider.NewChain()` với ollama/openai provider từ `cfg.Backend`, gọi `.WithChain()` trên handler
- **GetInsights**: Thêm stub `GetInsights()` vào `ai_handler.go` + register `r.Get("/insights")` trong `NewRouter`

## Phân Tích Thực Tế

**Handlers đã implement** (`internal/delivery/http/ai_handler.go`):
- ✅ `GetTriageQueue` — line 253+ — đã có đầy đủ
- ✅ `GetEnrichmentStatus` — line 388+ — đã có
- ✅ `TriggerEnrichment` — line 436+ — đã có

**Vấn đề** (`cmd/server/embed.go`):
```go
// Handlers được khởi tạo với TẤT CẢ nil:
httpHandler := httpdelivery.NewAIHTTPHandler(nil, nil, nil, nil, nil, zerolog.Nop())
```

Theo comment trong embed.go:
```
// causing GetTriageQueue/GetEnrichmentStatus/TriggerEnrichment to return
```
→ Hàm bị return early vì dependencies nil, nhưng handler đã registered trong router.

**Kiểm tra router** (`internal/delivery/http/`):

```bash
find services/ai-service -name "router.go" | xargs cat 2>/dev/null
```

## Việc Cần Làm

### Bước 1: Kiểm tra AI router có register queue/enrichment chưa

```bash
find services/ai-service -name "router.go" | xargs cat 2>/dev/null
```

**Nếu router.go chưa có** — tạo mới hoặc kiểm tra embed.go mount pattern:
```go
// embed.go mount:
aiRouter := httpdelivery.NewRouter(httpHandler)
mainMux.Handle("/api/v1/ai/", aiRouter)
```

### Bước 2: Tạo/verify router.go mount tất cả handlers

File: `services/ai-service/internal/delivery/http/router.go`

```go
package http

import (
    "net/http"
    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
)

// NewRouter creates the chi router for the AI HTTP service.
func NewRouter(h *AIHTTPHandler) http.Handler {
    r := chi.NewRouter()
    r.Use(middleware.Recoverer)
    r.Use(middleware.RequestID)

    // Health
    r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.Write([]byte(`{"status":"ok","service":"ai-service"}`))
    })

    // ── Legacy (body-param based) ──
    r.Post("/api/v1/ai/triage", h.TriageFinding)
    r.Post("/api/v1/ai/enrich", h.EnrichCVE)

    // ── Triage by finding ID ──
    // CRITICAL: literal /triage/queue BEFORE /triage/{findingId}
    r.Get("/api/v1/ai/triage/queue", h.GetTriageQueue)              // ← FIX: ensure registered
    r.Post("/api/v1/ai/triage/{findingId}", h.TriageFindingByID)
    r.Post("/api/v1/ai/triage/{findingId}/review", h.ReviewTriage)

    // ── Enrichment ──
    // CRITICAL: literal /enrichment/trigger BEFORE /enrichment/{cveId}
    r.Get("/api/v1/ai/enrichment", h.GetEnrichmentStatus)           // ← FIX: ensure registered
    r.Post("/api/v1/ai/enrichment/trigger", h.TriggerEnrichment)    // ← FIX: ensure registered
    r.Get("/api/v1/ai/enrichment/{cveId}", h.GetCVEEnrichment)

    return r
}
```

### Bước 3: Fix embed.go — wire Redis cho handlers

File: `services/ai-service/cmd/server/embed.go`

**Vấn đề**: `httpdelivery.NewAIHTTPHandler(nil, nil, nil, nil, nil, ...)` — redis là `nil`.
Khi redis nil, `GetTriageQueue` và `GetEnrichmentStatus` không thể đọc queue state.

```go
// Tìm đoạn khởi tạo:
httpHandler := httpdelivery.NewAIHTTPHandler(nil, nil, nil, nil, nil, zerolog.Nop())

// Sửa thành — init redis client từ env:
redisAddr := os.Getenv("REDIS_ADDR")  // e.g., "redis:6379"
if redisAddr == "" { redisAddr = "localhost:6379" }

redisClient := redis.NewClient(&redis.Options{Addr: redisAddr})
httpHandler := httpdelivery.NewAIHTTPHandler(
    enrichHandler, // hoặc nil nếu chưa có
    batchUC,       // hoặc nil
    embeddingUC,   // hoặc nil
    triageUC,      // hoặc nil
    redisClient,   // ← PHẢI không nil cho queue/enrichment status
    log.Logger,
)
```

### Bước 4: Verify GetTriageQueue với nil redis

Kiểm tra handler có graceful fallback không:

```go
// In GetTriageQueue (ai_handler.go line 253+):
func (h *AIHTTPHandler) GetTriageQueue(w http.ResponseWriter, r *http.Request) {
    // P2-01: graceful degradation — return empty queue if LLM not ready
    // Nếu redis nil → return empty queue (không panic)
    if h.redis == nil {
        respondJSON(w, http.StatusOK, map[string]interface{}{
            "items": []interface{}{},
            "total": 0,
            "stats": map[string]int{"pending": 0, "processing": 0, "completed": 0},
        })
        return
    }
    // ...
}
```

Nếu không có nil check → thêm vào.

### Bước 5: Kiểm tra GetInsights (BUG-011 phần insights)

```bash
grep -n "insights\|Insights\|GetInsights" \
  services/ai-service/internal/delivery/http/ai_handler.go
```

Nếu không có → thêm stub handler:

```go
// GetInsights handles GET /api/v1/ai/insights — BUG-011
func (h *AIHTTPHandler) GetInsights(w http.ResponseWriter, r *http.Request) {
    // Graceful stub — trả về empty insights khi LLM chưa ready
    if !h.isReady() {
        respondJSON(w, http.StatusOK, map[string]interface{}{
            "items":        []interface{}{},
            "generated_at": time.Now().UTC(),
            "status":       "ai_unavailable",
        })
        return
    }

    // TODO: implement LLM-powered insights generation
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "items":        []interface{}{},
        "generated_at": time.Now().UTC(),
    })
}
```

Register trong router:
```go
r.Get("/api/v1/ai/insights", h.GetInsights)  // THÊM
```

Gateway đã có route:
```go
// apps/osv/internal/gateway/router.go — kiểm tra có không
// Nếu chưa có: thêm vào gateway
mux.Handle("GET /api/v1/ai/insights", protected(proxy.Forward("ai-service:9103")))
```

### Bước 6: Build & Test

```bash
cd services/ai-service && go build ./...
```

**Test**:
```bash
TOKEN="your_jwt_token"
BASE="https://c12.openledger.vn"

# Triage queue
curl -s "$BASE/api/v1/ai/triage/queue" \
  -H "Authorization: Bearer $TOKEN" | jq .
# Expected: 200 OK {items: [], total: 0, stats: {...}}

# Enrichment status
curl -s "$BASE/api/v1/ai/enrichment" \
  -H "Authorization: Bearer $TOKEN" | jq .
# Expected: 200 OK {pipeline: {...}}

# Trigger enrichment
curl -s -X POST "$BASE/api/v1/ai/enrichment/trigger" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"cve_ids": []}' | jq .
# Expected: 202 Accepted

# Insights
curl -s "$BASE/api/v1/ai/insights" \
  -H "Authorization: Bearer $TOKEN" | jq .
# Expected: 200 OK {items: []}
```

## Acceptance Criteria

- [x] `GET /api/v1/ai/triage/queue` → `200 OK` (không crash khi redis nil)
- [x] `GET /api/v1/ai/enrichment` → `200 OK`
- [x] `POST /api/v1/ai/enrichment/trigger` → `202 Accepted`
- [x] `GET /api/v1/ai/insights` → `200 OK`
- [x] `go build ./...` không lỗi
- [x] Không có nil pointer panic khi các use cases chưa được wired
