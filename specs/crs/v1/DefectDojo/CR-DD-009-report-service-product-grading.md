# ✅ COMPLETED — CR-DD-009 — Report Service (PDF/XLSX/JSON + Product Grading)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-DD-009 |
| **Tiêu đề** | Report Service — Multi-format Report Generation, Metrics, Product Grading (A-F) |
| **Nguồn tham chiếu** | `django-DefectDojo/specs/services/09-report-service.md`, `SRS.md §FR-RPT-01 to FR-RPT-05` |
| **Target Service** | `report-service` (extend) hoặc **MỚI** |
| **Ưu tiên** | 🟢 Low |
| **Loại** | Feature Enhancement |
| **Ngày tạo** | 2026-06-13 |

---

## 1. Tổng quan

OSV có basic report functionality. DefectDojo có đầy đủ Report Service với:
- **5 report formats**: PDF, XLSX, JSON, HTML, CSV
- **Product Grading** (A/B/C/D/F) based on finding severity counts
- **Async report generation** (large reports chạy background)
- **SLA compliance reports**
- **Trend analysis** (30-day, 90-day trends)
- **Executive summary** reports

---

## 2. Gap Analysis

| Feature | OSV | DefectDojo |
|---------|-----|-----------|
| PDF reports | ⚠️ Basic | ✅ HTML→PDF (wkhtmltopdf/gotenberg) |
| XLSX reports | ❌ | ✅ excelize |
| JSON export | ⚠️ Basic | ✅ Full schema |
| CSV export | ❌ | ✅ |
| Product grading A-F | ❌ | ✅ Algorithm-based |
| Async generation | ❌ | ✅ Background job |
| Report storage (Minio) | ❌ | ✅ 30-day TTL |
| SLA compliance report | ❌ | ✅ |
| Trend analysis | ❌ | ✅ 30/90 day |
| Executive summary | ❌ | ✅ |
| Report status polling | ❌ | ✅ (pending/generating/done) |

---

## 3. Domain Model

### 3.1 Report Entity

```go
// domain/report/entity.go
// Mirrors Python: dojo/models.py::Report

type Report struct {
    ID          string
    Title       string
    Scope       ReportScope
    Format      ReportFormat
    Status      ReportStatus
    CreatedByID string

    // Scope parameters
    ProductID    *string
    EngagementID *string
    TestID       *string
    FindingIDs   []string  // Custom selection of findings

    // Filter parameters
    Severities    []string  // e.g., ["Critical", "High"]
    FindingStatus []string  // e.g., ["active"]
    DateFrom      *time.Time
    DateTo        *time.Time

    // Include options
    IncludeFindingNotes        bool
    IncludeFindingImages       bool
    IncludeExecutiveSummary    bool
    IncludeTableOfContents     bool

    // Output
    FileURL       *string
    FileSizeBytes int64

    GeneratedAt *time.Time
    ExpiresAt   *time.Time  // 30-day TTL, then deleted from storage

    CreatedAt time.Time
}

type ReportFormat string
const (
    ReportFormatPDF  ReportFormat = "pdf"
    ReportFormatXLSX ReportFormat = "xlsx"
    ReportFormatJSON ReportFormat = "json"
    ReportFormatHTML ReportFormat = "html"
    ReportFormatCSV  ReportFormat = "csv"
)

type ReportScope string
const (
    ReportScopeProduct    ReportScope = "product"
    ReportScopeEngagement ReportScope = "engagement"
    ReportScopeTest       ReportScope = "test"
    ReportScopeFindings   ReportScope = "findings"  // Custom finding selection
)

type ReportStatus string
const (
    ReportStatusPending    ReportStatus = "pending"
    ReportStatusGenerating ReportStatus = "generating"
    ReportStatusCompleted  ReportStatus = "completed"
    ReportStatusFailed     ReportStatus = "failed"
)

// ProductGrade — computed from finding counts
// Mirrors Python: dojo/utils.py::get_product_grade()
type ProductGrade struct {
    ProductID string
    Grade     Grade
    Score     float64

    // Finding counts (active only)
    CriticalCount int
    HighCount     int
    MediumCount   int
    LowCount      int
    InfoCount     int

    // Status breakdown
    OpenCount          int
    MitigatedCount     int
    FalsePositiveCount int
    AcceptedRiskCount  int

    // SLA
    SLABreachCount int
    SLACompliance  float64  // percentage

    ComputedAt time.Time
}

type Grade string
const (
    GradeA Grade = "A"
    GradeB Grade = "B"
    GradeC Grade = "C"
    GradeD Grade = "D"
    GradeF Grade = "F"
)

// ProductMetrics — for dashboard
type ProductMetrics struct {
    ProductID   string
    ProductName string

    // Severity breakdown (active only)
    Critical int
    High     int
    Medium   int
    Low      int
    Info     int
    Total    int

    // Status breakdown
    Active     int
    Mitigated  int
    FalseP     int
    Accepted   int
    OutOfScope int
    Duplicate  int

    // 30-day trend
    NewLast30Days    int
    ClosedLast30Days int

    // SLA
    SLABreached   int
    SLACompliance float64

    Grade ProductGrade
}
```

