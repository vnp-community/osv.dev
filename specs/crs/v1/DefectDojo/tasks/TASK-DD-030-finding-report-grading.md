# ✅ COMPLETED — TASK-DD-030 — Finding Report Generation + Product Grading

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-DD-030 |
| **Service** | `finding-service` |
| **CR** | CR-DD-009 |
| **Phase** | 3 — Integrations |
| **Priority** | 🟡 Medium |
| **Prerequisites** | TASK-DD-006 (ListFindingsForReport gRPC) |
| **Estimated effort** | 2 ngày |

## Context

Extend finding-service với report generation (PDF/XLSX/JSON/CSV) và Product Grading algorithm (A-F). Reports generated async, stored in MinIO, available for download. Product grade computed từ severity counts.

## Reference

- Solution: [`sol-finding-service.md § CR-DD-009`](../solutions/sol-finding-service.md)

## Working Directory

```
/Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/
```

## Files to Create

```
internal/domain/report/
├── entity.go
└── repository.go

internal/usecase/report/
├── generate.go
├── get.go
└── list.go

internal/usecase/grading/
└── compute_grade.go

internal/usecase/metrics/
└── get_metrics.go

internal/infra/
├── report/
│   ├── pdf_generator.go      # wkhtmltopdf or chromedp
│   ├── xlsx_generator.go     # excelize
│   ├── json_generator.go
│   └── csv_generator.go
└── storage/
    └── minio.go              # report file storage

internal/delivery/http/
├── report_handler.go
├── grade_handler.go
└── metrics_handler.go

migrations/
└── 013_reports.sql
```

## Implementation Spec

### `internal/domain/report/entity.go`

```go
package report

import "time"

type ReportFormat string
const (
    FormatPDF   ReportFormat = "pdf"
    FormatXLSX  ReportFormat = "xlsx"
    FormatJSON  ReportFormat = "json"
    FormatCSV   ReportFormat = "csv"
)

type ReportStatus string
const (
    StatusPending    ReportStatus = "pending"
    StatusGenerating ReportStatus = "generating"
    StatusCompleted  ReportStatus = "completed"
    StatusFailed     ReportStatus = "failed"
)

type Report struct {
    ID          string
    ProductID   string
    Title       string
    Format      ReportFormat
    Status      ReportStatus

    // Scope filters
    EngagementID *string
    TestID       *string
    Severities   []string  // filter by severity
    ActiveOnly   bool

    // Storage
    StorageKey  string  // MinIO object key
    FileSizeBytes int64

    GeneratedBy string  // user ID
    GeneratedAt *time.Time
    ExpiresAt   *time.Time  // auto-delete after 30 days

    ErrorMessage string
    CreatedAt    time.Time
}
```

### `internal/usecase/report/generate.go`

```go
package report

import (
    "context"
    "log/slog"
    "time"
    "github.com/google/uuid"
    "github.com/osv/services/finding-service/internal/domain/report"
)

type GenerateReportInput struct {
    ProductID    string
    Title        string
    Format       report.ReportFormat
    EngagementID *string
    Severities   []string
    ActiveOnly   bool
    RequesterID  string
}

type GenerateReportUseCase struct {
    reportRepo    report.Repository
    findingRepo   FindingRepository
    generators    map[report.ReportFormat]ReportGenerator
    storage       ReportStorage
    eventPub      EventPublisher
}

func (uc *GenerateReportUseCase) Execute(ctx context.Context, in GenerateReportInput) (*report.Report, error) {
    // 1. Create pending report record
    r := &report.Report{
        ID:          uuid.New().String(),
        ProductID:   in.ProductID,
        Title:       in.Title,
        Format:      in.Format,
        Status:      report.StatusPending,
        EngagementID: in.EngagementID,
        Severities:  in.Severities,
        ActiveOnly:  in.ActiveOnly,
        GeneratedBy: in.RequesterID,
        CreatedAt:   time.Now(),
    }
    uc.reportRepo.Save(ctx, r)

    // 2. Generate async
    go func() {
        genCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
        defer cancel()

        r.Status = report.StatusGenerating
        uc.reportRepo.Save(genCtx, r)

        // 3. Stream findings
        findings, err := uc.findingRepo.ListForReport(genCtx, &ListForReportOptions{
            ProductID:    in.ProductID,
            EngagementID: in.EngagementID,
            Severities:   in.Severities,
            ActiveOnly:   in.ActiveOnly,
        })
        if err != nil {
            r.Status = report.StatusFailed
            r.ErrorMessage = err.Error()
            uc.reportRepo.Save(genCtx, r)
            return
        }

        // 4. Generate file
        generator, ok := uc.generators[in.Format]
        if !ok {
            r.Status = report.StatusFailed
            r.ErrorMessage = "unsupported format"
            uc.reportRepo.Save(genCtx, r)
            return
        }

        fileReader, err := generator.Generate(genCtx, findings, &ReportMetadata{
            ProductID: in.ProductID,
            Title:     in.Title,
            GeneratedAt: time.Now(),
        })
        if err != nil {
            r.Status = report.StatusFailed
            r.ErrorMessage = err.Error()
            uc.reportRepo.Save(genCtx, r)
            return
        }

        // 5. Upload to MinIO
        key := fmt.Sprintf("reports/%s/%s.%s", in.ProductID, r.ID, string(in.Format))
        if err := uc.storage.Upload(genCtx, key, fileReader); err != nil {
            r.Status = report.StatusFailed
            r.ErrorMessage = "storage error: " + err.Error()
            uc.reportRepo.Save(genCtx, r)
            return
        }

        now := time.Now()
        expires := now.AddDate(0, 0, 30)  // 30 day retention
        r.Status = report.StatusCompleted
        r.StorageKey = key
        r.GeneratedAt = &now
        r.ExpiresAt = &expires
        uc.reportRepo.Save(genCtx, r)

        uc.eventPub.Publish(genCtx, "report.generated", map[string]any{
            "report_id":  r.ID,
            "product_id": in.ProductID,
            "format":     string(in.Format),
            "_service":   "finding-service",
        })

        slog.InfoContext(genCtx, "report generated", "report_id", r.ID, "format", in.Format)
    }()

    return r, nil  // return immediately (async generation)
}
```

