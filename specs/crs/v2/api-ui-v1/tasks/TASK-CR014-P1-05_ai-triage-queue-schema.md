# TASK-CR014-P1-05 — AI Triage Queue: DB Migration + Full Schema

**Phase:** Phase 2 — Blocking UI  
**Nguồn giải pháp:** [`solutions/SOL-014`](../solutions/SOL-014-ai-triage-queue-human-decision.md)  
**Ưu tiên:** 🔴 P1 — Review buttons không có tác dụng, stats hardcode  
**Phụ thuộc:** Không có  
**Status:** ✅ **DONE** — 2026-06-19  

---

## Mục tiêu

1. DB migration: thêm `human_decision`, `human_note`, `reviewed_by`, `reviewed_at` vào `triage_queue`
2. `GET /api/v1/ai/triage/queue` → full `AITriageQueueResponse` với `ai_result` + `stats`
3. `POST /api/v1/ai/triage/{findingId}/review` → lưu decision + 409 conflict handling

---

## Điều tra trước khi code

```bash
# 1. Xem triage_queue table hiện tại
docker exec osv-backend-postgres-1 psql -U osv -d osv \
  -c "\d triage_queue" 2>/dev/null || echo "TABLE NOT FOUND"

# 2. Xem handler triage hiện tại
grep -rn "triage\|TriageQueue\|GetTriageQueue" \
  services/ai-service/ --include="*.go" -l

# 3. Xem response hiện tại
curl -s -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/ai/triage/queue | jq .

# 4. Xem gateway routes AI
grep -n "ai/triage" apps/osv/internal/gateway/router.go
```

---

## Bước 1: DB Migration

**File:** `services/ai-service/migrations/20260619_001_triage_queue_human_decision.sql`

```sql
-- Step 1: Tạo table nếu chưa có
CREATE TABLE IF NOT EXISTS triage_queue (
    id              VARCHAR(36) PRIMARY KEY DEFAULT gen_random_uuid()::text,
    finding_id      VARCHAR(36) NOT NULL UNIQUE,
    finding_title   TEXT,
    cve_id          VARCHAR(50),
    severity        VARCHAR(20) NOT NULL DEFAULT 'Medium',
    remarks         VARCHAR(20) NOT NULL DEFAULT 'Unexplored'
        CHECK (remarks IN ('Confirmed', 'FalsePositive', 'NotAffected', 'Unexplored')),
    confidence      DECIMAL(5,4) NOT NULL DEFAULT 0
        CHECK (confidence BETWEEN 0 AND 1),
    justification   TEXT NOT NULL DEFAULT '',
    actions         TEXT[] NOT NULL DEFAULT '{}',
    ai_generated_at TIMESTAMPTZ,
    queued_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Step 2: Thêm human decision columns
ALTER TABLE triage_queue
    ADD COLUMN IF NOT EXISTS human_decision VARCHAR(20)
        CHECK (human_decision IN ('accepted', 'overridden', 'rejected')),
    ADD COLUMN IF NOT EXISTS human_note     TEXT,
    ADD COLUMN IF NOT EXISTS reviewed_by    VARCHAR(255),
    ADD COLUMN IF NOT EXISTS reviewed_at    TIMESTAMPTZ;

-- Step 3: Indexes
CREATE INDEX IF NOT EXISTS idx_triage_human_decision ON triage_queue(human_decision);
CREATE INDEX IF NOT EXISTS idx_triage_reviewed_at    ON triage_queue(reviewed_at DESC);
CREATE INDEX IF NOT EXISTS idx_triage_severity       ON triage_queue(severity);
CREATE INDEX IF NOT EXISTS idx_triage_remarks        ON triage_queue(remarks);
```

```bash
# Chạy migration
docker exec osv-backend-postgres-1 psql -U osv -d osv \
  -f /migrations/20260619_001_triage_queue_human_decision.sql

# Verify
docker exec osv-backend-postgres-1 psql -U osv -d osv \
  -c "SELECT column_name FROM information_schema.columns \
      WHERE table_name='triage_queue' \
      AND column_name IN ('human_decision','human_note','reviewed_by','reviewed_at');"
```

---

## Bước 2: Repository Methods

**Tìm repo:**
```bash
grep -rn "triage_queue\|TriageRepo\|GetQueueItem" \
  services/ai-service/ --include="*.go" -l
```

**Thêm/sửa repository:**

