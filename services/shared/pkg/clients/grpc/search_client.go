package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// SearchClient queries the search-service via its REST HTTP API.
// (A gRPC proto for search-service is not yet finalized; REST is the current interface.)
// Used by: apps/cli/cmd/query, apps/osv (gateway /v1/search)
type SearchClient struct {
	baseURL    string
	httpClient *http.Client
}

// SearchResult is a single vulnerability search result from search-service.
type SearchResult struct {
	ID       string  `json:"id"`
	Summary  string  `json:"summary"`
	Severity string  `json:"severity"`
	Score    float64 `json:"cvss_score"`
}

// NewSearchClient creates a client pointing to search-service HTTP API.
// addr format: "localhost:8083" (without scheme) or provide full base URL.
func NewSearchClient(addr string) *SearchClient {
	baseURL := addr
	if len(addr) > 0 && addr[0] != 'h' {
		baseURL = "http://" + addr
	}
	return &SearchClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// Search performs a full-text vulnerability search.
// Calls: GET /v1/search?q=<query>&limit=<limit>
func (c *SearchClient) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	reqURL := fmt.Sprintf("%s/v1/search?q=%s&limit=%d",
		c.baseURL, url.QueryEscape(query), limit)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("search request build: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search-service returned %d", resp.StatusCode)
	}

	var result struct {
		Items []SearchResult `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}
	return result.Items, nil
}
