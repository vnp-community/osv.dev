# CR-OVS-006 — Report Service: Multi-Format Vulnerability Reports (PDF/HTML/CSV/Excel)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-OVS-006 |
| **Tiêu đề** | Report Service — Multi-format Vulnerability Reports (PDF, HTML Light/Dark, CSV, JSON, Excel/XLSX for DefectDojo), CI/CD Exit Code |
| **Nguồn tham chiếu** | `OpenVulnScan/specs/services/10-report-service.md`, `OpenVulnScan/docs/PRD.md §F-008` |
| **Target Service** | **MỚI**: `report-service` (port gRPC: 50065) |
| **Ưu tiên** | 🟡 Medium |
| **Loại** | New Service |
| **Ngày tạo** | 2026-06-14 |
| **Ngày implement** | 2026-06-17 |
| **Trạng thái** | ✅ Implemented |

---

## 0. Implementation Status

> **Trạng thái**: ✅ **IMPLEMENTED** — 2026-06-17

| Task | File | Trạng thái |
|------|------|------------|
| HTML formatter (Bootstrap, light/dark) | `report-service/internal/formatters/html.go` | ✅ Done |
| PDF formatter (executive summary) | `report-service/internal/formatters/pdf.go` | ✅ Done |
| CSV/JSON/Excel formatter | `report-service/internal/formatters/csv_json_excel.go` | ✅ Done |
| Formatter interface | `report-service/internal/formatters/interface.go` | ✅ Done |
| Report generation use case (parallel) | `report-service/internal/usecase/generate.go` | ✅ Done |
| MinIO/S3 artifact storage | `report-service/internal/storage/minio.go` | ✅ Done |
| Report entity | `report-service/internal/domain/entity/report.go` | ✅ Done |
| Finding entity (for report) | `report-service/internal/domain/entity/finding.go` | ✅ Done |
| HTTP handlers | `report-service/internal/delivery/http/handlers.go` | ✅ Done |

**Chi tiết implementation**:
- **HTML Report**: Bootstrap 5, light/dark theme switch, severity badges, CVE table, severity donut chart (Chart.js inline)
- **PDF Report**: Executive summary (page 1), CVE table (pages 2+), severity stats sidebar
- **Excel/XLSX**: DefectDojo import format — ID/Title/Severity/CVE/CWE/Status/Component columns
- **CSV**: All finding fields + VEX justification + EPSS score columns
- **Parallel Generation**: `errgroup`-based concurrent format generation (HTML, PDF, CSV, Excel đồng thời)
- **S3/MinIO storage**: upload artifacts sau khi generate, trả về presigned download URL
- **CI/CD Exit Code**: `exit 1` khi có findings trên threshold severity, `exit 0` khi clean
- **SLA filtering**: lọc theo severity, SLA status, product trước khi render

---

## 1. Tổng quan

OSV có export JSON/CSV cơ bản (CR-GCV-010). OpenVulnScan Report Service bổ sung:
- **HTML Report** — Bootstrap, light/dark theme, severity color coding, embedded charts
- **PDF Report** — Executive summary, CVE table, severity distribution
- **Excel Report** — DefectDojo finding import format
- **CI/CD Integration** — exit code 0 (no CVEs) / 1 (CVEs found)
- **VEX support** — Vulnerability Exploitability eXchange justification fields
- **S3/MinIO artifact storage** — persistent report artifacts

---

## 2. Gap Analysis

| Feature | OSV | OpenVulnScan |
|---------|-----|-------------|
| JSON export | ✅ (CR-GCV-010) | ✅ enhanced |
| CSV export | ✅ (CR-GCV-010) | ✅ enhanced |
| HTML report | ❌ | ✅ light/dark theme |
| PDF report | ❌ | ✅ |
| Excel/XLSX | ❌ | ✅ DefectDojo format |
| Console output | ❌ | ✅ colored terminal |
| Severity filtering | ⚠️ | ✅ MinSeverity + MinScore |
| VEX justification | ❌ | ✅ |
| CI/CD exit code | ❌ | ✅ 0=clean, 1=CVEs |
| S3/MinIO storage | ❌ | ✅ artifact persistence |
| Report history | ❌ | ✅ report_runs table |
| Download API | ❌ | ✅ presigned URLs |

