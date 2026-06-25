// Package importdomain defines domain entities for the scan import pipeline.
package importdomain

import (
	"io"
	"time"
)

// Severity defines vulnerability severity levels.
type Severity string

const (
	SeverityCritical Severity = "Critical"
	SeverityHigh     Severity = "High"
	SeverityMedium   Severity = "Medium"
	SeverityLow      Severity = "Low"
	SeverityInfo     Severity = "Info"
)

// SeverityOrder maps severity values to numeric ranks for comparison.
var SeverityOrder = map[Severity]int{
	SeverityCritical: 5,
	SeverityHigh:     4,
	SeverityMedium:   3,
	SeverityLow:      2,
	SeverityInfo:     1,
}

// GroupByMode controls how findings are grouped when creating FindingGroups.
type GroupByMode string

const (
	GroupByNone          GroupByMode = ""
	GroupByComponentName GroupByMode = "component_name"
	GroupByFilePath      GroupByMode = "file_path"
	GroupByFindingTitle  GroupByMode = "finding_title"
)

// ImportOptions defines all parameters for a scan import operation.
// Zero values are safe defaults: no filtering, no auto-create, no tag application.
type ImportOptions struct {
	ScanType string
	File     io.Reader
	Filename string

	// Context identifiers (at least one of TestID or ProductID required when AutoCreateContext=false)
	TestID       *string
	EngagementID *string
	ProductID    *string

	// Auto-create context (Product/Engagement/Test)
	AutoCreateContext bool
	ProductName       string // used when auto-creating product
	EngagementName    string // used when auto-creating engagement
	TestTitle         string
	EngagementType    string // "Interactive" | "CI/CD"

	// Filtering
	Active              bool
	Verified            bool
	MinimumSeverity     Severity // filter findings below this severity
	CloseOldFindings    bool     // mitigate findings not in current scan
	CloseOldFindingsProductScope bool // close scope: true=product, false=test
	DoNotReactivate     bool     // if false, reactivate mitigated findings re-found

	// Deduplication
	DeduplicationOnEngagement bool

	// Metadata
	Version    string
	BuildID    string
	CommitHash string
	BranchTag  string
	Service    string

	// Grouping
	GroupBy                          GroupByMode
	CreateFindingGroupsForAllFindings bool

	// Tags
	Tags                []string
	ApplyTagsToFindings bool

	// Auth
	RequestorUserID string
}

// ImportResult summarizes what happened during a scan import.
type ImportResult struct {
	TestID        string
	TestImportID  string
	TotalFindings int
	NewFindings   int
	Closed        int
	Reactivated   int
	Untouched     int
	ScanType      string
	ImportType    string // "import" | "reimport"
}

// TestImport records the history of each scan import operation.
// Persisted to the import_histories table.
type TestImport struct {
	ID         string
	TestID     string
	ImportType string // "import" | "reimport"
	Version    string
	BranchTag  string
	BuildID    string
	CommitHash string

	NewFindings    int
	ClosedFindings int
	Reactivated    int
	Untouched      int

	ScanFileKey    string                 // MinIO object key
	ImportSettings map[string]interface{} // JSON blob of ImportOptions
	CreatedAt      time.Time
}
