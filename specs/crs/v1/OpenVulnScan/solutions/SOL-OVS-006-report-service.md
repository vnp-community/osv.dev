# SOL-OVS-006 — Giải Pháp: Report Service (PDF/HTML/CSV/Excel)

| Trường | Giá trị |
|--------|---------|
| **Solution ID** | SOL-OVS-006 |
| **CR tham chiếu** | CR-OVS-006 |
| **Tiêu đề** | Report Service — Multi-format Vulnerability Reports (PDF, HTML, CSV, Excel), CI/CD Exit Code, S3/MinIO Storage |
| **Ngày tạo** | 2026-06-16 |
| **Ngày implement** | 2026-06-17 |
| **Trạng thái** | ✅ Implemented |

---

## 0. Implementation Status

> **Trạng thái**: ✅ **IMPLEMENTED** — 2026-06-17

| Task | File | Trạng thái |
|------|------|------------|
| T-RPT-001 | `report-service/internal/formatters/html.go` | ✅ Done |
| T-RPT-002 | `report-service/internal/formatters/pdf.go` | ✅ Done |
| T-RPT-003 | `report-service/internal/formatters/csv_json_excel.go` | ✅ Done |
| T-RPT-004 | `report-service/internal/storage/minio.go` | ✅ Done |
| T-RPT-005 | `report-service/internal/usecase/generate.go` | ✅ Done |
| T-RPT-006 | `report-service/internal/delivery/http/handlers.go` | ✅ Done |

**Chi tiết implementation**:
- **HTML Formatter**: Bootstrap 5.3 + Chart.js donut chart, light/dark theme via `data-bs-theme`, severity color badges
- **PDF Formatter**: chromedp headless Chrome strategy, `page.PrintToPDF` A4 format
- **CSV Formatter**: RFC 4180-compliant với 19 columns; JSON formatter với `JSONReport` struct
- **Excel Formatter**: DefectDojo-compatible columns (17 fields), severity color cells
- **MinIO Storage**: presigned URL (7-day default), path convention `reports/{date}/{run_id}.{ext}`, MIME detection
- **GenerateReport**: Async goroutine per format, `sync.WaitGroup`, exit_code=1 nếu Critical/High found
- **REST API**: `POST /reports/generate`, `GET /reports/{runID}`, `GET /reports/{runID}/download/{format}`, `GET /reports/{runID}/exit-code`

---

## 1. Tổng Quan Giải Pháp

### 1.1 Bối Cảnh

OSV.dev có JSON/CSV export cơ bản. `report-service` bổ sung hệ thống báo cáo đầy đủ cho enterprise security teams với:
- Nhiều format output (PDF, HTML, CSV, Excel)
- Theme support (light/dark)
- DefectDojo-compatible Excel format
- CI/CD integration qua exit code

### 1.2 Design Decisions

| Decision | Rationale |
|----------|-----------|
| **HTML → PDF** (via Chromium headless) | Thay vì `wkhtmltopdf` (unmaintained), dùng `chromedp` để render HTML thành PDF |
| **`excelize`** cho Excel | Mature Go library, DefectDojo format compliance |
| **S3/MinIO** artifact storage | Persistent reports, presigned URLs cho download |
| **Async generation** | Report generation có thể mất >1s; async với status polling |
| **Parallel format generation** | HTML, CSV, Excel render song song qua goroutines |

---

## 2. Kiến Trúc

### 2.1 Report Generation Flow

```
POST /api/v1/reports
  {scan_id, formats: ["pdf","html","csv"], min_severity: "medium"}
        │
        │ Create report_run record (status=pending)
        │ Start async goroutine
        ▼
GenerateReportUseCase.Execute() [async]
        │
        ├── Fetch findings from finding-service gRPC
        │
        ├── Build CVEData map (group by product)
        │
        ├── Parallel format generation:
        │   ├── HTMLFormatter.Format() → bytes
        │   ├── PDFFormatter.Format() [HTML → Chromium PDF]
        │   ├── CSVFormatter.Format()
        │   └── ExcelFormatter.Format()
        │
        ├── Upload to S3/MinIO: reports/{run_id}/{run_id}.{format}
        │
        └── Update report_run: status=completed, exit_code

Client polls: GET /api/v1/reports/{run_id}
Client downloads: GET /api/v1/reports/{run_id}/download/{format}
```

