package service_test

import (
	"testing"

	"github.com/osv/finding-service/internal/domain/report"
	"github.com/osv/finding-service/internal/domain/report/service"
)

var testCVEs = []report.CVEData{
	{CVENumber: "CVE-2021-1", Severity: "CRITICAL", Score: 9.8},
	{CVENumber: "CVE-2021-2", Severity: "HIGH", Score: 7.5},
	{CVENumber: "CVE-2021-3", Severity: "MEDIUM", Score: 5.0},
	{CVENumber: "CVE-2021-4", Severity: "LOW", Score: 2.5},
	{CVENumber: "CVE-2021-5", Severity: "UNKNOWN", Score: 0},
}

func TestFilterBySeverity_AllThrough(t *testing.T) {
	got := service.FilterBySeverity(testCVEs, "")
	if len(got) != len(testCVEs) {
		t.Errorf("expected %d, got %d", len(testCVEs), len(got))
	}
}

func TestFilterBySeverity_HighAndAbove(t *testing.T) {
	got := service.FilterBySeverity(testCVEs, "HIGH")
	if len(got) != 2 {
		t.Errorf("expected 2 (HIGH+CRITICAL), got %d", len(got))
	}
	for _, c := range got {
		if c.Severity != "HIGH" && c.Severity != "CRITICAL" {
			t.Errorf("unexpected severity: %s", c.Severity)
		}
	}
}

func TestFilterBySeverity_Critical(t *testing.T) {
	got := service.FilterBySeverity(testCVEs, "CRITICAL")
	if len(got) != 1 || got[0].CVENumber != "CVE-2021-1" {
		t.Errorf("expected only CRITICAL CVE, got %v", got)
	}
}

func TestFilterByScore(t *testing.T) {
	got := service.FilterByScore(testCVEs, 7.0)
	if len(got) != 2 {
		t.Errorf("expected 2 (score>=7), got %d", len(got))
	}
}

func TestFilterByScore_Zero(t *testing.T) {
	got := service.FilterByScore(testCVEs, 0)
	if len(got) != len(testCVEs) {
		t.Errorf("expected all %d, got %d", len(testCVEs), len(got))
	}
}

func TestSortBySeverity_CriticalFirst(t *testing.T) {
	// Shuffle input
	mixed := []report.CVEData{
		{CVENumber: "low", Severity: "LOW", Score: 2},
		{CVENumber: "critical", Severity: "CRITICAL", Score: 9.8},
		{CVENumber: "medium", Severity: "MEDIUM", Score: 5},
	}
	sorted := service.SortBySeverity(mixed)

	if sorted[0].Severity != "CRITICAL" {
		t.Errorf("first should be CRITICAL, got %s", sorted[0].Severity)
	}
	if sorted[len(sorted)-1].Severity != "LOW" {
		t.Errorf("last should be LOW, got %s", sorted[len(sorted)-1].Severity)
	}
}

func TestSortBySeverity_SameRank_ScoreOrder(t *testing.T) {
	mixed := []report.CVEData{
		{CVENumber: "h1", Severity: "HIGH", Score: 7.0},
		{CVENumber: "h2", Severity: "HIGH", Score: 8.5},
	}
	sorted := service.SortBySeverity(mixed)
	if sorted[0].Score != 8.5 {
		t.Errorf("expected score 8.5 first, got %f", sorted[0].Score)
	}
}

func TestOutputFormat_IsValid(t *testing.T) {
	valid := []report.OutputFormat{
		report.OutFormatConsole, report.OutFormatCSV, report.OutFormatJSON,
		report.OutFormatJSON2, report.OutFormatHTML, report.OutFormatPDF,
	}
	for _, f := range valid {
		if !f.IsValid() {
			t.Errorf("expected %q to be valid", f)
		}
	}
	if report.OutputFormat("invalid").IsValid() {
		t.Error("expected 'invalid' to be invalid")
	}
}

func TestAllFormats_Count(t *testing.T) {
	formats := report.AllFormats()
	if len(formats) != 6 {
		t.Errorf("expected 6 formats, got %d", len(formats))
	}
}
