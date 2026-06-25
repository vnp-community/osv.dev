# AI Task 002: Implement AI Enrichment Service (SOL-002)

**Status**: ✅ COMPLETED 100% — 2026-06-18

## Checklist Công Việc

### 1. Khởi tạo Service Mới (`ai-service`)
- [x] Tạo thư mục `services/ai-service`
- [x] `go mod init osv/ai-service`
- [x] Cấu trúc Clean Architecture (`internal/domain`, `internal/usecase`, `internal/adapter`, `internal/infra`)
- [x] `services/ai-service/cmd/main.go` — HTTP Server port 9103

### 2. NATS JetStream Integration
- [x] Consumer lắng nghe `osv.vuln.imported`
- [x] Parse message + đẩy vào `AIEnrichmentUseCase`

### 3. Triage & Enrichment Use Cases
- [x] `triage_usecase.go` — `TriageFinding(finding)` gọi LLM
- [x] `enrichment_usecase.go` — `EnrichCVE(cve)` tóm tắt + vector hóa
- [x] LLM Client interface + OpenAI/Vertex AI adapter
- [x] Embedding 1536-dim cho CVE

### 4. HTTP Handlers (Adapter)
- [x] `POST /api/v1/ai/triage` — legacy body-param
- [x] `POST /api/v1/ai/enrich` — legacy body-param
- [x] `POST /triage/{findingId}/review` — CR-002
- [x] Tất cả endpoints registered on HTTP Router

### 5. API Gateway Routing
- [x] `/api/v1/ai/*` → ai-service:9103
- [x] All CR-002 routes registered in `apps/osv/internal/gateway/router.go`

## CR-002 Gap Fixes (thực thi 2026-06-18)

### ✅ ai-service — New handlers in `internal/delivery/http/ai_handler.go`
- [x] `TriageFindingByID` — POST /triage/{findingId} (async, 202)
- [x] `ReviewTriage` — POST /triage/{findingId}/review
- [x] `GetTriageQueue` — GET /triage/queue (filter by status)
- [x] `GetEnrichmentStatus` — GET /enrichment
- [x] `TriggerEnrichment` — POST /enrichment/trigger (async batch)
- [x] `GetEnrichmentByCVE` — GET /enrichment/{cveId}
- [x] Route ordering đúng: `/triage/queue` trước `/triage/{findingId}`

### ✅ Gateway — New AI routes in `apps/osv/internal/gateway/router.go`
- [x] `POST /api/v1/ai/triage/{findingId}` → ai-service:9103 (60s timeout)
- [x] `POST /api/v1/ai/triage/{findingId}/review` → ai-service:9103 (30s timeout)
- [x] `GET /api/v1/ai/triage/queue` → ai-service:9103
- [x] `GET /api/v1/ai/enrichment` → ai-service:9103
- [x] `POST /api/v1/ai/enrichment/trigger` → ai-service:9103 (60s timeout)
- [x] `GET /api/v1/ai/enrichment/{cveId}` → ai-service:9103

### ✅ Redis Persistence (2026-06-18)

#### Triage Queue — `ai:triage:queue` (Redis List)
- [x] Thêm `redis *redis.Client` vào `AIHTTPHandler` struct
- [x] Thêm `redisClient *redis.Client` vào `NewAIHTTPHandler()`
- [x] `TriageFindingByID`: `RPush` finding vào Redis list `ai:triage:queue` trước khi launch goroutine
  - item format: `{finding_id, status: "pending", queued_at}`
  - TTL 24h trên queue key
  - `queue_position` = actual list length sau khi push
- [x] Goroutine update status: `ai:triage:status:{findingId}` → `{status: "completed"|"failed", ...}`
- [x] `GetTriageQueue`: `LRANGE ai:triage:queue 0 -1` + filter theo `status` query param

#### Enrichment Status — `ai:enrichment:status` (Redis Hash)
- [x] `TriggerEnrichment`: set `ai:enrichment:status` → `{status: "running", last_run_at, queued: N}` trước khi launch goroutine
- [x] Goroutine: sau mỗi CVE thành công → `ai:enrichment:{cveId}` = `{status, enriched_at}` (TTL 30 ngày)
- [x] Goroutine: cuối → update `ai:enrichment:status` → `{status: "idle", total_enriched, last_run_at}`
- [x] `GetEnrichmentStatus`: `HGETALL ai:enrichment:status` → trả về real status/count/last_run_at

#### GetEnrichmentByCVE — `ai:enrichment:{cveId}` (Redis Hash)
- [x] Check `HGETALL ai:enrichment:{cveId}` — nếu có → 200 với `{cve_id, status, enriched_at, summary, tags}`
- [x] Nếu không có → 404 `{error: "NOT_ENRICHED", detail: "Use POST /enrichment/trigger..."}`

## Build Status
- `services/ai-service/internal/delivery/...` → ✅ Build OK
- `apps/osv/internal/gateway/...` → ✅ Build OK

## Ghi chú thiết kế
- Redis client được inject vào `AIHTTPHandler` (nil-safe: tất cả Redis calls đều có `if h.redis != nil` guard)
- `context.Background()` dùng cho goroutine để tránh request context bị cancel sau khi trả về 202
- Triage queue dùng Redis List (FIFO); enrichment status dùng Redis Hash
