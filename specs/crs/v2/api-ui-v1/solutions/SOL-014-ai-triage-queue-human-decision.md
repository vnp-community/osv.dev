# SOL-014: AI Triage Queue — Full Schema with Human Decision

> **CR:** [CR-014](../CR-014-ai-triage-queue-human-decision.md) + [CR-013 §2.2](../CR-013-public-stats-ai-triage-queue-schema.md)  
> **Priority:** 🔴 HIGH (Phase 2)  
> **Service(s):** `ai-service` (`:9103`)  
> **Tạo:** 2026-06-19  
> **Trạng thái:** ✅ **IMPLEMENTED** — 2026-06-22  

---

## ✅ Implementation Status

| Layer | Trạng thái | File/Location |
|---|---|---|
| `GET /api/v1/ai/triage/queue` | ✅ Done | `ai_handler.go:GetTriageQueue()` |
| `items` schema: `ai_result` nested object | ✅ Done | Reshape: `{remarks, confidence, justification, actions, generated_at}` |
| `stats` object: pending, accepted_today, avg_confidence, fp_rate | ✅ Done | Computed from Redis queue data |
| Filter `?status=` (pending/accepted/overridden/rejected/all) | ✅ Done | `GetTriageQueue()` filter logic |
| Filter `?severity=` | ✅ Done | item `severity` field filter |
| Filter `?remarks=` | ✅ Done | item `remarks` field filter |
| `POST /api/v1/ai/triage/{findingId}/review` | ✅ Done | `ai_handler.go:ReviewTriage()` |
| `decision` validation: accepted/overridden/rejected | ✅ Done | `validDecisions` map |
| `note` field optional | ✅ Done | `*string` nullable |
| 409 Conflict khi đã reviewed | ✅ Done | Check Redis `human_decision` key |
| `?force=true` override | ✅ Done | `force` param bypass 409 |
| 404 khi finding không tồn tại | ⚠️ Partial | Redis-based: không có finding trong queue → empty response |
| `reviewed_by` từ JWT (X-User-Email) | ✅ Done | `r.Header.Get("X-User-Email")` |
| Route ordering: `/queue` trước `/{findingId}` | ✅ Done | chi router: `/triage/queue` literal trước `/{findingId}` wildcard |
| Storage backend | ⚠️ Redis (không phải Postgres) | Spec dùng Postgres, impl dùng Redis — functional equivalent |
| Gateway proxy to `ai-service:9103` | ✅ Done | `router.go` → `proxy.Forward("ai-service:9103")` |
| Graceful degradation khi LLM chưa sẵn sàng | ✅ Done | `isReady()` check → empty queue response |

> **Lưu ý về storage:** Implementation dùng Redis thay vì Postgres. Queue data được lưu trong `ai:triage:queue` list và `ai:triage:status:{findingId}` hash. Thiết kế này đơn giản hơn và không cần DB migration trong môi trường development.

---

## 1. Tóm tắt Giải pháp

CR-014 là implementation chi tiết nhất trong batch này. Thay đổi bao gồm:

| Layer | Thay đổi |
|---|---|
| **DB** | Migration: thêm `human_decision`, `human_note`, `reviewed_by`, `reviewed_at`, `ai_result` JSONB |
| **Domain** | Full `TriageQueueItem` struct với AI result + human decision |
| **Repository** | `GetQueueItems` với filters, `GetQueueStats` SQL aggregation |
| **Handler** | `GetTriageQueue` (full schema), `ReviewFinding` (409/force/404) |
| **Gateway** | Verify route ordering: `/queue` trước `/{findingId}` |

---

## 2. DB Migration

### File: `services/ai-service/migrations/20260619_001_triage_queue_human_decision.sql`

