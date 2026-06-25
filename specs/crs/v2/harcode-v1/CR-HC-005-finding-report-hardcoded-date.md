# CR-HC-005: finding-service — Hardcoded Timestamp trong Report Create Response

## Trạng thái: 🟠 High

## Vấn đề
File: `services/finding-service/internal/delivery/http/report_handler.go:109`

```go
if h.generateUC == nil {
    reportID := uuid.New().String()
    writeJSON(w, http.StatusAccepted, map[string]interface{}{
        "id":         reportID,
        "name":       req.Title,
        "type":       req.Format,
        "status":     "pending",
        "created_at": fmt.Sprintf("%s", "2026-06-22T00:00:00Z"),  // ← HARDCODED DATE!
        "message":    "Report generation queued...",
    })
    return
}
```

Hai vi phạm:
1. `created_at` hardcode thành `"2026-06-22T00:00:00Z"` — sai về mặt nghiệp vụ
2. `generateUC == nil` là **stub mode** — report không thực sự được tạo và lưu vào DB

## Giải pháp

### Phần 1: Fix hardcoded timestamp (quick fix)
```go
"created_at": time.Now().UTC().Format(time.RFC3339),
```

### Phần 2: Implement thật — Report phải được persist trong DB

Ngay cả khi MinIO (file storage) chưa configured, report metadata PHẢI được lưu vào DB:

```go
func (h *ReportHandler) Create(w http.ResponseWriter, r *http.Request) {
    // ... decode request

    userID := r.Header.Get("X-User-ID")
    
    // LUÔN tạo report record trong DB (dù storage chưa sẵn sàng)
    rep := &report.Report{
        ID:          uuid.New(),
        Title:       req.Title,
        Format:      report.ReportFormat(req.Format),
        Status:      report.StatusPending,
        GeneratedBy: userID,
        CreatedAt:   time.Now().UTC(),
    }
    
    if err := h.repo.Create(r.Context(), rep); err != nil {
        writeJSON(w, http.StatusInternalServerError, apiErr("failed to create report"))
        return
    }
    
    // Nếu generateUC available → trigger async generation
    if h.generateUC != nil {
        go func() {
            ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
            defer cancel()
            _ = h.generateUC.Execute(ctx, rep.ID)
        }()
    }
    
    writeJSON(w, http.StatusAccepted, reportToResponse(rep))
}
```

### Phần 3: Thêm `Create` method vào ReportRepository
```go
type Repository interface {
    Create(ctx context.Context, r *Report) error   // NEW
    FindByID(ctx context.Context, id string) (*Report, error)
    ListByProduct(ctx context.Context, productID, userID string, limit, offset int) ([]*Report, int, error)
    UpdateStatus(ctx context.Context, id string, status Status, errMsg string) error // NEW
    Delete(ctx context.Context, id string) error
    DeleteExpired(ctx context.Context) (int, error)
}
```

## Files cần thay đổi
- `services/finding-service/internal/delivery/http/report_handler.go` — fix create logic
- `services/finding-service/internal/domain/report/report.go` — thêm `Create` to interface
- `services/finding-service/internal/infra/postgres/report_repo.go` — implement `Create`

## Migration SQL cần kiểm tra
Table `reports` đã tồn tại với đủ columns — chỉ cần implement `Create` method:
```sql
-- Đã có: id, product_id, title, format, status, generated_by, created_at
-- Chỉ cần INSERT implementation
```

## Acceptance Criteria
- [ ] `POST /api/v1/reports` luôn tạo record trong DB
- [ ] `GET /api/v1/reports` trả về report vừa tạo
- [ ] `created_at` trong response là thời gian thực, không hardcode
- [ ] Report status transition: `pending` → `generating` → `completed`/`failed`
