// Package models — Unit tests for Bug struct and ComputeIndexes.
package models

import (
	"testing"
	"time"

	"github.com/ossf/osv-schema/bindings/go/osvschema"
)

func TestComputeIndexes_Basic(t *testing.T) {
	b := &Bug{
		DBID:   "CVE-2023-44487",
		Source: "cve",
		Status: BugStatusProcessed,
		AffectedPackages: []AffectedPackage{
			{
				Package:   PackageRef{Name: "nghttp2", Ecosystem: "Go"},
				Ecosystem: "Go",
				PURL:      "pkg:golang/golang.org/x/net@0.18.0",
				Versions:  []string{"0.17.0", "0.16.0"},
				Ranges: []VersionRange{
					{
						Type: "SEMVER",
						Events: []RangeEvent{
							{Introduced: "0.0.0"},
							{Fixed: "0.18.0"},
						},
					},
				},
			},
		},
		Withdrawn: nil,
	}

	ComputeIndexes(b)

	if !b.HasAffected {
		t.Error("expected HasAffected=true")
	}
	if !b.IsFixed {
		t.Error("expected IsFixed=true")
	}
	if !b.IsPublic {
		t.Error("expected IsPublic=true")
	}
	if len(b.Ecosystem) != 1 || b.Ecosystem[0] != "Go" {
		t.Errorf("unexpected ecosystems: %v", b.Ecosystem)
	}
	if len(b.Project) != 1 || b.Project[0] != "nghttp2" {
		t.Errorf("unexpected project: %v", b.Project)
	}
	if len(b.SemverFixed) != 1 || b.SemverFixed[0] != "0.18.0" {
		t.Errorf("unexpected semver fixed: %v", b.SemverFixed)
	}
	// Tokens: "cve", "2023", "44487", "nghttp2"
	tokenMap := make(map[string]bool)
	for _, tok := range b.SearchIndices {
		tokenMap[tok] = true
	}
	for _, expected := range []string{"cve", "2023", "44487", "nghttp2"} {
		if !tokenMap[expected] {
			t.Errorf("expected token %q in SearchIndices, got: %v", expected, b.SearchIndices)
		}
	}
}

func TestComputeIndexes_Withdrawn(t *testing.T) {
	now := time.Now()
	b := &Bug{
		DBID:      "GHSA-xxx-yyy-zzz",
		Status:    BugStatusProcessed,
		Withdrawn: &now,
	}
	ComputeIndexes(b)
	if b.IsPublic {
		t.Error("withdrawn bug should not be IsPublic")
	}
}

func TestConvertToOSVSchema(t *testing.T) {
	b := &Bug{
		DBID:        "CVE-2023-44487",
		Summary:     "HTTP/2 Rapid Reset Attack",
		Details:     "Some details.",
		Timestamp:   time.Date(2023, 10, 10, 0, 0, 0, 0, time.UTC),
		LastModified: time.Date(2023, 10, 15, 0, 0, 0, 0, time.UTC),
		CVSSVector:  "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:N/A:H",
		Aliases:     []string{"GHSA-qppq-h3kr-g3sv"},
		AffectedPackages: []AffectedPackage{
			{
				Package:   PackageRef{Name: "golang.org/x/net", Ecosystem: "Go"},
				Ecosystem: "Go",
				Versions:  []string{"0.17.0"},
				Ranges: []VersionRange{
					{
						Type:   "SEMVER",
						Events: []RangeEvent{{Introduced: "0.0.0"}, {Fixed: "0.18.0"}},
					},
				},
			},
		},
	}

	vuln := ConvertToOSVSchema(b)

	if vuln.Id != "CVE-2023-44487" {
		t.Errorf("ID mismatch: %s", vuln.Id)
	}
	if vuln.Summary != b.Summary {
		t.Errorf("Summary mismatch: %s", vuln.Summary)
	}
	if len(vuln.Affected) != 1 {
		t.Errorf("expected 1 affected, got %d", len(vuln.Affected))
	}
	if len(vuln.Severity) != 1 {
		t.Errorf("expected 1 severity, got %d", len(vuln.Severity))
	}
}

func TestUpdateFromOSVSchema_RoundTrip(t *testing.T) {
	original := &Bug{
		DBID:   "CVE-2023-44487",
		Status: BugStatusProcessed,
		Summary: "HTTP/2 Rapid Reset",
		AffectedPackages: []AffectedPackage{
			{
				Package:   PackageRef{Name: "nghttp2", Ecosystem: "Go"},
				Ecosystem: "Go",
				Versions:  []string{"0.17.0"},
				Ranges: []VersionRange{
					{Type: "SEMVER", Events: []RangeEvent{{Fixed: "0.18.0"}}},
				},
			},
		},
	}
	ComputeIndexes(original)

	// Convert to OSV and back
	vuln := ConvertToOSVSchema(original)

	recovered := &Bug{Status: BugStatusProcessed}
	UpdateFromOSVSchema(recovered, vuln)

	if recovered.DBID != original.DBID {
		t.Errorf("DBID mismatch: got %s, want %s", recovered.DBID, original.DBID)
	}
	if len(recovered.AffectedPackages) != len(original.AffectedPackages) {
		t.Errorf("AffectedPackages count mismatch")
	}
	if !recovered.IsFixed {
		t.Error("recovered bug should be IsFixed")
	}
}

func TestTokenize(t *testing.T) {
	cases := []struct {
		input    string
		expected []string
	}{
		{"CVE-2023-44487", []string{"cve", "2023", "44487"}},
		{"golang.org/x/net", []string{"golang", "org", "x", "net"}},
		{"GHSA-qppq-h3kr-g3sv", []string{"ghsa", "qppq", "h3kr", "g3sv"}},
	}
	for _, tc := range cases {
		got := tokenize(tc.input)
		if len(got) != len(tc.expected) {
			t.Errorf("tokenize(%q): got %v, want %v", tc.input, got, tc.expected)
			continue
		}
		for i, tok := range got {
			if tok != tc.expected[i] {
				t.Errorf("tokenize(%q)[%d]: got %q, want %q", tc.input, i, tok, tc.expected[i])
			}
		}
	}
}

func TestAliasGroup(t *testing.T) {
	g := &AliasGroup{
		BugID:   "CVE-2023-44487",
		Members: []string{"CVE-2023-44487"},
	}

	added := g.AddMember("GHSA-xxx-yyy-zzz")
	if !added {
		t.Error("expected AddMember to return true")
	}
	if len(g.Members) != 2 {
		t.Errorf("expected 2 members, got %d", len(g.Members))
	}

	// Add duplicate — should not add
	added = g.AddMember("GHSA-xxx-yyy-zzz")
	if added {
		t.Error("expected AddMember duplicate to return false")
	}
	if len(g.Members) != 2 {
		t.Error("duplicate should not increase member count")
	}

	if !g.Contains("CVE-2023-44487") {
		t.Error("expected Contains to find CVE-2023-44487")
	}
	g.RemoveMember("GHSA-xxx-yyy-zzz")
	if g.Contains("GHSA-xxx-yyy-zzz") {
		t.Error("expected removed member to not be found")
	}
}

// Ensure Bug types imported from osvschema compile correctly.
var _ = &osvschema.Vulnerability{}