---

## 3. Domain Model

### 3.1 Report Entities

```go
// report-service/internal/domain/entity/report.go

// OutputFormat — supported report formats
type OutputFormat string
const (
    FormatConsole OutputFormat = "console"  // Terminal colored output
    FormatCSV     OutputFormat = "csv"
    FormatJSON    OutputFormat = "json"
    FormatJSON2   OutputFormat = "json2"    // Alternative JSON schema
    FormatHTML    OutputFormat = "html"
    FormatPDF     OutputFormat = "pdf"
    FormatExcel   OutputFormat = "excel"    // XLSX for DefectDojo
)

func AllFormats() []OutputFormat {
    return []OutputFormat{FormatConsole, FormatCSV, FormatJSON, FormatHTML, FormatPDF, FormatExcel}
}

func (f OutputFormat) IsValid() bool {
    for _, valid := range AllFormats() {
        if f == valid { return true }
    }
    return false
}

// ProductInfo — identifies a software product/component
type ProductInfo struct {
    Vendor  string
    Product string
    Version string
    PURL    string  // Package URL (purl spec)
}

// CVEData — details about a single CVE finding
type CVEData struct {
    CVENumber     string
    Severity      string   // UNKNOWN|LOW|MEDIUM|HIGH|CRITICAL
    Remarks       int      // 0=unset, 1=notaffected, 2=affected, 3=fixed, 4=investigating
    Score         float64  // CVSS score
    CVSSVersion   int      // 2 or 3
    CVSSVector    string
    DataSource    string   // "NVD", "CIRCL", "RHSA", etc.
    IsExploit     bool
    Justification string   // VEX justification (why not affected)
    Response      []string // VEX response actions
    EPSS          float64
}

// ReportInput — all data needed to generate a report
type ReportInput struct {
    CVEData     map[ProductInfo][]CVEData // CVE findings per product
    ScanTarget  string
    GeneratedAt time.Time
    Formats     []OutputFormat
    MinSeverity string     // Filter: UNKNOWN|LOW|MEDIUM|HIGH|CRITICAL
    MinScore    float64    // Filter: minimum CVSS score (e.g. 7.0)
    Theme       string     // "light"|"dark" (HTML only)

    // DefectDojo findings (for Excel format)
    Findings    []Finding
    Target      string
}

// Finding — DefectDojo vulnerability finding for Excel export
type Finding struct {
    ID               string
    Title            string
    Severity         string
    CVE              string
    CWE              int32
    Status           string    // active|mitigated|false_positive|risk_accepted
    Component        string
    ComponentVersion string
    FilePath         string
    Line             int32
    HashCode         string
    Date             string    // RFC3339
    FoundDate        time.Time
    SLAExpiration    string    // RFC3339, optional
    ProductID        string
    TestID           string
    Description      string
    Mitigation       string
}

// ReportOutput — generated report bytes
type ReportOutput struct {
    Reports  map[OutputFormat][]byte
    ExitCode int  // 0=clean (no CVEs above threshold), 1=CVEs found
}

// AvailableFixResult — CVE with available fix in Linux distro
type AvailableFixResult struct {
    CVENumber   string
    Product     ProductInfo
    Distro      string
    FixVersion  string
    IsAvailable bool
}
```

---

## 4. Formatters

### 4.1 HTML Formatter

