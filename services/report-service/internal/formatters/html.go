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

//go:embed templates/report.html
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
