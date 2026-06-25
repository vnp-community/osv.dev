# TASK-004 — Fix BUG-010: Consistent nilReportRepo & Config-Driven MinIO Wiring

> **Bug**: BUG-010  
> **Priority**: 🔴 High — User confusion: list reports trả về 200 OK với [] dù storage chưa configured  
> **Depends on**: TASK-000  
> **Solution ref**: [SOL-GROUP-D](../solutions/SOL-GROUP-D-finding-service-logic.md#bug-010)  
> **Trạng thái**: ✅ DONE — 2026-06-22  
> **Ghi chú**: Rename `nilReportRepo` → `notConfiguredReportRepo`. Thêm `errReportStorageNotConfigured` sentinel error. `ListByProduct` giờ trả error (không còn trả empty list OK). `FindByID` trả sentinel error (không còn trả "report not found"). `Delete`/`DeleteExpired` trả error nhất quán. Thêm NATS_URL warn log. Config-driven MinIO wiring với warn log. Build pass.

## Files Cần Đọc Trước

```
services/finding-service/embedded.go                              (toàn bộ file)
services/finding-service/internal/domain/report/                  (xem Report entity + Repository interface)
services/finding-service/internal/delivery/http/report_handler.go (xem CreateReport, ListReports)
```

## Files Sẽ Bị Sửa

```
services/finding-service/embedded.go                              [MODIFY]
services/finding-service/internal/delivery/http/report_handler.go [MODIFY — nếu cần thêm 503 response]
```

## Thay Đổi Chi Tiết

### Bước 1: Đọc `embedded.go` thực tế

Tìm chính xác:
- Tên struct stub hiện tại (có thể là `nilReportRepo` hoặc tên khác)
- Các methods của stub: `Save`, `FindByID`, `ListByProduct` (và signature đầy đủ)
- Dòng khởi tạo `reportHandler`

### Bước 2: Rename và fix stub trong `embedded.go`

**Rename** `nilReportRepo` → `notConfiguredReportRepo` và thêm sentinel error:

```go
// Thêm sentinel error (trước struct definition):
var errReportStorageNotConfigured = errors.New(
    "report storage not configured: set MINIO_ENDPOINT env var to enable report features",
)

// notConfiguredReportRepo là stub dùng khi MinIO chưa được cấu hình.
// [FIX] Tất cả methods đều trả về errReportStorageNotConfigured — consistent behavior.
type notConfiguredReportRepo struct{}
```

**Fix mỗi method** — tất cả phải trả về `errReportStorageNotConfigured`:

```go
func (n *notConfiguredReportRepo) Save(ctx context.Context, r *report.Report) error {
    return errReportStorageNotConfigured
}

func (n *notConfiguredReportRepo) FindByID(ctx context.Context, id uuid.UUID) (*report.Report, error) {
    // [FIX] Trước: return nil, fmt.Errorf("report not found")  ← misleading!
    return nil, errReportStorageNotConfigured
}

// Signature của ListByProduct phụ thuộc vào interface thực tế — đọc trước
func (n *notConfiguredReportRepo) ListByProduct(
    ctx context.Context, productID uuid.UUID, page, limit int, sort string,
) ([]*report.Report, int, error) {
    // [FIX] Trước: return []*report.Report{}, 0, nil  ← hides the problem!
    return nil, 0, errReportStorageNotConfigured
}
```

> **Quan trọng**: Đọc interface thực tế trong `internal/domain/report/` để lấy đúng method signatures.

### Bước 3: Config-driven MinIO wiring

Thay dòng khởi tạo cứng `reportHandler := httpdelivery.NewReportHandler(nil, &nilReportRepo{}, nil)`:

```go
// Config-driven report storage wiring
var reportRepo report.Repository = &notConfiguredReportRepo{}

minioEndpoint := os.Getenv("MINIO_ENDPOINT")
if minioEndpoint == "" {
    log.Warn().Msg("MINIO_ENDPOINT not set — report storage disabled. " +
        "Reports cannot be created or downloaded.")
} else {
    minioBucket := os.Getenv("MINIO_BUCKET")
    if minioBucket == "" {
        minioBucket = "reports"
    }
    // Khởi tạo MinIO repo — chỉ thực hiện nếu MinIO implementation đã có
    // Nếu chưa có minio.NewReportRepository, giữ notConfiguredReportRepo
    // và thêm TODO comment rõ ràng
    log.Info().
        Str("endpoint", minioEndpoint).
        Str("bucket", minioBucket).
        Msg("MinIO endpoint configured — attempting to connect report storage")
    // TODO: wire real MinIO repo khi implementation sẵn sàng
    // realRepo, err := minio.NewReportRepository(ctx, minioEndpoint, minioBucket)
    // if err != nil { log.Warn()... } else { reportRepo = realRepo }
}

reportHandler := httpdelivery.NewReportHandler(generateUC, reportRepo, templateStore)
```

> **Lưu ý**: Chỉ wire MinIO thực nếu `minio.NewReportRepository` đã tồn tại trong codebase.
> Nếu chưa có, giữ `notConfiguredReportRepo` nhưng log WARN rõ ràng.

### Bước 4: (Optional) Sửa report_handler.go để trả về 503

Đọc `report_handler.go`. Nếu `CreateReport` trả về lỗi từ repo với HTTP 500,
sửa để phân biệt `errReportStorageNotConfigured` → 503:

```go
func (h *ReportHandler) CreateReport(w http.ResponseWriter, r *http.Request) {
    // ...
    if err := h.repo.Save(ctx, rpt); err != nil {
        if errors.Is(err, errReportStorageNotConfigured) {
            // 503 Service Unavailable — storage not configured, not a server error
            respondError(w, http.StatusServiceUnavailable,
                "report storage not available — contact administrator")
            return
        }
        respondError(w, http.StatusInternalServerError, err.Error())
        return
    }
}
```

**Tương tự cho `ListReports`**:
```go
func (h *ReportHandler) ListReports(w http.ResponseWriter, r *http.Request) {
    reports, total, err := h.repo.ListByProduct(...)
    if err != nil {
        if errors.Is(err, errReportStorageNotConfigured) {
            respondError(w, http.StatusServiceUnavailable, "report storage not available")
            return
        }
        // ...
    }
}
```

> **Quan trọng**: `errReportStorageNotConfigured` cần được export hoặc truy cập được từ handler.
> Xem xét đặt nó trong domain package thay vì embedded.go.

## Imports Cần Thêm

```go
import (
    "errors"  // [ADD] cho errors.New và errors.Is
    // ... rest unchanged
)
```

## Verification

```bash
# Build
go build ./services/finding-service/...

# Test: list reports trả về 503 khi MINIO_ENDPOINT không set
MINIO_ENDPOINT="" go run ./services/finding-service/cmd/server/ &
curl -s http://localhost:8085/api/v1/reports?product_id=test | jq .
# → {"error": "report storage not available"} với HTTP 503

# Test: không còn "report not found" misleading error
curl -s http://localhost:8085/api/v1/reports/some-uuid | jq .error
# → "report storage not configured..." (không phải "report not found")

# Test: WARN log khi MINIO_ENDPOINT không set
MINIO_ENDPOINT="" go run ./services/finding-service/cmd/server/ 2>&1 | grep -i "minio\|report storage"
# → WARN: "MINIO_ENDPOINT not set — report storage disabled"
```

## Acceptance Criteria

- [ ] Stub được rename từ `nilReportRepo` → `notConfiguredReportRepo`
- [ ] Sentinel error `errReportStorageNotConfigured` được khai báo
- [ ] `ListByProduct` trả về error (không phải empty list OK)
- [ ] `FindByID` trả về `errReportStorageNotConfigured` (không phải "report not found")
- [ ] Wiring bọc trong MinIO config check với WARN log
- [ ] `go build ./services/finding-service/...` thành công
