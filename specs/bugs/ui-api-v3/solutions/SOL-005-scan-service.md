# SOL-005: Scan-Service — Scan History & Import

> **Bugs giải quyết**: BUG-005  
> **Service**: `services/scan-service`  
> **Port**: 8084  
> **Architecture ref**: §3.6 Scan-Service  
> **Status**: `[x] DONE`

## Kết Quả Thực Thi

**Đã hoàn thành trong scan-service:**

| Fix | File | Trạng thái |
|---|---|---|
| `GetScanHistory` handler cho `GET /scans/history` | `internal/delivery/http/router.go` | ✅ Thêm mới (TASK-009) |
| Route `/scans/history` register **trước** `/{id}` | `internal/delivery/http/router.go` | ✅ Đúng thứ tự |
| Gateway: `GET /api/v1/scans/history` **trước** `/{id}` | `apps/osv/internal/gateway/router.go` | ✅ Fixed (TASK-009) |
| Handler graceful: `{scans: [], total: 0}` khi repo nil | `internal/delivery/http/router.go` | ✅ Đã có |
| `?status=` query param filter | `internal/delivery/http/router.go` | ✅ Đã có |

**Build verify**: `go build ./...` ✅ scan-service, apps/osv


---

## BUG-005a: GET /api/v1/scans/history (404)

### Nguyên Nhân Sâu

Architecture §3.6 mô tả scan-service với schema `osv_scan`:
- `scans` — state machine: pending → queued → running → completed/failed/cancelled

`/scans/history` là subset của `/scans` với filter `status IN ('completed', 'failed', 'cancelled')`. Route này chưa được register trong scan-service router.

**Routing conflict risk**: `/scans/history` là static path — nếu router đăng ký `/scans/{id}` trước mà không handle static prefix ưu tiên, request đến `/scans/history` sẽ match `/scans/{id}` với `id="history"`, trả 400 (invalid UUID) hoặc 404 (not found in DB).

**Cách kiểm tra**:
```bash
curl https://c12.openledger.vn/api/v1/scans/history -H "Authorization: Bearer $TOKEN"
# Nếu 400 Bad Request (not 404) → đang vào /scans/{id} với id="history"
# Nếu 404 Not Found → route truly missing
```

### Giải Pháp

```go
// services/scan-service/internal/delivery/http/scan_handler.go

// GET /api/v1/scans/history?status=completed&page=1&limit=20
func (h *ScanHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
    filter := ScanHistoryFilter{
        Status: []string{"completed", "failed", "cancelled"}, // Default
        Page:   parseIntParam(r, "page", 1),
        Limit:  parseIntParam(r, "limit", 20),
    }
    
    // Allow filtering by specific status
    if s := r.URL.Query().Get("status"); s != "" {
        filter.Status = []string{s}
    }
    
    // Date range
    if from := r.URL.Query().Get("from"); from != "" {
        filter.From, _ = time.Parse(time.RFC3339, from)
    }
    
    scans, total, err := h.scanUC.GetHistory(r.Context(), filter)
    if err != nil {
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "items": scans,
        "total": total,
        "page":  filter.Page,
        "limit": filter.Limit,
    })
}
```

```go
// services/scan-service/internal/delivery/http/router.go

// CRITICAL: Static routes TRƯỚC wildcard route
r.GET("/api/v1/scans/history",    authMiddleware(h.GetHistory))    // THÊM MỚI — TRƯỚC
r.GET("/api/v1/scans/scheduled",  authMiddleware(h.GetScheduled))  // Đã có — TRƯỚC
r.GET("/api/v1/scans",            authMiddleware(h.List))           // Đã có
r.POST("/api/v1/scans",           authMiddleware(h.Create))         // Đã có
r.GET("/api/v1/scans/{id}",       authMiddleware(h.GetByID))        // Đã có — SAU
r.GET("/api/v1/scans/{id}/stream", sseAuth(h.Stream))              // Đã có
```

**Use case**:
```go
// services/scan-service/internal/usecase/scan_usecase.go

func (uc *ScanUseCase) GetHistory(ctx context.Context, filter ScanHistoryFilter) ([]Scan, int, error) {
    return uc.repo.ListByStatus(ctx, filter.Status, filter.Page, filter.Limit, filter.From)
}
```

