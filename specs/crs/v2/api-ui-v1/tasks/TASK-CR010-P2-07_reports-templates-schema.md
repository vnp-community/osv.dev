# TASK-CR010-P2-07 — Reports: Templates Endpoint + Schema + MinIO Upload

**Phase:** Phase 3 — Degraded UX  
**Nguồn giải pháp:** [`solutions/SOL-010`](../solutions/SOL-010-reports-templates-schema-update.md)  
**Ưu tiên:** 🟡 P2 — Reports UI hiển thị sai, không download được  
**Phụ thuộc:** MinIO phải đang chạy (verify trước)  
**Status:** ✅ **DONE** — 2026-06-19  

---

## Mục tiêu

1. `GET /api/v1/reports/templates` → 3 templates (Executive, Technical, Compliance)
2. `GET /api/v1/reports` → thêm `last_generated_at` field
3. Mỗi `Report` object thêm: `format`, `finding_count`, `file_size_bytes`, `generated_at`, `artifact_url`, `expires_at`
4. Sau generate xong → upload lên MinIO, lưu presigned URL

---

## Điều tra trước khi code

```bash
# 1. Xem reports table hiện tại
docker exec osv-backend-postgres-1 psql -U osv -d osv \
  -c "\d reports"

# 2. Tìm reports handler
grep -rn "GetReports\|ListReports\|/api/v1/reports" \
  services/finding-service/ --include="*.go" -l

# 3. Kiểm tra MinIO
docker ps | grep minio
# Nếu có MinIO:
docker exec osv-backend-minio-1 mc ls local/ 2>/dev/null || \
  mc ls myminio/ 2>/dev/null || echo "Check MinIO config"

# 4. Kiểm tra gateway routes
grep -n "reports" apps/osv/internal/gateway/router.go

# 5. Xem response hiện tại
curl -s -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/reports | jq '.reports[0] | keys'
```

---

## Bước 1: DB Migration

**File:** `services/finding-service/migrations/20260619_001_reports_schema.sql`

```sql
ALTER TABLE reports
    ADD COLUMN IF NOT EXISTS format          VARCHAR(10) NOT NULL DEFAULT 'pdf'
        CHECK (format IN ('pdf', 'html', 'csv', 'excel', 'json')),
    ADD COLUMN IF NOT EXISTS finding_count   INTEGER,
    ADD COLUMN IF NOT EXISTS file_size_bytes BIGINT,
    ADD COLUMN IF NOT EXISTS generated_at    TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS artifact_url    TEXT,
    ADD COLUMN IF NOT EXISTS expires_at      TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_reports_generated_at
    ON reports(generated_at DESC)
    WHERE status = 'completed';
```

```bash
docker exec osv-backend-postgres-1 psql -U osv -d osv \
  -f /migrations/20260619_001_reports_schema.sql

# Verify
docker exec osv-backend-postgres-1 psql -U osv -d osv \
  -c "SELECT column_name FROM information_schema.columns \
      WHERE table_name='reports' \
      AND column_name IN ('format','finding_count','artifact_url','expires_at');"
```

---

## Bước 2: Static Report Templates Config

**Tạo file config:**

```bash
# Kiểm tra có config package chưa
ls services/finding-service/internal/config/ 2>/dev/null || \
  mkdir -p services/finding-service/internal/config/
```

**File:** `services/finding-service/internal/config/report_templates.go`

```go
package config

type ReportTemplate struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description"`
    Type        string `json:"type"`
}

var DefaultReportTemplates = []ReportTemplate{
    {
        ID:          "exec",
        Name:        "Executive Summary",
        Description: "High-level overview for C-level presentations. Bao gồm risk posture, top vulnerabilities, và SLA compliance.",
        Type:        "Executive",
    },
    {
        ID:          "tech",
        Name:        "Technical Report",
        Description: "Chi tiết findings với CVE details, CVSS scores, và hướng dẫn remediation.",
        Type:        "Technical",
    },
    {
        ID:          "comp",
        Name:        "Compliance Report",
        Description: "Mapped tới PCI DSS, ISO 27001, SOC2, NIST frameworks.",
        Type:        "Compliance",
    },
}
```

---

## Bước 3: Handler Updates

**Tìm handler file:**
```bash
grep -rn "ListReports\|GetReports\|func.*reports" \
  services/finding-service/internal/ --include="*.go" -n
