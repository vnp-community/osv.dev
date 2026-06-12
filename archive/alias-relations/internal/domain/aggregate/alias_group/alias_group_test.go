// domain/aggregate/alias_group/alias_group_test.go
package aliasgroup_test

import (
	"testing"

	aliasgroup "github.com/osv/alias-relations/internal/domain/aggregate/alias_group"
	"github.com/osv/alias-relations/internal/domain/valueobject"
)

func TestNewAliasGroup_SetsIDs(t *testing.T) {
	g := aliasgroup.NewAliasGroup(
		[]string{"CVE-2021-44228", "GHSA-jfh8-c2jp-5v3q"},
		valueobject.DetectionSourceDeclared,
	)
	ids := g.BugIDs()
	if len(ids) != 2 {
		t.Fatalf("expected 2 IDs, got %d", len(ids))
	}
}

func TestAddID_Idempotent(t *testing.T) {
	g := aliasgroup.NewAliasGroup([]string{"CVE-2021-44228"}, valueobject.DetectionSourceDeclared)
	added := g.AddID("CVE-2021-44228")
	if added {
		t.Error("AddID should return false for existing ID")
	}
	if g.Size() != 1 {
		t.Errorf("size should still be 1, got %d", g.Size())
	}
}

func TestAddID_NewID(t *testing.T) {
	g := aliasgroup.NewAliasGroup([]string{"CVE-2021-44228"}, valueobject.DetectionSourceDeclared)
	added := g.AddID("GHSA-jfh8-c2jp-5v3q")
	if !added {
		t.Error("AddID should return true for new ID")
	}
	if g.Size() != 2 {
		t.Errorf("expected size 2, got %d", g.Size())
	}
}

func TestCanonicalID_CVEPriority(t *testing.T) {
	g := aliasgroup.NewAliasGroup(
		[]string{"GHSA-abc", "CVE-2021-44228", "OSV-2021-001"},
		valueobject.DetectionSourceDeclared,
	)
	if g.CanonicalID() != "CVE-2021-44228" {
		t.Errorf("expected CVE-2021-44228 as canonical, got %s", g.CanonicalID())
	}
}

func TestCanonicalID_GHSAWhenNoCVE(t *testing.T) {
	g := aliasgroup.NewAliasGroup(
		[]string{"GHSA-abc", "OSV-2021-001"},
		valueobject.DetectionSourceDeclared,
	)
	if g.CanonicalID() != "GHSA-abc" {
		t.Errorf("expected GHSA-abc as canonical, got %s", g.CanonicalID())
	}
}

func TestCanonicalID_AlphabeticalFallback(t *testing.T) {
	g := aliasgroup.NewAliasGroup(
		[]string{"PYSEC-2021-001", "RUSTSEC-2021-001"},
		valueobject.DetectionSourceDeclared,
	)
	// alphabetically first should be returned
	canonical := g.CanonicalID()
	if canonical != "PYSEC-2021-001" {
		t.Errorf("expected PYSEC-2021-001, got %s", canonical)
	}
}

func TestMerge_UnionOfIDs(t *testing.T) {
	g1 := aliasgroup.NewAliasGroup([]string{"CVE-2021-44228", "GHSA-abc"}, valueobject.DetectionSourceDeclared)
	g2 := aliasgroup.NewAliasGroup([]string{"GHSA-abc", "PYSEC-001"}, valueobject.DetectionSourceDeclared)

	g1.Merge(g2)
	ids := g1.BugIDs()
	if len(ids) != 3 {
		t.Errorf("expected 3 unique IDs after merge, got %d: %v", len(ids), ids)
	}
}

func TestMerge_Idempotent(t *testing.T) {
	g1 := aliasgroup.NewAliasGroup([]string{"CVE-2021-44228"}, valueobject.DetectionSourceDeclared)
	g2 := aliasgroup.NewAliasGroup([]string{"CVE-2021-44228"}, valueobject.DetectionSourceDeclared)
	// Drain events first
	_ = g1.Events()
	g1.Merge(g2)
	// No change, no new events
	evts := g1.Events()
	if len(evts) != 0 {
		t.Errorf("expected 0 events for no-op merge, got %d", len(evts))
	}
}

func TestEvents_EmptyAfterDrain(t *testing.T) {
	g := aliasgroup.NewAliasGroup([]string{"CVE-2021-44228"}, valueobject.DetectionSourceDeclared)
	evts1 := g.Events()
	if len(evts1) == 0 {
		t.Error("expected events after creation")
	}
	evts2 := g.Events()
	if len(evts2) != 0 {
		t.Error("expected empty events after drain")
	}
}

func TestBugIDs_Sorted(t *testing.T) {
	g := aliasgroup.NewAliasGroup(
		[]string{"Z-999", "A-001", "M-500"},
		valueobject.DetectionSourceDeclared,
	)
	ids := g.BugIDs()
	for i := 1; i < len(ids); i++ {
		if ids[i] < ids[i-1] {
			t.Errorf("BugIDs not sorted at index %d: %v", i, ids)
		}
	}
}
