// Package client provides the HTTP client for the OSV API.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is an HTTP client for the OSV API.
type Client struct {
	serverURL  string
	apiKey     string
	httpClient *http.Client
}

// New creates a new API client.
func New(serverURL, apiKey string) *Client {
	return &Client{
		serverURL: strings.TrimRight(serverURL, "/"),
		apiKey:    apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// VulnSummary is a minimal vulnerability summary from API responses.
type VulnSummary struct {
	ID       string    `json:"id"`
	Summary  string    `json:"summary"`
	Severity string    `json:"severity,omitempty"`
	Modified time.Time `json:"modified"`
}

// EnrichmentData mirrors the API v2 enrichment response.
type EnrichmentData struct {
	VulnID           string         `json:"vuln_id"`
	KEV              *KEVData       `json:"kev,omitempty"`
	EPSS             *EPSSData      `json:"epss,omitempty"`
	Tags             []string       `json:"tags,omitempty"`
	CWEIDs           []string       `json:"cwe_ids,omitempty"`
	ExploitAvailable bool           `json:"exploit_available"`
	AISummary        string         `json:"ai_summary,omitempty"`
}

// KEVData mirrors the KEV portion of enrichment.
type KEVData struct {
	IsKEV          bool   `json:"is_kev"`
	DateAdded      string `json:"date_added,omitempty"`
	DueDate        string `json:"due_date,omitempty"`
	RequiredAction string `json:"required_action,omitempty"`
}

// EPSSData mirrors the EPSS portion of enrichment.
type EPSSData struct {
	Score      float64 `json:"score"`
	Percentile float64 `json:"percentile"`
	Tier       string  `json:"tier"`
	Date       string  `json:"date"`
}

// SourceStatus holds the status of a CVE data source.
type SourceStatus struct {
	Name            string    `json:"name"`
	State           string    `json:"state"`
	LastSyncAt      time.Time `json:"last_sync_at,omitempty"`
	CVECountLastSync int      `json:"cve_count_last_sync,omitempty"`
	ErrorCount24h   int       `json:"error_count_24h,omitempty"`
}

// SearchResult holds a vulnerability search result.
type SearchResult struct {
	VulnID   string  `json:"vuln_id"`
	Summary  string  `json:"summary"`
	Severity string  `json:"severity,omitempty"`
	Score    float32 `json:"_score,omitempty"`
	Reason   string  `json:"reason,omitempty"` // for related vulns: "alias", "same_package", "semantic"
}

// ── Methods ───────────────────────────────────────────────────────────────────

// GetVuln fetches a vulnerability by ID from the OSV API.
func (c *Client) GetVuln(ctx context.Context, id string) (map[string]any, error) {
	var result map[string]any
	if err := c.getJSON(ctx, fmt.Sprintf("/v1/vulns/%s", id), &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetEnrichment fetches enrichment data for a vulnerability (API v2).
func (c *Client) GetEnrichment(ctx context.Context, id string) (*EnrichmentData, error) {
	var result EnrichmentData
	if err := c.getJSON(ctx, fmt.Sprintf("/v2/vulns/%s/enrichment", id), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetRelated fetches related vulnerabilities (API v2).
func (c *Client) GetRelated(ctx context.Context, id string) ([]SearchResult, error) {
	var result struct {
		Related []SearchResult `json:"related"`
	}
	if err := c.getJSON(ctx, fmt.Sprintf("/v2/vulns/%s/related", id), &result); err != nil {
		return nil, err
	}
	return result.Related, nil
}

// SearchVulns searches vulnerabilities by keyword.
func (c *Client) SearchVulns(ctx context.Context, query string) ([]SearchResult, error) {
	var result struct {
		Results []SearchResult `json:"results"`
	}
	path := fmt.Sprintf("/v1/query?query=%s", query)
	if err := c.getJSON(ctx, path, &result); err != nil {
		return nil, err
	}
	return result.Results, nil
}

// ListSources fetches all CVE data sources.
func (c *Client) ListSources(ctx context.Context) ([]SourceStatus, error) {
	var result struct {
		Sources []SourceStatus `json:"sources"`
	}
	if err := c.getJSON(ctx, "/admin/v1/sources", &result); err != nil {
		return nil, err
	}
	return result.Sources, nil
}

// TriggerSync triggers a manual sync for a source.
func (c *Client) TriggerSync(ctx context.Context, name string) error {
	return c.postJSON(ctx, fmt.Sprintf("/admin/v1/sources/%s/sync", name), nil, nil)
}

// PauseSource pauses a CVE source.
func (c *Client) PauseSource(ctx context.Context, name string) error {
	return c.postJSON(ctx, fmt.Sprintf("/admin/v1/sources/%s/pause", name), nil, nil)
}

// ResumeSource resumes a CVE source.
func (c *Client) ResumeSource(ctx context.Context, name string) error {
	return c.postJSON(ctx, fmt.Sprintf("/admin/v1/sources/%s/resume", name), nil, nil)
}

// WithdrawVuln withdraws a vulnerability.
func (c *Client) WithdrawVuln(ctx context.Context, id, reason string) error {
	return c.postJSON(ctx, fmt.Sprintf("/admin/v1/vulns/%s/withdraw", id), map[string]string{
		"reason": reason,
	}, nil)
}

// ReprocessVuln triggers reprocessing of a vulnerability.
func (c *Client) ReprocessVuln(ctx context.Context, id, reason string) error {
	return c.postJSON(ctx, fmt.Sprintf("/admin/v1/vulns/%s/reprocess", id), map[string]string{
		"reason": reason,
	}, nil)
}

// GetStats fetches vulnerability statistics from admin API.
func (c *Client) GetStats(ctx context.Context) (map[string]any, error) {
	var result map[string]any
	if err := c.getJSON(ctx, "/admin/v1/vulns/stats", &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ── HTTP helpers ──────────────────────────────────────────────────────────────

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.serverURL+path, nil)
	if err != nil {
		return err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("API error %d: %s", resp.StatusCode, body)
	}
	return json.Unmarshal(body, out)
}

func (c *Client) postJSON(ctx context.Context, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = strings.NewReader(string(b))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.serverURL+path, body)
	if err != nil {
		return err
	}
	c.setHeaders(req)
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("POST %s: %w", path, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("API error %d: %s", resp.StatusCode, respBody)
	}
	if out != nil {
		return json.Unmarshal(respBody, out)
	}
	return nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "cvectl/1.0")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
}
