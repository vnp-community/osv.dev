// Package cisa provides a client for the CISA Known Exploited Vulnerabilities (KEV) catalog.
package cisa

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/globalcve/kev-service/internal/domain/entity"
)

const defaultKEVURL = "https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json"

// cisaResponse is the top-level JSON structure returned by the CISA API.
type cisaResponse struct {
	CatalogVersion  string      `json:"catalogVersion"`
	DateReleased    string      `json:"dateReleased"`
	Count           int         `json:"count"`
	Vulnerabilities []cisaVuln  `json:"vulnerabilities"`
}

type cisaVuln struct {
	CveID             string `json:"cveID"`
	VendorProject     string `json:"vendorProject"`
	Product           string `json:"product"`
	VulnerabilityName string `json:"vulnerabilityName"`
	DateAdded         string `json:"dateAdded"` // "2021-11-03"
	DueDate           string `json:"dueDate"`   // "2021-11-17"
	Notes             string `json:"notes"`
}

// Client fetches the CISA KEV catalog over HTTP.
type Client struct {
	httpClient *http.Client
	kevURL     string
}

// NewClient creates a CISA client with the given catalog URL.
// Pass an empty kevURL to use the official CISA endpoint.
func NewClient(kevURL string) *Client {
	if kevURL == "" {
		kevURL = defaultKEVURL
	}
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		kevURL:     kevURL,
	}
}

// FetchKEVCatalog downloads and parses the full CISA KEV catalog.
func (c *Client) FetchKEVCatalog(ctx context.Context) ([]*entity.KEVEntry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.kevURL, nil)
	if err != nil {
		return nil, fmt.Errorf("cisa: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cisa: request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cisa: unexpected status %d", resp.StatusCode)
	}

	var cr cisaResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return nil, fmt.Errorf("cisa: decode response: %w", err)
	}

	entries := make([]*entity.KEVEntry, 0, len(cr.Vulnerabilities))
	for _, v := range cr.Vulnerabilities {
		e := &entity.KEVEntry{
			CVEID:             v.CveID,
			VendorProject:     v.VendorProject,
			Product:           v.Product,
			VulnerabilityName: v.VulnerabilityName,
			Notes:             v.Notes,
		}
		if t, err := time.Parse("2006-01-02", v.DateAdded); err == nil {
			e.DateAdded = t
		}
		if t, err := time.Parse("2006-01-02", v.DueDate); err == nil {
			e.DueDate = t
		}
		entries = append(entries, e)
	}
	return entries, nil
}
