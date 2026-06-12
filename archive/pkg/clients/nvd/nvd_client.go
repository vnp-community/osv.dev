// Package nvd provides a rate-limited HTTP client for the NVD 2.0 API.
package nvd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

const (
	defaultBaseURL        = "https://services.nvd.nist.gov/rest/json/cves/2.0"
	rateNoKey      int    = 5  // requests per 30s without API key
	rateWithKey    int    = 50 // requests per 30s with API key
	windowSeconds         = 30
	defaultTimeout        = 15 * time.Second
)

// Client is a rate-limited NVD 2.0 API client.
// Use NewClient to create an instance.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	limiter    *rate.Limiter
}

// NewClient creates a new NVD client.
// If apiKey is empty, the anonymous rate limit (5 req/30s) is applied.
// If apiKey is set, the authenticated limit (50 req/30s) applies.
func NewClient(apiKey string) *Client {
	var requestsPerInterval int
	if apiKey == "" {
		requestsPerInterval = rateNoKey
	} else {
		requestsPerInterval = rateWithKey
	}

	// Convert to per-second rate for the token bucket.
	// e.g. 5 req/30s = 0.1667 req/s → burst=5 (allow short bursts)
	ratePerSec := rate.Limit(float64(requestsPerInterval) / float64(windowSeconds))
	limiter := rate.NewLimiter(ratePerSec, requestsPerInterval)

	return &Client{
		baseURL: defaultBaseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		limiter: limiter,
	}
}

// NewClientWithOptions creates a client with a custom base URL and HTTP client.
// Useful for testing with a mock server.
func NewClientWithOptions(baseURL, apiKey string, httpClient *http.Client) *Client {
	c := NewClient(apiKey)
	c.baseURL = baseURL
	c.httpClient = httpClient
	return c
}

// GetCVE retrieves full CVE data for the given CVE ID from the NVD 2.0 API.
// Returns ErrNotFound if the CVE does not exist in NVD.
// Respects the rate limiter before making the request.
func (c *Client) GetCVE(ctx context.Context, cveID string) (*NVDCVEItem, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("nvd rate limit wait: %w", err)
	}

	url := fmt.Sprintf("%s?cveId=%s", c.baseURL, cveID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("nvd build request: %w", err)
	}

	req.Header.Set("User-Agent", "OpenVulnScan/1.0 (https://github.com/osv/openvulnscan)")
	if c.apiKey != "" {
		req.Header.Set("apiKey", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nvd request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("nvd read body: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound{CVEID: cveID}
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("nvd rate limit exceeded (HTTP 429)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nvd unexpected status %d: %s", resp.StatusCode, truncate(body, 200))
	}

	var apiResp nvdAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("nvd parse response: %w", err)
	}

	if apiResp.TotalResults == 0 || len(apiResp.Vulnerabilities) == 0 {
		return nil, ErrNotFound{CVEID: cveID}
	}

	return mapNVDCVE(&apiResp.Vulnerabilities[0].CVE), nil
}

// mapNVDCVE converts the internal NVD API response into an NVDCVEItem.
func mapNVDCVE(raw *nvdCVE) *NVDCVEItem {
	item := &NVDCVEItem{
		ID:         raw.ID,
		Published:  parseTime(raw.Published),
		Modified:   parseTime(raw.LastModified),
		References: make([]NVDReference, 0, len(raw.References)),
		Weaknesses: make([]NVDWeakness, 0, len(raw.Weaknesses)),
	}

	// English description
	for _, d := range raw.Descriptions {
		if d.Lang == "en" {
			item.Description = d.Value
			break
		}
	}

	// CVSS v3.1 (preferred) or v3.0
	if len(raw.Metrics.CVSSMetricV31) > 0 {
		cv := raw.Metrics.CVSSMetricV31[0].CVSSData
		item.Metrics.CVSSv3Score = cv.BaseScore
		item.Metrics.CVSSv3Vector = cv.VectorString
		item.Metrics.CVSSv3Severity = cv.BaseSeverity
	} else if len(raw.Metrics.CVSSMetricV30) > 0 {
		cv := raw.Metrics.CVSSMetricV30[0].CVSSData
		item.Metrics.CVSSv3Score = cv.BaseScore
		item.Metrics.CVSSv3Vector = cv.VectorString
		item.Metrics.CVSSv3Severity = cv.BaseSeverity
	}

	// CVSS v2
	if len(raw.Metrics.CVSSMetricV2) > 0 {
		cv := raw.Metrics.CVSSMetricV2[0].CVSSData
		item.Metrics.CVSSv2Score = cv.BaseScore
		item.Metrics.CVSSv2Vector = cv.VectorString
	}

	// References
	for _, r := range raw.References {
		item.References = append(item.References, NVDReference{
			URL:    r.URL,
			Source: r.Source,
			Tags:   r.Tags,
		})
	}

	// Weaknesses (CWEs)
	for _, w := range raw.Weaknesses {
		for _, d := range w.Description {
			if d.Lang == "en" {
				item.Weaknesses = append(item.Weaknesses, NVDWeakness{
					CWEID:       d.Value,
					Description: d.Value,
				})
			}
		}
	}

	return item
}

// ErrNotFound is returned when a CVE ID is not found in NVD.
type ErrNotFound struct {
	CVEID string
}

func (e ErrNotFound) Error() string {
	return fmt.Sprintf("nvd: CVE %s not found", e.CVEID)
}

func parseTime(s string) time.Time {
	// NVD uses RFC 3339 format: "2021-12-10T10:15:09.143"
	for _, layout := range []string{
		"2006-01-02T15:04:05.999",
		"2006-01-02T15:04:05",
		time.RFC3339,
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