```go
// GetQueueStats — tính từ DB, KHÔNG hardcode
func (r *TriageRepo) GetQueueStats(ctx context.Context) (map[string]interface{}, error) {
    row := r.db.QueryRow(ctx, `
        SELECT
            COUNT(*) FILTER (WHERE human_decision IS NULL)               AS pending,
            COUNT(*) FILTER (
                WHERE human_decision = 'accepted'
                  AND reviewed_at >= CURRENT_DATE
            )                                                            AS accepted_today,
            COALESCE(ROUND(AVG(confidence) * 100, 1), 0)                AS avg_confidence,
            COALESCE(ROUND(
                100.0 * COUNT(*) FILTER (WHERE remarks = 'FalsePositive')
                / NULLIF(COUNT(*), 0)
            , 1), 0)                                                     AS false_positive_rate
        FROM triage_queue
    `)

    var pending, acceptedToday int
    var avgConf, fpRate float64
    if err := row.Scan(&pending, &acceptedToday, &avgConf, &fpRate); err != nil {
        return map[string]interface{}{
            "pending": 0, "accepted_today": 0,
            "avg_confidence": 0.0, "false_positive_rate": 0.0,
        }, nil
    }
    return map[string]interface{}{
        "pending":             pending,
        "accepted_today":      acceptedToday,
        "avg_confidence":      avgConf,
        "false_positive_rate": fpRate,
    }, nil
}

// GetQueueItem — lấy 1 item để check trước review
func (r *TriageRepo) GetQueueItem(ctx context.Context, findingID string) (map[string]interface{}, error) {
    row := r.db.QueryRow(ctx, `
        SELECT finding_id, human_decision, remarks
        FROM triage_queue WHERE finding_id = $1
    `, findingID)

    var fid string
    var humanDecision *string
    var remarks string
    if err := row.Scan(&fid, &humanDecision, &remarks); err != nil {
        return nil, err // pgx.ErrNoRows → 404
    }
    return map[string]interface{}{
        "finding_id":     fid,
        "human_decision": humanDecision,
        "remarks":        remarks,
    }, nil
}

// UpdateHumanDecision — ghi decision
func (r *TriageRepo) UpdateHumanDecision(ctx context.Context, findingID, decision, note, reviewer string) error {
    _, err := r.db.Exec(ctx, `
        UPDATE triage_queue
        SET human_decision = $1,
            human_note     = $2,
            reviewed_by    = $3,
            reviewed_at    = NOW(),
            updated_at     = NOW()
        WHERE finding_id = $4
    `, decision, note, reviewer, findingID)
    return err
}
```

---

## Bước 3: Handler Updates

**Tìm handler:**
```bash
grep -rn "GetTriageQueue\|TriageQueue\|triage/queue" \
  services/ai-service/ --include="*.go" -n
```

**Update GetTriageQueue:**

```go
// GET /api/v1/ai/triage/queue
func (h *TriageHandler) GetTriageQueue(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    q   := r.URL.Query()

    // Parse filters
    status   := q.Get("status")   // "pending"|"accepted"|"overridden"|"rejected"|"all"
    severity := q.Get("severity") // "Critical"|"High"|...
    remarks  := q.Get("remarks")  // "Confirmed"|"FalsePositive"|...
    page     := parseIntDefault(q.Get("page"), 1)
    pageSize := min(parseIntDefault(q.Get("page_size"), 20), 100)

    // Build query
    baseQ := `
        SELECT finding_id, COALESCE(finding_title,''), cve_id, severity,
               remarks, confidence, COALESCE(justification,''), actions,
               ai_generated_at, human_decision, human_note, reviewed_by, reviewed_at
        FROM triage_queue WHERE 1=1
    `
    args := []interface{}{}
    idx  := 1

    switch status {
    case "pending":
        baseQ += " AND human_decision IS NULL"
    case "accepted", "overridden", "rejected":
        baseQ += fmt.Sprintf(" AND human_decision = $%d", idx)
        args = append(args, status); idx++
    }
    if severity != "" {
        baseQ += fmt.Sprintf(" AND severity = $%d", idx)
        args = append(args, severity); idx++
    }
    if remarks != "" {
        baseQ += fmt.Sprintf(" AND remarks = $%d", idx)
        args = append(args, remarks); idx++
    }

    // Count
    var total int
    countQ := strings.Replace(baseQ,
        "SELECT finding_id, COALESCE(finding_title,''), cve_id, severity, remarks, confidence, COALESCE(justification,''), actions, ai_generated_at, human_decision, human_note, reviewed_by, reviewed_at",
        "SELECT COUNT(*)", 1)
    h.db.QueryRow(ctx, countQ, args...).Scan(&total)

    // Paginate
    baseQ += fmt.Sprintf(" ORDER BY queued_at DESC LIMIT $%d OFFSET $%d", idx, idx+1)
    args = append(args, pageSize, (page-1)*pageSize)

    rows, err := h.db.Query(ctx, baseQ, args...)
    if err != nil {
        respondError(w, 500, "failed to query triage queue")
        return
    }
    defer rows.Close()

    var items []map[string]interface{}
    for rows.Next() {
        var fid, title, severity, remarks, justification string
        var cveID *string
        var confidence float64
        var actions []string
        var aiGenAt *time.Time
        var humanDecision, humanNote, reviewedBy *string
        var reviewedAt *time.Time

        rows.Scan(&fid, &title, &cveID, &severity,
            &remarks, &confidence, &justification, &actions,
            &aiGenAt, &humanDecision, &humanNote, &reviewedBy, &reviewedAt)

        item := map[string]interface{}{
            "finding_id":    fid,
            "finding_title": title,
            "cve_id":        cveID,
            "severity":      severity,
            "ai_result": map[string]interface{}{
                "remarks":       remarks,
                "confidence":    confidence,
                "justification": justification,
                "actions":       actions,
                "generated_at":  aiGenAt,
            },
            "human_decision": humanDecision,
            "human_note":     humanNote,
            "reviewed_by":    reviewedBy,
            "reviewed_at":    reviewedAt,
        }
        items = append(items, item)
    }
    if items == nil {
        items = []map[string]interface{}{}
    }

    stats, _ := h.repo.GetQueueStats(ctx)

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "items": items,
        "total": total,
        "stats": stats,
    })
}
```

