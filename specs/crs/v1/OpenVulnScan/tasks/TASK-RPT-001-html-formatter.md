# TASK-RPT-001 — HTML Report Formatter + Bootstrap Template

| Field | Value |
|-------|-------|
| **Task ID** | T-RPT-001 |
| **Service** | `report-service` (new service) |
| **Status** | ✅ Completed |
| **Solution Ref** | SOL-OVS-006 §3 HTML Template Design |
| **Priority** | 🟡 Medium |
| **Depends On** | — |
| **Estimated** | 4h |

---

## Context

`report-service` là service mới cần được tạo từ đầu. Task này tập trung vào HTML formatter — foundation cho PDF generator (T-RPT-002).

HTML report sử dụng:
- Bootstrap 5.3 với dark mode support (`data-bs-theme`)
- Chart.js cho severity distribution donut chart
- Sortable table cho CVE listing
- Severity color coding với badges
- Print-friendly CSS

---

## Goal

Scaffold `report-service` directory + implement HTML formatter với Go template.

---

## Target Files (New Service)

| Action | File Path |
|--------|-----------|
| CREATE | `services/report-service/go.mod` |
| CREATE | `services/report-service/cmd/server/main.go` |
| CREATE | `services/report-service/internal/domain/entity/finding.go` |
| CREATE | `services/report-service/internal/domain/entity/report.go` |
| CREATE | `services/report-service/internal/formatters/interface.go` |
| CREATE | `services/report-service/internal/formatters/html.go` |
| CREATE | `services/report-service/internal/formatters/html_test.go` |
| CREATE | `services/report-service/templates/report.html` |
| CREATE | `services/report-service/migrations/001_create_report_tables.sql` |

---

## Implementation

### File 1: `services/report-service/go.mod`

```go
module github.com/google/osv.dev/services/report-service

go 1.22

require (
    github.com/google/uuid v1.6.0
    github.com/rs/zerolog v1.33.0
    github.com/go-chi/chi/v5 v5.0.12
    github.com/minio/minio-go/v7 v7.0.74
    github.com/chromedp/chromedp v0.9.5
    github.com/xuri/excelize/v2 v2.8.1
)
```

### File 2: `services/report-service/internal/domain/entity/finding.go`

```go
package entity

import "time"

// ReportFinding is the finding data model used by the report service
// (simplified copy from finding-service, contains only fields needed for reports)
type ReportFinding struct {
    ID               string
    Title            string
    Description      string
    Mitigation       string
    Severity         string  // "Critical" | "High" | "Medium" | "Low" | "Info"
    CVE              string
    CWE              int
    CVSSv3Score      *float64
    EPSSScore        *float64
    IsExploit        bool
    IsInCISAKEV      bool
    Status           string  // "active" | "mitigated" | "false_positive" etc.
    ComponentName    string
    ComponentVersion string
    Date             time.Time
    SLAExpirationDate *time.Time
    DaysUntilSLA     *int
    DataSource       string  // "nmap" | "zap" | "agent" | "manual"
    ProductID        string
    ProductName      string
    EngagementID     string
    EngagementName   string
    Tags             []string
}

// ScanStats aggregates finding counts for the report header
type ScanStats struct {
    CriticalCount int
    HighCount     int
    MediumCount   int
    LowCount      int
    InfoCount     int
    TotalCount    int
}

// ProductSection groups findings by product for the HTML report
type ProductSection struct {
    ProductName    string
    ProductID      string
    TotalFindings  int
    Findings       []*ReportFinding
}
```

### File 3: `services/report-service/internal/domain/entity/report.go`