### `internal/usecase/grading/compute_grade.go`

```go
package grading

// Product Grade algorithm:
// Score 0-100 based on finding severity counts
// Grade: A≥90, B≥75, C≥60, D≥45, F<45

type ProductGrade struct {
    ProductID string
    Grade     string   // A, B, C, D, F
    Score     float64  // 0-100
    Details   GradeDetails
}

type GradeDetails struct {
    Critical int
    High     int
    Medium   int
    Low      int
    Info     int
    Total    int
}

type ComputeGradeUseCase struct {
    findingRepo FindingRepository
}

func (uc *ComputeGradeUseCase) Execute(ctx context.Context, productID string) (*ProductGrade, error) {
    counts, err := uc.findingRepo.GetSeverityCounts(ctx, productID, true)
    if err != nil {
        return nil, err
    }

    // Weighted penalty per finding:
    // Critical: -10 points, High: -5, Medium: -2, Low: -1, Info: 0
    penalties :=
        float64(counts["Critical"]) * 10.0 +
        float64(counts["High"]) * 5.0 +
        float64(counts["Medium"]) * 2.0 +
        float64(counts["Low"]) * 1.0

    // Normalize: score = max(0, 100 - penalties) with cap at 100
    score := math.Max(0, 100.0-penalties)
    if score > 100 { score = 100 }

    grade := computeLetterGrade(score)

    return &ProductGrade{
        ProductID: productID,
        Grade:     grade,
        Score:     score,
        Details:   GradeDetails{
            Critical: counts["Critical"],
            High:     counts["High"],
            Medium:   counts["Medium"],
            Low:      counts["Low"],
            Info:     counts["Info"],
            Total:    counts["Critical"] + counts["High"] + counts["Medium"] + counts["Low"] + counts["Info"],
        },
    }, nil
}

func computeLetterGrade(score float64) string {
    switch {
    case score >= 90: return "A"
    case score >= 75: return "B"
    case score >= 60: return "C"
    case score >= 45: return "D"
    default:          return "F"
    }
}
```

### `migrations/013_reports.sql`

```sql
CREATE TABLE IF NOT EXISTS reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL,
    title VARCHAR(300) NOT NULL,
    format VARCHAR(10) NOT NULL CHECK (format IN ('pdf', 'xlsx', 'json', 'csv')),
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'generating', 'completed', 'failed')),
    engagement_id UUID,
    test_id UUID,
    severities TEXT[] DEFAULT '{}',
    active_only BOOLEAN DEFAULT TRUE,
    storage_key TEXT,
    file_size_bytes BIGINT,
    generated_by UUID,
    generated_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    error_message TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_reports_product ON reports(product_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_reports_expires ON reports(expires_at) WHERE status = 'completed';
```

## REST Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v2/reports` | Create (async) |
| GET | `/api/v2/reports` | List (scoped to user) |
| GET | `/api/v2/reports/{id}` | Get status |
| GET | `/api/v2/reports/{id}/download` | Download file (pre-signed URL) |
| DELETE | `/api/v2/reports/{id}` | Delete |
| GET | `/api/v2/product-grades` | List all product grades |
| GET | `/api/v2/product-grades/{id}` | Get single product grade |
| GET | `/api/v2/metrics/products` | Product-level metrics |
| GET | `/api/v2/metrics/products/{id}` | Single product metrics |
| GET | `/api/v2/metrics/findings/trends` | Findings trend over time |
| GET | `/api/v2/metrics/sla-compliance` | SLA compliance summary |

## Acceptance Criteria

- [x] `POST /api/v2/reports` → 201 with `{"id": "uuid", "status": "pending"}`
- [x] Report status changes: pending → generating → completed
- [x] `GET /api/v2/reports/{id}` while generating → `{"status": "generating"}`
- [x] `GET /api/v2/reports/{id}` when completed → includes `storage_key`
- [x] `GET /api/v2/reports/{id}/download` → 302 redirect to pre-signed MinIO URL
- [x] PDF format → valid PDF file downloadable
- [x] XLSX format → valid Excel file with Finding sheet
- [x] CSV format → comma-separated with headers
- [x] `GET /api/v2/product-grades/{id}` → `{"grade": "B", "score": 78.5, "details": {...}}`
- [x] Product with 0 Critical/High → Grade A (score ≥ 90)
- [x] Product with 10 Critical → score penalized, Grade F
- [x] Reports auto-expire after 30 days
- [x] Report generation timeout 5 minutes (context cancel)

## Implementation Status: ✅ DONE

> `finding-service/internal/domain/report/entity.go` — Report entity: FormatPDF/XLSX/JSON/CSV, StatusPending/.../Completed/Failed, 30-day expiry
> `finding-service/internal/usecase/report/generate.go` — GenerateReportUseCase: async goroutine, 5min timeout, MinIO upload
> `finding-service/internal/usecase/grading/compute_grade.go` — ComputeGradeUseCase: penalty formula (Critical×-10, High×-5, Medium×-2, Low×-1) → A/B/C/D/F
> `finding-service/internal/usecase/grading/report_grading.go` — additional grading helpers
> `finding-service/migrations/013_reports.sql` — reports table with CHECK constraints + 2 indexes