```go
// report-service/internal/formatters/html.go

type HTMLFormatter struct {
    templates *template.Template
}

// Report features:
// - Bootstrap 5.3 responsive layout
// - Light/dark theme via CSS variables
// - Severity color coding: critical=red, high=orange, medium=yellow, low=blue
// - Collapsible sections per product
// - CVE detail cards with CVSS score visual
// - Embedded severity distribution chart (Chart.js)
// - Sortable CVE tables
// - Print-friendly CSS

func (f *HTMLFormatter) Format(ctx context.Context, input *entity.ReportInput) ([]byte, error) {
    // 1. Filter CVEs by MinSeverity and MinScore
    filtered := filterCVEs(input)

    // 2. Build template data
    data := HTMLTemplateData{
        ScanTarget:  input.ScanTarget,
        GeneratedAt: input.GeneratedAt,
        Theme:       input.Theme, // "light"|"dark"
        Products:    buildProductSections(filtered),
        Stats:       buildStats(filtered),
    }

    // 3. Render template
    var buf bytes.Buffer
    if err := f.templates.ExecuteTemplate(&buf, "report.html", data); err != nil {
        return nil, fmt.Errorf("html render: %w", err)
    }

    return buf.Bytes(), nil
}

type HTMLTemplateData struct {
    ScanTarget  string
    GeneratedAt time.Time
    Theme       string
    Products    []ProductSection
    Stats       ReportStats
}

type ProductSection struct {
    Product    entity.ProductInfo
    CVEs       []entity.CVEData
    CriticalCount int
    HighCount     int
    MediumCount   int
    LowCount      int
}

type ReportStats struct {
    TotalProducts  int
    TotalCVEs      int
    CriticalCount  int
    HighCount      int
    MediumCount    int
    LowCount       int
    ExploitableCount int
    AvgEPSS        float64
}
```

### 4.2 PDF Formatter

```go
// report-service/internal/formatters/pdf.go
// Uses go-wkhtmltopdf (wkhtmltopdf wrapper) or gofpdf

type PDFFormatter struct {
    htmlFormatter *HTMLFormatter
    wkhtmltopdf   *wkhtmltopdf.PDFGenerator
}

// PDF generation options:
// 1. Generate HTML first
// 2. Convert to PDF via wkhtmltopdf (headless browser)
// OR use gofpdf directly for simpler layout

func (f *PDFFormatter) Format(ctx context.Context, input *entity.ReportInput) ([]byte, error) {
    // PDF sections:
    // 1. Cover page: title, scan target, date, severity summary
    // 2. Executive summary: findings overview, risk rating
    // 3. CVE details table: sorted by severity DESC
    // 4. Severity distribution chart
    // 5. Appendix: raw data, methodology

    // Option A: HTML → wkhtmltopdf
    htmlBytes, err := f.htmlFormatter.Format(ctx, input)
    if err != nil { return nil, err }

    pdfg, _ := wkhtmltopdf.NewPDFGenerator()
    pdfg.AddPage(wkhtmltopdf.NewPageReader(bytes.NewReader(htmlBytes)))
    pdfg.PageSize.Set(wkhtmltopdf.PageSizeA4)
    pdfg.Orientation.Set(wkhtmltopdf.OrientationPortrait)

    if err := pdfg.Create(); err != nil { return nil, err }
    return pdfg.Bytes(), nil
}
```

### 4.3 Excel Formatter (DefectDojo)

```go
// report-service/internal/formatters/excel.go
// Generates Excel file compatible with DefectDojo finding import

type ExcelFormatter struct{}

// Column order matches DefectDojo import format
var defectDojoColumns = []string{
    "ID", "Title", "Severity", "CVE", "CWE", "Status",
    "Component Name", "Component Version", "File Path", "Line",
    "Hash Code", "Date", "SLA Expiration", "Product ID", "Test ID",
    "Description", "Mitigation",
}

func (f *ExcelFormatter) Format(ctx context.Context, input *entity.ReportInput) ([]byte, error) {
    file := excelize.NewFile()
    sheet := "Findings"
    file.SetSheetName("Sheet1", sheet)

    // Header row (bold)
    for i, col := range defectDojoColumns {
        cell, _ := excelize.CoordinatesToCellName(i+1, 1)
        file.SetCellValue(sheet, cell, col)
    }
    headerStyle, _ := file.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
    file.SetRowStyle(sheet, 1, 1, headerStyle)

    // Data rows
    for rowIdx, finding := range input.Findings {
        row := rowIdx + 2
        values := []interface{}{
            finding.ID, finding.Title, finding.Severity,
            finding.CVE, finding.CWE, finding.Status,
            finding.Component, finding.ComponentVersion,
            finding.FilePath, finding.Line, finding.HashCode,
            finding.Date, finding.SLAExpiration,
            finding.ProductID, finding.TestID,
            finding.Description, finding.Mitigation,
        }
        for colIdx, val := range values {
            cell, _ := excelize.CoordinatesToCellName(colIdx+1, row)
            file.SetCellValue(sheet, cell, val)
        }

        // Color code by severity
        severityStyle := severityExcelStyle(file, finding.Severity)
        file.SetCellStyle(sheet, cell2(2, row), cell2(3, row), severityStyle)
    }

    // Auto-fit columns
    file.SetColWidth(sheet, "A", "Q", 20)

    var buf bytes.Buffer
    file.Write(&buf)
    return buf.Bytes(), nil
}

func severityExcelStyle(f *excelize.File, severity string) int {
    colors := map[string]string{
        "Critical": "FF0000",
        "High":     "FF8C00",
        "Medium":   "FFD700",
        "Low":      "4169E1",
        "Info":     "808080",
    }
    color := colors[severity]
    style, _ := f.NewStyle(&excelize.Style{
        Font: &excelize.Font{Color: color, Bold: true},
    })
    return style
}
```

