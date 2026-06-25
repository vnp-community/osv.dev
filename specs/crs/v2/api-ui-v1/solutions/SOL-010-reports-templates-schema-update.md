# SOL-010: Reports — Templates Endpoint & Schema Updates

> **CR:** [CR-010](../CR-010-reports-templates-schema-update.md)  
> **Priority:** 🔴 HIGH (Phase 3)  
> **Service(s):** `finding-service` (`:8085`)  
> **Tạo:** 2026-06-19  
> **Cập nhật:** 2026-06-22  
> **Trạng thái:** ❌ PENDING — Chưa implement (xác nhận bởi API test 2026-06-22)

---

## 1. Tóm tắt Giải pháp

| Thay đổi | Loại | Mức độ |
|---|---|---|
| `GET /api/v1/reports/templates` | Endpoint mới (static config) | HIGH |
| `Report` schema thêm 6 fields | DB migration + model update | HIGH |
| `ReportsListResponse.last_generated_at` | Query aggregation | HIGH |
| `POST /api/v1/reports` request body | Schema update (backward compatible) | MEDIUM |
| Report artifact upload (MinIO) | Integration | MEDIUM |

---

## 2. DB Migration

### File: `services/finding-service/migrations/20260619_001_reports_schema.sql`

```sql
-- Thêm các fields mới vào bảng reports
ALTER TABLE reports
    ADD COLUMN IF NOT EXISTS format          VARCHAR(10)  NOT NULL DEFAULT 'pdf'
        CHECK (format IN ('pdf', 'html', 'csv', 'excel', 'json')),
    ADD COLUMN IF NOT EXISTS finding_count   INTEGER,
    ADD COLUMN IF NOT EXISTS file_size_bytes BIGINT,
    ADD COLUMN IF NOT EXISTS generated_at    TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS artifact_url    TEXT,
    ADD COLUMN IF NOT EXISTS expires_at      TIMESTAMPTZ;

-- Backfill: set format = 'pdf' cho các reports cũ (đã có DEFAULT)
-- Không cần explicit UPDATE vì có DEFAULT clause

-- Index cho last_generated_at query
CREATE INDEX IF NOT EXISTS idx_reports_generated_at
    ON reports(generated_at DESC)
    WHERE status = 'completed';
```

---

## 3. Domain Model

### File: `services/finding-service/internal/domain/report.go`

```go
package domain

import "time"

// Report — domain entity với đầy đủ fields
type Report struct {
    ID            string     `json:"id"`
    Name          string     `json:"name"`
    Type          string     `json:"type"`          // "Executive" | "Technical" | "Compliance"
    Format        string     `json:"format"`        // ← MỚI: "pdf" | "html" | "csv" | "excel" | "json"
    Status        string     `json:"status"`        // "pending" | "generating" | "completed" | "failed"
    FindingCount  *int       `json:"finding_count"` // ← MỚI: nullable khi chưa generate
    FileSizeBytes *int64     `json:"file_size_bytes"` // ← MỚI: nullable
    GeneratedAt   *time.Time `json:"generated_at"`  // ← MỚI: nullable khi đang generate
    ArtifactURL   *string    `json:"artifact_url"`  // ← MỚI: presigned URL
    ExpiresAt     *time.Time `json:"expires_at"`    // ← MỚI: URL expiry
    CreatedAt     time.Time  `json:"created_at"`
    CompletedAt   *time.Time `json:"completed_at"`
    CreatedBy     string     `json:"created_by"`
}

// ReportTemplate — static config (không cần DB)
type ReportTemplate struct {
    ID          string `json:"id"`          // "exec" | "tech" | "comp"
    Name        string `json:"name"`
    Description string `json:"description"`
    Type        string `json:"type"`        // "Executive" | "Technical" | "Compliance"
}

// ReportTemplatesResponse
type ReportTemplatesResponse struct {
    Templates []ReportTemplate `json:"templates"`
}

// ReportsListResponse — thêm last_generated_at
type ReportsListResponse struct {
    Reports         []Report   `json:"reports"`
    Total           int        `json:"total"`
    LastGeneratedAt *time.Time `json:"last_generated_at"` // ← MỚI: nullable
}

// CreateReportRequest — backward compatible, chỉ type+format required
type CreateReportRequest struct {
    Type         string     `json:"type"`          // required
    Format       string     `json:"format"`        // required: "pdf"|"html"|"csv"|"excel"|"json"
    ProductID    *string    `json:"product_id"`    // optional
    EngagementID *string    `json:"engagement_id"` // optional
    MinSeverity  *string    `json:"min_severity"`  // optional: "Critical"|"High"|"Medium"|"Low"
    MinScore     *float64   `json:"min_score"`     // optional
    DateFrom     *string    `json:"date_from"`     // optional: "2026-01-01"
    DateTo       *string    `json:"date_to"`       // optional: "2026-06-30"
}

// ReportUpdate — cho finalizeReport
type ReportUpdate struct {
    Status        string
    FindingCount  int
    FileSizeBytes int64
    ArtifactURL   string
    ExpiresAt     time.Time
    GeneratedAt   time.Time
    CompletedAt   time.Time
}
```