### 2.2 Cấu Trúc Thư Mục

```
services/report-service/
├── cmd/server/main.go
├── internal/
│   ├── domain/
│   │   └── entity/
│   │       ├── report.go              # OutputFormat, ReportInput, ReportOutput, etc.
│   │       └── finding.go             # Finding entity for report
│   ├── usecase/
│   │   └── generate/
│   │       └── usecase.go
│   ├── formatters/
│   │   ├── html.go                    # Bootstrap HTML formatter
│   │   ├── pdf.go                     # HTML → PDF via chromedp
│   │   ├── excel.go                   # DefectDojo XLSX formatter
│   │   ├── csv.go
│   │   ├── json.go
│   │   └── console.go                 # ANSI colored output
│   ├── templates/
│   │   ├── report.html                # Go template with Bootstrap
│   │   ├── report_dark.css
│   │   └── report_light.css
│   ├── adapter/
│   │   ├── repository/postgres/
│   │   │   ├── report_run_repo.go
│   │   │   └── artifact_repo.go
│   │   ├── storage/
│   │   │   ├── s3.go                  # AWS S3 implementation
│   │   │   └── minio.go               # MinIO implementation
│   │   └── client/
│   │       └── finding_service_client.go
│   └── delivery/
│       └── http/
│           └── report_handler.go
├── migrations/
│   └── 001_create_report_tables.sql
└── config/config.yaml
```

---

## 3. HTML Template Design

### 3.1 Template Structure

```html
<!-- templates/report.html -->
<!DOCTYPE html>
<html lang="en" data-bs-theme="{{ .Theme }}">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Vulnerability Report — {{ .ScanTarget }}</title>
  
  <!-- Bootstrap 5.3 with dark mode support -->
  <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css" rel="stylesheet">
  <link href="https://cdn.jsdelivr.net/npm/bootstrap-icons/font/bootstrap-icons.css" rel="stylesheet">
  
  <style>
    /* Severity color coding */
    .severity-critical { color: #dc3545; font-weight: bold; }
    .severity-high     { color: #fd7e14; font-weight: bold; }
    .severity-medium   { color: #ffc107; }
    .severity-low      { color: #0d6efd; }
    .severity-info     { color: #6c757d; }
    
    /* Severity badge */
    .badge-critical { background-color: #dc3545; }
    .badge-high     { background-color: #fd7e14; }
    .badge-medium   { background-color: #ffc107; color: #000; }
    .badge-low      { background-color: #0d6efd; }
    
    /* Print styles */
    @media print {
      .no-print { display: none; }
      .page-break { page-break-before: always; }
    }
  </style>
</head>
<body>
  <!-- Header -->
  <div class="container-fluid py-3 bg-dark text-white">
    <h1>🔒 Vulnerability Scan Report</h1>
    <p>Target: {{ .ScanTarget }} | Generated: {{ .GeneratedAt.Format "2006-01-02 15:04:05 UTC" }}</p>
    
    <!-- Theme toggle (no-print) -->
    <div class="no-print">
      <button onclick="toggleTheme()" class="btn btn-sm btn-outline-light">Toggle Theme</button>
    </div>
  </div>
  
  <!-- Summary Cards -->
  <div class="container my-4">
    <div class="row g-3">
      <div class="col-md-3">
        <div class="card border-danger">
          <div class="card-body text-center">
            <h2 class="text-danger">{{ .Stats.CriticalCount }}</h2>
            <p class="mb-0">Critical</p>
          </div>
        </div>
      </div>
      <!-- High, Medium, Low cards... -->
    </div>
  </div>
  
  <!-- Severity Distribution Chart -->
  <div class="container my-4">
    <canvas id="severityChart" width="400" height="200"></canvas>
  </div>
  
  <!-- Products Table -->
  {{ range .Products }}
  <div class="container my-4">
    <h2>{{ .Product.Vendor }} {{ .Product.Product }} {{ .Product.Version }}</h2>
    
    <!-- Sortable CVE Table -->
    <table class="table table-striped table-hover" id="cve-table-{{ .Product.Product }}">
      <thead>
        <tr>
          <th>CVE</th>
          <th>Severity</th>
          <th>CVSS Score</th>
          <th>EPSS</th>
          <th>Exploit</th>
          <th>Data Source</th>
        </tr>
      </thead>
      <tbody>
        {{ range .CVEs }}
        <tr>
          <td><a href="https://nvd.nist.gov/vuln/detail/{{ .CVENumber }}" target="_blank">{{ .CVENumber }}</a></td>
          <td><span class="badge badge-{{ lower .Severity }}">{{ .Severity }}</span></td>
          <td>{{ printf "%.1f" .Score }}</td>
          <td>{{ printf "%.4f" .EPSS }}</td>
          <td>{{ if .IsExploit }}⚠️ Yes{{ else }}No{{ end }}</td>
          <td>{{ .DataSource }}</td>
        </tr>
        {{ end }}
      </tbody>
    </table>
  </div>
  {{ end }}
  
  <!-- Scripts -->
  <script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/js/bootstrap.bundle.min.js"></script>
  <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
  <script>
    // Severity distribution donut chart
    const ctx = document.getElementById('severityChart').getContext('2d');
    new Chart(ctx, {
      type: 'doughnut',
      data: {
        labels: ['Critical', 'High', 'Medium', 'Low'],
        datasets: [{
          data: [{{ .Stats.CriticalCount }}, {{ .Stats.HighCount }}, 
                 {{ .Stats.MediumCount }}, {{ .Stats.LowCount }}],
          backgroundColor: ['#dc3545', '#fd7e14', '#ffc107', '#0d6efd']
        }]
      }
    });
    
    // Theme toggle
    function toggleTheme() {
      const html = document.documentElement;
      html.setAttribute('data-bs-theme', 
        html.getAttribute('data-bs-theme') === 'dark' ? 'light' : 'dark');
    }
  </script>
</body>
</html>
```

