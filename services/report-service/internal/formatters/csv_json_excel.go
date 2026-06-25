package formatters

import (
    "bytes"
    "context"
    "encoding/csv"
    "encoding/json"
    "fmt"
    "strconv"
    "time"
)

// Finding is the shared data structure for CSV/Excel/JSON formatters.
type Finding struct {
    ID               string
    Title            string
    Severity         string
    CVE              string
    CWE              int
    Status           string
    ComponentName    string
    ComponentVersion string
    FilePath         string
    LineNumber       int
    HashCode         string
    Date             time.Time
    SLAExpiration    *time.Time
    ProductID        string
    TestID           string
    Description      string
    Mitigation       string
    CVSSv3Score      *float64
    EPSSScore        *float64
}

// CSVFormatter generates RFC 4180-compliant CSV reports.
type CSVFormatter struct{}

// NewCSVFormatter creates a CSVFormatter.
func NewCSVFormatter() *CSVFormatter { return &CSVFormatter{} }

// csvHeaders are the column headers.
var csvHeaders = []string{
    "ID", "Title", "Severity", "CVE", "CWE", "Status",
    "Component Name", "Component Version", "File Path", "Line",
    "Hash Code", "Date", "SLA Expiration",
    "Product ID", "Test ID", "CVSSv3 Score", "EPSS Score",
    "Description", "Mitigation",
}

// Format generates CSV bytes from a list of findings.
func (f *CSVFormatter) Format(_ context.Context, findings []*Finding) ([]byte, error) {
    var buf bytes.Buffer
    w := csv.NewWriter(&buf)

    // Write header
    if err := w.Write(csvHeaders); err != nil {
        return nil, fmt.Errorf("csv header: %w", err)
    }

    // Write rows
    for _, fd := range findings {
        slaStr := ""
        if fd.SLAExpiration != nil {
            slaStr = fd.SLAExpiration.Format(time.RFC3339)
        }
        cvssStr := ""
        if fd.CVSSv3Score != nil {
            cvssStr = strconv.FormatFloat(*fd.CVSSv3Score, 'f', 1, 64)
        }
        epssStr := ""
        if fd.EPSSScore != nil {
            epssStr = strconv.FormatFloat(*fd.EPSSScore, 'f', 4, 64)
        }

        row := []string{
            fd.ID, fd.Title, fd.Severity,
            fd.CVE, strconv.Itoa(fd.CWE), fd.Status,
            fd.ComponentName, fd.ComponentVersion,
            fd.FilePath, strconv.Itoa(fd.LineNumber),
            fd.HashCode, fd.Date.Format(time.RFC3339),
            slaStr, fd.ProductID, fd.TestID,
            cvssStr, epssStr,
            fd.Description, fd.Mitigation,
        }
        if err := w.Write(row); err != nil {
            return nil, fmt.Errorf("csv row: %w", err)
        }
    }

    w.Flush()
    if err := w.Error(); err != nil {
        return nil, fmt.Errorf("csv flush: %w", err)
    }

    return buf.Bytes(), nil
}

// JSONFormatter generates JSON reports.
type JSONFormatter struct{}

// NewJSONFormatter creates a JSONFormatter.
func NewJSONFormatter() *JSONFormatter { return &JSONFormatter{} }

// JSONReport is the root structure for JSON reports.
type JSONReport struct {
    GeneratedAt time.Time  `json:"generated_at"`
    TotalCount  int        `json:"total_count"`
    Findings    []*Finding `json:"findings"`
}

// Format generates JSON bytes from a list of findings.
func (f *JSONFormatter) Format(_ context.Context, findings []*Finding) ([]byte, error) {
    report := JSONReport{
        GeneratedAt: time.Now().UTC(),
        TotalCount:  len(findings),
        Findings:    findings,
    }
    data, err := json.MarshalIndent(report, "", "  ")
    if err != nil {
        return nil, fmt.Errorf("json marshal: %w", err)
    }
    return data, nil
}

// ExcelFormatter generates DefectDojo-compatible Excel reports.
type ExcelFormatter struct{}

// NewExcelFormatter creates an ExcelFormatter.
func NewExcelFormatter() *ExcelFormatter { return &ExcelFormatter{} }

// defectDojoColumns matches DefectDojo's finding import format.
var defectDojoColumns = []string{
    "ID", "Title", "Severity", "CVE", "CWE", "Status",
    "Component Name", "Component Version", "File Path", "Line",
    "Hash Code", "Date", "SLA Expiration", "Product ID", "Test ID",
    "Description", "Mitigation",
}

// Format generates Excel-compatible CSV bytes (XLSX generation requires excelize dependency).
// Falls back to CSV-in-XLSX format for CI environments without excelize.
func (f *ExcelFormatter) Format(ctx context.Context, findings []*Finding) ([]byte, error) {
    // In production, use github.com/xuri/excelize/v2:
    //   file := excelize.NewFile()
    //   sheet := "Findings"
    //   ... set header row with bold style ...
    //   ... set data rows ...
    //   return file.WriteToBuffer()
    //
    // For now, generate CSV with DefectDojo column order
    var buf bytes.Buffer
    w := csv.NewWriter(&buf)

    _ = w.Write(defectDojoColumns)

    for _, fd := range findings {
        slaStr := ""
        if fd.SLAExpiration != nil {
            slaStr = fd.SLAExpiration.Format("2006-01-02")
        }
        row := []string{
            fd.ID, fd.Title, fd.Severity,
            fd.CVE, strconv.Itoa(fd.CWE), fd.Status,
            fd.ComponentName, fd.ComponentVersion,
            fd.FilePath, strconv.Itoa(fd.LineNumber),
            fd.HashCode, fd.Date.Format("2006-01-02"),
            slaStr, fd.ProductID, fd.TestID,
            fd.Description, fd.Mitigation,
        }
        _ = w.Write(row)
    }

    w.Flush()
    return buf.Bytes(), nil
}
