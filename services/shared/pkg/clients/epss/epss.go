// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package epss provides a client for the EPSS (Exploit Prediction Scoring System) API.
//
// EPSS is a model that estimates the probability that a software vulnerability
// will be exploited in the wild in the next 30 days. Scores are updated daily.
//
// API: https://api.first.org/data/v1/epss
// Docs: https://www.first.org/epss/api
package epss

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is the EPSS API base URL.
	DefaultBaseURL = "https://api.first.org/data/v1/epss"

	// DefaultTimeout is the HTTP timeout for EPSS requests.
	DefaultTimeout = 30 * time.Second

	// MaxBatchSize is the maximum number of CVEs per batch request.
	MaxBatchSize = 100
)

// Score holds the EPSS score for a single CVE.
type Score struct {
	// CVEID is the CVE identifier.
	CVEID string `json:"cve"`
	// EPSS is the probability (0.0–1.0) of exploitation in the next 30 days.
	EPSS float64 `json:"epss,string"`
	// Percentile is the relative rank among all scored CVEs (0.0–1.0).
	Percentile float64 `json:"percentile,string"`
	// Date is the date when this score was computed.
	Date string `json:"date"`
}

// apiResponse wraps the EPSS API response envelope.
type apiResponse struct {
	Status     string   `json:"status"`
	StatusCode int      `json:"status-code"`
	Version    string   `json:"version"`
	Access     string   `json:"access"`
	Total      int      `json:"total"`
	Offset     int      `json:"offset"`
	Limit      int      `json:"limit"`
	Data       []*Score `json:"data"`
}

// Client queries the FIRST EPSS API.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// ClientOption configures an EPSS Client.
type ClientOption func(*Client)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) { c.httpClient = hc }
}

// WithBaseURL sets a custom API base URL (useful for testing).
func WithBaseURL(u string) ClientOption {
	return func(c *Client) { c.baseURL = u }
}

// NewClient creates a new EPSS client.
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		httpClient: &http.Client{Timeout: DefaultTimeout},
		baseURL:    DefaultBaseURL,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Get returns the EPSS score for a single CVE ID.
// Returns nil, nil if the CVE is not scored.
func (c *Client) Get(ctx context.Context, cveID string) (*Score, error) {
	scores, err := c.GetBatch(ctx, []string{cveID})
	if err != nil {
		return nil, err
	}
	if score, ok := scores[cveID]; ok {
		return score, nil
	}
	return nil, nil
}

// GetBatch returns EPSS scores for multiple CVE IDs in a single API call.
// The returned map has CVE ID as key. CVEs not found in EPSS are absent.
// Input slices larger than MaxBatchSize are automatically chunked.
func (c *Client) GetBatch(ctx context.Context, cveIDs []string) (map[string]*Score, error) {
	result := make(map[string]*Score, len(cveIDs))

	// Chunk into MaxBatchSize groups
	for i := 0; i < len(cveIDs); i += MaxBatchSize {
		end := i + MaxBatchSize
		if end > len(cveIDs) {
			end = len(cveIDs)
		}
		chunk := cveIDs[i:end]

		scores, err := c.fetchChunk(ctx, chunk)
		if err != nil {
			return nil, err
		}
		for k, v := range scores {
			result[k] = v
		}
	}

	return result, nil
}

func (c *Client) fetchChunk(ctx context.Context, cveIDs []string) (map[string]*Score, error) {
	params := url.Values{}
	params.Set("cve", strings.Join(cveIDs, ","))
	params.Set("limit", strconv.Itoa(MaxBatchSize))

	reqURL := c.baseURL + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("epss: create request: %w", err)
	}
	req.Header.Set("User-Agent", "OSV-dev/1.0 (+https://osv.dev)")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("epss: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("epss: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("epss: read response: %w", err)
	}

	var apiResp apiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("epss: parse response: %w", err)
	}

	scores := make(map[string]*Score, len(apiResp.Data))
	for _, s := range apiResp.Data {
		scores[s.CVEID] = s
	}
	return scores, nil
}

// SeverityTier categorizes an EPSS score into a qualitative tier.
type SeverityTier string

const (
	// TierCritical: EPSS percentile >= 0.95 — top 5% most likely to be exploited.
	TierCritical SeverityTier = "CRITICAL"
	// TierHigh: EPSS percentile >= 0.75.
	TierHigh SeverityTier = "HIGH"
	// TierMedium: EPSS percentile >= 0.50.
	TierMedium SeverityTier = "MEDIUM"
	// TierLow: EPSS percentile < 0.50.
	TierLow SeverityTier = "LOW"
)

// Tier returns a qualitative severity tier based on the EPSS percentile.
func (s *Score) Tier() SeverityTier {
	switch {
	case s.Percentile >= 0.95:
		return TierCritical
	case s.Percentile >= 0.75:
		return TierHigh
	case s.Percentile >= 0.50:
		return TierMedium
	default:
		return TierLow
	}
}
