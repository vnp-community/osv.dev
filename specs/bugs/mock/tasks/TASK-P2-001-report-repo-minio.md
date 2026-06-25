# TASK-P2-001 — Wire PostgreSQL ReportRepo + MinIO Storage

**Bug:** MOCK-001  
**Priority:** 🟡 P2 — Feature bị disabled  
**Effort:** ~3 giờ  
**Service:** `finding-service`  
**Loại thay đổi:** New files (ReportRepo + MinIO adapter) + DB migration + Wire embedded.go  
**Depends on:** TASK-P0-001 (nil-check đã được thêm)

---

## Mục tiêu

`nilReportRepo` không lưu report vào đâu. Cần:
1. Tạo `PostgresReportRepo` thực sự (CRUD vào bảng `reports`)
2. Tạo `MinIOReportStorage` (Upload + PresignedURL)
3. Wire vào `embedded.go` với conditional logic (chỉ enable khi `MINIO_ENDPOINT` được set)

---

## Preconditions

- [ ] Đọc `services/finding-service/internal/domain/report/` — xem Report entity
- [ ] Đọc `services/finding-service/internal/delivery/http/report_handler.go` — xem interface cần implement
- [ ] Kiểm tra migration folder: `find services/finding-service -name "*.sql" | head -10`
- [ ] Kiểm tra xem minio-go đã có trong go.mod chưa:
  ```bash
  grep "minio" services/finding-service/go.mod
  ```
- [ ] Xác định `ReportRepository` interface (grep tìm interface đó)

---

## Steps

### Step 1 — Xác định Report domain entity và interfaces

```bash
find services/finding-service -path "*/domain/report*" -o -path "*/domain/*report*"
grep -rn "ReportRepository\|type.*Report.*interface" services/finding-service/internal/
```

Ghi lại:
- Tên và location của `Report` entity
- Method list của `ReportRepository` interface
- Method list của `Storage` interface (nếu có)

### Step 2 — Tạo DB migration

**File mới** (đặt theo convention migrations hiện có):
```
services/finding-service/internal/infra/postgres/migrations/XXX_add_reports.sql
```

```sql
CREATE TABLE IF NOT EXISTS reports (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id   UUID NOT NULL,
    title        VARCHAR(255) NOT NULL,
    format       VARCHAR(10) NOT NULL CHECK (format IN ('pdf','xlsx','json','csv','html')),
    status       VARCHAR(20) NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending','processing','completed','failed')),
    storage_key  TEXT,
    generated_by UUID,
    generated_at TIMESTAMPTZ,
    error_msg    TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_reports_product_id   ON reports(product_id);
CREATE INDEX IF NOT EXISTS idx_reports_generated_by ON reports(generated_by);
CREATE INDEX IF NOT EXISTS idx_reports_status       ON reports(status);
```

### Step 3 — Tạo PostgreSQL ReportRepo

Kiểm tra convention repo khác (ví dụ FindingRepo):
```bash
head -60 services/finding-service/internal/infra/postgres/finding_repo.go 2>/dev/null
```

**File mới**: `services/finding-service/internal/infra/postgres/report_repo.go`

Implement đầy đủ các method của `ReportRepository` interface theo cấu trúc đọc được ở Step 1. Sử dụng `pgxpool.Pool` nhất quán với các repo khác.

Key methods:
- `Save(ctx, *report.Report) error` — INSERT ON CONFLICT UPDATE
- `FindByID(ctx, id string) (*report.Report, error)`
- `ListByProduct(ctx, productID, userID string, limit, offset int) ([]*report.Report, int, error)`
- `Delete(ctx, id string) error`

### Step 4 — Thêm minio-go dependency (nếu chưa có)

```bash
cd services/finding-service
grep "minio" go.mod
# Nếu không có:
go get github.com/minio/minio-go/v7
```

### Step 5 — Tạo MinIO Storage Adapter

**File mới**: `services/finding-service/internal/infra/minio/report_storage.go`

