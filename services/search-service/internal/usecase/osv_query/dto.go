// Package osv_query — dto.go
// DTOs for the OSV v1 compatible query API.
// Reference: https://osv.dev/docs/#section/OSV-API
package osv_query

// OSVPackage identifies a package in the OSV ecosystem.
type OSVPackage struct {
	Ecosystem string `json:"ecosystem"` // e.g., "Go", "npm", "PyPI"
	Name      string `json:"name"`      // e.g., "github.com/gin-gonic/gin"
	PURL      string `json:"purl"`      // e.g., "pkg:golang/github.com/gin-gonic/gin"
}

// OSVQueryRequest is the input for POST /v1/query.
type OSVQueryRequest struct {
	Version string     `json:"version"` // package version to query
	Package OSVPackage `json:"package"`
	Commit  string     `json:"commit"` // git commit hash (alternative to version)
}

// OSVBatchRequest is the input for POST /v1/querybatch.
type OSVBatchRequest struct {
	Queries []OSVQueryRequest `json:"queries"` // max 1000
}

// OSVVulnListParams are query params for GET /v1/vulns/list.
type OSVVulnListParams struct {
	PageToken     string `json:"page_token"`
	ModifiedSince string `json:"modified_since"` // RFC3339
	Ecosystem     string `json:"ecosystem"`
}

// OSVVulnListResponse is the response for GET /v1/vulns/list.
type OSVVulnListResponse struct {
	Vulns         []OSVVulnSummary `json:"vulns"`
	NextPageToken string           `json:"next_page_token,omitempty"`
}

// OSVQueryResult is the response for a single POST /v1/query.
type OSVQueryResult struct {
	Vulns []OSVVulnSummary `json:"vulns"`
}

// OSVBatchResult is the response for POST /v1/querybatch.
type OSVBatchResult struct {
	Results []OSVQueryResult `json:"results"`
}

// OSVVulnSummary is a brief vulnerability record in OSV schema format.
type OSVVulnSummary struct {
	ID       string `json:"id"`
	Modified string `json:"modified"` // RFC3339
	Aliases  []string `json:"aliases,omitempty"`
	Summary  string `json:"summary,omitempty"`
	Severity string `json:"severity,omitempty"` // CRITICAL | HIGH | MEDIUM | LOW
	Published string `json:"published,omitempty"`
}
