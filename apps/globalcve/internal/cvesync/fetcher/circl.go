// Package fetcher — CIRCL CVE API fetcher.
// Port from TypeScript (globalcve/src/app/api/cves/route.ts).
// Source: https://cve.circl.lu/api
package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/globalcve/mono/internal/cvesearch/domain/entity"
	"github.com/globalcve/mono/internal/cvesync/domain/repository"
)

// CIRCLFetcher fetches CVE data from the CIRCL CVE Search API.
type CIRCLFetcher struct {
	baseURL string
	client  *http.Client
	cveRepo repository.CVEWriteRepository
}

// NewCIRCLFetcher creates a new CIRCL CVE fetcher.
func NewCIRCLFetcher(baseURL string, timeout time.Duration, cveRepo repository.CVEWriteRepository) *CIRCLFetcher {
	if baseURL == "" {
		baseURL = "https://cve.circl.lu/api"
	}
	return &CIRCLFetcher{
		baseURL: baseURL,
		cveRepo: cveRepo,
		client:  &http.Client{Timeout: timeout},
	}
}

func (f *CIRCLFetcher) Source() entity.SourceName { return entity.SourceNameCIRCL }

// circlCVE represents a CVE record from the CIRCL API.
type circlCVE struct {
	ID          string   `json:"id"`
	Summary     string   `json:"summary"`
	Published   string   `json:"Published"`
	Modified    string   `json:"Modified"`
	CVSS        float64  `json:"cvss"`
	CVSS3       float64  `json:"cvss3"`
	CVSSVector  string   `json:"cvss-vector"`
	References  []string `json:"references"`
	CWE         string   `json:"cwe"`
	Vendors     []string `json:"vendors"`
	Products    []string `json:"products"`
	VulnSoft    []struct {
		Vendor  string `json:"vendor"`
		Product string `json:"product"`
	} `json:"vulnerable_configuration"`
}

// Fetch retrieves recent CVEs from CIRCL API.
// CIRCL doesn't have a bulk "all CVEs" endpoint like NVD, so we fetch by recent activity.
func (f *CIRCLFetcher) Fetch(ctx context.Context, opts FetchOptions) (int, error) {
	total := 0
	// Fetch last N pages of recently modified CVEs
	// CIRCL API: GET /api/last/{N} — returns last N CVE records
	const batchSize = 100
	const maxPages = 20 // ~2000 CVEs per sync

	for page := 0; page < maxPages; page++ {
		cves, err := f.fetchLastCVEs(ctx, batchSize, page*batchSize)
		if err != nil {
			log.Ctx(ctx).Warn().Err(err).Int("page", page).Msg("circl: fetch page error, stopping")
			break
		}
		if len(cves) == 0 {
			break
		}

		converted := make([]*entity.CVE, 0, len(cves))
		for _, c := range cves {
			converted = append(converted, f.convert(c))
		}

		inserted, updated, err := f.cveRepo.UpsertBatch(ctx, converted)
		if err != nil {
			return total, fmt.Errorf("circl upsert batch: %w", err)
		}
		total += inserted + updated

		select {
		case <-ctx.Done():
			return total, ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}

	return total, nil
}

func (f *CIRCLFetcher) fetchLastCVEs(ctx context.Context, limit, skip int) ([]*circlCVE, error) {
	reqURL := fmt.Sprintf("%s/last/%d", f.baseURL, limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("circl http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return nil, fmt.Errorf("circl status %d: %s", resp.StatusCode, body)
	}

	var cves []*circlCVE
	if err := json.NewDecoder(resp.Body).Decode(&cves); err != nil {
		return nil, fmt.Errorf("circl decode: %w", err)
	}
	return cves, nil
}

func (f *CIRCLFetcher) convert(c *circlCVE) *entity.CVE {
	cve := &entity.CVE{
		ID:          c.ID,
		Description: c.Summary,
		Summary:     c.Summary,
		Source:      entity.SourceCIRCL,
		Severity:    entity.SeverityUnknown,
		References:  c.References,
		Link:        "https://cve.circl.lu/cve/" + c.ID,
	}

	// Parse dates — CIRCL uses RFC3339 or "2021-12-10T00:00:00" format
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, c.Published); err == nil {
			cve.Published = t
			break
		}
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, c.Modified); err == nil {
			cve.Modified = t
			break
		}
	}

	// CVSS scores
	if c.CVSS3 > 0 {
		score := c.CVSS3
		cve.CVSS3Score = &score
		cve.Severity = entity.SeverityFromCVSS3(c.CVSS3)
	} else if c.CVSS > 0 {
		score := c.CVSS
		cve.CVSSScore = &score
		cve.Severity = entity.SeverityFromCVSS3(c.CVSS)
	}
	if c.CVSSVector != "" {
		cve.CVSS3Vector = c.CVSSVector
	}

	// CWE
	if c.CWE != "" {
		cve.CWE = []string{c.CWE}
	}

	// Vendors and products
	cve.Vendors = c.Vendors
	cve.Products = c.Products
	for _, vs := range c.VulnSoft {
		if vs.Vendor != "" {
			cve.Vendors = append(cve.Vendors, vs.Vendor)
		}
		if vs.Product != "" {
			cve.Products = append(cve.Products, vs.Product)
		}
	}

	return cve
}
