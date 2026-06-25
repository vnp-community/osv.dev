# SOL-005: Fix Hardcoded Timestamp trong Report — finding-service

**CR:** CR-HC-005 | **Priority:** 🟠 High | **Sprint:** 1  
**Service:** `services/finding-service` | **Độ phức tạp:** Low (1 dòng fix + logic cải thiện)

---

## Implementation Status

**✅ IMPLEMENTED** — 2026-06-24
**Task:** TASK-HC-001
**Note:** Hardcoded date thay bằng time.Now()
**Build:** ✅ `go build ./...` passes

---

---

## Context phân tích code

**File:** `finding-service/internal/delivery/http/report_handler.go:109`

```go
// Đây là stub mode khi generateUC == nil
writeJSON(w, http.StatusAccepted, map[string]interface{}{
    "id":         reportID,
    "name":       req.Title,
    "type":       req.Format,
    "status":     "pending",
    "format":     req.Format,
    "title":      req.Title,
    "created_by": r.Header.Get("X-User-ID"),
    "created_at": fmt.Sprintf("%s", "2026-06-22T00:00:00Z"),  // ← BUG
    "message":    "Report generation queued...",
})
return
```

**Repository đã có:** `ReportRepository.FindByID()` và `ListByProduct()` nhưng `Create()` chưa có trong interface.

**Vấn đề kép:**
1. `created_at` hardcode sai ngày
2. Stub mode không lưu report vào DB → `GET /reports` không trả về report vừa tạo

---

## Solution

### Part A: Fix hardcoded date (Quick Fix — 1 dòng)

**File sửa:** `finding-service/internal/delivery/http/report_handler.go`

```diff
-"created_at": fmt.Sprintf("%s", "2026-06-22T00:00:00Z"),
+"created_at": time.Now().UTC().Format(time.RFC3339),
```

Thêm import `"time"` nếu chưa có.

### Part B: Persist report trong DB ngay cả khi generateUC == nil

**Bước 1: Thêm `Create` vào ReportRepository interface**

**File:** `finding-service/internal/domain/report/report.go` (hoặc ports file)

```go
type Repository interface {
    // Existing:
    FindByID(ctx context.Context, id string) (*Report, error)
    ListByProduct(ctx context.Context, productID, userID string, limit, offset int) ([]*Report, int, error)
    Delete(ctx context.Context, id string) error
    DeleteExpired(ctx context.Context) (int, error)
    
    // [FIX CR-HC-005] NEW:
    Create(ctx context.Context, r *Report) error
    UpdateStatus(ctx context.Context, id string, status Status, errorMsg string) error
}
```

**Bước 2: Implement `Create` trong PostgreSQL repo**

**File:** `finding-service/internal/infra/postgres/report_repo.go`

```go
// [FIX CR-HC-005] Implement Create — persist report record immediately
func (r *ReportRepo) Create(ctx context.Context, rep *report.Report) error {
    _, err := r.pool.Exec(ctx, `
        INSERT INTO reports (
            id, title, format, status, product_id, engagement_id,
            generated_by, created_at, updated_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
        ON CONFLICT (id) DO NOTHING
    `,
        rep.ID, rep.Title, rep.Format, rep.Status,
        rep.ProductID, rep.EngagementID, rep.GeneratedBy,
    )
    if err != nil {
        return fmt.Errorf("report_repo.Create: %w", err)
    }
    return nil
}

func (r *ReportRepo) UpdateStatus(ctx context.Context, id string, status report.Status, errMsg string) error {
    _, err := r.pool.Exec(ctx, `
        UPDATE reports SET status = $2, error_message = $3, updated_at = NOW()
        WHERE id = $1
    `, id, status, errMsg)
    return err
}
```

**Bước 3: Refactor handler — LUÔN persist, bất kể generateUC**

**File sửa:** `finding-service/internal/delivery/http/report_handler.go`

```go
func (h *ReportHandler) Create(w http.ResponseWriter, r *http.Request) {
    var req CreateReportRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSON(w, http.StatusBadRequest, apiErr("invalid request body"))
        return
    }

    userID := r.Header.Get("X-User-ID")

    // [FIX CR-HC-005] ALWAYS create report record in DB
    rep := &report.Report{
        ID:           uuid.New(),
        Title:        req.Title,
        Format:       report.ReportFormat(req.Format),
        Status:       report.StatusPending,
        ProductID:    req.ProductID,
        EngagementID: req.EngagementID,
        GeneratedBy:  userID,
        // CreatedAt auto-set by DB NOW()
    }

    if err := h.repo.Create(r.Context(), rep); err != nil {
        h.log.Error().Err(err).Msg("ReportHandler.Create: persist failed")
        writeJSON(w, http.StatusInternalServerError, apiErr("failed to create report"))
        return
    }

    // Trigger async generation if usecase available
    if h.generateUC != nil {
        go func() {
            ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
            defer cancel()
            if err := h.generateUC.Execute(ctx, reportuc.GenerateInput{
                ReportID:     rep.ID.String(),
                ProductID:    req.ProductID,
                Title:        req.Title,
                Format:       report.ReportFormat(req.Format),
                EngagementID: req.EngagementID,
                Severities:   req.Severities,
                ActiveOnly:   req.ActiveOnly,
                RequesterID:  userID,
            }); err != nil {
                h.log.Error().Err(err).Str("report_id", rep.ID.String()).
                    Msg("async report generation failed")
                _ = h.repo.UpdateStatus(context.Background(), rep.ID.String(),
                    report.StatusFailed, err.Error())
            }
        }()
    }

    // [FIX CR-HC-005] created_at từ time.Now(), không hardcode
    writeJSON(w, http.StatusAccepted, map[string]interface{}{
        "id":         rep.ID,
        "title":      rep.Title,
        "format":     rep.Format,
        "status":     rep.Status,
        "created_by": userID,
        "created_at": time.Now().UTC().Format(time.RFC3339),
        "message":    "Report queued. Download available when status = completed.",
    })
}
```

**Bước 4: Kiểm tra migration — thêm cột `error_message` nếu thiếu**

```sql
-- finding-service/migrations/add_report_error_col.sql (idempotent)
ALTER TABLE reports ADD COLUMN IF NOT EXISTS error_message TEXT;
```

---

## Files cần tạo/sửa

| Action | File |
|--------|------|
| MODIFY | `finding-service/internal/delivery/http/report_handler.go` — fix date + logic |
| MODIFY | `finding-service/internal/domain/report/report.go` — add Create/UpdateStatus to interface |
| MODIFY | `finding-service/internal/infra/postgres/report_repo.go` — implement Create/UpdateStatus |
| NEW | `finding-service/migrations/009_report_error_col.sql` |

---

## Verification

```bash
# Build
cd services/finding-service && go build ./...

# Test create report
curl -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title":"Test Report","format":"pdf","product_id":"..."}' \
  "https://c12.openledger.vn/api/v2/reports"
# Expect: created_at = current time, NOT "2026-06-22T00:00:00Z"

# Verify report is in DB
psql $DATABASE_URL -c "SELECT id, title, status, created_at FROM reports ORDER BY created_at DESC LIMIT 5;"

# GET report should find it
curl -H "Authorization: Bearer $TOKEN" "https://c12.openledger.vn/api/v2/reports/{id}"
# Expect: 200 (not 404)
```