---

## 4. PDF Generation (Chromium Headless)

```go
// report-service/internal/formatters/pdf.go

import "github.com/chromedp/chromedp"

type PDFFormatter struct {
    htmlFormatter *HTMLFormatter
    // Chromium path (from chromedp auto-detection or custom)
}

func (f *PDFFormatter) Format(ctx context.Context, input *entity.ReportInput) ([]byte, error) {
    // 1. Generate HTML first
    htmlBytes, err := f.htmlFormatter.Format(ctx, input)
    if err != nil { return nil, fmt.Errorf("html generation: %w", err) }
    
    // 2. Data URL for chromedp
    htmlBase64 := base64.StdEncoding.EncodeToString(htmlBytes)
    dataURL := "data:text/html;base64," + htmlBase64
    
    // 3. Headless Chrome rendering
    allocCtx, allocCancel := chromedp.NewRemoteAllocator(ctx, "ws://chromium:9222")
    defer allocCancel()
    
    chromdpCtx, chromdpCancel := chromedp.NewContext(allocCtx)
    defer chromdpCancel()
    
    var pdfBuf []byte
    if err := chromedp.Run(chromdpCtx,
        chromedp.Navigate(dataURL),
        chromedp.WaitReady("body"),
        chromedp.ActionFunc(func(ctx context.Context) error {
            var err error
            pdfBuf, _, err = page.PrintToPDF().
                WithPrintBackground(true).
                WithPaperWidth(8.27).    // A4
                WithPaperHeight(11.69).  // A4
                WithMarginTop(0.5).
                WithMarginBottom(0.5).
                WithMarginLeft(0.5).
                WithMarginRight(0.5).
                Do(ctx)
            return err
        }),
    ); err != nil {
        return nil, fmt.Errorf("pdf generation: %w", err)
    }
    
    return pdfBuf, nil
}
```

---

## 5. Excel Formatter (DefectDojo Compatible)