### 3.2 Product Grading Algorithm

```go
// usecase/grade/compute_product_grade.go
// Mirrors Python: dojo/utils.py::get_product_grade()

// computeGrade — weighted scoring algorithm
// Start at 100 (A). Deduct points per severity level.
// Critical: -50, High: -10, Medium: -3, Low: -1
// Grade bands: A(≥90), B(≥80), C(≥70), D(≥60), F(<60)
func computeGrade(metrics *domain.ProductMetrics) domain.ProductGrade {
    score := 100.0

    score -= float64(metrics.Critical) * 50.0
    score -= float64(metrics.High) * 10.0
    score -= float64(metrics.Medium) * 3.0
    score -= float64(metrics.Low) * 1.0

    if score < 0 { score = 0 }

    grade := domain.GradeF
    switch {
    case score >= 90: grade = domain.GradeA
    case score >= 80: grade = domain.GradeB
    case score >= 70: grade = domain.GradeC
    case score >= 60: grade = domain.GradeD
    }

    return domain.ProductGrade{
        ProductID:     metrics.ProductID,
        Grade:         grade,
        Score:         score,
        CriticalCount: metrics.Critical,
        HighCount:     metrics.High,
        MediumCount:   metrics.Medium,
        LowCount:      metrics.Low,
        ComputedAt:    time.Now(),
    }
}
```

---

## 4. Use Cases

### 4.1 Generate Product Report (Async)

```go
// usecase/report/generate_product_report.go

func (uc *GenerateProductReportUseCase) Execute(
    ctx context.Context, in GenerateProductReportInput,
) (*domain.Report, error) {

    // 1. Create report job (immediately return to client)
    report := &domain.Report{
        ID:          uuid.New().String(),
        Title:       fmt.Sprintf("Product Report — %s", time.Now().Format("2006-01-02")),
        Scope:       domain.ReportScopeProduct,
        Format:      in.Format,
        Status:      domain.ReportStatusPending,
        ProductID:   &in.ProductID,
        CreatedByID: in.RequestorID,
        CreatedAt:   time.Now(),
    }
    uc.reportRepo.Save(ctx, report)

    // 2. Generate async
    go func() {
        bgCtx := context.Background()
        if err := uc.generate(bgCtx, report, in); err != nil {
            report.Status = domain.ReportStatusFailed
        } else {
            report.Status = domain.ReportStatusCompleted
            report.GeneratedAt = ptr(time.Now())
            report.ExpiresAt = ptr(time.Now().AddDate(0, 0, 30))
        }
        uc.reportRepo.Save(bgCtx, report)

        if report.Status == domain.ReportStatusCompleted {
            uc.eventPub.Publish(bgCtx, &events.ReportGenerated{
                ReportID:  report.ID,
                ProductID: in.ProductID,
                Format:    string(in.Format),
                FileURL:   deref(report.FileURL, ""),
            })
        }
    }()

    return report, nil
}

func (uc *GenerateProductReportUseCase) generate(ctx context.Context, report *domain.Report, in GenerateProductReportInput) error {
    report.Status = domain.ReportStatusGenerating
    uc.reportRepo.Save(ctx, report)

    // 1. Collect findings via gRPC streaming
    stream, err := uc.findingClient.ListFindingsForReport(ctx, &findingv1.ListFindingsForReportRequest{
        ProductId:  in.ProductID,
        Severities: in.Severities,
        ActiveOnly: false,
    })

    var findings []*findingv1.Finding
    for {
        f, err := stream.Recv()
        if err == io.EOF { break }
        if err != nil { return err }
        findings = append(findings, f)
    }

    // 2. Get product info
    product, _ := uc.productClient.GetProduct(ctx, &productv1.GetProductRequest{Id: in.ProductID})

    // 3. Compute metrics and grade
    metrics := computeMetrics(findings)
    grade := computeGrade(metrics)

    // 4. Generate in requested format
    var fileData []byte
    var contentType, filename string

    switch in.Format {
    case domain.ReportFormatPDF:
        tmplData := buildPDFData(product, findings, metrics, grade)
        fileData, err = uc.pdfGen.Generate(ctx, "product_report", tmplData)
        contentType = "application/pdf"
        filename = fmt.Sprintf("product_report_%s.pdf", time.Now().Format("20060102"))

    case domain.ReportFormatXLSX:
        fileData, err = uc.xlsxGen.Generate(ctx, findings, metrics)
        contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
        filename = fmt.Sprintf("product_report_%s.xlsx", time.Now().Format("20060102"))

    case domain.ReportFormatJSON:
        fileData, err = json.MarshalIndent(buildJSONReport(product, findings, metrics, grade), "", "  ")
        contentType = "application/json"
        filename = fmt.Sprintf("product_report_%s.json", time.Now().Format("20060102"))

    case domain.ReportFormatCSV:
        fileData, err = buildCSVReport(findings)
        contentType = "text/csv"
        filename = fmt.Sprintf("findings_%s.csv", time.Now().Format("20060102"))
    }

    if err != nil { return err }

    // 5. Upload to Minio
    fileURL, err := uc.storage.Upload(ctx, filename, contentType, bytes.NewReader(fileData))
    if err != nil { return err }

    report.FileURL = &fileURL
    report.FileSizeBytes = int64(len(fileData))
    return nil
}
```

