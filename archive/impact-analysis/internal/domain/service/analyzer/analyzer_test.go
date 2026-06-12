package analyzer_test

import (
	"context"
	"sort"
	"testing"

	"github.com/osv/impact-analysis/internal/domain/service/analyzer"
)

// ---- Mock Version Enumerator ----

type mockEnumerator struct {
	// supported ecosystems
	supported map[string]bool
	// known versions: ecosystem/pkg → all versions
	versions map[string][]string
}

func newMockEnumerator(supported []string, versions map[string][]string) *mockEnumerator {
	m := &mockEnumerator{
		supported: make(map[string]bool),
		versions:  versions,
	}
	for _, e := range supported {
		m.supported[e] = true
	}
	return m
}

func (m *mockEnumerator) IsSupported(eco string) bool { return m.supported[eco] }

func (m *mockEnumerator) EnumerateVersions(_ context.Context, eco, pkg, introduced, fixed, lastAffected string, _ []string) ([]string, error) {
	key := eco + "/" + pkg
	all := m.versions[key]
	var result []string
	inRange := introduced == "0" // if introduced==0, start from beginning
	for _, v := range all {
		if v == introduced {
			inRange = true
		}
		if fixed != "" && v == fixed {
			break // stop before fixed (exclusive)
		}
		if inRange {
			result = append(result, v)
		}
		if lastAffected != "" && v == lastAffected {
			break
		}
	}
	return result, nil
}

// ---- Tests ----

func TestAnalyze_EcosystemVersions(t *testing.T) {
	ctx := context.Background()

	knownVersions := map[string][]string{
		"npm/lodash": {"4.0.0", "4.1.0", "4.2.0", "4.17.0", "4.17.20", "4.17.21"},
	}
	ve := newMockEnumerator([]string{"npm"}, knownVersions)
	a := analyzer.NewAnalyzer(ve, nil)

	req := analyzer.AnalyzeRequest{
		VulnID: "GHSA-test-1234",
		Affected: []analyzer.AffectedPackage{
			{
				Name:      "lodash",
				Ecosystem: "npm",
				Ranges: []analyzer.AffectedRange{
					{
						Type: analyzer.RangeTypeEcosystem,
						Events: []analyzer.Event{
							{Introduced: "4.0.0", Fixed: "4.17.21"},
						},
					},
				},
			},
		},
		AnalyzeGit: false,
	}

	result, err := a.Analyze(ctx, req)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !result.HasChanges {
		t.Error("expected HasChanges=true")
	}
	vers := result.NewVersions["npm/lodash"]
	sort.Strings(vers)
	if len(vers) == 0 {
		t.Error("expected versions to be populated")
	}
	// 4.17.21 should NOT be included (it's the fixed version)
	for _, v := range vers {
		if v == "4.17.21" {
			t.Error("fixed version 4.17.21 should not be in affected versions")
		}
	}
}

func TestAnalyze_UnsupportedEcosystem(t *testing.T) {
	ctx := context.Background()
	ve := newMockEnumerator([]string{"npm"}, nil)
	a := analyzer.NewAnalyzer(ve, nil)

	req := analyzer.AnalyzeRequest{
		VulnID: "CVE-2024-0001",
		Affected: []analyzer.AffectedPackage{
			{
				Name:      "some-lib",
				Ecosystem: "unsupported-eco",
				Ranges: []analyzer.AffectedRange{
					{Type: analyzer.RangeTypeEcosystem, Events: []analyzer.Event{{Introduced: "1.0.0"}}},
				},
			},
		},
	}

	result, err := a.Analyze(ctx, req)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.HasChanges {
		t.Error("should not have changes for unsupported ecosystem")
	}
	if len(result.Notes) == 0 {
		t.Error("expected a note for unsupported ecosystem")
	}
}

func TestAnalyze_NoVersionEnumerator(t *testing.T) {
	ctx := context.Background()
	a := analyzer.NewAnalyzer(nil, nil)

	req := analyzer.AnalyzeRequest{
		VulnID: "CVE-2024-0002",
		Affected: []analyzer.AffectedPackage{
			{Name: "pkg", Ecosystem: "npm", Ranges: []analyzer.AffectedRange{
				{Type: analyzer.RangeTypeEcosystem},
			}},
		},
	}
	result, err := a.Analyze(ctx, req)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	// No changes expected when enumerator is nil
	if result.HasChanges {
		t.Error("should not have changes when no version enumerator")
	}
}

func TestAnalyze_GitAnalysisSkippedWhenFlagFalse(t *testing.T) {
	ctx := context.Background()
	a := analyzer.NewAnalyzer(nil, nil)

	req := analyzer.AnalyzeRequest{
		VulnID:     "CVE-2024-0003",
		AnalyzeGit: false,
		Affected: []analyzer.AffectedPackage{
			{
				Name: "test-pkg",
				Ranges: []analyzer.AffectedRange{
					{Type: analyzer.RangeTypeGIT, RepoURL: "https://github.com/test/repo"},
				},
			},
		},
	}
	result, err := a.Analyze(ctx, req)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(result.AffectedCommits) > 0 {
		t.Error("git analysis should be skipped when AnalyzeGit=false")
	}
}

func TestAnalyze_ZeroIntroducedEvent(t *testing.T) {
	ctx := context.Background()

	knownVersions := map[string][]string{
		"pypi/requests": {"2.0.0", "2.1.0", "2.2.0", "2.28.0", "2.29.0"},
	}
	ve := newMockEnumerator([]string{"pypi"}, knownVersions)
	a := analyzer.NewAnalyzer(ve, nil)

	req := analyzer.AnalyzeRequest{
		VulnID: "PYSEC-test",
		Affected: []analyzer.AffectedPackage{
			{
				Name:      "requests",
				Ecosystem: "pypi",
				Ranges: []analyzer.AffectedRange{
					{
						Type: analyzer.RangeTypeEcosystem,
						Events: []analyzer.Event{
							{Introduced: "0", Fixed: "2.28.0"}, // all versions before 2.28
						},
					},
				},
			},
		},
	}
	result, err := a.Analyze(ctx, req)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	_ = result // just checking it doesn't panic with "0" introduced
}

func TestIsSemverEcosystem(t *testing.T) {
	tests := []struct {
		eco      string
		expected bool
	}{
		{"npm", true},
		{"cargo", true},
		{"go", true},
		{"golang", true},
		{"pypi", false},
		{"maven", false},
		{"npm", true},
	}
	for _, tt := range tests {
		got := analyzer.IsSemverEcosystem(tt.eco)
		if got != tt.expected {
			t.Errorf("IsSemverEcosystem(%q) = %v, want %v", tt.eco, got, tt.expected)
		}
	}
}

func TestAnalyze_EmptyAffected(t *testing.T) {
	ctx := context.Background()
	a := analyzer.NewAnalyzer(nil, nil)
	result, err := a.Analyze(ctx, analyzer.AnalyzeRequest{VulnID: "CVE-X"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.HasChanges {
		t.Error("empty affected should have no changes")
	}
}