```go
package entity

import (
    "time"

    "github.com/google/uuid"
)

// OutputFormat identifies the report output format
type OutputFormat string

const (
    FormatPDF     OutputFormat = "pdf"
    FormatHTML    OutputFormat = "html"
    FormatCSV     OutputFormat = "csv"
    FormatExcel   OutputFormat = "excel"
    FormatJSON    OutputFormat = "json"
    FormatConsole OutputFormat = "console"
)

// Theme controls the HTML report color scheme
type Theme string

const (
    ThemeLight Theme = "light"
    ThemeDark  Theme = "dark"
)

// ReportRunStatus tracks async report generation progress
type ReportRunStatus string

const (
    StatusPending    ReportRunStatus = "pending"
    StatusGenerating ReportRunStatus = "generating"
    StatusCompleted  ReportRunStatus = "completed"
    StatusFailed     ReportRunStatus = "failed"
)

// ReportInput is the input data for report generation
type ReportInput struct {
    ScanID      *uuid.UUID
    ProductID   *uuid.UUID
    ScanTarget  string
    GeneratedAt time.Time
    Theme       Theme
    MinSeverity string  // "critical" | "high" | "medium" | "low" | ""
    MinScore    *float64

    Findings []*ReportFinding
    Stats    ScanStats
    Products []*ProductSection
}

// ReportArtifact represents a generated report file
type ReportArtifact struct {
    Format      OutputFormat
    StoragePath string  // S3/MinIO path
    SizeBytes   int64
    ContentType string
    URL         string  // Presigned download URL
    URLExpiresAt *time.Time
}

// ReportRun represents an async report generation job
type ReportRun struct {
    ID          uuid.UUID
    ScanID      *uuid.UUID
    ProductID   *uuid.UUID
    Formats     []OutputFormat
    MinSeverity string
    MinScore    *float64
    Theme       Theme
    Status      ReportRunStatus
    ExitCode    *int  // 0=no issues, 1=issues found
    ErrorMsg    string
    CreatedBy   uuid.UUID
    CreatedAt   time.Time
    CompletedAt *time.Time
    Artifacts   []*ReportArtifact
}
```

### File 4: `services/report-service/internal/formatters/interface.go`

```go
package formatters

import (
    "context"

    "github.com/google/osv.dev/services/report-service/internal/domain/entity"
)

// Formatter defines the interface for report format generators
type Formatter interface {
    // Format generates the report bytes for the given input
    Format(ctx context.Context, input *entity.ReportInput) ([]byte, error)

    // ContentType returns the MIME type for this format
    ContentType() string

    // FileExtension returns the file extension (without dot)
    FileExtension() string
}

// MeetsFilter checks if a finding passes the severity/score filter
func MeetsFilter(f *entity.ReportFinding, minSeverity string, minScore *float64) bool {
    // Score filter takes priority
    if minScore != nil && f.CVSSv3Score != nil && *f.CVSSv3Score < *minScore {
        return false
    }

    if minSeverity == "" {
        return true
    }

    severityOrder := map[string]int{
        "critical": 4,
        "high":     3,
        "medium":   2,
        "low":      1,
        "info":     0,
    }

    findingSeverityNum, _ := severityOrder[normalizeSeverity(f.Severity)]
    minSeverityNum, _ := severityOrder[normalizeSeverity(minSeverity)]

    return findingSeverityNum >= minSeverityNum
}

func normalizeSeverity(s string) string {
    switch {
    case len(s) == 0:
        return ""
    default:
        // Case-insensitive comparison
        r := []byte(s)
        for i, c := range r {
            if c >= 'A' && c <= 'Z' {
                r[i] = c + 32
            }
        }
        return string(r)
    }
}
```

### File 5: `services/report-service/internal/formatters/html.go`

