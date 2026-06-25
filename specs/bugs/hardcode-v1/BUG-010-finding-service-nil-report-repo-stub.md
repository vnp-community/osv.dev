# BUG-010 — Finding Service: Nil Report Repository (nilReportRepo) Trả Về Lỗi Ẩn

## Metadata
- **ID**: BUG-010
- **Service**: `finding-service`
- **File**: [`embedded.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/embedded.go)
- **Lines**: 64–112
- **Severity**: High
- **Category**: Stub / Temporary Object
- **Status**: Open

## Mô tả

`finding-service/embedded.go` tạo một `nilReportRepo` stub được hardcode để thay thế
report storage thực. Stub này được dùng khi MinIO chưa được cấu hình:

```go
// Line 64-67
// CR-010: ReportHandler — GetTemplates is static (no DB/storage needed).
// Pass nil for generateUC and storage — only GetTemplates is fully functional.
// List/Create/Download require full wiring (future work when MinIO is configured).
reportHandler := httpdelivery.NewReportHandler(nil, &nilReportRepo{}, nil)

// Lines 96-112: nilReportRepo implementation
type nilReportRepo struct{}

func (n *nilReportRepo) Save(_, _) error {
    return fmt.Errorf("report storage not configured")  // silently fails
}
func (n *nilReportRepo) FindByID(_, _) (*report.Report, error) {
    return nil, fmt.Errorf("report not found")           // misleading error
}
func (n *nilReportRepo) ListByProduct(_, _, _, _, _) ([]*report.Report, int, error) {
    return []*report.Report{}, 0, nil                    // returns empty but no error!
}
```

### Vấn đề:

1. **Inconsistent error responses**: `Save` trả về lỗi, `ListByProduct` trả về empty
   list thành công — client không thể biết storage chưa được cấu hình.

2. **Misleading error**: `FindByID` trả về `"report not found"` thay vì
   `"report storage not configured"` — engineer debug sẽ nghĩ record bị xóa.

3. **Stub tồn tại vĩnh viễn**: Không có flag/config để detect khi nào MinIO được
   cấu hình để switch từ stub sang real implementation. Code comment nói
   "future work" nhưng không có tracking mechanism.

## Tác động

1. Users tạo report sẽ nhận lỗi 500 "report storage not configured" nhưng:
   - List reports vẫn trả về 200 với `[]` — UI nghĩ report đã được tạo/xóa.
   - UI confusion: create fail nhưng list empty → "Where did my report go?"

2. Không có health check nào phát hiện storage chưa configured → admin không biết.

## Fix Proposal

### 1. Consistent error behavior trong stub

```go
type notConfiguredReportRepo struct{}

var errStorageNotConfigured = errors.New("report storage not configured: set MINIO_ENDPOINT")

func (n *notConfiguredReportRepo) Save(_, _) error {
    return errStorageNotConfigured
}
func (n *notConfiguredReportRepo) FindByID(_, _) (*report.Report, error) {
    return nil, errStorageNotConfigured  // FIX: not "report not found"
}
func (n *notConfiguredReportRepo) ListByProduct(_, _, _, _, _) ([]*report.Report, int, error) {
    return nil, 0, errStorageNotConfigured  // FIX: return error, not empty list
}
```

### 2. Config-driven wiring

```go
func WireEmbedded(ctx context.Context, ...) error {
    var reportRepo report.Repository = &notConfiguredReportRepo{}

    if minioEndpoint := os.Getenv("MINIO_ENDPOINT"); minioEndpoint != "" {
        bucket := os.Getenv("MINIO_BUCKET")
        if bucket == "" { bucket = "reports" }
        realRepo, err := minio.NewReportRepository(minioEndpoint, bucket)
        if err != nil {
            logger.Warn().Err(err).Msg("MinIO unavailable, report storage disabled")
        } else {
            reportRepo = realRepo
            logger.Info().Str("endpoint", minioEndpoint).Msg("MinIO report storage enabled")
        }
    } else {
        logger.Warn().Msg("MINIO_ENDPOINT not set, report storage disabled")
    }

    reportHandler := httpdelivery.NewReportHandler(generateUC, reportRepo, store)
}
```

### 3. Health endpoint nên expose storage status

```json
// GET /health response
{
  "status": "degraded",
  "components": {
    "database": "ok",
    "report_storage": "not_configured"
  }
}
```

## References

- [finding-service/embedded.go L64-112](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/embedded.go#L64-L112)
- [finding-service/embedded.go L96-112 (nilReportRepo)](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/embedded.go#L96-L112)
