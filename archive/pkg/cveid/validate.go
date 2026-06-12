// Package cveid provides CVE identifier validation, normalization, and parsing.
package cveid

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// pattern matches canonical CVE IDs: CVE-YYYY-NNNNN (4+ digits).
var pattern = regexp.MustCompile(`^CVE-\d{4}-\d{4,}$`)

// IsValid reports whether id is a syntactically valid CVE identifier.
// A valid ID matches CVE-YYYY-NNNNN where NNNNN is at least 4 digits.
func IsValid(id string) bool {
	return pattern.MatchString(id)
}

// Normalize returns the canonical form of a CVE ID: uppercased and trimmed.
// It does NOT validate; call IsValid if needed.
func Normalize(id string) string {
	return strings.ToUpper(strings.TrimSpace(id))
}

// Year extracts the 4-digit year from a CVE ID (e.g. "CVE-2021-44228" → 2021).
// Returns an error if the ID format is invalid.
func Year(id string) (int, error) {
	if !IsValid(id) {
		return 0, fmt.Errorf("cveid: invalid CVE ID %q", id)
	}
	parts := strings.Split(id, "-")
	// parts = ["CVE", "YYYY", "NNNNN"]
	return strconv.Atoi(parts[1])
}

// Sequence extracts the numeric sequence part from a CVE ID
// (e.g. "CVE-2021-44228" → 44228).
func Sequence(id string) (int, error) {
	if !IsValid(id) {
		return 0, fmt.Errorf("cveid: invalid CVE ID %q", id)
	}
	parts := strings.Split(id, "-")
	return strconv.Atoi(parts[2])
}

// NormalizeAndValidate is a convenience function that normalizes id and then
// reports whether it is valid. Returns the normalized form and validity flag.
func NormalizeAndValidate(id string) (normalized string, ok bool) {
	n := Normalize(id)
	return n, IsValid(n)
}
