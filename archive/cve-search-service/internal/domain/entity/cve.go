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
	SortNewest SortOrder = "newest"
	SortOldest SortOrder = "oldest"
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
	IsKEV       bool      `json:"kev"`
	Link        string    `json:"link,omitempty"`
	CVSSScore   *float64  `json:"cvss,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SearchFilter holds all search parameters.
type SearchFilter struct {
	Query    string
	Severity *Severity
	Source   *Source
	Sort     SortOrder
	Page     int
	Limit    int
}

// Validate clamps and defaults filter values.
func (f *SearchFilter) Validate() {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 50
	}
	if f.Page < 0 {
		f.Page = 0
	}
	if f.Sort != SortOldest {
		f.Sort = SortNewest
	}
}
