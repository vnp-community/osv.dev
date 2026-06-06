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

// Package kev provides a client for the CISA Known Exploited Vulnerabilities (KEV) catalog.
//
// The KEV catalog is published by CISA (Cybersecurity and Infrastructure Security Agency)
// and lists vulnerabilities that are known to be actively exploited in the wild.
//
// Catalog URL: https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json
// Docs: https://www.cisa.gov/known-exploited-vulnerabilities-catalog
package kev

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// DefaultCatalogURL is the official CISA KEV catalog JSON feed URL.
	DefaultCatalogURL = "https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json"

	// DefaultTimeout is the HTTP client timeout for fetching the catalog.
	DefaultTimeout = 60 * time.Second
)

// Entry represents a single entry in the CISA KEV catalog.
type Entry struct {
	// CVEID is the CVE identifier (e.g., "CVE-2023-44487").
	CVEID string `json:"cveID"`
	// VendorProject is the vendor or project name.
	VendorProject string `json:"vendorProject"`
	// Product is the affected product name.
	Product string `json:"product"`
	// VulnerabilityName is a short human-readable name.
	VulnerabilityName string `json:"vulnerabilityName"`
	// DateAdded is when the entry was added to the KEV catalog.
	DateAdded string `json:"dateAdded"`
	// ShortDescription is a brief description of the vulnerability.
	ShortDescription string `json:"shortDescription"`
	// RequiredAction is what organizations must do to remediate.
	RequiredAction string `json:"requiredAction"`
	// DueDate is the deadline for federal agencies to remediate.
	DueDate string `json:"dueDate"`
	// KnownRansomwareCampaignUse indicates if used in ransomware.
	KnownRansomwareCampaignUse string `json:"knownRansomwareCampaignUse"`
	// Notes are additional notes about the entry.
	Notes string `json:"notes"`
}

// Catalog represents the full CISA KEV catalog.
type Catalog struct {
	Title           string    `json:"title"`
	CatalogVersion  string    `json:"catalogVersion"`
	DateReleased    time.Time `json:"dateReleased"`
	Count           int       `json:"count"`
	Vulnerabilities []*Entry  `json:"vulnerabilities"`
}

// Client fetches and queries the CISA KEV catalog.
type Client struct {
	httpClient *http.Client
	catalogURL string
}

// ClientOption configures a KEV Client.
type ClientOption func(*Client)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) { c.httpClient = hc }
}

// WithCatalogURL sets a custom catalog URL (useful for testing).
func WithCatalogURL(url string) ClientOption {
	return func(c *Client) { c.catalogURL = url }
}

// NewClient creates a new KEV catalog client with the given options.
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		httpClient: &http.Client{Timeout: DefaultTimeout},
		catalogURL: DefaultCatalogURL,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// FetchCatalog downloads and parses the full KEV catalog.
func (c *Client) FetchCatalog(ctx context.Context) (*Catalog, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.catalogURL, nil)
	if err != nil {
		return nil, fmt.Errorf("kev: create request: %w", err)
	}
	req.Header.Set("User-Agent", "OSV-dev/1.0 (+https://osv.dev)")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("kev: fetch catalog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("kev: unexpected status %d fetching catalog", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("kev: read response: %w", err)
	}

	var catalog Catalog
	if err := json.Unmarshal(body, &catalog); err != nil {
		return nil, fmt.Errorf("kev: parse catalog JSON: %w", err)
	}

	return &catalog, nil
}

// BuildIndex creates an in-memory lookup map from CVE ID to KEV entry.
// This is efficient for repeated lookups after a single catalog fetch.
func BuildIndex(catalog *Catalog) map[string]*Entry {
	idx := make(map[string]*Entry, catalog.Count)
	for _, e := range catalog.Vulnerabilities {
		idx[e.CVEID] = e
	}
	return idx
}

// InMemoryLookup provides fast KEV lookups from a pre-fetched catalog.
type InMemoryLookup struct {
	index     map[string]*Entry
	fetchedAt time.Time
	catalog   *Catalog
}

// NewInMemoryLookup creates a lookup from a fetched catalog.
func NewInMemoryLookup(catalog *Catalog) *InMemoryLookup {
	return &InMemoryLookup{
		index:     BuildIndex(catalog),
		fetchedAt: time.Now(),
		catalog:   catalog,
	}
}

// IsKEV returns true if the given CVE ID is in the KEV catalog.
func (l *InMemoryLookup) IsKEV(cveID string) bool {
	_, ok := l.index[cveID]
	return ok
}

// Get returns the KEV entry for a CVE ID, or nil if not found.
func (l *InMemoryLookup) Get(cveID string) *Entry {
	return l.index[cveID]
}

// Count returns the number of entries in the catalog.
func (l *InMemoryLookup) Count() int {
	return len(l.index)
}

// FetchedAt returns when the catalog was last fetched.
func (l *InMemoryLookup) FetchedAt() time.Time {
	return l.fetchedAt
}

// CatalogVersion returns the version string of the fetched catalog.
func (l *InMemoryLookup) CatalogVersion() string {
	if l.catalog == nil {
		return ""
	}
	return l.catalog.CatalogVersion
}
