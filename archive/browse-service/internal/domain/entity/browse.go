// Package entity defines browse-service domain types.
package entity

// BrowseResult is returned by vendor/product listing endpoints.
type BrowseResult struct {
	// Items holds the list of vendors or products.
	Items []string `json:"items"`
	// Total is the number of items returned.
	Total int `json:"total"`
	// Vendor is set when listing products for a specific vendor.
	Vendor string `json:"vendor,omitempty"`
}

// SearchResult is returned by the vendor/product search endpoint.
type SearchResult struct {
	Items   []string `json:"items"`
	Total   int      `json:"total"`
	Query   string   `json:"query"`
}
