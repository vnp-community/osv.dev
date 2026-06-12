// Package severity provides CVSS score to severity level mapping.
// Follows CVSS v3.1 rating scale (NVD standard).
package severity

import "strings"

// Severity represents a CVE severity level.
type Severity string

const (
	Critical Severity = "CRITICAL"
	High     Severity = "HIGH"
	Medium   Severity = "MEDIUM"
	Low      Severity = "LOW"
	Unknown  Severity = "UNKNOWN"
)

// All returns all valid severity values in descending order.
func All() []Severity {
	return []Severity{Critical, High, Medium, Low, Unknown}
}

// IsValid reports whether s is a recognised severity level.
func IsValid(s string) bool {
	switch Severity(strings.ToUpper(s)) {
	case Critical, High, Medium, Low, Unknown:
		return true
	}
	return false
}

// InferFromCVSS converts a CVSS base score and optional raw severity string
// to a Severity level using CVSS v3.1 rating:
//
//	CRITICAL: 9.0 – 10.0
//	HIGH:     7.0 – 8.9
//	MEDIUM:   4.0 – 6.9
//	LOW:      0.1 – 3.9
//	UNKNOWN:  nil score or 0.0
//
// rawSeverity (if non-empty) is checked first, so an explicit "HIGH" string
// always wins even when score is 9.5.
func InferFromCVSS(score *float64, rawSeverity string) Severity {
	// 1. Prefer explicit raw severity string (NVD / CIRCL often provides this).
	switch Severity(strings.ToUpper(strings.TrimSpace(rawSeverity))) {
	case Critical:
		return Critical
	case High:
		return High
	case Medium:
		return Medium
	case Low:
		return Low
	}

	// 2. Fall back to numeric CVSS score.
	if score == nil {
		return Unknown
	}
	v := *score
	switch {
	case v >= 9.0:
		return Critical
	case v >= 7.0:
		return High
	case v >= 4.0:
		return Medium
	case v > 0.0:
		return Low
	default:
		return Unknown
	}
}

// MustParse converts a string to Severity, returning Unknown for unrecognised values.
func MustParse(s string) Severity {
	up := Severity(strings.ToUpper(strings.TrimSpace(s)))
	if IsValid(string(up)) {
		return up
	}
	return Unknown
}