```go
// report-service/internal/formatters/excel.go

// DefectDojo import column specification
// https://defectdojo.readthedocs.io/en/stable/integrations/importing.html

var defectDojoColumns = []string{
    "ID",                 // Finding UUID
    "Title",              // Vulnerability title
    "Severity",           // Critical|High|Medium|Low|Info
    "CVE",                // CVE-XXXX-XXXXX
    "CWE",                // Integer
    "Status",             // active|mitigated|false_positive|risk_accepted
    "Component Name",     // Package or host name
    "Component Version",  // Package version
    "File Path",          // Source file path (if applicable)
    "Line",               // Line number
    "Hash Code",          // SHA-256 dedup hash
    "Date",               // RFC3339 discovery date
    "SLA Expiration",     // RFC3339 SLA deadline
    "Product ID",         // UUID
    "Test ID",            // UUID
    "Description",        // Full vulnerability description
    "Mitigation",         // Remediation steps
}

// Severity color mapping for cells
var excelSeverityColors = map[string]string{
    "Critical": "FFdc3545",  // Red
    "High":     "FFfd7e14",  // Orange
    "Medium":   "FFffc107",  // Yellow
    "Low":      "FF0d6efd",  // Blue
    "Info":     "FF6c757d",  // Gray
}
```

---

## 6. Storage (S3/MinIO)

### 6.1 Storage Interface

```go
// report-service/internal/adapter/storage/storage.go

type StorageBackend interface {
    Put(ctx context.Context, path string, data []byte) error
    Get(ctx context.Context, path string) ([]byte, error)
    PresignedURL(ctx context.Context, path string, ttl time.Duration) (string, error)
    Delete(ctx context.Context, path string) error
}

// MinIO implementation
type MinIOStorage struct {
    client     *minio.Client
    bucketName string
}

func (s *MinIOStorage) Put(ctx context.Context, path string, data []byte) error {
    reader := bytes.NewReader(data)
    _, err := s.client.PutObject(ctx, s.bucketName, path, reader,
        int64(len(data)), minio.PutObjectOptions{
            ContentType: detectContentType(path),
        })
    return err
}

func (s *MinIOStorage) PresignedURL(ctx context.Context, path string, ttl time.Duration) (string, error) {
    u, err := s.client.PresignedGetObject(ctx, s.bucketName, path, ttl, nil)
    if err != nil { return "", err }
    return u.String(), nil
}

// Path convention:
// reports/{report_run_id}/{report_run_id}.pdf
// reports/{report_run_id}/{report_run_id}.html
// reports/{report_run_id}/{report_run_id}.csv
// reports/{report_run_id}/{report_run_id}.xlsx
```

---

## 7. Database Schema

```sql
-- migrations/001_create_report_tables.sql

CREATE TABLE report_runs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scan_id      UUID,           -- Nullable: report for specific scan
    product_id   UUID,           -- Nullable: report for product
    formats      TEXT[] NOT NULL DEFAULT '{}',
    min_severity VARCHAR(10),
    min_score    FLOAT,
    theme        VARCHAR(10) DEFAULT 'light',  -- light|dark
    status       VARCHAR(20) NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending','generating','completed','failed')),
    exit_code    INT,            -- 0=no CVEs, 1=CVEs found
    error_msg    TEXT,
    created_by   UUID NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_report_runs_scan    ON report_runs(scan_id) WHERE scan_id IS NOT NULL;
CREATE INDEX idx_report_runs_product ON report_runs(product_id) WHERE product_id IS NOT NULL;
CREATE INDEX idx_report_runs_status  ON report_runs(status) WHERE status IN ('pending','generating');
CREATE INDEX idx_report_runs_created ON report_runs(created_at DESC);

CREATE TABLE report_artifacts (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    report_run_id  UUID NOT NULL REFERENCES report_runs(id) ON DELETE CASCADE,
    format         VARCHAR(20) NOT NULL,  -- pdf|html|csv|excel|json|console
    storage_path   TEXT NOT NULL,         -- S3/MinIO path
    size_bytes     BIGINT,
    content_type   VARCHAR(100),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_artifacts_run ON report_artifacts(report_run_id);
```