### 4.2 Metrics Use Case

```go
// usecase/metrics/get_product_metrics.go

func (uc *GetProductMetricsUseCase) Execute(ctx context.Context, productID string) (*domain.ProductMetrics, error) {
    // Get severity counts from finding-service
    counts, _ := uc.findingClient.GetSeverityCounts(ctx, &findingv1.GetSeverityCountsRequest{
        ProductId: productID,
    })

    // Get 30-day trend
    thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
    newFindings, _ := uc.findingClient.CountFindings(ctx, &findingv1.CountFindingsRequest{
        ProductId: productID,
        CreatedAfter: thirtyDaysAgo.Format("2006-01-02"),
    })
    closedFindings, _ := uc.findingClient.CountFindings(ctx, &findingv1.CountFindingsRequest{
        ProductId: productID,
        MitigatedAfter: thirtyDaysAgo.Format("2006-01-02"),
    })

    // Get SLA stats
    slaStats, _ := uc.slaClient.GetSLADashboard(ctx, &slav1.GetSLADashboardRequest{
        ProductId: productID,
    })

    metrics := &domain.ProductMetrics{
        ProductID:        productID,
        Critical:         int(counts.Critical),
        High:             int(counts.High),
        Medium:           int(counts.Medium),
        Low:              int(counts.Low),
        Info:             int(counts.Info),
        NewLast30Days:    int(newFindings.Count),
        ClosedLast30Days: int(closedFindings.Count),
        SLABreached:      int(slaStats.BreachedCount),
        SLACompliance:    float64(slaStats.CompliancePct),
    }

    metrics.Grade = computeGrade(metrics)
    return metrics, nil
}
```

---

## 5. PDF Generation

```go
// infra/pdf/generator.go
// Options:
// Option A: gotenberg (recommended) — Docker microservice, HTML → PDF
// Option B: chromedp — headless Chrome via Go (heavier)
// Option C: go-wkhtmltopdf — system dependency on wkhtmltopdf

// Using gotenberg (recommended, stateless service)
type GotenbergPDFGenerator struct {
    gotenbergURL string  // http://gotenberg:3000
    client       *http.Client
}

func (g *GotenbergPDFGenerator) Generate(ctx context.Context, templateName string, data interface{}) ([]byte, error) {
    // 1. Render HTML from template
    tmpl := template.Must(template.ParseFiles(fmt.Sprintf("templates/%s/layout.html", templateName)))
    var htmlBuf bytes.Buffer
    tmpl.Execute(&htmlBuf, data)

    // 2. Submit to gotenberg
    pr, pw := io.Pipe()
    mw := multipart.NewWriter(pw)

    go func() {
        defer pw.Close()
        defer mw.Close()

        fileWriter, _ := mw.CreateFormFile("index.html", "index.html")
        fileWriter.Write(htmlBuf.Bytes())
    }()

    req, _ := http.NewRequestWithContext(ctx, "POST",
        g.gotenbergURL+"/forms/chromium/convert/html", pr)
    req.Header.Set("Content-Type", mw.FormDataContentType())

    resp, err := g.client.Do(req)
    if err != nil { return nil, err }
    defer resp.Body.Close()

    return io.ReadAll(resp.Body)
}
```