```go
package formatters

import (
    "bytes"
    "context"
    _ "embed"
    "fmt"
    "html/template"
    "strings"
    "time"

    "github.com/google/osv.dev/services/report-service/internal/domain/entity"
)

//go:embed ../../templates/report.html
var reportHTMLTemplate string

// HTMLFormatter generates Bootstrap 5.3 HTML reports
type HTMLFormatter struct {
    tmpl *template.Template
}

// NewHTMLFormatter creates an HTMLFormatter
func NewHTMLFormatter() (*HTMLFormatter, error) {
    funcMap := template.FuncMap{
        "lower":     strings.ToLower,
        "upper":     strings.ToUpper,
        "formatDate": func(t time.Time) string {
            return t.UTC().Format("2006-01-02 15:04 UTC")
        },
        "formatScore": func(score *float64) string {
            if score == nil {
                return "N/A"
            }
            return fmt.Sprintf("%.1f", *score)
        },
        "severityClass": func(severity string) string {
            switch strings.ToLower(severity) {
            case "critical":
                return "danger"
            case "high":
                return "warning"
            case "medium":
                return "warning"
            case "low":
                return "info"
            default:
                return "secondary"
            }
        },
        "severityBadgeStyle": func(severity string) string {
            switch strings.ToLower(severity) {
            case "critical":
                return "background-color: #dc3545; color: white;"
            case "high":
                return "background-color: #fd7e14; color: white;"
            case "medium":
                return "background-color: #ffc107; color: #000;"
            case "low":
                return "background-color: #0d6efd; color: white;"
            default:
                return "background-color: #6c757d; color: white;"
            }
        },
        "boolIcon": func(b bool) string {
            if b {
                return "✅"
            }
            return "—"
        },
    }

    tmpl, err := template.New("report").Funcs(funcMap).Parse(reportHTMLTemplate)
    if err != nil {
        return nil, fmt.Errorf("parse HTML template: %w", err)
    }

    return &HTMLFormatter{tmpl: tmpl}, nil
}

func (f *HTMLFormatter) ContentType() string   { return "text/html; charset=utf-8" }
func (f *HTMLFormatter) FileExtension() string { return "html" }

// Format generates the HTML report as bytes
func (f *HTMLFormatter) Format(_ context.Context, input *entity.ReportInput) ([]byte, error) {
    // Apply severity/score filter
    filteredInput := filterFindings(input)

    var buf bytes.Buffer
    if err := f.tmpl.Execute(&buf, filteredInput); err != nil {
        return nil, fmt.Errorf("execute HTML template: %w", err)
    }

    return buf.Bytes(), nil
}

// filterFindings creates a copy of input with filtered findings
func filterFindings(input *entity.ReportInput) *entity.ReportInput {
    // Build filtered copy
    filtered := *input
    filtered.Products = nil
    filtered.Stats = entity.ScanStats{}

    for _, section := range input.Products {
        var filteredFindings []*entity.ReportFinding
        for _, f := range section.Findings {
            if MeetsFilter(f, input.MinSeverity, input.MinScore) {
                filteredFindings = append(filteredFindings, f)
                // Update stats
                switch strings.ToLower(f.Severity) {
                case "critical":
                    filtered.Stats.CriticalCount++
                case "high":
                    filtered.Stats.HighCount++
                case "medium":
                    filtered.Stats.MediumCount++
                case "low":
                    filtered.Stats.LowCount++
                default:
                    filtered.Stats.InfoCount++
                }
                filtered.Stats.TotalCount++
            }
        }

        if len(filteredFindings) > 0 {
            filteredSection := *section
            filteredSection.Findings = filteredFindings
            filteredSection.TotalFindings = len(filteredFindings)
            filtered.Products = append(filtered.Products, &filteredSection)
        }
    }

    return &filtered
}
```

### File 6: `services/report-service/templates/report.html`