```sql
-- Step 1: Thêm AI result columns nếu chưa tồn tại
ALTER TABLE triage_queue
    ADD COLUMN IF NOT EXISTS remarks         VARCHAR(20)
        CHECK (remarks IN ('Confirmed', 'FalsePositive', 'NotAffected', 'Unexplored')),
    ADD COLUMN IF NOT EXISTS confidence      DECIMAL(5,4)
        CHECK (confidence BETWEEN 0 AND 1),
    ADD COLUMN IF NOT EXISTS justification   TEXT,
    ADD COLUMN IF NOT EXISTS actions         TEXT[]       NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS ai_generated_at TIMESTAMPTZ;

-- Step 2: Thêm human decision columns
ALTER TABLE triage_queue
    ADD COLUMN IF NOT EXISTS human_decision  VARCHAR(20)
        CHECK (human_decision IN ('accepted', 'overridden', 'rejected')),
    ADD COLUMN IF NOT EXISTS human_note      TEXT,
    ADD COLUMN IF NOT EXISTS reviewed_by     VARCHAR(255),
    ADD COLUMN IF NOT EXISTS reviewed_at     TIMESTAMPTZ;

-- Step 3: Thêm enrichment fields từ finding
ALTER TABLE triage_queue
    ADD COLUMN IF NOT EXISTS finding_title   TEXT,
    ADD COLUMN IF NOT EXISTS cve_id          VARCHAR(50),
    ADD COLUMN IF NOT EXISTS severity        VARCHAR(20) NOT NULL DEFAULT 'Medium',
    ADD COLUMN IF NOT EXISTS updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- Step 4: Indexes cho filter + stats queries
CREATE INDEX IF NOT EXISTS idx_triage_human_decision
    ON triage_queue(human_decision);
CREATE INDEX IF NOT EXISTS idx_triage_reviewed_at
    ON triage_queue(reviewed_at DESC);
CREATE INDEX IF NOT EXISTS idx_triage_severity
    ON triage_queue(severity);
CREATE INDEX IF NOT EXISTS idx_triage_remarks
    ON triage_queue(remarks);
CREATE INDEX IF NOT EXISTS idx_triage_finding_id
    ON triage_queue(finding_id);

-- Step 5: updated_at trigger
CREATE OR REPLACE FUNCTION update_triage_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER triage_queue_updated_at
    BEFORE UPDATE ON triage_queue
    FOR EACH ROW EXECUTE FUNCTION update_triage_updated_at();
```

**Full table schema sau migration:**

```sql
-- Tham khảo: schema đầy đủ sau migration
CREATE TABLE triage_queue (
    -- Identity
    id              VARCHAR(36) PRIMARY KEY DEFAULT gen_random_uuid()::text,
    finding_id      VARCHAR(36) NOT NULL UNIQUE,
    finding_title   TEXT,
    cve_id          VARCHAR(50),
    severity        VARCHAR(20) NOT NULL DEFAULT 'Medium',

    -- AI Result fields (denormalized từ LLM output)
    remarks         VARCHAR(20) NOT NULL DEFAULT 'Unexplored'
        CHECK (remarks IN ('Confirmed', 'FalsePositive', 'NotAffected', 'Unexplored')),
    confidence      DECIMAL(5,4) NOT NULL DEFAULT 0
        CHECK (confidence BETWEEN 0 AND 1),
    justification   TEXT NOT NULL DEFAULT '',
    actions         TEXT[] NOT NULL DEFAULT '{}',
    ai_generated_at TIMESTAMPTZ,

    -- Human Decision (nullable = pending)
    human_decision  VARCHAR(20)
        CHECK (human_decision IN ('accepted', 'overridden', 'rejected')),
    human_note      TEXT,
    reviewed_by     VARCHAR(255),   -- email của reviewer
    reviewed_at     TIMESTAMPTZ,

    -- Metadata
    queued_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

---

## 3. Domain Model

### File: `services/ai-service/internal/domain/triage.go`

```go
package domain

import "time"

// AIResult — output từ LLM triage
type AIResult struct {
    Remarks       string     `json:"remarks"`        // "Confirmed" | "FalsePositive" | "NotAffected" | "Unexplored"
    Confidence    float64    `json:"confidence"`     // 0.0 - 1.0
    Justification string     `json:"justification"`
    Actions       []string   `json:"actions"`
    GeneratedAt   time.Time  `json:"generated_at"`
}