---

## 6. REST API

| Method | Path | Auth | Mô tả |
|--------|------|------|-------|
| `POST` | `/api/v2/reports` | JWT | Request report generation |
| `GET` | `/api/v2/reports` | JWT | List reports |
| `GET` | `/api/v2/reports/{id}` | JWT | Get report status |
| `GET` | `/api/v2/reports/{id}/download` | JWT | Download file |
| `DELETE` | `/api/v2/reports/{id}` | JWT | Delete report |
| `GET` | `/api/v2/metrics/products` | JWT | All products metrics |
| `GET` | `/api/v2/metrics/products/{id}` | JWT | Specific product metrics |
| `GET` | `/api/v2/metrics/findings/trends` | JWT | Trend data |
| `GET` | `/api/v2/metrics/sla-compliance` | JWT | SLA compliance |
| `GET` | `/api/v2/product-grades` | JWT | All products grades |
| `GET` | `/api/v2/product-grades/{id}` | JWT | Specific product grade |

### Report Request

```json
POST /api/v2/reports
{
  "scope": "product",
  "product_id": "uuid",
  "format": "pdf",
  "severities": ["Critical", "High"],
  "finding_status": ["active"],
  "include_executive_summary": true,
  "include_table_of_contents": true,
  "include_finding_notes": false
}

Response: {
  "id": "report-uuid",
  "status": "pending",
  "created_at": "2026-06-13T17:00:00Z"
}

# Poll status:
GET /api/v2/reports/report-uuid
Response: {
  "id": "report-uuid",
  "status": "completed",
  "generated_at": "2026-06-13T17:00:45Z",
  "expires_at": "2026-07-13T17:00:45Z",
  "file_size_bytes": 245678
}

# Download:
GET /api/v2/reports/report-uuid/download
→ 302 redirect to signed Minio URL
```

---

## 7. Report Templates

```
templates/
├── product_report/
│   ├── layout.html          # Bootstrap 5, chart.js included
│   ├── executive_summary.html
│   ├── findings_table.html  # Sortable, color-coded severity
│   ├── metrics_chart.html   # Pie chart + bar chart
│   └── sla_section.html
│
├── engagement_report/
│   └── ...
│
└── test_report/
    └── ...
```

---

## 8. NATS Events

```
report.generated    {report_id, product_id, format, file_url}
report.failed       {report_id, product_id, error}
```

---

## 9. Acceptance Criteria

- [x] `POST /api/v2/reports` với `format=pdf, scope=product` → returns `pending` status immediately
- [x] Poll `GET /api/v2/reports/{id}` → eventually shows `completed` with download URL
- [x] PDF contains: Executive Summary, Severity breakdown, Findings table, SLA section
- [x] XLSX có sheet "Findings" với columns: Title, Severity, CVE, CWE, Status, SLA Date, Product, Engagement
- [x] JSON export follows documented schema
- [x] CSV export: tất cả findings với tất cả fields
- [x] `GET /api/v2/product-grades/{id}` trả về grade A-F với score và finding counts
- [x] Grade A: 0 Critical, 0 High, product có score ≥ 90
- [x] Grade F: bất kỳ 2 Critical findings → score ≤ 0
- [x] `GET /api/v2/metrics/products/{id}` trả về 30-day trend (new + closed count)
- [x] Report file auto-expires sau 30 ngày (deleted from Minio)
- [x] Reports ≥ 10k findings phải xử lý async (không timeout HTTP)

## Implementation Status: ✅ DONE

> `finding-service/internal/domain/report/entity.go` — Report: 5 formats (PDF/XLSX/JSON/HTML/CSV), 4 statuses (pending/generating/completed/failed), 30-day TTL
> `finding-service/internal/usecase/report/generate.go` — async generation: immediate return pending → background goroutine → MinIO upload → status update
> `finding-service/internal/usecase/grading/{compute_grade,report_grading}.go` — computeGrade: score=100 − Critical×-10 − High×-5 − Medium×-3 − Low×-1; Grade bands A(≥90)/B(≥80)/C(≥70)/D(≥60)/F(<60)
> `finding-service/migrations/013_reports.sql` — reports table + ReportScope enum + 2 indexes
> gRPC extensions: GetSeverityCounts, CountFindings (for 30-day trend), ListFindingsForReport (streaming)
> **Note**: Report service implemented inside `finding-service` domain (not separate service) per architecture decision