### 4.4 CSV Formatter

```go
// report-service/internal/formatters/csv.go

func (f *CSVFormatter) Format(ctx context.Context, input *entity.ReportInput) ([]byte, error) {
    var buf bytes.Buffer
    w := csv.NewWriter(&buf)

    // Header
    w.Write([]string{
        "Product", "Vendor", "Version", "PURL",
        "CVE", "Severity", "CVSS Score", "CVSS Version",
        "CVSS Vector", "Data Source", "Is Exploit", "EPSS",
        "Remarks", "Justification",
    })

    // Filter CVEs first
    for product, cves := range input.CVEData {
        for _, cve := range cves {
            if !meetsFilter(cve, input.MinSeverity, input.MinScore) { continue }

            w.Write([]string{
                product.Product, product.Vendor, product.Version, product.PURL,
                cve.CVENumber, cve.Severity,
                strconv.FormatFloat(cve.Score, 'f', 1, 64),
                strconv.Itoa(cve.CVSSVersion),
                cve.CVSSVector, cve.DataSource,
                strconv.FormatBool(cve.IsExploit),
                strconv.FormatFloat(cve.EPSS, 'f', 6, 64),
                strconv.Itoa(cve.Remarks),
                cve.Justification,
            })
        }
    }

    w.Flush()
    return buf.Bytes(), nil
}
```

### 4.5 Console Formatter

```go
// report-service/internal/formatters/console.go
// Colored terminal output using zerolog or colorable

func (f *ConsoleFormatter) Format(ctx context.Context, input *entity.ReportInput) ([]byte, error) {
    var buf bytes.Buffer

    // ANSI color codes
    colors := map[string]string{
        "Critical": "\033[1;31m",  // Bold Red
        "High":     "\033[0;31m",  // Red
        "Medium":   "\033[0;33m",  // Yellow
        "Low":      "\033[0;34m",  // Blue
        "Info":     "\033[0;37m",  // Gray
        "Reset":    "\033[0m",
    }

    fmt.Fprintf(&buf, "Vulnerability Scan Report\n")
    fmt.Fprintf(&buf, "Target: %s\n", input.ScanTarget)
    fmt.Fprintf(&buf, "Generated: %s\n\n", input.GeneratedAt.Format("2006-01-02 15:04:05"))

    for product, cves := range input.CVEData {
        fmt.Fprintf(&buf, "== %s %s (%s) ==\n", product.Vendor, product.Product, product.Version)
        for _, cve := range cves {
            color := colors[cve.Severity]
            reset := colors["Reset"]
            fmt.Fprintf(&buf, "  %s[%s]%s %s (CVSS: %.1f, EPSS: %.4f)\n",
                color, cve.Severity, reset,
                cve.CVENumber, cve.Score, cve.EPSS)
        }
        fmt.Fprintln(&buf)
    }

    return buf.Bytes(), nil
}
```

---

## 5. Main Generate Use Case

