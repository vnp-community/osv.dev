package rangecollector_test

import (
	"testing"

	"github.com/osv/impact-service/internal/domain/impact/rangecollector"
)

func TestRangeCollectorBasic(t *testing.T) {
	rc := rangecollector.New()

	// Add a single range
	rc.Add("abc123", "def456", "")
	ranges := rc.Ranges()
	if len(ranges) != 1 {
		t.Fatalf("expected 1 range, got %d", len(ranges))
	}
	r := ranges[0]
	if r.Introduced != "abc123" || r.Fixed != "def456" {
		t.Errorf("unexpected range: %+v", r)
	}
}

func TestFixedSupersedesLastAffected(t *testing.T) {
	rc := rangecollector.New()
	// First add with lastAffected only
	rc.Add("intro1", "", "last1")
	// Now add with fixed — should supersede the lastAffected
	rc.Add("intro1", "fix1", "")
	ranges := rc.Ranges()
	for _, r := range ranges {
		if r.LastAffected != "" && r.Fixed != "" {
			t.Errorf("should not have both fixed and lastAffected: %+v", r)
		}
	}
	// There should be at most 1 effective range for intro1
	hasFixed := false
	for _, r := range ranges {
		if r.Fixed == "fix1" {
			hasFixed = true
		}
	}
	if !hasFixed {
		t.Error("expected fixed range to be present after superseding lastAffected")
	}
}

func TestOpenEndedRangeDedup(t *testing.T) {
	rc := rangecollector.New()
	rc.Add("intro1", "", "") // open-ended
	rc.Add("intro2", "fix2", "")
	ranges := rc.Ranges()
	// intro1 open-ended should be kept (still vulnerable)
	// intro2 should have a fix
	foundOpen := false
	foundFixed := false
	for _, r := range ranges {
		if r.Introduced == "intro1" && r.Fixed == "" && r.LastAffected == "" {
			foundOpen = true
		}
		if r.Introduced == "intro2" && r.Fixed == "fix2" {
			foundFixed = true
		}
	}
	if !foundOpen {
		t.Error("expected open-ended range for intro1")
	}
	if !foundFixed {
		t.Error("expected fixed range for intro2")
	}
}

func TestFixedAndLastAffectedMutuallyExclusive(t *testing.T) {
	rc := rangecollector.New()
	// Python: if fixed_in and affected_in: affected_in = None
	rc.Add("intro1", "fix1", "last1") // lastAffected should be dropped
	ranges := rc.Ranges()
	if len(ranges) == 0 {
		t.Fatal("expected at least one range")
	}
	for _, r := range ranges {
		if r.LastAffected != "" {
			t.Errorf("lastAffected should be cleared when fixed is provided: %+v", r)
		}
	}
}

func TestInsertionOrderPreserved(t *testing.T) {
	rc := rangecollector.New()
	rc.Add("c1", "f1", "")
	rc.Add("c2", "f2", "")
	rc.Add("c3", "f3", "")
	ranges := rc.Ranges()
	if len(ranges) < 3 {
		t.Fatalf("expected 3 ranges, got %d", len(ranges))
	}
	if ranges[0].Introduced != "c1" || ranges[1].Introduced != "c2" || ranges[2].Introduced != "c3" {
		t.Error("insertion order not preserved")
	}
}

func TestDeduplication(t *testing.T) {
	rc := rangecollector.New()
	rc.Add("c1", "f1", "")
	rc.Add("c1", "f1", "") // exact duplicate
	ranges := rc.Ranges()
	count := 0
	for _, r := range ranges {
		if r.Introduced == "c1" && r.Fixed == "f1" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected dedup to 1 range, got %d", count)
	}
}

func TestEmptyCollector(t *testing.T) {
	rc := rangecollector.New()
	ranges := rc.Ranges()
	if len(ranges) != 0 {
		t.Errorf("expected 0 ranges for empty collector, got %d", len(ranges))
	}
}
