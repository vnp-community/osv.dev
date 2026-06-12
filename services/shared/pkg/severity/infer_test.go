package severity_test

import (
	"testing"

	"github.com/osv/shared/pkg/severity"
)

func ptr(f float64) *float64 { return &f }

func TestInferFromCVSS(t *testing.T) {
	tests := []struct {
		name        string
		score       *float64
		rawSeverity string
		want        severity.Severity
	}{
		// rawSeverity override wins regardless of score
		{"raw CRITICAL wins over score", ptr(3.0), "CRITICAL", severity.Critical},
		{"raw HIGH wins over score", ptr(1.0), "HIGH", severity.High},
		{"raw MEDIUM wins over score", ptr(9.5), "MEDIUM", severity.Medium},
		{"raw LOW wins", ptr(0.0), "LOW", severity.Low},
		{"raw case insensitive critical", ptr(0.0), "critical", severity.Critical},
		{"raw case insensitive high", ptr(0.0), "High", severity.High},
		{"raw unknown string ignored", ptr(9.5), "n/a", severity.Critical}, // falls back to score

		// Score-based classification (rawSeverity = "")
		{"score 10.0 → CRITICAL", ptr(10.0), "", severity.Critical},
		{"score 9.0 → CRITICAL", ptr(9.0), "", severity.Critical},
		{"score 8.9 → HIGH", ptr(8.9), "", severity.High},
		{"score 7.0 → HIGH", ptr(7.0), "", severity.High},
		{"score 6.9 → MEDIUM", ptr(6.9), "", severity.Medium},
		{"score 4.0 → MEDIUM", ptr(4.0), "", severity.Medium},
		{"score 3.9 → LOW", ptr(3.9), "", severity.Low},
		{"score 0.1 → LOW", ptr(0.1), "", severity.Low},
		{"score 0.0 → UNKNOWN", ptr(0.0), "", severity.Unknown},
		{"nil score → UNKNOWN", nil, "", severity.Unknown},
		{"nil score + empty raw → UNKNOWN", nil, "", severity.Unknown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := severity.InferFromCVSS(tc.score, tc.rawSeverity)
			if got != tc.want {
				t.Errorf("InferFromCVSS(%v, %q) = %q, want %q", tc.score, tc.rawSeverity, got, tc.want)
			}
		})
	}
}

func TestIsValid(t *testing.T) {
	valid := []string{"CRITICAL", "HIGH", "MEDIUM", "LOW", "UNKNOWN", "critical", "High"}
	for _, s := range valid {
		if !severity.IsValid(s) {
			t.Errorf("IsValid(%q) = false, want true", s)
		}
	}

	invalid := []string{"", "n/a", "NONE", "INFO", "SEVERE"}
	for _, s := range invalid {
		if severity.IsValid(s) {
			t.Errorf("IsValid(%q) = true, want false", s)
		}
	}
}

func TestMustParse(t *testing.T) {
	if got := severity.MustParse("CRITICAL"); got != severity.Critical {
		t.Errorf("got %q", got)
	}
	if got := severity.MustParse("junk"); got != severity.Unknown {
		t.Errorf("got %q", got)
	}
}