```

**Thêm GetReportTemplates:**

```go
// GET /api/v1/reports/templates
// ⚠️ PHẢI đăng ký trước /reports/{id} trong router
func (h *ReportHandler) GetReportTemplates(w http.ResponseWriter, r *http.Request) {
    // Static config — không cần DB
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "templates": config.DefaultReportTemplates,
    })
}
```

**Update ListReports — thêm last_generated_at:**

```go
// GET /api/v1/reports — thêm last_generated_at
func (h *ReportHandler) ListReports(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    // ... existing list logic ...

    // Lấy last_generated_at (nullable)
    var lastGenAt *time.Time
    h.db.QueryRow(ctx, `
        SELECT MAX(generated_at) FROM reports WHERE status = 'completed'
    `).Scan(&lastGenAt)

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "reports":          reports, // existing
        "total":            total,   // existing
        "last_generated_at": lastGenAt, // ← MỚI (null nếu chưa có)
    })
}
```

**Update Report query — include new fields:**

```go
// Cập nhật SELECT để include new columns
rows, err := h.db.Query(ctx, `
    SELECT id, name, type, format, status,
           finding_count, file_size_bytes, generated_at,
           artifact_url, expires_at, created_at, created_by
    FROM reports
    ORDER BY created_at DESC
    LIMIT $1 OFFSET $2
`, pageSize, offset)

// Update scan trong rows.Next():
var report struct {
    ID            string     `json:"id"`
    Name          string     `json:"name"`
    Type          string     `json:"type"`
    Format        string     `json:"format"`        // ← MỚI
    Status        string     `json:"status"`
    FindingCount  *int       `json:"finding_count"` // ← MỚI (nullable)
    FileSizeBytes *int64     `json:"file_size_bytes"` // ← MỚI
    GeneratedAt   *time.Time `json:"generated_at"`  // ← MỚI
    ArtifactURL   *string    `json:"artifact_url"`  // ← MỚI
    ExpiresAt     *time.Time `json:"expires_at"`    // ← MỚI
    CreatedAt     time.Time  `json:"created_at"`
    CreatedBy     string     `json:"created_by"`
}
rows.Scan(&report.ID, &report.Name, &report.Type, &report.Format, &report.Status,
    &report.FindingCount, &report.FileSizeBytes, &report.GeneratedAt,
    &report.ArtifactURL, &report.ExpiresAt, &report.CreatedAt, &report.CreatedBy)
```

---

## Bước 4: MinIO Upload (Async)

**Kiểm tra MinIO dependency:**
```bash
grep "minio-go" services/finding-service/go.mod 2>/dev/null || \
  grep "minio" services/finding-service/go.mod
```

Nếu chưa có:
```bash
cd services/finding-service && go get github.com/minio/minio-go/v7
```

**Storage service:** `services/finding-service/internal/infra/storage/minio.go`

```go
package storage

import (
    "context"
    "net/url"
    "time"

    "github.com/minio/minio-go/v7"
    "github.com/minio/minio-go/v7/pkg/credentials"
)

type MinioStorage struct {
    client *minio.Client
    bucket string
}

func NewMinioStorage(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*MinioStorage, error) {
    client, err := minio.New(endpoint, &minio.Options{
        Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
        Secure: useSSL,
    })
    if err != nil {
        return nil, err
    }
    return &MinioStorage{client: client, bucket: bucket}, nil
}