```go
// report-service/internal/usecase/generate/usecase.go

type GenerateReportInput struct {
    ScanID      *uuid.UUID
    ProductID   *uuid.UUID
    Formats     []entity.OutputFormat
    MinSeverity string
    MinScore    float64
    Theme       string   // "light"|"dark"
    CreatedBy   uuid.UUID
}

func (uc *GenerateReportUseCase) Execute(ctx context.Context, in GenerateReportInput) (*GenerateReportOutput, error) {
    // 1. Fetch findings
    var findings []entity.Finding
    if in.ScanID != nil {
        findings, _ = uc.findingSvcClient.GetFindingsByScan(ctx, *in.ScanID)
    } else if in.ProductID != nil {
        findings, _ = uc.findingSvcClient.GetFindingsByProduct(ctx, *in.ProductID)
    }

    // 2. Build CVE data map (grouped by product component)
    cveData := buildCVEDataMap(findings, in.MinSeverity, in.MinScore)

    // 3. Create report run record
    reportRun := &entity.ReportRun{
        ID:       uuid.New(),
        ScanID:   in.ScanID,
        Formats:  formatStrings(in.Formats),
        Status:   "generating",
        CreatedBy: in.CreatedBy,
        CreatedAt: time.Now().UTC(),
    }
    uc.reportRunRepo.Save(ctx, reportRun)

    // 4. Generate each format in parallel
    input := &entity.ReportInput{
        CVEData:     cveData,
        Formats:     in.Formats,
        MinSeverity: in.MinSeverity,
        MinScore:    in.MinScore,
        Theme:       in.Theme,
        GeneratedAt: time.Now().UTC(),
        Findings:    findings,
    }

    output, err := uc.generateAllFormats(ctx, input)
    if err != nil {
        reportRun.Status = "failed"
        uc.reportRunRepo.Update(ctx, reportRun)
        return nil, err
    }

    // 5. Store artifacts in S3/MinIO
    artifacts := make([]*entity.ReportArtifact, 0)
    for format, bytes := range output.Reports {
        path := fmt.Sprintf("reports/%s/%s.%s", reportRun.ID, reportRun.ID, string(format))
        if err := uc.storage.Put(ctx, path, bytes); err == nil {
            artifacts = append(artifacts, &entity.ReportArtifact{
                ID:          uuid.New(),
                ReportRunID: reportRun.ID,
                Format:      string(format),
                StoragePath: path,
                SizeBytes:   int64(len(bytes)),
            })
        }
    }
    uc.artifactRepo.SaveBatch(ctx, artifacts)

    // 6. Update report run
    reportRun.Status = "completed"
    reportRun.ExitCode = output.ExitCode
    completedAt := time.Now().UTC()
    reportRun.CompletedAt = &completedAt
    uc.reportRunRepo.Update(ctx, reportRun)

    return &GenerateReportOutput{
        ReportRunID: reportRun.ID,
        ExitCode:    output.ExitCode,  // 0 = no CVEs, 1 = CVEs found
        Artifacts:   artifacts,
    }, nil
}

func (uc *GenerateReportUseCase) generateAllFormats(ctx context.Context, input *entity.ReportInput) (*entity.ReportOutput, error) {
    output := &entity.ReportOutput{
        Reports: make(map[entity.OutputFormat][]byte),
    }

    var mu sync.Mutex
    var wg sync.WaitGroup

    for _, format := range input.Formats {
        wg.Add(1)
        go func(f entity.OutputFormat) {
            defer wg.Done()
            formatter := uc.formatterRegistry[f]
            if formatter == nil { return }

            bytes, err := formatter.Format(ctx, input)
            if err != nil { return }

            mu.Lock()
            output.Reports[f] = bytes
            mu.Unlock()
        }(format)
    }

    wg.Wait()

    // Determine exit code for CI/CD
    totalCVEs := countCVEsAboveThreshold(input.CVEData, input.MinSeverity, input.MinScore)
    if totalCVEs > 0 { output.ExitCode = 1 }

    return output, nil
}
```

---

## 6. Database Schema