---

## 4. Report Templates (Static Config)

### File: `services/finding-service/internal/config/report_templates.go`

```go
package config

import "your-project/finding-service/internal/domain"

// DefaultReportTemplates — static list, không cần DB
// Có thể override qua config file nếu cần custom templates
var DefaultReportTemplates = []domain.ReportTemplate{
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

## 5. Handler

### File: `services/finding-service/internal/delivery/http/report_handler.go`

```go
package http

import (
    "encoding/json"
    "net/http"

    "your-project/finding-service/internal/config"
    "your-project/finding-service/internal/domain"
)

type ReportHandler struct {
    repo      ReportRepository
    generator ReportGenerator
    storage   StorageService // MinIO/S3
}

// GetReportTemplates godoc
// GET /api/v1/reports/templates
// ⚠️ PHẢI đăng ký TRƯỚC /api/v1/reports/{id} trong router
func (h *ReportHandler) GetReportTemplates(w http.ResponseWriter, r *http.Request) {
    // Nếu cần custom templates từ DB hoặc config file, override ở đây
    // Hiện tại: static config
    templates := config.DefaultReportTemplates

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(domain.ReportTemplatesResponse{
        Templates: templates,
    })
}

// ListReports godoc
// GET /api/v1/reports
// Response: ReportsListResponse với last_generated_at
func (h *ReportHandler) ListReports(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // Parse pagination
    page     := parseIntDefault(r.URL.Query().Get("page"), 1)
    pageSize := parseIntDefault(r.URL.Query().Get("page_size"), 20)

    // Get user context (chỉ xem reports của mình hoặc admin xem tất cả)
    userID   := r.Header.Get("X-User-ID")
    userRole := r.Header.Get("X-User-Role")

    reports, total, err := h.repo.List(ctx, ReportListFilter{
        UserID:   userID,
        IsAdmin:  userRole == "Admin",
        Page:     page,
        PageSize: pageSize,
    })
    if err != nil {
        jsonError(w, 500, "INTERNAL", "Failed to list reports")
        return
    }

    // Lấy last_generated_at
    lastGenAt, _ := h.repo.GetLastGeneratedAt(ctx)

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(domain.ReportsListResponse{
        Reports:         reports,
        Total:           total,
        LastGeneratedAt: lastGenAt, // nullable
    })
}

// CreateReport godoc
// POST /api/v1/reports
// Returns: 202 Accepted (async generation)
func (h *ReportHandler) CreateReport(w http.ResponseWriter, r *http.Request) {
    var req domain.CreateReportRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        jsonError(w, 400, "INVALID_BODY", "Invalid request body")
        return
    }

    // Validation
    validTypes   := map[string]bool{"Executive": true, "Technical": true, "Compliance": true}
    validFormats := map[string]bool{"pdf": true, "html": true, "csv": true, "excel": true, "json": true}
    if !validTypes[req.Type] {
        jsonError(w, 400, "VALIDATION_ERROR", "type must be Executive|Technical|Compliance")
        return
    }
    if req.Format == "" {
        req.Format = "pdf" // default
    }
    if !validFormats[req.Format] {
        jsonError(w, 400, "VALIDATION_ERROR", "format must be pdf|html|csv|excel|json")
        return
    }

    // Create report record (pending)
    report := &domain.Report{
        Name:      req.Type + " Report",
        Type:      req.Type,
        Format:    req.Format,
        Status:    "pending",
        CreatedBy: r.Header.Get("X-User-Email"),
    }

    created, err := h.repo.Create(r.Context(), report)
    if err != nil {
        jsonError(w, 500, "INTERNAL", "Failed to create report")
        return
    }

    // Trigger async generation
    go h.generator.Generate(report.ID, req)

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusAccepted) // 202
    json.NewEncoder(w).Encode(created)
}
```

---

## 6. Report Generator — Artifact Upload

### File: `services/finding-service/internal/service/report_generator.go`

```go
package service

