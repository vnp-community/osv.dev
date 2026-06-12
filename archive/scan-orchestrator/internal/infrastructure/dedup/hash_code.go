// Package dedup implements DefectDojo's finding deduplication via hash code computation.
// The hash uniquely identifies a vulnerability across scans for deduplication.
package dedup

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/defectdojo/scan-orchestrator/internal/domain/parser"
)

// ComputeHashCode generates the deduplication hash for a single parsed finding.
// Algorithm mirrors Django DefectDojo's Finding.compute_hash_code().
// The hash is stable: the same vulnerability in different scans produces the same hash.
func ComputeHashCode(f *parser.ParsedFinding) string {
	// Ordered list of fields used for hashing — ordering is fixed for stability.
	parts := []string{
		strings.ToLower(strings.TrimSpace(f.Title)),
		strings.ToLower(strings.TrimSpace(f.Severity)),
		strings.ToLower(strings.TrimSpace(f.CVE)),
		strings.TrimSpace(f.FilePath),
		fmt.Sprintf("%d", f.CWE),
		strings.ToLower(strings.TrimSpace(f.ComponentName)),
		strings.ToLower(strings.TrimSpace(f.ComponentVersion)),
		strings.TrimSpace(f.VulnIDFromTool),
	}
	combined := strings.Join(parts, "|")
	hash := sha256.Sum256([]byte(combined))
	return fmt.Sprintf("%x", hash)
}

// ComputeHashCodes computes and sets HashCode on all findings in-place.
// Skips findings that already have a HashCode set (e.g. from the parser itself).
func ComputeHashCodes(findings []*parser.ParsedFinding) {
	for _, f := range findings {
		if f.HashCode == "" {
			f.HashCode = ComputeHashCode(f)
		}
	}
}

// FilterBySeverity returns only findings at or above the given minimum severity.
func FilterBySeverity(findings []*parser.ParsedFinding, minimum string) []*parser.ParsedFinding {
	minNum := severityNum(minimum)
	if minNum == 0 {
		return findings // "Info" = no filter
	}
	var result []*parser.ParsedFinding
	for _, f := range findings {
		if severityNum(f.Severity) >= minNum {
			result = append(result, f)
		}
	}
	return result
}

func severityNum(s string) int {
	switch strings.ToLower(s) {
	case "critical":
		return 5
	case "high":
		return 4
	case "medium":
		return 3
	case "low":
		return 2
	case "info", "informational":
		return 1
	default:
		return 0
	}
}