---

## 8. API Routes Chi Tiết

```
# Async report generation
POST /api/v1/reports
  Body: {scan_id|product_id, formats[], min_severity, min_score, theme}
  Response 202: {report_run_id, status, created_at}

GET  /api/v1/reports
  Query: ?scan_id=&product_id=&status=&page=&limit=
  Response 200: {items: [ReportRun], total, page}

GET  /api/v1/reports/{id}
  Response 200: {id, status, exit_code, formats, artifacts[{format, url, size}]}

DELETE /api/v1/reports/{id}
  Response 204: Delete report run + artifacts from storage

# Download
GET  /api/v1/reports/{id}/download/{format}
  Response: Redirect to presigned URL (1 hour TTL)
  OR: Direct bytes with Content-Disposition header

# Convenience endpoints (sync, small scans only)
GET  /api/v1/scans/{scan_id}/report/pdf
  Response: PDF bytes (inline), generates on-the-fly
GET  /api/v1/scans/{scan_id}/report/html
  Response: HTML bytes (inline)
GET  /api/v1/scans/{scan_id}/report/csv
  Response: CSV bytes with Content-Disposition: attachment
```

### 8.1 Report Run Status Response

```json
// GET /api/v1/reports/{id}
{
  "id": "uuid",
  "scan_id": "uuid",
  "formats": ["pdf", "html", "csv"],
  "min_severity": "medium",
  "theme": "dark",
  "status": "completed",
  "exit_code": 1,
  "created_by": "user-uuid",
  "created_at": "2026-06-16T11:00:00Z",
  "completed_at": "2026-06-16T11:00:35Z",
  "duration_seconds": 35,
  "artifacts": [
    {
      "format": "pdf",
      "url": "https://minio.example.com/reports/uuid/uuid.pdf?X-Amz-...",
      "url_expires_at": "2026-06-16T12:00:00Z",
      "size_bytes": 245760
    },
    {
      "format": "html",
      "url": "https://minio.example.com/reports/uuid/uuid.html?X-Amz-...",
      "url_expires_at": "2026-06-16T12:00:00Z",
      "size_bytes": 102400
    }
  ]
}
```

---

## 9. CI/CD Integration

### 9.1 Exit Code Semantics

```
exit_code = 0: No CVEs found above threshold (min_severity/min_score)
exit_code = 1: CVEs found above threshold

Used in CI pipelines to fail builds when vulnerabilities are found.
```

### 9.2 GitHub Actions Example

```yaml
# .github/workflows/security.yml
- name: Run vulnerability scan
  run: |
    # Create scan
    SCAN_ID=$(curl -sX POST "$API_URL/api/v1/scans" \
      -H "Authorization: Bearer $OVNS_TOKEN" \
      -d '{"targets":["https://api.example.com"],"type":"web"}' \
      | jq -r '.id')
    
    # Wait for scan completion (poll SSE or status)
    while true; do
      STATUS=$(curl -s "$API_URL/api/v1/scans/$SCAN_ID" | jq -r '.status')
      [ "$STATUS" = "completed" ] && break
      [ "$STATUS" = "failed" ] && exit 1
      sleep 10
    done
    
    # Generate report with min_severity=high
    REPORT_ID=$(curl -sX POST "$API_URL/api/v1/reports" \
      -H "Authorization: Bearer $OVNS_TOKEN" \
      -d "{\"scan_id\":\"$SCAN_ID\",\"formats\":[\"csv\",\"html\"],\"min_severity\":\"high\"}" \
      | jq -r '.report_run_id')
    
    # Wait for report
    while true; do
      REPORT=$(curl -s "$API_URL/api/v1/reports/$REPORT_ID")
      STATUS=$(echo $REPORT | jq -r '.status')
      [ "$STATUS" = "completed" ] && break
      sleep 5
    done
    
    # Check exit code
    EXIT_CODE=$(echo $REPORT | jq -r '.exit_code')
    
    # Download HTML report as artifact
    curl -sL $(echo $REPORT | jq -r '.artifacts[] | select(.format=="html") | .url') \
      -o vulnerability-report.html
    
    exit $EXIT_CODE  # Fail CI if CVEs found

- name: Upload report
  uses: actions/upload-artifact@v4
  if: always()
  with:
    name: vulnerability-report
    path: vulnerability-report.html
```