```go
package minio

import (
    "bytes"
    "context"
    "fmt"
    "time"
    "github.com/minio/minio-go/v7"
    "github.com/minio/minio-go/v7/pkg/credentials"
)

type ReportStorage struct {
    client *minio.Client
    bucket string
}

func NewReportStorage(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*ReportStorage, error) {
    client, err := minio.New(endpoint, &minio.Options{
        Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
        Secure: useSSL,
    })
    if err != nil {
        return nil, fmt.Errorf("minio.New: %w", err)
    }
    // Ensure bucket exists
    ctx := context.Background()
    exists, err := client.BucketExists(ctx, bucket)
    if err != nil {
        return nil, fmt.Errorf("check bucket %q: %w", bucket, err)
    }
    if !exists {
        if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
            return nil, fmt.Errorf("create bucket %q: %w", bucket, err)
        }
    }
    return &ReportStorage{client: client, bucket: bucket}, nil
}

func (s *ReportStorage) Upload(ctx context.Context, key string, data []byte, contentType string) error {
    _, err := s.client.PutObject(ctx, s.bucket, key,
        bytes.NewReader(data), int64(len(data)),
        minio.PutObjectOptions{ContentType: contentType})
    return err
}

func (s *ReportStorage) PresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
    u, err := s.client.PresignedGetObject(ctx, s.bucket, key, expiry, nil)
    if err != nil {
        return "", fmt.Errorf("presigned URL for %q: %w", key, err)
    }
    return u.String(), nil
}

func (s *ReportStorage) Delete(ctx context.Context, key string) error {
    return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}
```

### Step 6 — Wire vào embedded.go

Mở `services/finding-service/embedded.go`.

Tìm dòng `nilReportRepo` và thay bằng:

```go
// FIX MOCK-001: Wire real PostgreSQL ReportRepo
reportRepo := postgres.NewReportRepo(pool)

// FIX MOCK-001: Wire MinIO storage (conditional — chỉ khi MINIO_ENDPOINT set)
var generateUC reportuc.GenerateUseCase  // dùng đúng type trong codebase
var reportStorage reportuc.Storage

minioEndpoint  := os.Getenv("MINIO_ENDPOINT")
minioAccessKey := os.Getenv("MINIO_ACCESS_KEY")
minioSecretKey := os.Getenv("MINIO_SECRET_KEY")
minioBucket    := os.Getenv("MINIO_REPORT_BUCKET")
if minioBucket == "" {
    minioBucket = "osv-reports"
}

if minioEndpoint != "" && minioAccessKey != "" {
    storage, err := miniorepo.NewReportStorage(
        minioEndpoint, minioAccessKey, minioSecretKey, minioBucket, false)
    if err != nil {
        logger.Warn().Err(err).Msg("MinIO storage init failed, report generation disabled")
    } else {
        reportStorage = storage
        generateUC = reportuc.NewGenerateUseCase(findingRepo, reportRepo, storage)
        logger.Info().Str("bucket", minioBucket).Msg("Report generation enabled via MinIO")
    }
} else {
    logger.Warn().Msg("MINIO_ENDPOINT not set — report generation disabled")
}

reportHandler := httpdelivery.NewReportHandler(generateUC, reportRepo, reportStorage)
```

---

## Acceptance Criteria

- [ ] `nilReportRepo` không còn được dùng trong `embedded.go`
- [ ] `GET /api/v1/reports` trả danh sách từ PostgreSQL (có thể rỗng nếu chưa tạo)
- [ ] Khi `MINIO_ENDPOINT` được set → `POST /api/v1/reports` tạo report và lưu vào DB + MinIO
- [ ] Khi `MINIO_ENDPOINT` không set → `POST /api/v1/reports` trả 503 (nhờ TASK-P0-001 fix)
- [ ] `go build ./services/finding-service/...` — thành công

---

## Test Commands

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev
go build ./services/finding-service/...
go vet ./services/finding-service/...

# Verify nilReportRepo removed
grep -n "nilReportRepo" services/finding-service/embedded.go
# Expected: no output

# Run tests
go test ./services/finding-service/internal/infra/... -v -run Report
```
