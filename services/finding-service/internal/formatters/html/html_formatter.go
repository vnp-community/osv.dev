// Package html implements the HTML report formatter.
package html

import (
	"bytes"
	"context"
	"encoding/json"
	_ "embed"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/osv/finding-service/internal/domain/report"
	"github.com/osv/finding-service/internal/domain/report/service"
	"github.com/osv/finding-service/internal/formatters"
)

//go:embed templates/report.html.tmpl
var reportTemplate string

//go:embed templates/styles.css
var stylesCSS string

// ─────────────────────────────────────────────────────────────────────────────
// Template data structures
// ─────────────────────────────────────────────────────────────────────────────

type templateData struct {
	Target         string
	Timestamp      string
	Theme          string
	TotalCVEs      int
	CSS            template.CSS
	ChartJS        template.JS
	Products       []productData
	SeverityCounts map[string]int
}

type productData struct {
	Vendor  string
	Product string
	Version string
	PURL    string
	CVEs    []cveData
}

type cveData struct {
	CVENumber   string
	Severity    string
	Score       float64
	CVSSVersion int
	CVSSVector  string
	DataSource  string
	IsExploit   bool
}

// ─────────────────────────────────────────────────────────────────────────────
// HTMLFormatter
// ─────────────────────────────────────────────────────────────────────────────

// HTMLFormatter renders a self-contained HTML report with Plotly charts.
type HTMLFormatter struct {
	tmpl *template.Template
}

// New creates a new HTMLFormatter.
func New() (*HTMLFormatter, error) {
	funcMap := template.FuncMap{
		"lower": strings.ToLower,
	}
	tmpl, err := template.New("report").Funcs(funcMap).Parse(reportTemplate)
	if err != nil {
		return nil, fmt.Errorf("html formatter: parse template: %w", err)
	}
	return &HTMLFormatter{tmpl: tmpl}, nil
}

// ContentType returns text/html.
func (f *HTMLFormatter) ContentType() string { return "text/html; charset=utf-8" }

// FileExtension returns ".html".
func (f *HTMLFormatter) FileExtension() string { return ".html" }

// Format renders the HTML report.
func (f *HTMLFormatter) Format(_ context.Context, data report.ReportInput, opts formatters.FormatOptions) ([]byte, error) {
	theme := opts.Theme
	if theme == "" {
		theme = "light"
	}

	// Build product entries
	var products []productData
	severityCounts := map[string]int{"CRITICAL": 0, "HIGH": 0, "MEDIUM": 0, "LOW": 0, "UNKNOWN": 0}
	totalCVEs := 0

	for product, cves := range data.CVEData {
		filtered := service.FilterAndSort(cves, data.MinSeverity, data.MinScore)
		if len(filtered) == 0 {
			continue
		}

		pd := productData{
			Vendor:  product.Vendor,
			Product: product.Product,
			Version: product.Version,
			PURL:    product.PURL,
		}
		for _, c := range filtered {
			pd.CVEs = append(pd.CVEs, cveData{
				CVENumber:   c.CVENumber,
				Severity:    c.Severity,
				Score:       c.Score,
				CVSSVersion: c.CVSSVersion,
				CVSSVector:  c.CVSSVector,
				DataSource:  c.DataSource,
				IsExploit:   c.IsExploit,
			})
			sev := strings.ToUpper(c.Severity)
			if _, ok := severityCounts[sev]; ok {
				severityCounts[sev]++
			} else {
				severityCounts["UNKNOWN"]++
			}
			totalCVEs++
		}
		products = append(products, pd)
	}

	// Build Plotly chart JS
	chartJS := buildPlotlyChart(severityCounts)

	ts := data.GeneratedAt
	if ts.IsZero() {
		ts = time.Now()
	}

	td := templateData{
		Target:         data.ScanTarget,
		Timestamp:      ts.UTC().Format("2006-01-02 15:04:05 UTC"),
		Theme:          theme,
		TotalCVEs:      totalCVEs,
		CSS:            template.CSS(stylesCSS),
		ChartJS:        template.JS(chartJS),
		Products:       products,
		SeverityCounts: severityCounts,
	}

	var buf bytes.Buffer
	if err := f.tmpl.Execute(&buf, td); err != nil {
		return nil, fmt.Errorf("html formatter: render: %w", err)
	}
	return buf.Bytes(), nil
}

// buildPlotlyChart generates JavaScript for a Plotly pie chart.
func buildPlotlyChart(counts map[string]int) string {
	labels := []string{"CRITICAL", "HIGH", "MEDIUM", "LOW", "UNKNOWN"}
	colors := []string{"#dc2626", "#ea580c", "#d97706", "#65a30d", "#6b7280"}

	values := make([]int, len(labels))
	for i, l := range labels {
		values[i] = counts[l]
	}

	labelsJSON, _ := json.Marshal(labels)
	valuesJSON, _ := json.Marshal(values)
	colorsJSON, _ := json.Marshal(colors)

	return fmt.Sprintf(`
Plotly.newPlot('severity-chart', [{
    type: 'pie',
    labels: %s,
    values: %s,
    marker: { colors: %s },
    textinfo: 'label+percent',
    hovertemplate: '%%{label}: %%{value}<extra></extra>'
}], {
    title: { text: 'Severity Distribution' },
    paper_bgcolor: 'transparent',
    plot_bgcolor: 'transparent',
    showlegend: true,
    margin: { t: 40, b: 0, l: 0, r: 0 }
}, { responsive: true, displayModeBar: false });
`, labelsJSON, valuesJSON, colorsJSON)
}

// Ensure interface compliance.
var _ formatters.Formatter = (*HTMLFormatter)(nil)