```html
<!DOCTYPE html>
<html lang="en" data-bs-theme="{{ .Theme }}">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Vulnerability Report — {{ .ScanTarget }}</title>
  <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css" rel="stylesheet">
  <style>
    .severity-badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 0.8rem; font-weight: 600; }
    .stat-card { border-radius: 12px; }
    @media print { .no-print { display: none !important; } }
  </style>
</head>
<body>

<!-- Header -->
<div class="bg-dark text-white py-4 px-4">
  <div class="container-fluid">
    <div class="d-flex align-items-center justify-content-between">
      <div>
        <h1 class="h3 mb-1">🔒 Vulnerability Scan Report</h1>
        <p class="mb-0 text-muted">Target: <strong class="text-white">{{ .ScanTarget }}</strong> &nbsp;|&nbsp; Generated: {{ formatDate .GeneratedAt }}</p>
      </div>
      <div class="no-print">
        <button class="btn btn-outline-light btn-sm" onclick="document.documentElement.setAttribute('data-bs-theme', document.documentElement.getAttribute('data-bs-theme')==='dark'?'light':'dark')">
          Toggle Theme
        </button>
        <button class="btn btn-outline-light btn-sm ms-2" onclick="window.print()">🖨 Print</button>
      </div>
    </div>
  </div>
</div>

<!-- Summary Stats -->
<div class="container-fluid py-4">
  <div class="row g-3 mb-4">
    <div class="col-6 col-md-3">
      <div class="card stat-card text-center border-danger">
        <div class="card-body py-3">
          <div class="display-5 fw-bold text-danger">{{ .Stats.CriticalCount }}</div>
          <div class="small text-muted">Critical</div>
        </div>
      </div>
    </div>
    <div class="col-6 col-md-3">
      <div class="card stat-card text-center border-warning">
        <div class="card-body py-3">
          <div class="display-5 fw-bold text-warning">{{ .Stats.HighCount }}</div>
          <div class="small text-muted">High</div>
        </div>
      </div>
    </div>
    <div class="col-6 col-md-3">
      <div class="card stat-card text-center">
        <div class="card-body py-3">
          <div class="display-5 fw-bold text-warning">{{ .Stats.MediumCount }}</div>
          <div class="small text-muted">Medium</div>
        </div>
      </div>
    </div>
    <div class="col-6 col-md-3">
      <div class="card stat-card text-center border-info">
        <div class="card-body py-3">
          <div class="display-5 fw-bold text-info">{{ .Stats.LowCount }}</div>
          <div class="small text-muted">Low</div>
        </div>
      </div>
    </div>
  </div>

  <!-- Chart -->
  <div class="row mb-4">
    <div class="col-md-4">
      <div class="card">
        <div class="card-body">
          <h6 class="card-title">Severity Distribution</h6>
          <canvas id="severityChart" height="200"></canvas>
        </div>
      </div>
    </div>
  </div>

  <!-- Products -->
  {{ range .Products }}
  <div class="card mb-4">
    <div class="card-header d-flex justify-content-between">
      <h5 class="mb-0">📦 {{ .ProductName }}</h5>
      <span class="badge bg-secondary">{{ .TotalFindings }} findings</span>
    </div>
    <div class="card-body p-0">
      <div class="table-responsive">
        <table class="table table-hover table-sm mb-0">
          <thead class="table-dark">
            <tr>
              <th>CVE</th>
              <th>Severity</th>
              <th>CVSS</th>
              <th>EPSS</th>
              <th>Exploit</th>
              <th>Component</th>
              <th>Title</th>
            </tr>
          </thead>
          <tbody>
            {{ range .Findings }}
            <tr>
              <td>
                {{ if .CVE }}
                <a href="https://nvd.nist.gov/vuln/detail/{{ .CVE }}" target="_blank" rel="noopener">{{ .CVE }}</a>
                {{ else }}—{{ end }}
              </td>
              <td>
                <span class="severity-badge" style="{{ severityBadgeStyle .Severity }}">{{ .Severity }}</span>
              </td>
              <td>{{ formatScore .CVSSv3Score }}</td>
              <td>{{ formatScore .EPSSScore }}</td>
              <td>{{ boolIcon .IsExploit }}</td>
              <td><code>{{ .ComponentName }}</code>{{ if .ComponentVersion }} <small class="text-muted">{{ .ComponentVersion }}</small>{{ end }}</td>
              <td class="text-truncate" style="max-width: 300px;" title="{{ .Title }}">{{ .Title }}</td>
            </tr>
            {{ end }}
          </tbody>
        </table>
      </div>
    </div>
  </div>
  {{ end }}
</div>

<!-- Scripts -->
<script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/js/bootstrap.bundle.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/chart.js@4/dist/chart.umd.min.js"></script>
<script>
  new Chart(document.getElementById('severityChart'), {
    type: 'doughnut',
    data: {
      labels: ['Critical', 'High', 'Medium', 'Low'],
      datasets: [{
        data: [{{ .Stats.CriticalCount }}, {{ .Stats.HighCount }}, {{ .Stats.MediumCount }}, {{ .Stats.LowCount }}],
        backgroundColor: ['#dc3545', '#fd7e14', '#ffc107', '#0d6efd'],
        borderWidth: 2
      }]
    },
    options: {
      plugins: { legend: { position: 'bottom' } },
      cutout: '60%'
    }
  });
</script>
</body>
</html>
```

### File 7: `services/report-service/internal/formatters/html_test.go`