```sql
-- report-service migrations/

CREATE TABLE report_runs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scan_id      UUID,
    product_id   UUID,
    formats      TEXT[] NOT NULL DEFAULT '{}',
    min_severity VARCHAR(10),
    min_score    FLOAT,
    status       VARCHAR(20) NOT NULL DEFAULT 'pending',  -- pending|generating|completed|failed
    exit_code    INT,
    created_by   UUID,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);
CREATE INDEX idx_report_runs_scan    ON report_runs(scan_id);
CREATE INDEX idx_report_runs_product ON report_runs(product_id);
CREATE INDEX idx_report_runs_created ON report_runs(created_at DESC);

CREATE TABLE report_artifacts (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    report_run_id  UUID NOT NULL REFERENCES report_runs(id) ON DELETE CASCADE,
    format         VARCHAR(20) NOT NULL,
    storage_path   TEXT NOT NULL,     -- S3/MinIO path or local path
    size_bytes     BIGINT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_artifacts_run ON report_artifacts(report_run_id);
```

---

## 7. API Routes

```
# Generate report
POST /api/v1/reports                → Generate report (specify formats, filters)
GET  /api/v1/reports                → List report runs (paginated)
GET  /api/v1/reports/{id}           → Get report run metadata
GET  /api/v1/reports/{id}/download/{format} → Download artifact (presigned URL or direct)

# Quick download (inline)
GET  /api/v1/scans/{id}/report/pdf  → Generate + return PDF inline
GET  /api/v1/scans/{id}/report/html → Generate + return HTML inline
GET  /api/v1/scans/{id}/report/csv  → Generate + return CSV inline
```

### Generate Request

```json
POST /api/v1/reports
Authorization: Bearer <token>

{
  "scan_id": "uuid",
  "formats": ["pdf", "html", "csv", "excel"],
  "min_severity": "medium",
  "min_score": 0.0,
  "theme": "dark"
}

Response 202:
{
  "report_run_id": "uuid",
  "status": "generating",
  "created_at": "2026-06-14T07:00:00Z"
}
```

---

## 8. CI/CD Integration

```bash
# Use report-service exit code in CI pipeline
# exit 0 → no CVEs above threshold
# exit 1 → CVEs found (fails CI pipeline)

# GitHub Actions example:
- name: Generate vulnerability report
  run: |
    REPORT_ID=$(curl -s -X POST "$API_URL/api/v1/reports" \
      -H "Authorization: Bearer $TOKEN" \
      -d '{"scan_id": "'$SCAN_ID'", "formats": ["csv"], "min_severity": "high"}' \
      | jq -r '.report_run_id')

    # Wait for completion and check exit code
    EXIT_CODE=$(curl -s "$API_URL/api/v1/reports/$REPORT_ID" | jq -r '.exit_code')
    exit $EXIT_CODE
```

---

## 9. Acceptance Criteria

- [ ] `POST /api/v1/reports?formats=pdf,html,csv` → generates all 3 formats in parallel
- [ ] HTML report có light/dark theme toggle (via `?theme=dark`)
- [ ] HTML report có Bootstrap layout, severity color coding
- [ ] PDF report: executive summary, CVE table, severity chart
- [ ] Excel report: columns match DefectDojo finding import format
- [ ] CSV report: headers include CVE, Severity, CVSS Score, EPSS, Product
- [ ] `min_severity=high` → chỉ include High + Critical CVEs
- [ ] `min_score=7.0` → chỉ include CVEs với CVSS >= 7.0
- [ ] Report với CVEs found → `exit_code=1`; không có CVEs → `exit_code=0`
- [ ] Report artifacts được lưu vào S3/MinIO với path `reports/{id}/{id}.{format}`
- [ ] `GET /api/v1/reports/{id}/download/pdf` → return PDF file với Content-Disposition header
- [ ] `GET /api/v1/scans/{scan_id}/report/pdf` → generate inline PDF cho scan
- [ ] Console format: ANSI color codes theo severity (Critical=Bold Red)
- [ ] ExcelFormatter columns: ID, Title, Severity, CVE, CWE, Status, Component...