func (s *MinioStorage) UploadAndSign(ctx context.Context, filePath, objectName string, ttl time.Duration) (string, time.Time, error) {
    // Upload
    _, err := s.client.FPutObject(ctx, s.bucket, objectName, filePath, minio.PutObjectOptions{})
    if err != nil {
        return "", time.Time{}, err
    }
    // Presigned URL TTL 7 ngày
    expiresAt := time.Now().Add(ttl)
    presigned, err := s.client.PresignedGetObject(ctx, s.bucket, objectName, ttl, url.Values{})
    if err != nil {
        return "", time.Time{}, err
    }
    return presigned.String(), expiresAt, nil
}
```

**Async report generator — sau generate xong:**

```go
// Sau khi render report file xong:
artifactURL, expiresAt, err := storage.UploadAndSign(
    ctx, tmpFilePath,
    fmt.Sprintf("reports/%s.%s", reportID, format),
    7*24*time.Hour, // 7 ngày
)

// Update record
h.db.Exec(ctx, `
    UPDATE reports
    SET status          = 'completed',
        format          = $2,
        finding_count   = $3,
        file_size_bytes = $4,
        artifact_url    = $5,
        expires_at      = $6,
        generated_at    = NOW(),
        completed_at    = NOW()
    WHERE id = $1
`, reportID, format, findingCount, fileSize, artifactURL, expiresAt)
```

> **Nếu MinIO chưa setup:** Để `artifact_url = null` tạm thời, implement MinIO sau. Report vẫn có `status = completed`, chỉ không có download URL.

---

## Bước 5: Router Registration

**Trong finding-service router:**
```bash
grep -rn "Handle\|r\.Get\|mux\." \
  services/finding-service/ --include="*.go" | grep "reports"
```

Thêm route templates (**TRƯỚC** `/{id}`):
```go
// /reports/templates PHẢI TRƯỚC /reports/{id}
mux.Handle("GET /api/v1/reports/templates", protected(h.GetReportTemplates))
// Routes hiện có
mux.Handle("GET /api/v1/reports", protected(h.ListReports))
mux.Handle("POST /api/v1/reports", rateLimit(h.CreateReport))
mux.Handle("GET /api/v1/reports/{id}", protected(h.GetReport))
```

**Trong gateway router:**
```bash
grep -n "reports" apps/osv/internal/gateway/router.go
```

```go
// Gateway — /reports/templates TRƯỚC /reports/{id}
mux.Handle("GET /api/v1/reports/templates",
    protected(proxy.Forward("finding-service:8085")))
// Hiện có:
// mux.Handle("GET /api/v1/reports", ...)
// mux.Handle("GET /api/v1/reports/{id}", ...)
```

---

## Acceptance Criteria

- [ ] `GET /api/v1/reports/templates` → `{ templates: [3 items] }` — HTTP 200
- [ ] Templates có: `id` (exec/tech/comp), `name`, `description`, `type`
- [ ] `/reports/templates` không bị 404 (route conflict với `/{id}`)
- [ ] `GET /api/v1/reports` → response có `last_generated_at` (null hoặc datetime)
- [ ] Mỗi report có `format` field (không null, default "pdf")
- [ ] Report `completed` có `artifact_url` (nếu MinIO configured)

## Verification

```bash
TOKEN="<your-token>"

# Templates — must return 3 items
curl -s https://c12.openledger.vn/api/v1/reports/templates \
  -H "Authorization: Bearer $TOKEN" | jq '.templates | length'
# Expected: 3

# Templates IDs
curl -s https://c12.openledger.vn/api/v1/reports/templates \
  -H "Authorization: Bearer $TOKEN" | jq '[.templates[].id]'
# Expected: ["exec", "tech", "comp"]

# Templates route không conflict với report-by-id
# "templates" không được trả về như một report ID
curl -s https://c12.openledger.vn/api/v1/reports/templates \
  -H "Authorization: Bearer $TOKEN" | jq 'has("templates")'
# Expected: true (không phải 404 hay report object)

# Reports list có last_generated_at
curl -s https://c12.openledger.vn/api/v1/reports \
  -H "Authorization: Bearer $TOKEN" | jq '{last_generated_at, count: (.reports | length)}'
```