```go
package formatters

import (
    "context"
    "strings"
    "testing"
    "time"

    "github.com/google/osv.dev/services/report-service/internal/domain/entity"
)

func buildTestInput(minSeverity string) *entity.ReportInput {
    score74 := 7.4
    score45 := 4.5

    return &entity.ReportInput{
        ScanTarget:  "192.168.1.0/24",
        GeneratedAt: time.Now(),
        Theme:       entity.ThemeDark,
        MinSeverity: minSeverity,
        Products: []*entity.ProductSection{
            {
                ProductName: "Test Product",
                Findings: []*entity.ReportFinding{
                    {CVE: "CVE-2021-44228", Severity: "Critical", CVSSv3Score: nil, Title: "Log4j RCE", ComponentName: "192.168.1.10"},
                    {CVE: "CVE-2021-45105", Severity: "High", CVSSv3Score: &score74, Title: "Log4j DoS", ComponentName: "192.168.1.10"},
                    {CVE: "CVE-2021-99999", Severity: "Medium", CVSSv3Score: &score45, Title: "Some Medium", ComponentName: "192.168.1.20"},
                },
            },
        },
    }
}

func TestHTMLFormatter_Format(t *testing.T) {
    f, err := NewHTMLFormatter()
    if err != nil {
        t.Fatalf("NewHTMLFormatter: %v", err)
    }

    html, err := f.Format(context.Background(), buildTestInput(""))
    if err != nil {
        t.Fatalf("Format(): %v", err)
    }
    if len(html) == 0 {
        t.Fatal("HTML output is empty")
    }

    output := string(html)
    if !strings.Contains(output, "192.168.1.0/24") {
        t.Error("HTML should contain scan target")
    }
    if !strings.Contains(output, "CVE-2021-44228") {
        t.Error("HTML should contain Critical CVE")
    }
    if !strings.Contains(output, "Vulnerability Scan Report") {
        t.Error("HTML should contain report title")
    }
    if !strings.Contains(output, "bootstrap") {
        t.Error("HTML should include Bootstrap")
    }
}

func TestHTMLFormatter_SeverityFilter(t *testing.T) {
    f, _ := NewHTMLFormatter()
    html, _ := f.Format(context.Background(), buildTestInput("high"))
    output := string(html)

    if !strings.Contains(output, "CVE-2021-44228") {
        t.Error("Critical should pass high filter")
    }
    if !strings.Contains(output, "CVE-2021-45105") {
        t.Error("High should pass high filter")
    }
    if strings.Contains(output, "CVE-2021-99999") {
        t.Error("Medium should NOT pass high filter")
    }
}

func TestHTMLFormatter_DarkTheme(t *testing.T) {
    f, _ := NewHTMLFormatter()
    html, _ := f.Format(context.Background(), buildTestInput(""))
    if !strings.Contains(string(html), `data-bs-theme="dark"`) {
        t.Error("HTML should have dark theme attribute")
    }
}

func TestMeetsFilter(t *testing.T) {
    score80 := 8.0
    score45 := 4.5

    criticalFinding := &entity.ReportFinding{Severity: "Critical", CVSSv3Score: nil}
    highFinding := &entity.ReportFinding{Severity: "High", CVSSv3Score: &score80}
    mediumFinding := &entity.ReportFinding{Severity: "Medium", CVSSv3Score: &score45}

    if !MeetsFilter(criticalFinding, "high", nil) {
        t.Error("Critical should pass high filter")
    }
    if !MeetsFilter(highFinding, "high", nil) {
        t.Error("High should pass high filter")
    }
    if MeetsFilter(mediumFinding, "high", nil) {
        t.Error("Medium should NOT pass high filter")
    }

    // Score filter
    minScore := 7.0
    if MeetsFilter(mediumFinding, "", &minScore) {
        t.Error("CVSS 4.5 should NOT pass minScore=7.0")
    }
    if !MeetsFilter(highFinding, "", &minScore) {
        t.Error("CVSS 8.0 should pass minScore=7.0")
    }
}
```

---

## Verification

```bash
cd services/report-service
go mod tidy
go build ./...
go test ./internal/formatters/... -v
```

**Expected**:
```
--- PASS: TestHTMLFormatter_Format
--- PASS: TestHTMLFormatter_SeverityFilter
--- PASS: TestHTMLFormatter_DarkTheme
--- PASS: TestMeetsFilter
```

### Checklist

- [x] `NewHTMLFormatter()` returns no error
- [x] `Format()` returns valid HTML (> 0 bytes)
- [x] HTML contains Bootstrap 5.3 reference
- [x] HTML contains Chart.js donut chart data
- [x] Severity filter: `min_severity=high` → only Critical + High in output
- [x] Score filter: `min_score=7.0` → only CVSSv3 >= 7.0
- [x] Theme toggle button in HTML (`no-print` class)
- [x] Dark mode: `data-bs-theme="dark"` attribute
- [x] CVE links: `https://nvd.nist.gov/vuln/detail/CVE-xxx`
