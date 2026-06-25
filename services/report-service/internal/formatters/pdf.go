package formatters

import (
    "bytes"
    "context"
    "fmt"
    "time"
)

// PDFFormatter generates PDF reports from HTML.
// Uses chromedp (headless Chrome) or wkhtmltopdf.
type PDFFormatter struct {
    ChromePath string // path to chromium/chrome binary
}

// New creates a PDFFormatter.
func New(chromePath string) *PDFFormatter {
    if chromePath == "" {
        chromePath = "/usr/bin/chromium-browser"
    }
    return &PDFFormatter{ChromePath: chromePath}
}

// ReportInput is a subset of report data needed for PDF generation.
type ReportInput struct {
    HTMLContent []byte
    ScanTarget  string
    GeneratedAt time.Time
    Title       string
}

// Format generates a PDF from HTML content.
// Strategy: HTML → chromedp headless Chrome → PDF bytes
func (f *PDFFormatter) Format(ctx context.Context, input *ReportInput) ([]byte, error) {
    // In production: use chromedp to render HTML to PDF
    // Example flow:
    //   1. Write HTML to temp file
    //   2. Run chromedp: page.PrintToPDF (A4, print background)
    //   3. Return PDF bytes
    //
    // For now: return a minimal PDF stub with metadata
    return generateMinimalPDF(input), nil
}

// generateMinimalPDF creates a placeholder PDF with report metadata.
// In production, replace with chromedp or wkhtmltopdf integration.
func generateMinimalPDF(input *ReportInput) []byte {
    // Minimal PDF 1.4 structure
    var buf bytes.Buffer

    title := input.Title
    if title == "" {
        title = "OpenVulnScan Security Report"
    }

    buf.WriteString("%PDF-1.4\n")
    buf.WriteString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")
    buf.WriteString("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")
    buf.WriteString("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>\nendobj\n")

    content := fmt.Sprintf("BT /F1 18 Tf 72 720 Td (%s) Tj ET\nBT /F1 12 Tf 72 680 Td (Target: %s) Tj ET\nBT /F1 12 Tf 72 660 Td (Generated: %s) Tj ET",
        title,
        input.ScanTarget,
        input.GeneratedAt.Format("2006-01-02 15:04:05 UTC"),
    )

    buf.WriteString(fmt.Sprintf("4 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n", len(content), content))
    buf.WriteString("5 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n")
    buf.WriteString("xref\n0 6\n")
    buf.WriteString("trailer\n<< /Size 6 /Root 1 0 R >>\n")
    buf.WriteString("%%EOF\n")

    return buf.Bytes()
}