**SQL**:
```sql
-- services/scan-service/internal/infra/postgres/scan_repo.go
SELECT id, target, type, status, created_at, completed_at,
       findings_count, duration_seconds,
       COUNT(*) OVER() AS total
FROM scans
WHERE status = ANY($1)
  AND ($4::timestamptz IS NULL OR created_at >= $4)
ORDER BY created_at DESC
LIMIT $2 OFFSET $3
```

### Response Schema

```go
type ScanHistoryItem struct {
    ID              string    `json:"id"`
    Target          string    `json:"target"`
    Type            string    `json:"type"`  // "nmap", "zap", "sca", "sbom"
    Status          string    `json:"status"`
    CreatedAt       time.Time `json:"created_at"`
    CompletedAt     *time.Time `json:"completed_at,omitempty"`
    DurationSeconds int        `json:"duration_seconds,omitempty"`
    FindingsCount   int        `json:"findings_count"`
    ScheduledByID   *string    `json:"scheduled_by_id,omitempty"`
}
```

---

## BUG-005b: POST /api/v1/scans/import (405 → 200)

### Phân Tích

Route tồn tại nhưng trả 405 Method Not Allowed. Có thể:
1. Server đang dùng `PUT` thay vì `POST`
2. Route xung đột với pattern khác
3. Multipart upload handler không accept content-type `application/json`

Architecture §6.2 mô tả Import Pipeline 12 Steps — đây là use case phức tạp đã implement.

**Cách kiểm tra thực tế**:
```bash
# Test với POST + multipart
curl -X POST https://c12.openledger.vn/api/v1/scans/import \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@test.xml" \
  -F "tool_name=nmap" \
  -F "product_id=test-product"

# Test với legacy path
curl -X POST https://c12.openledger.vn/api/v2/import-scan \
  -H "Authorization: Bearer $TOKEN"
```

### Giải Pháp

```go
// services/scan-service/internal/delivery/http/import_handler.go

// POST /api/v1/scans/import (multipart/form-data)
func (h *ScanHandler) Import(w http.ResponseWriter, r *http.Request) {
    // 32 MB max (configurable via env)
    if err := r.ParseMultipartForm(32 << 20); err != nil {
        respondError(w, http.StatusBadRequest, "invalid multipart form")
        return
    }
    
    file, header, err := r.FormFile("file")
    if err != nil {
        respondError(w, http.StatusBadRequest, "file field required")
        return
    }
    defer file.Close()
    
    toolName  := r.FormValue("tool_name")  // "nmap", "zap", "trivy", etc.
    productID := r.FormValue("product_id")
    testID    := r.FormValue("test_id")
    
    if toolName == "" || productID == "" {
        respondError(w, http.StatusBadRequest, "tool_name and product_id required")
        return
    }
    
    // Rate-limit: 10/minute (per architecture §3.1)
    // Already handled by gateway rate limiter for POST /api/v1/scans/import
    
    result, err := h.importUC.Import(r.Context(), ImportRequest{
        File:      file,
        FileName:  header.Filename,
        MaxSize:   32 << 20,
        ToolName:  toolName,
        ProductID: productID,
        TestID:    testID,
        UserID:    r.Header.Get("X-User-ID"),
    })
    if err != nil {
        respondError(w, http.StatusUnprocessableEntity, err.Error())
        return
    }
    
    respondJSON(w, http.StatusCreated, result)
}
```

**Router**:
```go
// CRITICAL: Đăng ký TRƯỚC /scans/{id}
r.POST("/api/v1/scans/import", 
    rateLimited("10/minute", authMiddleware(h.Import)))
```

**Nếu 405 do method mismatch** — kiểm tra trong router xem route đang dùng PUT:
```go
// Nếu đang dùng PUT:
r.PUT("/api/v1/scans/import",  authMiddleware(h.Import))  // WRONG
// Sửa thành:
r.POST("/api/v1/scans/import", authMiddleware(h.Import))  // CORRECT
```

### Result Schema

```go
type ImportResult struct {
    ScanID     string `json:"scan_id"`
    Created    int    `json:"created"`
    Duplicates int    `json:"duplicates"`
    Total      int    `json:"total"`
    Status     string `json:"status"`  // "completed", "processing"
}
```
