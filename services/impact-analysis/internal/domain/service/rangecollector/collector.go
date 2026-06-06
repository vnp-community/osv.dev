// Package rangecollector ports the RangeCollector from osv/impact.py.
// TASK-10-01: Preserve insertion-order commit ranges, deduplicating by introduced commit.
package rangecollector

// CommitRange represents a (introduced, fixed, lastAffected) tuple.
type CommitRange struct {
	Introduced   string // may be ""
	Fixed        string // may be ""
	LastAffected string // may be ""
}

// RangeCollector collects affected commit ranges and deduplicates them.
// Port of osv/impact.py:RangeCollector — preserves insertion order.
type RangeCollector struct {
	// grouped: introduced-commit → ordered list of CommitRange
	grouped map[string][]CommitRange
	// insertOrder tracks the order of first occurrence per introduced key
	insertOrder []string
}

// New creates an empty RangeCollector.
func New() *RangeCollector {
	return &RangeCollector{
		grouped: make(map[string][]CommitRange),
	}
}

// Add records a new commit range tuple.
// Mirrors osv/impact.py:RangeCollector.add():
//   - If fixed is provided, lastAffected is discarded (fixed takes precedence).
//   - Duplicate "open-ended" ranges (no fixed, no lastAffected) are suppressed.
//   - lastAffected ranges are promoted to fixed ranges when a fixed commit arrives.
func (rc *RangeCollector) Add(introduced, fixed, lastAffected string) {
	// last_affected is redundant if fixed is available (port of Python logic)
	if fixed != "" && lastAffected != "" {
		lastAffected = ""
	}

	existing, exists := rc.grouped[introduced]
	if !exists {
		// First time seeing this introduced commit
		rc.grouped[introduced] = []CommitRange{{introduced, fixed, lastAffected}}
		rc.insertOrder = append(rc.insertOrder, introduced)
		return
	}

	// Append the new range to the existing group
	existing = append(existing, CommitRange{introduced, fixed, lastAffected})

	// Filter: remove entries that are now redundant
	var kept []CommitRange
	for _, r := range existing {
		// Remove open-ended duplicates (no fixed, no lastAffected)
		if r.Fixed == "" && r.LastAffected == "" {
			// Only keep it if this is the only entry (open-ended = "still vulnerable")
			// Since we just added a new one, any open-ended existing entries can be dropped
			// if we now have a fixed or lastAffected.
			hasFixed := false
			for _, other := range existing {
				if other.Fixed != "" || other.LastAffected != "" {
					hasFixed = true
					break
				}
			}
			if hasFixed {
				continue // drop this open-ended entry
			}
		}
		// Remove lastAffected entries that are superseded by a fixed entry
		if fixed != "" && r.Fixed == "" && r.LastAffected != "" {
			continue // promoted: fixed takes precedence
		}
		kept = append(kept, r)
	}

	rc.grouped[introduced] = kept
}

// Ranges returns all collected commit ranges in insertion order, deduplicated.
func (rc *RangeCollector) Ranges() []CommitRange {
	seen := make(map[CommitRange]bool)
	var result []CommitRange

	for _, introduced := range rc.insertOrder {
		for _, r := range rc.grouped[introduced] {
			if !seen[r] {
				seen[r] = true
				result = append(result, r)
			}
		}
	}
	return result
}
