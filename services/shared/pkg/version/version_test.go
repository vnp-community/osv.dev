package version_test

import (
	"testing"

	"github.com/osv/shared/pkg/version"
)

func TestVersionCompare(t *testing.T) {
	tests := []struct {
		a, b   string
		expect int // -1: a<b, 0: a==b, 1: a>b
	}{
		// Numeric comparisons
		{"1.0.0", "1.0.1", -1},
		{"1.2.3", "1.2.3", 0},
		{"2.0.0", "1.9.9", 1},
		{"10.0", "9.9", 1},

		// OpenSSL-style patch suffix: letter AFTER numeric = patch release
		// In NVD CPE matching, 1.1.1k means the 11th patch of 1.1.1
		{"1.1.1k", "1.1.1j", 1},  // k > j (11th > 10th patch)
		{"1.1.1a", "1.1.1", 1},   // 1.1.1a is AFTER 1.1.1 (first patch)
		{"1.1.1", "1.1.1a", -1},  // base < patched
		{"1.1.1k", "1.1.1", 1},   // 1.1.1k is after 1.1.1

		// Pre-release with separator words (normalized to dot)
		// "beta", "rc", "pre" as separate tokens are pre-release keywords
		// They appear as standalone alpha tokens: [1,2,3,"beta"] vs [1,2,3]
		// The alpha part vs missing is: alpha > missing (patch, not pre-release)
		// This is consistent with NVD behavior where version strings are just compared

		// Version with zero padding
		{"1.0.0", "1.0", 0},
		{"1.2.3.0", "1.2.3", 0},

		// Empty and wildcard
		{"", "1.0", -1},
	}

	for _, tt := range tests {
		a := version.Parse(tt.a)
		b := version.Parse(tt.b)
		var got int
		switch {
		case a.LessThan(b):
			got = -1
		case a.Equal(b):
			got = 0
		default:
			got = 1
		}
		if got != tt.expect {
			t.Errorf("Compare(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.expect)
		}
	}
}

func TestInRange(t *testing.T) {
	tests := []struct {
		version            string
		startInc, startExc string
		endInc, endExc     string
		expect             bool
	}{
		// Within range [1.0.0, 2.0.0)
		{"1.5.0", "1.0.0", "", "", "2.0.0", true},
		{"2.0.0", "1.0.0", "", "", "2.0.0", false},  // exclusive end
		{"1.0.0", "1.0.0", "", "", "2.0.0", true},   // inclusive start

		// Within range (1.0.0, 2.0.0]
		{"1.0.0", "", "1.0.0", "2.0.0", "", false},  // exclusive start
		{"2.0.0", "", "1.0.0", "2.0.0", "", true},   // inclusive end

		// OpenSSL-style: range inclusive of patch release
		// CVE affects [1.1.1, 1.1.1k] (inclusive end)
		{"1.1.1k", "1.1.1", "", "1.1.1k", "", true},  // at end boundary
		{"1.1.1l", "1.1.1", "", "1.1.1k", "", false}, // past end boundary

		// No bounds = all versions
		{"9.9.9", "", "", "", "", true},
	}

	for _, tt := range tests {
		got := version.InRange(tt.version, tt.startInc, tt.startExc, tt.endInc, tt.endExc)
		if got != tt.expect {
			t.Errorf("InRange(%q, %q, %q, %q, %q) = %v, want %v",
				tt.version, tt.startInc, tt.startExc, tt.endInc, tt.endExc, got, tt.expect)
		}
	}
}

func TestExactMatch(t *testing.T) {
	if !version.ExactMatch("1.2.3", "1.2.3") {
		t.Error("exact match failed")
	}
	if !version.ExactMatch("anything", "*") {
		t.Error("wildcard match failed")
	}
	if version.ExactMatch("1.2.3", "1.2.4") {
		t.Error("should not match different versions")
	}
}
