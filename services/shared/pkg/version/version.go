// Package version provides extended version comparison for CVE range matching.
// Supports semantic versions (1.2.3), alpha suffixes (1.2.3a), and
// Python-style pre-releases (1.2.3-beta, 1.2.3_pre).
//
// This extends pkg/semver to handle the full range of version strings
// found in NVD/OSV CVE data, matching the Python cve-bin-tool behavior.
package version

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Version is a parsed, comparable version string.
type Version struct {
	parts []part
	raw   string
}

// part holds one segment of a version (either numeric or alphabetic).
type part struct {
	numeric int
	alpha   string
	isAlpha bool
}

var versionSplitter = regexp.MustCompile(`(\d+|[a-zA-Z]+)`)

// Parse parses a version string into a comparable Version.
// Empty string returns a zero Version (less than any real version).
func Parse(s string) Version {
	if s == "" || s == "*" || s == "N/A" || s == "n/a" {
		return Version{raw: s}
	}

	// Normalize: replace _ and - with . (but only between version segments)
	normalized := normalizeVersion(s)

	parts := versionSplitter.FindAllString(normalized, -1)
	result := make([]part, 0, len(parts))

	for _, p := range parts {
		if p == "" {
			continue
		}
		if isNumeric(p) {
			n, _ := strconv.Atoi(p)
			result = append(result, part{numeric: n})
		} else {
			result = append(result, part{alpha: strings.ToLower(p), isAlpha: true})
		}
	}

	return Version{parts: result, raw: s}
}

func normalizeVersion(s string) string {
	var b strings.Builder
	prev := rune(0)
	for _, r := range s {
		if (r == '-' || r == '_') &&
			(unicode.IsDigit(prev) || unicode.IsLetter(prev)) {
			b.WriteRune('.')
		} else {
			b.WriteRune(r)
		}
		prev = r
	}
	return b.String()
}

func isNumeric(s string) bool {
	for _, c := range s {
		if !unicode.IsDigit(c) {
			return false
		}
	}
	return true
}

// compare returns -1, 0, or 1 (like strings.Compare).
// Comparison rules:
//   - Numeric parts compare numerically
//   - Alpha parts compare lexicographically
//   - Alpha part vs MISSING part: alpha is GREATER (e.g. "1.1.1k" > "1.1.1")
//   - Alpha part vs NUMERIC part (0): numeric is considered a pre-release separator
//     so alpha like "beta","rc","pre" < numeric (e.g. "1.0.beta" < "1.0.0")
//     but a single trailing alpha (OpenSSL-style) > missing (e.g. "1.1.1k" > "1.1.1")
func (v Version) compare(other Version) int {
	maxLen := len(v.parts)
	if len(other.parts) > maxLen {
		maxLen = len(other.parts)
	}

	for i := 0; i < maxLen; i++ {
		// Sentinel: use a "missing" flag rather than zero-value part
		vMissing := i >= len(v.parts)
		oMissing := i >= len(other.parts)

		if vMissing && oMissing {
			break
		}

		var vp, op part
		if !vMissing {
			vp = v.parts[i]
		}
		if !oMissing {
			op = other.parts[i]
		}

		// Both present and both alpha
		if !vMissing && !oMissing && vp.isAlpha && op.isAlpha {
			if vp.alpha < op.alpha {
				return -1
			}
			if vp.alpha > op.alpha {
				return 1
			}
			continue
		}

		// vp is alpha, op is missing:
		// e.g. "1.1.1k" vs "1.1.1" — alpha suffix > missing → v is GREATER
		if !vMissing && vp.isAlpha && oMissing {
			return 1
		}
		// vp is missing, op is alpha:
		// e.g. "1.1.1" vs "1.1.1k" — missing < alpha → v is LESS
		if vMissing && !oMissing && op.isAlpha {
			return -1
		}

		// vp is alpha, op is numeric (and op is not 0 or op > 0 means a real version seg):
		// Pre-release alpha < numeric: "1.0.beta" < "1.0.1"
		// BUT single-char trailing alpha (OpenSSL) is a patch: handled above via missing check
		if !vMissing && !oMissing && vp.isAlpha && !op.isAlpha {
			// Pre-release keywords: beta, rc, alpha, pre → less than numeric
			return -1
		}
		if !vMissing && !oMissing && !vp.isAlpha && op.isAlpha {
			return 1
		}

		// Both numeric (or one is missing = treated as 0)
		vn := 0
		if !vMissing && !vp.isAlpha {
			vn = vp.numeric
		}
		on := 0
		if !oMissing && !op.isAlpha {
			on = op.numeric
		}

		if vn < on {
			return -1
		}
		if vn > on {
			return 1
		}
	}
	return 0
}

// LessThan returns true if v < other.
func (v Version) LessThan(other Version) bool { return v.compare(other) < 0 }

// Equal returns true if v == other.
func (v Version) Equal(other Version) bool { return v.compare(other) == 0 }

// GreaterThan returns true if v > other.
func (v Version) GreaterThan(other Version) bool { return v.compare(other) > 0 }

// LessOrEqual returns true if v <= other.
func (v Version) LessOrEqual(other Version) bool { return v.compare(other) <= 0 }

// GreaterOrEqual returns true if v >= other.
func (v Version) GreaterOrEqual(other Version) bool { return v.compare(other) >= 0 }

// String returns the original (unparsed) version string.
func (v Version) String() string { return v.raw }

// IsWildcard returns true if the version is a wildcard ("*") meaning all versions.
func (v Version) IsWildcard() bool { return v.raw == "*" }

// IsZero returns true if the version is the zero value (empty).
func (v Version) IsZero() bool { return v.raw == "" && len(v.parts) == 0 }

// InRange checks if productVersion falls within the given NVD/OSV version range.
// All range boundary strings may be empty (ignored) or "*" (wildcard = all).
//
//   - versionStartIncluding: product >= this  (inclusive lower bound)
//   - versionStartExcluding: product > this   (exclusive lower bound)
//   - versionEndIncluding:   product <= this  (inclusive upper bound)
//   - versionEndExcluding:   product < this   (exclusive upper bound)
func InRange(productVersion, versionStartIncluding, versionStartExcluding, versionEndIncluding, versionEndExcluding string) bool {
	v := Parse(productVersion)

	// Check lower bound
	passesStart := true
	switch {
	case versionStartIncluding != "":
		passesStart = v.GreaterOrEqual(Parse(versionStartIncluding))
	case versionStartExcluding != "":
		passesStart = v.GreaterThan(Parse(versionStartExcluding))
	}
	if !passesStart {
		return false
	}

	// Check upper bound
	switch {
	case versionEndIncluding != "":
		return v.LessOrEqual(Parse(versionEndIncluding))
	case versionEndExcluding != "":
		return v.LessThan(Parse(versionEndExcluding))
	}

	return true
}

// ExactMatch checks if productVersion exactly matches the given version string.
// Used for NVD exact version entries (no range).
func ExactMatch(productVersion, targetVersion string) bool {
	if targetVersion == "*" {
		return true
	}
	return Parse(productVersion).Equal(Parse(targetVersion))
}
