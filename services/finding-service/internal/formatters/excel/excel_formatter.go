// Package excel implements the Excel (.xlsx) report formatter for report-service.
// This wraps the GenerateFindingsReport function from defectdojo/report.
package excel

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/osv/finding-service/internal/domain/entity"
	"github.com/osv/finding-service/internal/formatters"
	"github.com/xuri/excelize/v2"
)

// Formatter implements formatters.Formatter for Excel (.xlsx) output.
type Formatter struct{}

// New returns a new ExcelFormatter.
func New() *Formatter { return &Formatter{} }

// ContentType returns the MIME type for xlsx files.
func (f *Formatter) ContentType() string {
	return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
}

// FileExtension returns the recommended file extension.
func (f *Formatter) FileExtension() string { return ".xlsx" }

// Format generates an Excel report from the given ReportInput.
func (f *Formatter) Format(_ context.Context, data entity.ReportInput, _ formatters.FormatOptions) ([]byte, error) {
	xf := excelize.NewFile()
	defer xf.Close()

	sheet := "Findings"
	xf.SetSheetName("Sheet1", sheet)

	headers := []string{
		"ID", "Title", "Severity", "CVE", "CWE", "Status",
		"Component", "Component Version", "File Path", "Line",
		"Hash Code", "Found Date", "Product ID",
	}
	for col, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		xf.SetCellValue(sheet, cell, h)
	}

	// Header style: bold, dark blue background, white font
	headerStyle, _ := xf.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#203864"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	xf.SetRowStyle(sheet, 1, 1, headerStyle)
	xf.SetRowHeight(sheet, 1, 20)

	severityColors := map[string]string{
		"Critical": "#FF0000",
		"High":     "#FF6600",
		"Medium":   "#FFAA00",
		"Low":      "#99CC00",
		"Info":     "#0070C0",
	}

	row := 2
	for _, finding := range data.Findings {
		severity := finding.Severity
		bgColor := "#FFFFFF"
		if c, ok := severityColors[severity]; ok {
			bgColor = c
		}
		rowStyle, _ := xf.NewStyle(&excelize.Style{
			Fill: excelize.Fill{Type: "pattern", Color: []string{bgColor}, Pattern: 1},
		})

		values := []interface{}{
			finding.ID,
			finding.Title,
			severity,
			finding.CVE,
			finding.CWE,
			finding.Status,
			finding.Component,
			finding.ComponentVersion,
			finding.FilePath,
			finding.Line,
			finding.HashCode,
			finding.FoundDate.Format(time.RFC3339),
			finding.ProductID,
		}
		for col, v := range values {
			cell, _ := excelize.CoordinatesToCellName(col+1, row)
			xf.SetCellValue(sheet, cell, v)
		}
		xf.SetRowStyle(sheet, row, row, rowStyle)
		row++
	}

	// Auto-fit columns
	for col := range headers {
		colName, _ := excelize.ColumnNumberToName(col + 1)
		xf.SetColWidth(sheet, colName, colName, 18)
	}

	// Summary sheet
	summarySheet := "Summary"
	xf.NewSheet(summarySheet)
	xf.SetCellValue(summarySheet, "A1", "Report Generated")
	xf.SetCellValue(summarySheet, "B1", time.Now().Format(time.RFC3339))
	xf.SetCellValue(summarySheet, "A2", "Total Findings")
	xf.SetCellValue(summarySheet, "B2", len(data.Findings))
	xf.SetCellValue(summarySheet, "A3", "Target")
	xf.SetCellValue(summarySheet, "B3", data.Target)

	var buf bytes.Buffer
	if err := xf.Write(&buf); err != nil {
		return nil, fmt.Errorf("excel write: %w", err)
	}
	return buf.Bytes(), nil
}
