// Package rest provides HTTP clients for OSV platform REST APIs.
// OSVClient implements the OSV v1 public API spec, targeting either:
//   - Local gateway-service: http://localhost:8080
//   - Public API: https://api.osv.dev
//
// This package is ADDITIVE — it does not modify existing code in shared/pkg/clients.
package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OSVClient is an HTTP client for the OSV v1 REST API.
// All query methods return OSV-schema-compatible responses.
type OSVClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewOSVClient creates a client pointing to the given base URL.
// baseURL examples: "http://localhost:8080" or "https://api.osv.dev"
func NewOSVClient(baseURL string) *OSVClient {
	return &OSVClient{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// ── Request / Response types (OSV v1 schema) ─────────────────────────────────

// PackageInfo identifies a package in a specific ecosystem.
type PackageInfo struct {
	Ecosystem string `json:"ecosystem"`
	Name      string `json:"name"`
	// PURL is the Package URL (alternative to ecosystem+name).
	PURL string `json:"purl,omitempty"`
}

// QueryRequest represents a POST /v1/query payload.
// Exactly one of Package+Version, Commit, or PURL should be set.
type QueryRequest struct {
	Package *PackageInfo `json:"package,omitempty"`
	Version string       `json:"version,omitempty"`
	// Commit is the full git commit hash for commit-based queries.
	Commit string `json:"commit,omitempty"`
}

// QueryResponse wraps the list of matching vulnerabilities.
type QueryResponse struct {
	Vulns []VulnSummary `json:"vulns"`
}

// VulnSummary is a brief vulnerability record (as returned by /v1/query).
type VulnSummary struct {
	ID       string   `json:"id"`
	Modified string   `json:"modified"`
	Aliases  []string `json:"aliases,omitempty"`
}

// BatchQueryRequest is the payload for POST /v1/querybatch.
type BatchQueryRequest struct {
	Queries []QueryRequest `json:"queries"`
}

// BatchQueryResponse is the response from POST /v1/querybatch.
type BatchQueryResponse struct {
	Results []QueryResponse `json:"results"`
}

// ── API Methods ───────────────────────────────────────────────────────────────

// QueryByPackage queries for vulnerabilities affecting a package+version.
// Satisfies URD UR-01: search by package and version.
//
// Example:
//
//	resp, err := client.QueryByPackage(ctx, "npm", "lodash", "4.17.20")
func (c *OSVClient) QueryByPackage(ctx context.Context, ecosystem, name, version string) (*QueryResponse, error) {
	return c.query(ctx, QueryRequest{
		Package: &PackageInfo{Ecosystem: ecosystem, Name: name},
		Version: version,
	})
}

// QueryByCommit queries for vulnerabilities affecting a git commit.
// Satisfies URD UR-04: commit-based lookup.
func (c *OSVClient) QueryByCommit(ctx context.Context, commit string) (*QueryResponse, error) {
	return c.query(ctx, QueryRequest{Commit: commit})
}

// QueryByCVEID looks up a specific vulnerability by ID.
// Calls GET /v1/vulns/{id}.
// Returns the raw decoded JSON as a map (preserves full OSV schema).
// Satisfies URD UR-02: get full vulnerability details.
func (c *OSVClient) QueryByCVEID(ctx context.Context, id string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/v1/vulns/%s", c.baseURL, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET /v1/vulns/%s: %w", id, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("vulnerability %s not found", id)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET /v1/vulns/%s returned %d: %s", id, resp.StatusCode, body)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode vuln response: %w", err)
	}
	return result, nil
}

// QueryBatch submits multiple queries in a single request.
// Satisfies URD UR-06: bulk/batch lookup.
func (c *OSVClient) QueryBatch(ctx context.Context, queries []QueryRequest) (*BatchQueryResponse, error) {
	body, err := json.Marshal(BatchQueryRequest{Queries: queries})
	if err != nil {
		return nil, fmt.Errorf("marshal batch request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/v1/querybatch", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST /v1/querybatch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("querybatch returned %d: %s", resp.StatusCode, b)
	}

	var result BatchQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode querybatch response: %w", err)
	}
	return &result, nil
}

// ── internal ──────────────────────────────────────────────────────────────────

func (c *OSVClient) query(ctx context.Context, q QueryRequest) (*QueryResponse, error) {
	body, err := json.Marshal(q)
	if err != nil {
		return nil, fmt.Errorf("marshal query request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/v1/query", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST /v1/query: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("query returned %d: %s", resp.StatusCode, b)
	}

	var result QueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode query response: %w", err)
	}
	return &result, nil
}
