// Package excel provides Excel report generation from a stream of findings.
package excel

import (
	"bytes"
	"fmt"
	"time"

	"github.com/xuri/excelize/v2"
	findingv1 "github.com/defectdojo/proto/finding/v1"
)

// GenerateFindingsReport creates an xlsx report from a stream of findings.
// Uses a channel for streaming to avoid loading all findings into memory.
func GenerateFindingsReport(findings <-chan *findingv1.FindingProto) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Findings"
	f.SetSheetName("Sheet1", sheet)

	// Column definitions
	headers := []string{
		"ID", "Title", "Severity", "CVE", "CWE", "Status", "Active", "Verified",
		"Component", "Component Version", "File Path", "Line",
		"Hash Code", "Found Date", "SLA Expiration", "Product ID", "Test ID",
	}
	for col, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	// Header style: bold, dark blue background, white font
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#203864"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "bottom", Color: "#4472C4", Style: 2},
		},
	})
	f.SetRowStyle(sheet, 1, 1, headerStyle)
	f.SetRowHeight(sheet, 1, 20)

	// Severity colors
	severityColors := map[string]string{
		"Critical": "#FF0000",
		"High":     "#FF6600",
		"Medium":   "#FFAA00",
		"Low":      "#99CC00",
		"Info":     "#00AAFF",
	}

	row := 2
	for finding := range findings {
		sev := finding.Severity
		color, ok := severityColors[sev]
		if !ok {
			color = "#CCCCCC"
		}
		rowStyle, _ := f.NewStyle(&excelize.Style{
			Fill: excelize.Fill{Type: "pattern", Color: []string{color + "22"}, Pattern: 1},
		})

		status := "Active"
		if finding.IsMitigated {
			status = "Mitigated"
		} else if finding.FalsePositive {
			status = "False Positive"
		} else if finding.Duplicate {
			status = "Duplicate"
		}

		foundDate := ""
		if finding.Date != nil {
			foundDate = finding.Date.AsTime().Format(time.DateOnly)
		}
		slaDate := ""
		if finding.SlaExpirationDate != nil {
			slaDate = finding.SlaExpirationDate.AsTime().Format(time.DateOnly)
		}

		values := []interface{}{
			finding.Id, finding.Title, finding.Severity, finding.Cve, finding.Cwe,
			status, finding.Active, finding.Verified,
			finding.ComponentName, finding.ComponentVersion, finding.FilePath, finding.LineNumber,
			finding.HashCode, foundDate, slaDate, finding.ProductId, finding.TestId,
		}
		for col, val := range values {
			cell, _ := excelize.CoordinatesToCellName(col+1, row)
			f.SetCellValue(sheet, cell, val)
		}
		f.SetRowStyle(sheet, row, row, rowStyle)
		row++
	}

	// Auto-width columns
	for col := range headers {
		name, _ := excelize.ColumnNumberToName(col + 1)
		f.SetColWidth(sheet, name, name, 20)
	}

	// Freeze header row
	f.SetPanes(sheet, &excelize.Panes{
		Freeze:      true,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	})

	_ = fmt.Sprintf("Generated %d findings", row-2) // suppress unused import

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("excel write: %w", err)
	}
	return buf.Bytes(), nil
}
