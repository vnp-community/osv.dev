// Package kev contains the KEV domain entities.
package kev

import "time"

// KEVEntry represents a single entry in the CISA Known Exploited Vulnerabilities catalog.
type KEVEntry struct {
	CVEID             string    `json:"cve_id"              db:"cve_id"`
	VendorProject     string    `json:"vendor"      db:"vendor_project"`
	Product           string    `json:"product"            db:"product"`
	VulnerabilityName string    `json:"vulnerability_name"  db:"vulnerability_name"`
	DateAdded         time.Time `json:"date_added"          db:"date_added"`
	DueDate           time.Time `json:"due_date"            db:"due_date"`
	Notes             string    `json:"notes"              db:"notes"`

	// CR-GCV-007: Extended fields from CISA KEV JSON
	ShortDescription    string `json:"short_description,omitempty"  db:"short_description"`
	RequiredAction      string `json:"required_action,omitempty"    db:"required_action"`
	IsKnownRansomware   bool   `json:"is_known_ransomware"           db:"is_known_ransomware"`
	KnownRansomwareCampaignUse string `json:"known_ransomware_campaign_use,omitempty" db:"ransomware_campaign_use"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// IsRansomware returns true if CISA classifies this CVE as known ransomware.
func (e *KEVEntry) IsRansomware() bool {
	return e.KnownRansomwareCampaignUse == "Known" || e.KnownRansomwareCampaignUse == "known" || e.IsKnownRansomware
}

// KEVFilter holds query parameters for listing KEV entries.
type KEVFilter struct {
	Query         string
	VendorProject string
	IsRansomware  *bool      // filter ransomware-associated KEVs
	Since         *time.Time
	Page          int
	Limit         int
	// CR-007: advanced filters
	DateFrom *time.Time // filter date_added >= DateFrom
	DateTo   *time.Time // filter date_added <= DateTo
	SortBy   string     // "date_added_desc" | "date_added_asc" | "vendor_asc"
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

// BulkCheckResult is the result of checking whether a CVE ID is in KEV.
type BulkCheckResult struct {
	CVEID string `json:"cve_id"`
	IsKEV bool   `json:"is_kev"`
}

// VendorCount holds a vendor name and its KEV entry count.
type VendorCount struct {
	Vendor string `json:"vendor" db:"vendor_project"`
	Count  int    `json:"count"  db:"count"`
}

// MonthCount holds a month and its KEV entry count.
type MonthCount struct {
	Month string `json:"month" db:"month"` // "2026-01"
	Count int    `json:"count" db:"count"`
}

// KEVStats holds statistical information about the KEV catalog.
type KEVStats struct {
	Total        int64     `json:"total"`
	AddedLast7d  int64     `json:"added_last_7_days"`
	AddedLast30d int64     `json:"added_last_30_days"`
	LastSyncAt   time.Time `json:"last_sync_at,omitempty"`

	// CR-GCV-007: Extended stats
	TotalRansomware int64         `json:"total_ransomware"`
	TopVendors      []VendorCount `json:"top_vendors"`
	ByMonth         []MonthCount  `json:"by_month"`
	AvgDaysToPatch  float64       `json:"avg_days_to_patch"`
}

