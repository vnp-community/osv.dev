// Package entity contains the KEV domain entity.
// Adapted from vulnerability-service/internal/domain/kev/kev.go.
package entity

import "time"

// KEVEntry represents a single entry in the CISA Known Exploited Vulnerabilities catalog.
type KEVEntry struct {
	CVEID             string    `json:"cveID"`
	VendorProject     string    `json:"vendorProject"`
	Product           string    `json:"product"`
	VulnerabilityName string    `json:"vulnerabilityName"`
	ShortDescription  string    `json:"shortDescription"`
	RequiredAction    string    `json:"requiredAction"`
	DateAdded         time.Time `json:"dateAdded"`
	DueDate           time.Time `json:"dueDate"`
	KnownRansomware   bool      `json:"knownRansomwareCampaignUse"`
	Notes             string    `json:"notes"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// KEVFilter holds query parameters for listing KEV entries.
type KEVFilter struct {
	Query         string
	VendorProject string
	Since         *time.Time
	Page          int
	Limit         int
}

// Validate clamps and defaults filter values.
func (f *KEVFilter) Validate() {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 50
	}
	if f.Page < 0 {
		f.Page = 0
	}
}

// BulkCheckResult is the result of checking whether a CVE ID is in the KEV catalog.
type BulkCheckResult struct {
	CVEID string `json:"cve_id"`
	IsKEV bool   `json:"is_kev"`
}

// KEVStats holds statistical information about the KEV catalog.
type KEVStats struct {
	Total        int64     `json:"total"`
	AddedLast7d  int64     `json:"added_last_7_days"`
	AddedLast30d int64     `json:"added_last_30_days"`
	LastSyncAt   time.Time `json:"last_sync_at,omitempty"`
}
