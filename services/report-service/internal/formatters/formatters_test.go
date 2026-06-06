package formatters_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
	"time"

	consolefmt "github.com/osv/report-service/internal/formatters/console"
	csvfmt "github.com/osv/report-service/internal/formatters/csv"
	jsonfmt "github.com/osv/report-service/internal/formatters/json"

	"github.com/osv/report-service/internal/domain/entity"
	"github.com/osv/report-service/internal/formatters"
)

// ─── Test Data ────────────────────────────────────────────────────────────────

var testInput = entity.ReportInput{
	ScanTarget:  "/firmware.bin",
	GeneratedAt: time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC),
	CVEData: map[entity.ProductInfo][]entity.CVEData{
		{Vendor: "openssl", Product: "openssl", Version: "1.1.1k"}: {
			{CVENumber: "CVE-2022-0778", Severity: "HIGH", Score: 7.5, DataSource: "NVD", IsExploit: false},
			{CVENumber: "CVE-2021-3711", Severity: "CRITICAL", Score: 9.8, DataSource: "NVD", IsExploit: true},
		},
		{Vendor: "curl", Product: "curl", Version: "7.68.0"}: {
			{CVENumber: "CVE-2021-22876", Severity: "MEDIUM", Score: 5.3, DataSource: "NVD", IsExploit: false},
		},
	},
}

var emptyInput = entity.ReportInput{
	ScanTarget:  "/clean.bin",
	GeneratedAt: time.Now(),
	CVEData:     map[entity.ProductInfo][]entity.CVEData{},
}

var opts = formatters.FormatOptions{Theme: "light"}

// ─── Console Formatter ────────────────────────────────────────────────────────

func TestConsoleFormatter_ContentType(t *testing.T) {
	f := consolefmt.New()
	if !strings.Contains(f.ContentType(), "text/plain") {
		t.Errorf("expected text/plain, got %q", f.ContentType())
	}
}

func TestConsoleFormatter_Format_HasCVEs(t *testing.T) {
	f := consolefmt.New()
	out, err := f.Format(context.Background(), testInput, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	outStr := string(out)

	// Must contain CVE numbers
	if !strings.Contains(outStr, "CVE-2022-0778") {
		t.Error("expected CVE-2022-0778 in output")
	}
	if !strings.Contains(outStr, "CVE-2021-3711") {
		t.Error("expected CVE-2021-3711 in output")
	}
	// Must have exploit marker
	if !strings.Contains(outStr, "EXPLOIT") {
		t.Error("expected EXPLOIT marker for exploitable CVE")
	}
	// Must have summary
	if !strings.Contains(outStr, "CVE(s) found") {
		t.Error("expected summary line")
	}
}

func TestConsoleFormatter_EmptyInput(t *testing.T) {
	f := consolefmt.New()
	out, err := f.Format(context.Background(), emptyInput, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should not error, may be empty body
	if out == nil {
		t.Error("expected non-nil output")
	}
}

// ─── CSV Formatter ────────────────────────────────────────────────────────────

func TestCSVFormatter_ContentType(t *testing.T) {
	f := csvfmt.New()
	if f.ContentType() != "text/csv" {
		t.Errorf("expected text/csv, got %q", f.ContentType())
	}
}

func TestCSVFormatter_Format_ValidCSV(t *testing.T) {
	f := csvfmt.New()
	out, err := f.Format(context.Background(), testInput, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := csv.NewReader(bytes.NewReader(out))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("invalid CSV: %v", err)
	}

	// First row must be header
	if len(records) < 1 {
		t.Fatal("expected at least header row")
	}
	header := records[0]
	if header[0] != "Vendor" || header[3] != "CVE_number" {
		t.Errorf("unexpected header: %v", header)
	}

	// Data rows: 3 CVEs total
	if len(records) != 4 { // 1 header + 3 CVEs
		t.Errorf("expected 4 rows (header+3 CVEs), got %d", len(records))
	}
}

func TestCSVFormatter_EmptyInput(t *testing.T) {
	f := csvfmt.New()
	out, err := f.Format(context.Background(), emptyInput, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should have header only
	r := csv.NewReader(bytes.NewReader(out))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("invalid CSV: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("expected header-only CSV for empty input, got %d rows", len(records))
	}
}

// ─── JSON Formatter ───────────────────────────────────────────────────────────

func TestJSONFormatter_ContentType(t *testing.T) {
	f := jsonfmt.New()
	if f.ContentType() != "application/json" {
		t.Errorf("expected application/json, got %q", f.ContentType())
	}
}

func TestJSONFormatter_ValidJSON(t *testing.T) {
	f := jsonfmt.New()
	out, err := f.Format(context.Background(), testInput, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}

	// Check metadata block
	meta, ok := result["metadata"].(map[string]interface{})
	if !ok {
		t.Fatal("expected metadata block")
	}
	if meta["scan_target"] != "/firmware.bin" {
		t.Errorf("expected scan_target=/firmware.bin, got %v", meta["scan_target"])
	}
	if meta["total_cves"].(float64) != 3 {
		t.Errorf("expected total_cves=3, got %v", meta["total_cves"])
	}

	// Check cves array
	cves, ok := result["cves"].([]interface{})
	if !ok || len(cves) != 3 {
		t.Errorf("expected 3 CVEs, got %d", len(cves))
	}
}

func TestJSONFormatter_EmptyInput(t *testing.T) {
	f := jsonfmt.New()
	out, err := f.Format(context.Background(), emptyInput, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	cves := result["cves"].([]interface{})
	if len(cves) != 0 {
		t.Errorf("expected 0 CVEs for empty input, got %d", len(cves))
	}
}

// ─── JSON2 Formatter ──────────────────────────────────────────────────────────

func TestJSON2Formatter_OrganizedByProduct(t *testing.T) {
	f := jsonfmt.NewJSON2()
	out, err := f.Format(context.Background(), testInput, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}

	results, ok := result["results"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'results' object in JSON2 output")
	}

	// Should have 2 products
	if len(results) != 2 {
		t.Errorf("expected 2 products, got %d", len(results))
	}
}

func TestJSON2Formatter_EmptyInput(t *testing.T) {
	f := jsonfmt.NewJSON2()
	out, err := f.Format(context.Background(), emptyInput, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	results := result["results"].(map[string]interface{})
	if len(results) != 0 {
		t.Errorf("expected 0 products for empty input")
	}
}
