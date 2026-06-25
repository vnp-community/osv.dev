// Package entity contains CVE domain entities for the search service.
package entity

import (
	"regexp"
	"strings"
	"time"
)

// Severity levels.
type Severity string

const (
	SeverityCritical Severity = "CRITICAL"
	SeverityHigh     Severity = "HIGH"
	SeverityMedium   Severity = "MEDIUM"
	SeverityLow      Severity = "LOW"
	SeverityUnknown  Severity = "UNKNOWN"
)

// Source data providers.
type Source string

const (
	SourceNVD       Source = "NVD"
	SourceCIRCL     Source = "CIRCL"
	SourceJVN       Source = "JVN"
	SourceExploitDB Source = "EXPLOITDB"
	SourceCVEOrg    Source = "CVE.ORG"
	SourceArchive   Source = "ARCHIVE"
)

// SortOrder controls result ordering.
type SortOrder string

const (
	SortNewest   SortOrder = "newest"
	SortOldest   SortOrder = "oldest"
	SortEPSSDesc SortOrder = "epss_desc" // CR-GCV-002: sort by EPSS score descending
	SortCVSS3    SortOrder = "cvss3_desc" // sort by CVSS3 score descending
)

var cvePattern = regexp.MustCompile(`^CVE-\d{4}-\d{4,}$`)

// IsValidID reports whether s is a syntactically valid CVE ID.
func IsValidID(id string) bool {
	return cvePattern.MatchString(strings.ToUpper(strings.TrimSpace(id)))
}

// CVE is the read model for CVE search results.
type CVE struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Severity    Severity  `json:"severity"`
	Published   time.Time `json:"published"`
	Source      Source    `json:"source"`
	IsKEV       bool      `json:"is_kev"`
	IsExploit   bool      `json:"is_exploit,omitempty"` // CR-GCV-003
	Link        string    `json:"link,omitempty"`
	CVSSScore   *float64  `json:"cvss,omitempty"`
	CVSS3Score  *float64  `json:"cvss3,omitempty"`
	EPSS        *float64  `json:"epss,omitempty"`        // CR-GCV-002
	EPSSPct     *float64  `json:"epss_percentile,omitempty"` // CR-GCV-002
	Vendors     []string  `db:"vendors" json:"vendors,omitempty"` // CR-GCV-005
	Products    []string  `db:"products" json:"products,omitempty"` // CR-GCV-005
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SearchFilter holds all search parameters.
type SearchFilter struct {
	Query     string
	Severity  *Severity
	Source    *Source
	Sort      SortOrder
	Page      int
	Limit     int
	// CR-GCV-002: EPSS filter
	MinEPSS   *float64
	MaxEPSS   *float64
	// CR-GCV-003: Exploit/KEV filter
	IsKEV     *bool
	IsExploit *bool
	// CR-GCV-005: CWE/Vendor/Product filter
	CWE       string
	Vendor    string
	Product   string
}

// Validate clamps and defaults filter values.
func (f *SearchFilter) Validate() {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 50
	}
	if f.Page < 0 {
		f.Page = 0
	}
	switch f.Sort {
	case SortOldest, SortEPSSDesc, SortCVSS3:
		// valid
	default:
		f.Sort = SortNewest
	}
}
