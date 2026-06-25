# TASK-HC-001: Fix Hardcoded Date trong Report Handler

**Status:** ✅ DONE  
**Sprint:** 1 | **Ước lượng:** 30 phút  
**Solution:** [SOL-005](../solutions/SOL-005-finding-report-date.md)  
**Service:** `services/finding-service`

---

## Mô tả

Handler `POST /api/v2/reports` đang trả response với `created_at: "2026-06-22T00:00:00Z"` hardcode. Cần sửa thành `time.Now().UTC()` và đảm bảo report record được persist vào DB.

---

## Acceptance Criteria

- [x] `created_at` trong response của `POST /api/v2/reports` = thời điểm thực tế (không phải `2026-06-22`)
- [x] Report record được INSERT vào table `reports` ngay khi request đến (trước khi generate async)
- [x] `GET /api/v2/reports/{id}` sau khi POST → trả 200 (không 404)
- [x] `go build ./...` pass trong `services/finding-service`

---

## Files cần sửa/tạo

| Action | File | Thay đổi |
|--------|------|---------|
| MODIFY | `services/finding-service/internal/delivery/http/report_handler.go` | Fix hardcode date + persist trước khi async |
| MODIFY | `services/finding-service/internal/domain/report/report.go` (hoặc interface file) | Thêm `Create(ctx, *Report) error` và `UpdateStatus(ctx, id, status, errMsg) error` vào interface |
| MODIFY | `services/finding-service/internal/infra/postgres/report_repo.go` | Implement `Create` và `UpdateStatus` |
| NEW | `services/finding-service/migrations/009_report_error_col.sql` | `ALTER TABLE reports ADD COLUMN IF NOT EXISTS error_message TEXT` |

---

## Bước thực thi

### 1. Tìm dòng hardcode
```bash
grep -n "2026-06-22" services/finding-service/internal/delivery/http/report_handler.go
```

### 2. Sửa dòng hardcode
Thay:
```go
"created_at": fmt.Sprintf("%s", "2026-06-22T00:00:00Z"),
```
Thành:
```go
"created_at": time.Now().UTC().Format(time.RFC3339),
```
Thêm import `"time"` nếu chưa có.

### 3. Kiểm tra ReportRepository interface
```bash
grep -n "Create\|UpdateStatus" services/finding-service/internal/domain/report/report.go
```
Nếu chưa có → thêm 2 methods vào interface.

### 4. Implement Create trong repo
```bash
grep -n "func.*ReportRepo.*Create\|func.*report.*repo.*Create" \
  services/finding-service/internal/infra/postgres/report_repo.go
```
Nếu chưa có → implement:
```go
func (r *ReportRepo) Create(ctx context.Context, rep *report.Report) error {
    _, err := r.pool.Exec(ctx, `
        INSERT INTO reports (id, title, format, status, product_id, engagement_id, generated_by, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
        ON CONFLICT (id) DO NOTHING
    `, rep.ID, rep.Title, rep.Format, rep.Status, rep.ProductID, rep.EngagementID, rep.GeneratedBy)
    if err != nil {
        return fmt.Errorf("report_repo.Create: %w", err)
    }
    return nil
}
```

### 5. Migration
```bash
psql $DATABASE_URL -f services/finding-service/migrations/009_report_error_col.sql
```

### 6. Build check
```bash
cd services/finding-service && go build ./...
```

---

## Verification

```bash
# Test 1: created_at không còn hardcode
TOKEN=$(curl -s -X POST https://c12.openledger.vn/api/v1/auth/login \
  -d '{"username":"admin","password":"admin"}' | jq -r '.access_token')

REPORT=$(curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title":"Test","format":"pdf","product_id":"00000000-0000-0000-0000-000000000001"}' \
  https://c12.openledger.vn/api/v2/reports)

echo $REPORT | jq '.created_at'
# PASS nếu không phải "2026-06-22T00:00:00Z"

# Test 2: report tồn tại trong DB
REPORT_ID=$(echo $REPORT | jq -r '.id')
curl -s -H "Authorization: Bearer $TOKEN" \
  "https://c12.openledger.vn/api/v2/reports/$REPORT_ID" | jq '.id'
# PASS nếu trả về ID (không phải null/404)
```
