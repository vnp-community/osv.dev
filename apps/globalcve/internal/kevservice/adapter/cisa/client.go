// Package cisa — CISA KEV API client.
// Port from TypeScript (globalcve/src/lib/kev.ts).
package cisa

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/globalcve/mono/internal/kevservice/domain/entity"
)

const defaultKEVURL = "https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json"

// Client fetches the CISA Known Exploited Vulnerabilities catalog.
type Client struct {
	kevURL     string
	httpClient *http.Client
}

// New creates a new CISA KEV client.
func New(kevURL string, timeout time.Duration) *Client {
	if kevURL == "" {
		kevURL = defaultKEVURL
	}
	return &Client{
		kevURL:     kevURL,
		httpClient: &http.Client{Timeout: timeout},
	}
}

// cisaResponse is the CISA KEV JSON response format.
type cisaResponse struct {
	Title          string             `json:"title"`
	CatalogVersion string             `json:"catalogVersion"`
	DateReleased   string             `json:"dateReleased"`
	Count          int                `json:"count"`
	Vulnerabilities []cisaVulnerability `json:"vulnerabilities"`
}

type cisaVulnerability struct {
	CveID                     string `json:"cveID"`
	VendorProject             string `json:"vendorProject"`
	Product                   string `json:"product"`
	VulnerabilityName         string `json:"vulnerabilityName"`
	DateAdded                 string `json:"dateAdded"`
	ShortDescription          string `json:"shortDescription"`
	RequiredAction            string `json:"requiredAction"`
	DueDate                   string `json:"dueDate"`
	KnownRansomwareCampaignUse string `json:"knownRansomwareCampaignUse"`
	Notes                     string `json:"notes"`
}

// FetchKEVCatalog downloads and parses the CISA KEV catalog.
func (c *Client) FetchKEVCatalog(ctx context.Context) ([]*entity.KEVEntry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.kevURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cisa http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return nil, fmt.Errorf("cisa status %d: %s", resp.StatusCode, body)
	}

	var data cisaResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("cisa decode: %w", err)
	}

	entries := make([]*entity.KEVEntry, 0, len(data.Vulnerabilities))
	for _, v := range data.Vulnerabilities {
		entry := &entity.KEVEntry{
			CVEID:             v.CveID,
			VendorProject:     v.VendorProject,
			Product:           v.Product,
			VulnerabilityName: v.VulnerabilityName,
			ShortDescription:  v.ShortDescription,
			RequiredAction:    v.RequiredAction,
			Notes:             v.Notes,
			KnownRansomware:   v.KnownRansomwareCampaignUse == "Known" || v.KnownRansomwareCampaignUse == "Yes",
		}

		// Parse dates: "2021-11-03" format
		if t, err := time.Parse("2006-01-02", v.DateAdded); err == nil {
			entry.DateAdded = t
		}
		if t, err := time.Parse("2006-01-02", v.DueDate); err == nil {
			entry.DueDate = t
		}

		entries = append(entries, entry)
	}

	return entries, nil
}