// TriageQueueItem — full domain entity
type TriageQueueItem struct {
    FindingID    string     `json:"finding_id"`
    FindingTitle string     `json:"finding_title"`
    CveID        *string    `json:"cve_id"`          // nullable
    Severity     string     `json:"severity"`
    AIResult     *AIResult  `json:"ai_result"`

    // Human decision fields — nullable = pending
    HumanDecision *string    `json:"human_decision"` // "accepted"|"overridden"|"rejected"|null
    HumanNote     *string    `json:"human_note"`
    ReviewedBy    *string    `json:"reviewed_by"`    // email
    ReviewedAt    *time.Time `json:"reviewed_at"`

    QueuedAt  time.Time `json:"queued_at,omitempty"`
    UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// QueueStats — tính từ DB, KHÔNG hardcode
type QueueStats struct {
    Pending          int     `json:"pending"`
    AcceptedToday    int     `json:"accepted_today"`
    AvgConfidence    float64 `json:"avg_confidence"`      // 0-100 (percent)
    FalsePositiveRate float64 `json:"false_positive_rate"` // percentage
}

// AITriageQueueResponse — full response schema
type AITriageQueueResponse struct {
    Items []TriageQueueItem `json:"items"`
    Total int               `json:"total"`
    Stats *QueueStats       `json:"stats"`
}

// TriageFilter — filter params từ query string
type TriageFilter struct {
    Status   string // "pending" | "accepted" | "overridden" | "rejected" | "all"
    Severity string // "Critical" | "High" | "Medium" | "Low"
    Remarks  string // "Confirmed" | "FalsePositive" | "NotAffected" | "Unexplored"
    Page     int
    PageSize int    // max 100
}

// HumanDecisionUpdate — payload để update human decision
type HumanDecisionUpdate struct {
    Decision   string
    Note       string
    ReviewedBy string
    ReviewedAt time.Time
}

// ReviewTriageRequest — request body
type ReviewTriageRequest struct {
    Decision string  `json:"decision"` // required
    Note     *string `json:"note"`     // optional, max 1000 chars
}
```

---

## 4. Repository

### File: `services/ai-service/internal/infra/postgres/triage_repo.go`

```go
package postgres

import (
    "context"
    "fmt"
    "strings"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
    "your-project/ai-service/internal/domain"
)

type TriageRepository struct {
    db *pgxpool.Pool
}

// GetQueueItems — với filtering và pagination
func (r *TriageRepository) GetQueueItems(ctx context.Context, f domain.TriageFilter) ([]domain.TriageQueueItem, int, error) {
    baseQuery := `
        SELECT finding_id, finding_title, cve_id, severity,
               remarks, confidence, justification, actions, ai_generated_at,
               human_decision, human_note, reviewed_by, reviewed_at,
               queued_at, updated_at
        FROM triage_queue
        WHERE 1=1
    `
    args := []interface{}{}
    idx := 1

    // Filter: status → human_decision
    switch f.Status {
    case "pending":
        baseQuery += " AND human_decision IS NULL"
    case "accepted":
        baseQuery += fmt.Sprintf(" AND human_decision = $%d", idx)
        args = append(args, "accepted")
        idx++
    case "overridden":
        baseQuery += fmt.Sprintf(" AND human_decision = $%d", idx)
        args = append(args, "overridden")
        idx++
    case "rejected":
        baseQuery += fmt.Sprintf(" AND human_decision = $%d", idx)
        args = append(args, "rejected")
        idx++
    // "all" — không thêm filter
    }

    if f.Severity != "" {
        baseQuery += fmt.Sprintf(" AND severity = $%d", idx)
        args = append(args, f.Severity)
        idx++
    }

    if f.Remarks != "" {
        baseQuery += fmt.Sprintf(" AND remarks = $%d", idx)
        args = append(args, f.Remarks)
        idx++
    }

    // Count total
    countQuery := "SELECT COUNT(*) FROM triage_queue WHERE 1=1" + extractWhereClause(baseQuery)
    var total int
    r.db.QueryRow(ctx, countQuery, args...).Scan(&total)

    // Paginate
    pageSize := f.PageSize
    if pageSize <= 0 || pageSize > 100 {
        pageSize = 20
    }
    page := f.Page
    if page <= 0 {
        page = 1
    }
    offset := (page - 1) * pageSize

    baseQuery += fmt.Sprintf(" ORDER BY queued_at DESC LIMIT $%d OFFSET $%d", idx, idx+1)
    args = append(args, pageSize, offset)

    rows, err := r.db.Query(ctx, baseQuery, args...)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()

    var items []domain.TriageQueueItem
    for rows.Next() {
        var item domain.TriageQueueItem
        var aiResult domain.AIResult
        var actionsArr []string

        err := rows.Scan(
            &item.FindingID, &item.FindingTitle, &item.CveID, &item.Severity,
            &aiResult.Remarks, &aiResult.Confidence, &aiResult.Justification,
            &actionsArr, &aiResult.GeneratedAt,
            &item.HumanDecision, &item.HumanNote, &item.ReviewedBy, &item.ReviewedAt,
            &item.QueuedAt, &item.UpdatedAt,
        )
        if err != nil {
            return nil, 0, err
        }

        aiResult.Actions = actionsArr
        if aiResult.Remarks != "" {
            item.AIResult = &aiResult
        }
        items = append(items, item)
    }
    return items, total, nil
}

// GetQueueStats — compute stats từ DB, không hardcode
func (r *TriageRepository) GetQueueStats(ctx context.Context) (*domain.QueueStats, error) {
    row := r.db.QueryRow(ctx, `
        SELECT
            COUNT(*) FILTER (WHERE human_decision IS NULL)
                AS pending,
            COUNT(*) FILTER (
                WHERE human_decision = 'accepted'
                  AND reviewed_at >= CURRENT_DATE
                  AND reviewed_at <  CURRENT_DATE + INTERVAL '1 day'
            )                                                           AS accepted_today,
            COALESCE(ROUND(AVG(confidence) * 100, 1), 0)               AS avg_confidence,
            COALESCE(
                ROUND(
                    100.0 * COUNT(*) FILTER (WHERE remarks = 'FalsePositive')
                    / NULLIF(COUNT(*), 0)
                , 1),
            0)                                                          AS false_positive_rate
        FROM triage_queue
    `)

    var stats domain.QueueStats
    err := row.Scan(&stats.Pending, &stats.AcceptedToday, &stats.AvgConfidence, &stats.FalsePositiveRate)
    if err != nil {
        return nil, err
    }
    return &stats, nil
}

// GetQueueItem — lấy 1 item theo finding_id (check existence)
func (r *TriageRepository) GetQueueItem(ctx context.Context, findingID string) (*domain.TriageQueueItem, error) {
    var item domain.TriageQueueItem
    var aiResult domain.AIResult
    var actionsArr []string

    err := r.db.QueryRow(ctx, `
        SELECT finding_id, finding_title, cve_id, severity,
               remarks, confidence, justification, actions, ai_generated_at,
               human_decision, human_note, reviewed_by, reviewed_at
        FROM triage_queue
        WHERE finding_id = $1
    `, findingID).Scan(
        &item.FindingID, &item.FindingTitle, &item.CveID, &item.Severity,
        &aiResult.Remarks, &aiResult.Confidence, &aiResult.Justification,
        &actionsArr, &aiResult.GeneratedAt,
        &item.HumanDecision, &item.HumanNote, &item.ReviewedBy, &item.ReviewedAt,
    )
    if err != nil {
        return nil, err // pgx.ErrNoRows → handler trả 404
    }
    aiResult.Actions = actionsArr
    item.AIResult = &aiResult
    return &item, nil
}

// UpdateHumanDecision — ghi human decision + audit trail
func (r *TriageRepository) UpdateHumanDecision(ctx context.Context, findingID string, upd domain.HumanDecisionUpdate) error {
    _, err := r.db.Exec(ctx, `
        UPDATE triage_queue
        SET human_decision = $1,
            human_note     = $2,
            reviewed_by    = $3,
            reviewed_at    = $4,
            updated_at     = NOW()
        WHERE finding_id = $5
    `, upd.Decision, upd.Note, upd.ReviewedBy, upd.ReviewedAt, findingID)
    return err
}
```

---

## 5. Handler

### File: `services/ai-service/internal/delivery/http/triage_handler.go`

```go
package http

import (
    "context"
    "encoding/json"
    "net/http"
    "time"

    "github.com/jackc/pgx/v5"
    "your-project/ai-service/internal/domain"
)

type TriageHandler struct {
    repo           TriageRepository
    findingClient  FindingServiceClient // optional — propagate decision
}

// GetTriageQueue godoc
// GET /api/v1/ai/triage/queue
// Query: status, severity, remarks, page, page_size
func (h *TriageHandler) GetTriageQueue(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    q := r.URL.Query()

    filter := domain.TriageFilter{
        Status:   q.Get("status"),
        Severity: q.Get("severity"),
        Remarks:  q.Get("remarks"),
        Page:     parseIntDefault(q.Get("page"), 1),
        PageSize: min(parseIntDefault(q.Get("page_size"), 20), 100),
    }

    items, total, err := h.repo.GetQueueItems(ctx, filter)
    if err != nil {
        jsonError(w, 500, "INTERNAL", "Failed to fetch triage queue")
        return
    }

    stats, err := h.repo.GetQueueStats(ctx)
    if err != nil {
        // Degraded — trả items mà không có stats
        stats = &domain.QueueStats{}
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(domain.AITriageQueueResponse{
        Items: items,
        Total: total,
        Stats: stats,
    })
}

// ReviewFinding godoc
// POST /api/v1/ai/triage/{findingId}/review
// Query: ?force=true (override đã-reviewed)
// Body: { decision, note? }
func (h *TriageHandler) ReviewFinding(w http.ResponseWriter, r *http.Request) {
    ctx    := r.Context()
    findingID := r.PathValue("findingId")
    force  := r.URL.Query().Get("force") == "true"

    // 1. Parse + validate body
    var req domain.ReviewTriageRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        jsonError(w, 400, "INVALID_BODY", "Invalid JSON request body")
        return
    }

    validDecisions := map[string]bool{
        "accepted": true, "overridden": true, "rejected": true,
    }
    if !validDecisions[req.Decision] {
        jsonError(w, 400, "INVALID_DECISION",
            "decision must be one of: accepted, overridden, rejected")
        return
    }

    // Note max length 1000 chars
    if req.Note != nil && len(*req.Note) > 1000 {
        jsonError(w, 400, "VALIDATION_ERROR", "note must be <= 1000 characters")
        return
    }

    // 2. Check finding exists
    existing, err := h.repo.GetQueueItem(ctx, findingID)
    if err != nil {
        if err == pgx.ErrNoRows {
            jsonError(w, 404, "NOT_FOUND", "Finding not in triage queue")
        } else {
            jsonError(w, 500, "INTERNAL", "Database error")
        }
        return
    }

    // 3. Check 409 Conflict — đã review rồi
    if existing.HumanDecision != nil && !force {
        jsonError(w, 409, "ALREADY_REVIEWED",
            "This finding has already been reviewed. Use ?force=true to override.")
        return
    }

    // 4. Extract reviewer identity từ JWT header (injected bởi gateway)
    reviewer := r.Header.Get("X-User-Email")
    if reviewer == "" {
        reviewer = r.Header.Get("X-User-ID")
    }

    // 5. Persist human decision
    note := ""
    if req.Note != nil {
        note = *req.Note
    }

    err = h.repo.UpdateHumanDecision(ctx, findingID, domain.HumanDecisionUpdate{
        Decision:   req.Decision,
        Note:       note,
        ReviewedBy: reviewer,
        ReviewedAt: time.Now().UTC(),
    })
    if err != nil {
        jsonError(w, 500, "INTERNAL", "Failed to save decision")
        return
    }

    // 6. (Optional) Propagate decision to finding-service
    if h.findingClient != nil {
        go h.propagateDecision(context.Background(), findingID, existing.AIResult, req.Decision)
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// propagateDecision — cập nhật finding status nếu decision = "accepted"
func (h *TriageHandler) propagateDecision(ctx context.Context, findingID string, aiResult *domain.AIResult, decision string) {
    if aiResult == nil || decision != "accepted" {
        return
    }
    switch aiResult.Remarks {
    case "FalsePositive":
        // Auto-close finding as false positive
        h.findingClient.UpdateStatus(ctx, findingID, "false_positive")
    case "Confirmed":
        // Tag finding as AI-confirmed (không auto-close)
        h.findingClient.AddTag(ctx, findingID, "ai-confirmed")
    }
}

// Helper
func parseIntDefault(s string, def int) int {
    if s == "" {
        return def
    }
    n, err := strconv.Atoi(s)
    if err != nil || n <= 0 {
        return def
    }
    return n
}

func jsonError(w http.ResponseWriter, status int, code, message string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(map[string]string{
        "error":   code,
        "message": message,
    })
}
```

---

## 6. Gateway Routing

### File: `apps/osv/internal/gateway/router.go`

```go
// ⚠️ CRITICAL: /queue phải đăng ký TRƯỚC /{findingId} 
// để tránh "queue" bị parse thành findingId

// AI Triage routes (theo Go 1.22 ServeMux longest-match)
mux.Handle("GET /api/v1/ai/triage/queue",
    protected(proxy.Forward("ai-service:9103")))

mux.Handle("POST /api/v1/ai/triage/{findingId}/review",
    protected(proxy.Forward("ai-service:9103")))

mux.Handle("GET /api/v1/ai/triage/{findingId}",
    protected(proxy.Forward("ai-service:9103")))

mux.Handle("POST /api/v1/ai/triage/{findingId}",
    protected(proxy.Forward("ai-service:9103")))
```

> **Lý do:** Go 1.22 ServeMux dùng longest-match, nhưng một số phiên bản có thể conflict nếu `/queue` và `/{id}` ở cùng method. Luôn đăng ký literal paths trước wildcard paths.

---

## 7. Tests

```go
// services/ai-service/internal/delivery/http/triage_handler_test.go

func TestGetTriageQueue_FullSchema(t *testing.T) {
    // Seed: 3 items trong triage_queue
    resp := GET("/api/v1/ai/triage/queue")
    var result domain.AITriageQueueResponse
    json.Unmarshal(resp.Body, &result)

    assert.NotEmpty(t, result.Items)
    assert.Greater(t, result.Total, 0)
    assert.NotNil(t, result.Stats)

    item := result.Items[0]
    assert.NotNil(t, item.AIResult)
    assert.Contains(t, []string{"Confirmed", "FalsePositive", "NotAffected", "Unexplored"},
        item.AIResult.Remarks)
    assert.GreaterOrEqual(t, item.AIResult.Confidence, 0.0)
    assert.LessOrEqual(t, item.AIResult.Confidence, 1.0)
}

func TestGetTriageQueue_FilterPending(t *testing.T) {
    resp := GET("/api/v1/ai/triage/queue?status=pending")
    var result domain.AITriageQueueResponse
    json.Unmarshal(resp.Body, &result)
    for _, item := range result.Items {
        assert.Nil(t, item.HumanDecision)
    }
}

func TestReviewFinding_Success(t *testing.T) {
    // Seed: pending finding
    body := `{"decision": "accepted", "note": "Confirmed by team"}`
    resp := POST("/api/v1/ai/triage/F-001/review", body)
    assert.Equal(t, 200, resp.StatusCode)

    // Verify: queue phản ánh decision
    queueResp := GET("/api/v1/ai/triage/queue")
    // F-001 phải có human_decision = "accepted"
}

func TestReviewFinding_409_AlreadyReviewed(t *testing.T) {
    // Already reviewed finding
    resp := POST("/api/v1/ai/triage/F-001/review", `{"decision":"accepted"}`)
    assert.Equal(t, 409, resp.StatusCode)
    assert.Contains(t, resp.Body, "ALREADY_REVIEWED")
}

func TestReviewFinding_409_ForceOverride(t *testing.T) {
    resp := POST("/api/v1/ai/triage/F-001/review?force=true", `{"decision":"rejected"}`)
    assert.Equal(t, 200, resp.StatusCode)
}

func TestReviewFinding_404_NotFound(t *testing.T) {
    resp := POST("/api/v1/ai/triage/NONEXISTENT/review", `{"decision":"accepted"}`)
    assert.Equal(t, 404, resp.StatusCode)
}

func TestReviewFinding_400_InvalidDecision(t *testing.T) {
    resp := POST("/api/v1/ai/triage/F-001/review", `{"decision":"invalid"}`)
    assert.Equal(t, 400, resp.StatusCode)
}

func TestStats_Pending_DecreasesAfterReview(t *testing.T) {
    // Get initial stats
    before := GET("/api/v1/ai/triage/queue").Stats.Pending
    // Review one item
    POST("/api/v1/ai/triage/F-NEW/review", `{"decision":"accepted"}`)
    after := GET("/api/v1/ai/triage/queue").Stats.Pending
    assert.Equal(t, before-1, after)
}

func TestStats_AcceptedToday_IncreasesAfterAccept(t *testing.T) {
    before := GET("/api/v1/ai/triage/queue").Stats.AcceptedToday
    POST("/api/v1/ai/triage/F-ANOTHER/review", `{"decision":"accepted"}`)
    after := GET("/api/v1/ai/triage/queue").Stats.AcceptedToday
    assert.Equal(t, before+1, after)
}
```

---

## 8. AI Triage Flow (Đầy đủ)

```
Finding phát hiện (scan completed)
    │
    ▼
scan-service emit NATS: "scan.scan.completed"
    │
    ▼
ai-service subscribes → LLM triage (Ollama/OpenAI chain)
    │
    ▼
ai-service lưu vào triage_queue:
    - finding_id, finding_title, cve_id, severity
    - remarks, confidence, justification, actions
    - ai_generated_at = NOW()
    - human_decision = NULL (pending)
    │
    ▼
Frontend poll: GET /api/v1/ai/triage/queue?status=pending
    │
    ▼
Analyst review → POST /api/v1/ai/triage/{id}/review
    { decision: "accepted", note: "..." }
    │
    ▼
ai-service cập nhật:
    - human_decision = "accepted"
    - reviewed_by = email từ JWT
    - reviewed_at = NOW()
    │
    ├── (optional) propagate tới finding-service
    │       "FalsePositive" → finding status = false_positive
    │       "Confirmed"     → finding tag += "ai-confirmed"
    │
    ▼
stats.pending giảm, stats.accepted_today tăng
```

---

## 9. Acceptance Criteria Checklist

### Schema & Data
- [ ] `GET /api/v1/ai/triage/queue` → `{ items, total, stats }` — HTTP 200
- [ ] Mỗi item có `ai_result` với: `remarks`, `confidence`, `justification`, `actions`, `generated_at`
- [ ] `remarks` enum: `Confirmed | FalsePositive | NotAffected | Unexplored`
- [ ] `confidence` là float 0.0–1.0
- [ ] `human_decision` null (pending) hoặc `accepted | overridden | rejected`
- [ ] `reviewed_by` = email người review (từ JWT)

### Stats (computed từ DB)
- [ ] `pending` = số items có `human_decision IS NULL`
- [ ] `accepted_today` = accept trong ngày UTC hiện tại
- [ ] `avg_confidence` = avg * 100 (percent)
- [ ] `false_positive_rate` = % items `remarks = 'FalsePositive'`

### Filter
- [ ] `?status=pending` → chỉ items chưa review
- [ ] `?status=accepted` → chỉ items đã accept
- [ ] `?severity=Critical` → chỉ Critical
- [ ] `?remarks=FalsePositive` → chỉ FP

### Review
- [ ] `POST /review` với valid `decision` → HTTP 200
- [ ] `note` được lưu vào DB
- [ ] `reviewed_by` từ JWT, không nhận từ body
- [ ] 404 nếu finding không tồn tại trong queue
- [ ] 409 nếu đã reviewed (không có `?force`)
- [ ] `?force=true` → override, HTTP 200
- [ ] Invalid `decision` → HTTP 400

### DB
- [ ] Sau review: `GET queue` phản ánh `human_decision`
- [ ] `pending` giảm sau mỗi review
- [ ] `accepted_today` tăng khi `decision = "accepted"`
