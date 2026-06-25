# TASK-P0-001 — Nil-check trong ReportHandler

**Bug:** MOCK-002  
**Priority:** 🔴 P0 — Production Crash Risk  
**Effort:** ~15 phút  
**Service:** `finding-service`  
**Loại thay đổi:** Code fix only (không cần DB migration)

---

## Mục tiêu

`ReportHandler.Create()` và `ReportHandler.Download()` sẽ panic (nil pointer dereference) khi `generateUC == nil` hoặc `storage == nil` — xảy ra trong embedded mode vì `nilReportRepo` được wire. Cần thêm nil-check trả về `503 Service Unavailable` thay vì crash.

---

## Preconditions

- [ ] Đọc file hiện tại: `services/finding-service/internal/delivery/http/report_handler.go`
- [ ] Xác định tất cả methods có dereference `h.generateUC` và `h.storage`

---

## Steps

### Step 1 — Đọc file hiện tại để hiểu cấu trúc

```
File: services/finding-service/internal/delivery/http/report_handler.go
```

Tìm các patterns:
- `h.generateUC.` — method nào gọi generateUC
- `h.storage.` — method nào gọi storage

### Step 2 — Thêm nil-check vào method `Create()`

Tìm function signature `func (h *ReportHandler) Create(` và thêm nil-check **ngay đầu hàm**, trước khi decode body:

```go
func (h *ReportHandler) Create(w http.ResponseWriter, r *http.Request) {
    // MOCK-002 FIX: nil-check trước khi dereference generateUC
    if h.generateUC == nil {
        // Tùy codebase — dùng cùng helper writeJSON/respondJSON/respondError đang có
        writeJSON(w, http.StatusServiceUnavailable, map[string]string{
            "error": "report generation not configured: MinIO storage required",
        })
        return
    }
    // ... phần còn lại giữ nguyên
```

> **Lưu ý**: Dùng đúng tên helper của codebase (có thể là `writeJSON`, `respondJSON`, `h.respond`, `respond`, `writeError`...). Đọc file để biết tên hàm chính xác.

### Step 3 — Thêm nil-check vào method `Download()` (hoặc `DownloadFile()`)

Tìm function xử lý download/presigned URL và thêm nil-check cho `h.storage`:

```go
func (h *ReportHandler) Download(w http.ResponseWriter, r *http.Request) {
    // MOCK-002 FIX: nil-check trước khi dereference storage
    if h.storage == nil {
        writeJSON(w, http.StatusServiceUnavailable, map[string]string{
            "error": "report download not configured: MinIO storage required",
        })
        return
    }
    // ... phần còn lại giữ nguyên
```

### Step 4 — Kiểm tra các methods khác (nếu có)

Grep tìm tất cả dereference:
```bash
grep -n "h\.generateUC\." services/finding-service/internal/delivery/http/report_handler.go
grep -n "h\.storage\." services/finding-service/internal/delivery/http/report_handler.go
```

Nếu có method khác (ví dụ `Generate`, `Render`, `GetTemplate`...) cũng dereference hai field này, thêm nil-check tương tự.

---

## Acceptance Criteria

- [ ] `POST /api/v1/reports` (hoặc `/api/v2/reports`) khi `generateUC == nil` → trả `503` với JSON error, không panic
- [ ] `GET /api/v1/reports/{id}/download` khi `storage == nil` → trả `503` với JSON error, không panic
- [ ] Tất cả methods khác của `ReportHandler` không còn dereference nil
- [ ] `go build ./services/finding-service/...` — build thành công
- [ ] `go vet ./services/finding-service/...` — không có warning

---

## Test Commands

```bash
# Build check
cd /Users/binhnt/Lab/sec/cve/osv.dev
go build ./services/finding-service/...

# Grep kiểm tra nil-check đã được thêm
grep -n "generateUC == nil\|storage == nil" services/finding-service/internal/delivery/http/report_handler.go

# Run existing tests
go test ./services/finding-service/internal/delivery/http/... -v -run Report
```

---

## Rollback

Nếu fix gây lỗi build: revert chỉ file `report_handler.go` về trạng thái trước.
