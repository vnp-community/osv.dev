# Change Request 002: Bổ sung API cho AI Triage & Enrichment

**Cập nhật:** 2026-06-18  
**Status:** Partial — `ai-service` đã tồn tại, gateway có 2 endpoints cơ bản, nhưng còn thiếu nhiều endpoints mà frontend yêu cầu.

## 1. Bối cảnh

Frontend (openapi.yaml) yêu cầu nhóm endpoints `/api/v1/ai/*`:

| Endpoint frontend | Backend hiện tại | Trạng thái |
|---|---|---|
| `POST /api/v1/ai/triage/{findingId}` | `POST /api/v1/ai/triage` (không có `{findingId}`) | ❌ **Path mismatch** — backend nhận finding qua body, frontend truyền qua path |
| `POST /api/v1/ai/triage/{findingId}/review` | **THIẾU** | ❌ |
| `GET /api/v1/ai/triage/queue` | **THIẾU** | ❌ |
| `GET /api/v1/ai/enrichment` | **THIẾU** | ❌ |
| `POST /api/v1/ai/enrichment/trigger` | **THIẾU** | ❌ |
| `GET /api/v1/ai/enrichment/{cveId}` | **THIẾU** | ❌ |
| `POST /api/v1/ai/enrich` | `POST /api/v1/ai/enrich` | ✅ Có (nhưng frontend không gọi endpoint này trực tiếp) |

**Source backend hiện tại** (`apps/osv/internal/gateway/router.go`):
```go
mux.Handle("POST /api/v1/ai/triage", protected(...))  // nhận finding_id trong body
mux.Handle("POST /api/v1/ai/enrich", protected(...))  // nhận cve_id trong body
```

**Source ai-service** (`services/ai-service/internal/delivery/http/ai_handler.go`):
```go
r.Post("/triage", h.TriageFinding)   // POST /triage
r.Post("/enrich", h.EnrichCVE)       // POST /enrich
```

## 2. Thay đổi Đề Xuất

### 2.1 [CRITICAL] Fix path `/api/v1/ai/triage/{findingId}`

Frontend gọi `POST /api/v1/ai/triage/{findingId}` (findingId là path param). Backend hiện dùng `/triage` (body param).

**Gateway** — thêm route mới với path param:
```go
// apps/osv/internal/gateway/router.go
mux.Handle("POST /api/v1/ai/triage/{findingId}",
    protected(proxy.ForwardWithTimeout("ai-service:9103", 60*time.Second)))
```

**ai-service** — thêm handler mới:
```go
// services/ai-service/internal/delivery/http/ai_handler.go
r.Post("/triage/{findingId}", h.TriageFindingByID)
```

Response `202 Accepted`:
```json
{ "status": "queued", "queue_position": 1 }
```

### 2.2 [HIGH] Thêm `POST /api/v1/ai/triage/{findingId}/review`

Cho phép user gửi feedback về kết quả AI triage.

**Gateway**:
```go
mux.Handle("POST /api/v1/ai/triage/{findingId}/review",
    protected(proxy.ForwardWithTimeout("ai-service:9103", 30*time.Second)))
```

**Request body**:
```json
{
  "accepted": true,
  "override_remarks": "string | null",
  "comment": "string | null"
}
```

**Response** — trả về `AITriageResult` đã cập nhật.

### 2.3 [HIGH] Thêm `GET /api/v1/ai/triage/queue`

Danh sách findings đang trong hàng đợi AI triage.

**Gateway**:
```go
mux.Handle("GET /api/v1/ai/triage/queue", protected(proxy.Forward("ai-service:9103")))
```

**Query params**: `status` (pending/processing/done/failed), `page`, `page_size`

**Response**:
```json
{
  "items": [ { "finding_id": "...", "status": "pending", ... } ],
  "total": 42
}
```

### 2.4 [MEDIUM] Thêm `GET /api/v1/ai/enrichment`

Trạng thái enrichment pipeline (idle/running/error, last_run_at, total_enriched).

```go
mux.Handle("GET /api/v1/ai/enrichment", protected(proxy.Forward("ai-service:9103")))
```

### 2.5 [MEDIUM] Thêm `POST /api/v1/ai/enrichment/trigger`

Trigger enrichment thủ công cho danh sách CVE IDs.

```go
mux.Handle("POST /api/v1/ai/enrichment/trigger",
    protected(proxy.ForwardWithTimeout("ai-service:9103", 60*time.Second)))
```

**Request body**:
```json
{ "cve_ids": ["CVE-2025-1234"], "force": false }
```

**Response `202 Accepted`**:
```json
{ "queued_count": 5, "status": "queued" }
```

### 2.6 [MEDIUM] Thêm `GET /api/v1/ai/enrichment/{cveId}`

Chi tiết AI enrichment của một CVE cụ thể (summary, attack_vectors, remediation).

```go
mux.Handle("GET /api/v1/ai/enrichment/{cveId}", protected(proxy.Forward("ai-service:9103")))
```

**Response** — `CVEEnrichmentDetail` schema:
```json
{
  "cve_id": "CVE-2025-1234",
  "summary": "...",
  "attack_vectors": ["..."],
  "remediation": "...",
  "references": ["..."],
  "enriched_at": "2026-06-18T..."
}
```

## 3. Thay đổi tại ai-service

Bổ sung các handlers mới trong `services/ai-service/internal/delivery/http/ai_handler.go`:

```go
func (h *Handler) RegisterRoutes(r chi.Router) {
    // Existing
    r.Post("/triage", h.TriageFinding)      // backward compat
    r.Post("/enrich", h.EnrichCVE)           // backward compat

    // New routes for frontend
    r.Post("/triage/{findingId}", h.TriageFindingByID)
    r.Post("/triage/{findingId}/review", h.ReviewTriage)
    r.Get("/triage/queue", h.GetTriageQueue)

    r.Get("/enrichment", h.GetEnrichmentStatus)
    r.Post("/enrichment/trigger", h.TriggerEnrichment)
    r.Get("/enrichment/{cveId}", h.GetEnrichmentByCVE)
}
```

Lưu trữ triage queue và enrichment status trong Redis:
- `ai:triage:queue` — sorted set theo queued_at
- `ai:triage:{findingId}` — hash với status, result
- `ai:enrichment:{cveId}` — hash với enrichment detail

## 4. Tiêu chí nghiệm thu (Acceptance Criteria)

1. `POST /api/v1/ai/triage/{findingId}` trả về `202 Accepted` với `{status: "queued"}`.
2. `POST /api/v1/ai/triage/{findingId}/review` ghi nhận feedback, trả về AITriageResult cập nhật.
3. `GET /api/v1/ai/triage/queue` trả về danh sách với phân trang, filter theo status.
4. `GET /api/v1/ai/enrichment` trả về `{total_enriched, status, last_run_at}`.
5. `POST /api/v1/ai/enrichment/trigger` với `cve_ids` hợp lệ trả về `202 Accepted`.
6. `GET /api/v1/ai/enrichment/{cveId}` trả về `CVEEnrichmentDetail` hoặc `404` nếu chưa enrich.
