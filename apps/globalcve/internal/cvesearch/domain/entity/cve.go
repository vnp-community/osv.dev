// Package entity contains the core CVE domain entity for GlobalCVE.
// Adapted from vulnerability-service/internal/domain/entity/cve.go.
package entity

import (
	"regexp"
	"time"
)

var cveIDPattern = regexp.MustCompile(`^CVE-\d{4}-\d{4,}$`)

// Severity represents the severity level of a vulnerability.
type Severity string

const (
	SeverityCritical Severity = "CRITICAL"
	SeverityHigh     Severity = "HIGH"
	SeverityMedium   Severity = "MEDIUM"
	SeverityLow      Severity = "LOW"
	SeverityUnknown  Severity = "UNKNOWN"
)

// SeverityFromCVSS3 derives severity from a CVSS v3 score.
func SeverityFromCVSS3(score float64) Severity {
	switch {
	case score >= 9.0:
		return SeverityCritical
	case score >= 7.0:
		return SeverityHigh
	case score >= 4.0:
		return SeverityMedium
	case score > 0:
		return SeverityLow
	default:
		return SeverityUnknown
	}
}

// Source is the origin data source identifier.
type Source string

const (
	SourceNVD       Source = "NVD"
	SourceCIRCL     Source = "CIRCL"
	SourceJVN       Source = "JVN"
	SourceExploitDB Source = "EXPLOITDB"
	SourceCVEOrg    Source = "CVE.ORG"
	SourceArchive   Source = "ARCHIVE"
)

// SourceName is a typed identifier used by the sync service to name data sources.
// It is the same string as Source but used in sync job tracking and scheduler config.
type SourceName = Source // type alias — same underlying string

const (
	SourceNameNVD       SourceName = SourceNVD
	SourceNameCIRCL     SourceName = SourceCIRCL
	SourceNameJVN       SourceName = SourceJVN
	SourceNameExploitDB SourceName = SourceExploitDB
	SourceNameCVEOrg    SourceName = SourceCVEOrg
	SourceNameEPSS      SourceName = "EPSS"
	SourceNameNVDCPE    SourceName = "NVD_CPE"
	SourceNameCAPEC     SourceName = "CAPEC"
	SourceNameCWE       SourceName = "CWE"
)

// CVE is the unified vulnerability record for GlobalCVE.
type CVE struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Summary     string    `json:"summary"`
	Severity    Severity  `json:"severity"`
	Published   time.Time `json:"published"`
	Modified    time.Time `json:"modified"`
	Source      Source    `json:"source"`
	IsKEV       bool      `json:"kev"`
	Link        string    `json:"link,omitempty"`

	// CVSS Scores
	CVSSScore  *float64 `json:"cvss,omitempty"`
	CVSS3Score *float64 `json:"cvss3,omitempty"`
	CVSSVector  string   `json:"cvss_vector,omitempty"`
	CVSS3Vector string   `json:"cvss3_vector,omitempty"`

	// EPSS (Exploit Prediction Scoring System)
	EPSS           *float64 `json:"epss,omitempty"`
	EPSSPercentile *float64 `json:"epss_percentile,omitempty"`

	// Enrichment
	Vendors  []string `json:"vendors,omitempty"`
	Products []string `json:"products,omitempty"`
	CWE      []string `json:"cwe,omitempty"`
	References []string `json:"references,omitempty"`

	// pgvector embedding for semantic search (not serialized to JSON)
	Embedding []float32 `json:"-"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// IsValidID returns true if the string matches CVE-YYYY-NNNN+ format.
func IsValidID(id string) bool { return cveIDPattern.MatchString(id) }

// SortOrder defines the sort order for CVE queries.
type SortOrder string

const (
	SortNewest SortOrder = "newest"    // published DESC
	SortOldest SortOrder = "oldest"    // published ASC
	SortCVSS   SortOrder = "cvss_desc" // cvss3_score DESC
	SortEPSS   SortOrder = "epss_desc" // epss DESC
)

// SearchFilter holds all query parameters for CVE search.
type SearchFilter struct {
	Query    string
	Severity *Severity
	Source   *Source
	Sort     SortOrder
	Page     int
	Limit    int
	IsKEV    *bool
	MinEPSS  *float64
}

// Validate clamps and defaults filter values.
func (f *SearchFilter) Validate() {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 50
	}
	if f.Page < 0 {
		f.Page = 0
	}
	if f.Sort == "" {
		f.Sort = SortNewest
	}
}