**Update ReviewFinding:**

```go
// POST /api/v1/ai/triage/{findingId}/review
func (h *TriageHandler) ReviewFinding(w http.ResponseWriter, r *http.Request) {
    ctx      := r.Context()
    findingID := r.PathValue("findingId") // Go 1.22+ hoặc chi.URLParam
    force    := r.URL.Query().Get("force") == "true"

    var req struct {
        Decision string  `json:"decision"`
        Note     *string `json:"note"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, 400, "invalid JSON body")
        return
    }

    // Validate decision enum
    valid := map[string]bool{"accepted": true, "overridden": true, "rejected": true}
    if !valid[req.Decision] {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(400)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "INVALID_DECISION",
            "message": "decision must be: accepted, overridden, or rejected",
        })
        return
    }

    // Check finding exists
    existing, err := h.repo.GetQueueItem(ctx, findingID)
    if err != nil {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(404)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "NOT_FOUND",
            "message": "Finding not in triage queue",
        })
        return
    }

    // 409 Conflict nếu đã reviewed
    if existing["human_decision"] != nil && !force {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(409)
        json.NewEncoder(w).Encode(map[string]string{
            "error":   "ALREADY_REVIEWED",
            "message": "Use ?force=true to override existing review",
        })
        return
    }

    // Extract reviewer từ JWT (injected bởi gateway)
    reviewer := r.Header.Get("X-User-Email")
    if reviewer == "" {
        reviewer = r.Header.Get("X-User-ID")
    }

    note := ""
    if req.Note != nil {
        note = *req.Note
    }

    if err := h.repo.UpdateHumanDecision(ctx, findingID, req.Decision, note, reviewer); err != nil {
        respondError(w, 500, "failed to save decision")
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
```

---

## Bước 4: Gateway Route Ordering

```bash
grep -n "ai/triage" apps/osv/internal/gateway/router.go
```

**Đảm bảo thứ tự đúng** (`/queue` TRƯỚC `/{findingId}`):

```go
// ⚠️ /queue PHẢI đăng ký TRƯỚC /{findingId}
mux.Handle("GET /api/v1/ai/triage/queue",
    protected(proxy.Forward("ai-service:9103")))

mux.Handle("POST /api/v1/ai/triage/{findingId}/review",
    protected(proxy.Forward("ai-service:9103")))

mux.Handle("GET /api/v1/ai/triage/{findingId}",
    protected(proxy.Forward("ai-service:9103")))
```

---

## Acceptance Criteria

- [ ] `GET /api/v1/ai/triage/queue` → `{ items, total, stats }` — HTTP 200
- [ ] Mỗi item có `ai_result.remarks`, `confidence` (0.0–1.0), `justification`, `actions`
- [ ] `human_decision` null nếu chưa review
- [ ] `stats.pending` = số items `human_decision IS NULL` trong DB
- [ ] `POST /review` với valid `decision` → HTTP 200
- [ ] `POST /review` với invalid `decision` → HTTP 400
- [ ] `POST /review` khi đã reviewed (không có `?force`) → HTTP 409
- [ ] `POST /review` với `?force=true` → HTTP 200, override
- [ ] `POST /review` với `findingId` không tồn tại → HTTP 404
- [ ] Sau review: `stats.pending` giảm

## Verification

```bash
TOKEN="<your-token>"

# Xem triage queue
curl -s https://c12.openledger.vn/api/v1/ai/triage/queue \
  -H "Authorization: Bearer $TOKEN" | jq '{total, stats, first: .items[0]}'

# Filter pending
curl -s "https://c12.openledger.vn/api/v1/ai/triage/queue?status=pending" \
  -H "Authorization: Bearer $TOKEN" | jq '.items | length'

# Review a finding
FINDING_ID=$(curl -s ... | jq -r '.items[0].finding_id')
curl -s -X POST \
  "https://c12.openledger.vn/api/v1/ai/triage/$FINDING_ID/review" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"decision":"accepted","note":"Confirmed by team"}' | jq .

# 409 test (review lại)
curl -s -X POST \
  "https://c12.openledger.vn/api/v1/ai/triage/$FINDING_ID/review" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"decision":"rejected"}' -o /dev/null -w "%{http_code}"
# Expected: 409

# Force override
curl -s -X POST \
  "https://c12.openledger.vn/api/v1/ai/triage/$FINDING_ID/review?force=true" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"decision":"rejected"}' -o /dev/null -w "%{http_code}"
# Expected: 200
```
