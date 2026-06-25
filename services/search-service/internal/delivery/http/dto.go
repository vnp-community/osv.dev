// Package http — dto.go
// Extended CVE response DTOs for the CVE Intelligence UI page.
// These supplement the domain entity.CVE with UI-specific fields.
package http

// CVEResponseItem is the search result item with UI-required fields.
type CVEResponseItem struct {
	ID             string    `json:"id"`
	Severity       string    `json:"severity"`
	CVSSv3         *float64  `json:"cvss_v3,omitempty"`
	CVSSv2         *float64  `json:"cvss_v2,omitempty"`
	EPSSScore      *float64  `json:"epss_score,omitempty"`
	EPSSPercentile *float64  `json:"epss_percentile,omitempty"` // NEW
	IsKEV          bool      `json:"is_kev"`           // NEW
	HasExploit     bool      `json:"has_exploit"`      // NEW (is_exploit in DB)
	ExploitDBURL   *string   `json:"exploit_db_url"`   // NEW: optional link
	Vendor         string    `json:"vendor"`
	Product        string    `json:"product"`
	CWEIds         []string  `json:"cwe_ids"`
	CAPECIds       []string  `json:"capec_ids"` // NEW
	Description    string    `json:"description"`
	PublishedAt    string    `json:"published_at"`
	UpdatedAt      string    `json:"updated_at"`
	Sources        []CVESource `json:"sources"`     // NEW
	AISeverity     *AISeverity `json:"ai_severity"` // NEW — nullable
	SimilarityScore *float64   `json:"similarity_score,omitempty"` // For semantic search
}

// CVESource represents a data source for a CVE.
type CVESource struct {
	Name         string `json:"name"`          // "NVD" | "JVN" | "OSS-Fuzz"
	URL          string `json:"url"`
	LastModified string `json:"last_modified"`
}

// AISeverity is the AI-enhanced severity prediction.
type AISeverity struct {
	Severity   string  `json:"severity"`   // "CRITICAL"|"HIGH"|"MEDIUM"|"LOW"
	Confidence float64 `json:"confidence"` // 0.0-1.0
	Reasoning  string  `json:"reasoning"`
	Source     string  `json:"source"` // "cvss_v3"|"cvss_v2"|"llm"
}

// CVEDetailResponse extends CVEResponseItem with nested detail objects.
type CVEDetailResponse struct {
	CVEResponseItem
	AffectedProducts []CPEEntry `json:"affected_products"` // NEW
	KEVDetail        *KEVDetail `json:"kev_detail"`         // NEW — null if !is_kev
	References       []string   `json:"references"`         // NEW: reference URLs
	Notes            []string   `json:"notes"`              // NEW: advisory notes
}

// CPEEntry is a CPE dictionary entry for an affected product.
type CPEEntry struct {
	CPE          string  `json:"cpe"`
	Vendor       string  `json:"vendor"`
	Product      string  `json:"product"`
	VersionStart *string `json:"version_start"`
	VersionEnd   *string `json:"version_end"`
}

// KEVDetail contains CISA KEV-specific metadata.
type KEVDetail struct {
	DateAdded                  string  `json:"date_added"`
	DueDate                    *string `json:"due_date"`
	KnownRansomwareCampaignUse bool    `json:"known_ransomware_campaign_use"`
	RequiredAction             string  `json:"required_action"`
	ShortDescription           string  `json:"short_description"`
}

// SearchResponse is the paginated search result with optional aggregations.
type SearchResponse struct {
	Data         []CVEResponseItem   `json:"data"`
	Total        int                 `json:"total"`
	Page         int                 `json:"page"`
	PageSize     int                 `json:"page_size"`
	Aggregations *SearchAggregations `json:"aggregations,omitempty"` // null unless ?include_aggregations=true
}

// SearchAggregations holds faceted aggregation counts for chart filters.
type SearchAggregations struct {
	BySeverity map[string]int `json:"by_severity"`
	TopVendors []VendorCount  `json:"top_vendors"`
	ByYear     []YearCount    `json:"by_year"`
}

// VendorCount is a vendor with CVE count.
type VendorCount struct {
	Vendor string `json:"vendor"`
	Count  int    `json:"count"`
}

// YearCount is a year with CVE count.
type YearCount struct {
	Year  string `json:"year"`
	Count int    `json:"count"`
}