import (
    "context"
    "os"
    "time"

    "your-project/finding-service/internal/domain"
    "your-project/finding-service/internal/infra/storage"
)

type ReportGenerator struct {
    repo      ReportRepository
    storage   storage.Service // MinIO/S3
    formatter Formatter
}

// Generate — async report generation pipeline
func (g *ReportGenerator) Generate(reportID string, req domain.CreateReportRequest) {
    ctx := context.Background()

    // 1. Update status → "generating"
    g.repo.UpdateStatus(ctx, reportID, "generating")

    // 2. Fetch findings data
    findings, err := g.fetchFindings(ctx, req)
    if err != nil {
        g.repo.UpdateStatus(ctx, reportID, "failed")
        return
    }

    // 3. Format report theo format type
    tmpPath, err := g.formatter.Format(ctx, req.Format, findings, req.Type)
    if err != nil {
        g.repo.UpdateStatus(ctx, reportID, "failed")
        return
    }
    defer os.Remove(tmpPath)

    // 4. Get file size
    info, err := os.Stat(tmpPath)
    if err != nil {
        g.repo.UpdateStatus(ctx, reportID, "failed")
        return
    }

    // 5. Upload tới MinIO/S3
    // TTL 7 ngày cho presigned URL
    artifactURL, expiresAt, err := g.storage.UploadAndSign(
        ctx, tmpPath,
        "reports/"+reportID+"."+req.Format,
        7*24*time.Hour,
    )
    if err != nil {
        g.repo.UpdateStatus(ctx, reportID, "failed")
        return
    }

    // 6. Update report record
    now := time.Now().UTC()
    err = g.repo.FinalizeReport(ctx, reportID, domain.ReportUpdate{
        Status:        "completed",
        FindingCount:  len(findings),
        FileSizeBytes: info.Size(),
        ArtifactURL:   artifactURL,
        ExpiresAt:     expiresAt,
        GeneratedAt:   now,
        CompletedAt:   now,
    })
    if err != nil {
        log.Error().Err(err).Str("report_id", reportID).Msg("failed to finalize report")
    }
}
```

### Repository queries mới

```go
// services/finding-service/internal/infra/postgres/report_repo.go

// GetLastGeneratedAt — lấy thời điểm report gần nhất được generate
func (r *ReportRepo) GetLastGeneratedAt(ctx context.Context) (*time.Time, error) {
    var t time.Time
    err := r.db.QueryRow(ctx, `
        SELECT MAX(generated_at) FROM reports WHERE status = 'completed'
    `).Scan(&t)
    if err != nil || t.IsZero() {
        return nil, nil // null nếu chưa có report nào completed
    }
    return &t, nil
}

// FinalizeReport — update sau khi generate xong
func (r *ReportRepo) FinalizeReport(ctx context.Context, id string, upd domain.ReportUpdate) error {
    _, err := r.db.Exec(ctx, `
        UPDATE reports
        SET status         = $1,
            finding_count  = $2,
            file_size_bytes = $3,
            artifact_url   = $4,
            expires_at     = $5,
            generated_at   = $6,
            completed_at   = $7
        WHERE id = $8
    `, upd.Status, upd.FindingCount, upd.FileSizeBytes,
        upd.ArtifactURL, upd.ExpiresAt, upd.GeneratedAt, upd.CompletedAt,
        id)
    return err
}
```

---

## 7. Gateway Routing

### File: `apps/osv/internal/gateway/router.go`

```go
// ⚠️ CRITICAL: /reports/templates phải đăng ký TRƯỚC /reports/{id}
// để tránh "templates" bị parse thành report ID

mux.Handle("GET /api/v1/reports/templates",
    protected(proxy.Forward("finding-service:8085")))

// Routes hiện có (verify chúng vẫn hoạt động)
mux.Handle("GET /api/v1/reports",
    protected(proxy.Forward("finding-service:8085")))
mux.Handle("POST /api/v1/reports",
    rateLimit("5/minute")(protected(proxy.Forward("finding-service:8085"))))