---

## 10. Configuration

```yaml
# config/config.yaml
server:
  grpc_port: 50065
  http_port: 8065

database:
  host: "${DB_HOST}"
  port: 5432
  name: "report_service"
  user: "${DB_USER}"
  password: "${DB_PASSWORD}"

storage:
  type: "minio"  # minio|s3
  
  minio:
    endpoint: "minio:9000"
    access_key: "${MINIO_ACCESS_KEY}"
    secret_key: "${MINIO_SECRET_KEY}"
    bucket: "reports"
    use_ssl: false
  
  s3:
    region: "${AWS_REGION}"
    bucket: "${S3_BUCKET_NAME}"
    # Uses AWS default credential chain

pdf:
  # Chromium headless via remote debugging
  chromium_ws: "ws://chromium:9222"
  # OR: use local chromium binary
  # chromium_path: "/usr/bin/chromium"

report:
  presigned_url_ttl: "1h"
  max_async_workers: 3  # Concurrent report generations

clients:
  finding_service:
    grpc_address: "finding-service:50060"
  scan_service:
    grpc_address: "scan-service:50058"

auth:
  jwt_public_key_path: "/secrets/jwt_public.pem"

logging:
  level: "info"
  format: "json"
```

---

## 11. Implementation Roadmap

### Phase 1 — Core Formatters (Sprint 1)
- [ ] Database migrations
- [ ] CSV formatter
- [ ] JSON formatter
- [ ] Console formatter (ANSI colors)
- [ ] GenerateReportUseCase (sync, basic)

### Phase 2 — Rich Formats (Sprint 2)
- [ ] HTML template (Bootstrap 5.3 + Chart.js)
- [ ] HTML formatter (with severity filtering)
- [ ] PDF formatter (chromedp)
- [ ] Excel formatter (DefectDojo compatible)

### Phase 3 — Storage + Async (Sprint 3)
- [ ] MinIO/S3 storage adapter
- [ ] Async report generation (background goroutine)
- [ ] Report run status tracking
- [ ] Presigned URL download endpoint

### Phase 4 — Integration + CI/CD (Sprint 4)
- [ ] finding-service gRPC client
- [ ] Exit code computation
- [ ] Convenience endpoints (`/scans/{id}/report/pdf`)
- [ ] Integration tests

---

## 12. Acceptance Criteria Mapping

| Criterion | Implementation |
|-----------|---------------|
| `POST /reports?formats=pdf,html,csv` → all 3 in parallel | `generateAllFormats()` goroutines |
| HTML: light/dark theme toggle | `data-bs-theme="{{ .Theme }}"` |
| HTML: Bootstrap layout, severity color coding | `badge-{{ lower .Severity }}` CSS |
| PDF: executive summary, CVE table, chart | `PDFFormatter` via chromedp |
| Excel: DefectDojo columns | `defectDojoColumns` slice |
| CSV: CVE, Severity, CVSS Score, EPSS, Product | `CSVFormatter` headers |
| `min_severity=high` → only High + Critical | `meetsFilter()` in formatters |
| `min_score=7.0` → only CVEs with CVSS >= 7.0 | `meetsFilter()` score check |
| CVEs found → `exit_code=1` | `countCVEsAboveThreshold() > 0` |
| Artifacts in S3/MinIO: `reports/{id}/{id}.{fmt}` | `StorageBackend.Put()` path |
| Download PDF → Content-Disposition header | `report_handler.go` response |
| `/scans/{scan_id}/report/pdf` → inline PDF | Convenience endpoint handler |
| Console: ANSI colors by severity | `ConsoleFormatter` ANSI codes |
| Excel: color-coded severity cells | `severityExcelStyle()` |
