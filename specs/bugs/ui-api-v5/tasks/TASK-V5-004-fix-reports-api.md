# TASK-V5-004: Sửa Reports API — 500 và Missing Fields

## Bug 1: `GET /api/v1/reports` → 500

### Root Cause
`ReportRepo.ListByProduct()` query:
```sql
SELECT COUNT(*) FROM reports WHERE product_id = $1
```
Khi `product_id` là empty string → PostgreSQL cố parse empty string thành UUID → error.

### Giải pháp
Trong `services/finding-service/internal/delivery/http/report_handler.go`:
```go
func (h *ReportHandler) List(w http.ResponseWriter, r *http.Request) {
    userID := r.URL.Query().Get("_user_id")
    // Lấy userID từ JWT context nếu không có query param
    if userID == "" {
        userID = r.Header.Get("X-User-ID")
    }
    productID := r.URL.Query().Get("product_id")
    // Nếu productID rỗng thì không filter theo product_id
    // Hoặc query theo generated_by = userID
    ...
}
```

Trong `ReportRepo.ListByProduct()`:
- Bỏ `WHERE product_id = $1` nếu `productID` rỗng
- Thêm fallback: `WHERE generated_by = $userID` khi không có `product_id`

## Bug 2: `POST /api/v1/reports` — Thiếu `name`, `type`, `created_by`

### Hiện tại
DB schema: `title`, `format`, `generated_by`  
Test kiểm tra: `name`, `type`, `created_by`

### Giải pháp
Trong `services/finding-service/internal/delivery/http/report_handler.go` — hàm `Create()`:
Map alias trong JSON response:
```go
type ReportResponse struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`      // alias for title
    Type      string    `json:"type"`      // alias for format
    Status    string    `json:"status"`
    CreatedBy string    `json:"created_by"` // alias for generated_by
    CreatedAt time.Time `json:"created_at"`
}
```

## Acceptance Criteria
- [ ] `GET /api/v1/reports` → 200 với `{ reports: [], total: 0 }` khi không có data
- [ ] `POST /api/v1/reports` response có `name`, `type`, `created_by` fields
- [ ] Tests `reports_list_returns_200`, `report_create_response_schema` → PASS

## Status: TODO