mux.Handle("GET /api/v1/reports/{id}",
    protected(proxy.Forward("finding-service:8085")))
mux.Handle("GET /api/v1/reports/{id}/download",
    protected(proxy.Forward("finding-service:8085")))
```

> **Go 1.22 note:** Với pattern matching `/reports/templates` vs `/reports/{id}`, Go 1.22 ServeMux tự động ưu tiên literal path. Tuy nhiên vẫn nên đăng ký explicit theo thứ tự để rõ ràng.

---

## 8. Storage Service Interface

### File: `services/finding-service/internal/infra/storage/minio.go`

```go
package storage

import (
    "context"
    "time"

    "github.com/minio/minio-go/v7"
)

type Service interface {
    UploadAndSign(ctx context.Context, filePath, objectName string, ttl time.Duration) (url string, expiresAt time.Time, err error)
}

type MinioService struct {
    client *minio.Client
    bucket string
}

func (s *MinioService) UploadAndSign(ctx context.Context, filePath, objectName string, ttl time.Duration) (string, time.Time, error) {
    // Upload file
    _, err := s.client.FPutObject(ctx, s.bucket, objectName, filePath, minio.PutObjectOptions{})
    if err != nil {
        return "", time.Time{}, err
    }

    // Generate presigned URL
    expiresAt := time.Now().Add(ttl)
    presignedURL, err := s.client.PresignedGetObject(ctx, s.bucket, objectName, ttl, nil)
    if err != nil {
        return "", time.Time{}, err
    }

    return presignedURL.String(), expiresAt, nil
}
```

---

## 9. Tests

```go
func TestGetReportTemplates_Returns3Templates(t *testing.T) {
    resp := GET("/api/v1/reports/templates")
    var result domain.ReportTemplatesResponse
    json.Unmarshal(resp.Body, &result)

    assert.Len(t, result.Templates, 3)
    // Verify IDs
    ids := []string{result.Templates[0].ID, result.Templates[1].ID, result.Templates[2].ID}
    assert.Contains(t, ids, "exec")
    assert.Contains(t, ids, "tech")
    assert.Contains(t, ids, "comp")
}

func TestGetReportTemplates_NoConflictWithReportByID(t *testing.T) {
    // /reports/templates không được hiểu là /reports/{id} với id="templates"
    resp := GET("/api/v1/reports/templates")
    assert.Equal(t, 200, resp.StatusCode)
    // Response phải là templates, không phải 404 "report not found"
    assert.NotContains(t, resp.Body, "not found")
}

func TestListReports_HasLastGeneratedAt(t *testing.T) {
    resp := GET("/api/v1/reports")
    var result domain.ReportsListResponse
    json.Unmarshal(resp.Body, &result)
    // last_generated_at có thể null nếu chưa có completed report
    // Nếu có, phải là valid datetime
}

func TestCreateReport_Returns202(t *testing.T) {
    body := `{"type": "Executive", "format": "pdf"}`
    resp := POST("/api/v1/reports", body)
    assert.Equal(t, 202, resp.StatusCode)
}

func TestCompletedReport_HasArtifactURL(t *testing.T) {
    // Trigger report generation
    // Wait for completion
    // GET /api/v1/reports/{id}
    // artifact_url phải không null
    // artifact_url phải có expiry >= 7 days from generated_at
}
```

---

## 10. Acceptance Criteria Checklist

> **Kết quả API Test (2026-06-22):**
> - `GET /api/v1/reports/templates` → ❌ **404** (endpoint chưa tồn tại)
> - `GET /api/v1/reports` → ❌ **404** (route chưa đăng ký trong gateway)
> - `POST /api/v1/reports` → ❌ **404** (route chưa đăng ký)

- [ ] `GET /api/v1/reports/templates` → `{ templates: [3 items] }` — HTTP 200
- [ ] Templates có: `id`, `name`, `description`, `type`
- [ ] `GET /api/v1/reports` → `last_generated_at` field (ISO datetime hoặc null)
- [ ] Mỗi `Report` có `format` field — không null
- [ ] Report `completed` có `artifact_url` (presigned URL) — không null
- [ ] `artifact_url` có hiệu lực ≥ 7 ngày từ generate time
- [ ] `POST /api/v1/reports` với `format: "pdf"` → HTTP 202
- [ ] `/reports/templates` không conflict với `/reports/{id}`
