// Package dedup defines the domain entities and interface for the deduplication engine.
package dedup

// DedupAlgorithm is the strategy used for identifying duplicate findings.
type DedupAlgorithm string

const (
	// DedupAlgorithmHashCode uses SHA-256 of severity+title+cwe+description+endpoints.
	DedupAlgorithmHashCode DedupAlgorithm = "hash_code"
	// DedupAlgorithmUniqueID uses the scanner-provided unique_id_from_tool (e.g. Snyk).
	DedupAlgorithmUniqueID DedupAlgorithm = "unique_id_from_tool"
	// DedupAlgorithmLegacy uses endpoint URL + CWE + title matching (DefectDojo v1 compat).
	DedupAlgorithmLegacy DedupAlgorithm = "legacy"
)

// DedupContext carries configuration and scope for a deduplication run.
type DedupContext struct {
	TestID       string
	EngagementID string
	ProductID    string
	// OnEngagement=true scopes dedup to the engagement level (not just test).
	OnEngagement bool
	// Algorithm overrides the default algorithm for this import.
	Algorithm DedupAlgorithm
	// FalsePositiveHistory auto-marks new findings as FP if their hash matches a known FP.
	FalsePositiveHistory bool
	// MaxDuplicates is the maximum number of duplicate references kept (default 10).
	MaxDuplicates int
	// DeleteDuplicates deletes the oldest duplicate when MaxDuplicates is exceeded.
	DeleteDuplicates bool
}

// ParsedFindingRef is a reference to a parsed finding used in dedup results.
type ParsedFindingRef struct {
	Title          string
	HashCode       string
	VulnIDFromTool string
	FalsePositive  bool
}

// ReactivatedFinding pairs a parsed finding with the ID of its existing record being reactivated.
type ReactivatedFinding struct {
	HashCode   string
	ExistingID string
}

// ExistingFinding pairs a parsed finding with an already-active existing record.
type ExistingFinding struct {
	HashCode   string
	ExistingID string
}

// DedupResult classifies each finding from the current scan into one of four categories.
type DedupResult struct {
	// NewFindings: findings with no existing record → will be created.
	NewFindings []string // hash codes
	// Reactivated: findings that were mitigated and are now seen again → will be reactivated.
	Reactivated []ReactivatedFinding
	// Untouched: findings that already exist and are active → no action.
	Untouched []ExistingFinding
	// Duplicates: findings whose original is FP/OOS/RA → marked as duplicate.
	Duplicates []ExistingFinding
}
