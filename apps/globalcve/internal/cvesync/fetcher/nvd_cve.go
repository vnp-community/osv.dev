// Package fetcher — NVD CVE fetcher.
// Adapted from ingestion-service/internal/fetcher/nvd_cve.go.
// Source: https://api.nvd.nist.gov/rest/json/cves/2.0
package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/globalcve/mono/internal/cvesearch/domain/entity"
	"github.com/globalcve/mono/internal/cvesync/domain/repository"
)

const (
	nvdBaseURL  = "https://api.nvd.nist.gov/rest/json/cves/2.0"
	nvdPageSize = 2000
)

// NVDFetcher fetches CVE data from the NVD REST API v2.
type NVDFetcher struct {
	apiKey  string
	client  *http.Client
	cveRepo repository.CVEWriteRepository
}

// NewNVDFetcher creates a new NVD CVE fetcher.
func NewNVDFetcher(apiKey string, timeout time.Duration, cveRepo repository.CVEWriteRepository) *NVDFetcher {
	return &NVDFetcher{
		apiKey:  apiKey,
		cveRepo: cveRepo,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (f *NVDFetcher) Source() entity.SourceName { return entity.SourceNameNVD }

// Fetch fetches all CVEs from NVD with pagination.
// If opts.Since is set, only fetches CVEs modified since that time.
func (f *NVDFetcher) Fetch(ctx context.Context, opts FetchOptions) (int, error) {
	total := 0
	startIndex := 0

	for {
		resp, err := f.fetchPage(ctx, startIndex, opts.Since)
		if err != nil {
			return total, fmt.Errorf("nvd page %d: %w", startIndex, err)
		}

		cves := f.convertPage(resp)
		if len(cves) > 0 {
			inserted, updated, err := f.cveRepo.UpsertBatch(ctx, cves)
			if err != nil {
				return total, fmt.Errorf("upsert batch (startIndex=%d): %w", startIndex, err)
			}
			total += inserted + updated
			log.Ctx(ctx).Info().
				Int("inserted", inserted).
				Int("updated", updated).
				Int("startIndex", startIndex).
				Msg("nvd: page processed")
		}

		startIndex += nvdPageSize
		if startIndex >= resp.TotalResults {
			break
		}

		// Rate limit: 100ms between pages with API key, 500ms without
		delay := 500 * time.Millisecond
		if f.apiKey != "" {
			delay = 100 * time.Millisecond
		}
		select {
		case <-ctx.Done():
			return total, ctx.Err()
		case <-time.After(delay):
		}
	}

	return total, nil
}

// nvdResponse is the NVD REST API v2 response structure.
type nvdResponse struct {
	ResultsPerPage int        `json:"resultsPerPage"`
	StartIndex     int        `json:"startIndex"`
	TotalResults   int        `json:"totalResults"`
	Vulnerabilities []nvdItem `json:"vulnerabilities"`
}

type nvdItem struct {
	CVE nvdCVE `json:"cve"`
}

type nvdCVE struct {
	ID           string        `json:"id"`
	Published    string        `json:"published"`
	LastModified string        `json:"lastModified"`
	Descriptions []nvdDesc     `json:"descriptions"`
	Metrics      nvdMetrics    `json:"metrics"`
	Weaknesses   []nvdWeakness `json:"weaknesses"`
	References   []nvdRef      `json:"references"`
}

type nvdDesc struct {
	Lang  string `json:"lang"`
	Value string `json:"value"`
}

type nvdMetrics struct {
	CVSSMetricV31 []nvdCVSSv3 `json:"cvssMetricV31"`
	CVSSMetricV30 []nvdCVSSv3 `json:"cvssMetricV30"`
	CVSSMetricV2  []nvdCVSSv2 `json:"cvssMetricV2"`
}

type nvdCVSSv3 struct {
	CvssData nvdCVSSv3Data `json:"cvssData"`
}

type nvdCVSSv3Data struct {
	BaseScore    float64 `json:"baseScore"`
	VectorString string  `json:"vectorString"`
}

type nvdCVSSv2 struct {
	CvssData nvdCVSSv2Data `json:"cvssData"`
}

type nvdCVSSv2Data struct {
	BaseScore    float64 `json:"baseScore"`
	VectorString string  `json:"vectorString"`
}

type nvdWeakness struct {
	Description []nvdDesc `json:"description"`
}

type nvdRef struct {
	URL string `json:"url"`
}

func (f *NVDFetcher) fetchPage(ctx context.Context, startIndex int, since *time.Time) (*nvdResponse, error) {
	params := url.Values{}
	params.Set("resultsPerPage", fmt.Sprintf("%d", nvdPageSize))
	params.Set("startIndex", fmt.Sprintf("%d", startIndex))
	if since != nil {
		params.Set("lastModStartDate", since.UTC().Format("2006-01-02T15:04:05.000"))
		params.Set("lastModEndDate", time.Now().UTC().Format("2006-01-02T15:04:05.000"))
	}

	reqURL := nvdBaseURL + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	if f.apiKey != "" {
		req.Header.Set("apiKey", f.apiKey)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("nvd api status %d: %s", resp.StatusCode, body)
	}

	var result nvdResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

func (f *NVDFetcher) convertPage(resp *nvdResponse) []*entity.CVE {
	cves := make([]*entity.CVE, 0, len(resp.Vulnerabilities))
	for _, item := range resp.Vulnerabilities {
		cve := f.convertItem(item.CVE)
		cves = append(cves, cve)
	}
	return cves
}

func (f *NVDFetcher) convertItem(item nvdCVE) *entity.CVE {
	cve := &entity.CVE{
		ID:       item.ID,
		Source:   entity.SourceNVD,
		Severity: entity.SeverityUnknown,
	}

	// Parse dates
	if t, err := time.Parse("2006-01-02T15:04:05.000", item.Published); err == nil {
		cve.Published = t
	}
	if t, err := time.Parse("2006-01-02T15:04:05.000", item.LastModified); err == nil {
		cve.Modified = t
	}

	// English description
	for _, d := range item.Descriptions {
		if d.Lang == "en" {
			cve.Description = d.Value
			break
		}
	}

	// CVSS v3.1 (prefer over v3.0)
	if len(item.Metrics.CVSSMetricV31) > 0 {
		score := item.Metrics.CVSSMetricV31[0].CvssData.BaseScore
		vec := item.Metrics.CVSSMetricV31[0].CvssData.VectorString
		cve.CVSS3Score = &score
		cve.CVSS3Vector = vec
		cve.Severity = entity.SeverityFromCVSS3(score)
	} else if len(item.Metrics.CVSSMetricV30) > 0 {
		score := item.Metrics.CVSSMetricV30[0].CvssData.BaseScore
		vec := item.Metrics.CVSSMetricV30[0].CvssData.VectorString
		cve.CVSS3Score = &score
		cve.CVSS3Vector = vec
		cve.Severity = entity.SeverityFromCVSS3(score)
	}

	// CVSS v2
	if len(item.Metrics.CVSSMetricV2) > 0 {
		score := item.Metrics.CVSSMetricV2[0].CvssData.BaseScore
		vec := item.Metrics.CVSSMetricV2[0].CvssData.VectorString
		cve.CVSSScore = &score
		cve.CVSSVector = vec
	}

	// CWE IDs
	for _, w := range item.Weaknesses {
		for _, d := range w.Description {
			if d.Lang == "en" && d.Value != "NVD-CWE-Other" && d.Value != "NVD-CWE-noinfo" {
				cve.CWE = append(cve.CWE, d.Value)
			}
		}
	}

	// References
	for _, ref := range item.References {
		if ref.URL != "" {
			cve.References = append(cve.References, ref.URL)
		}
	}

	// Link to NVD page
	cve.Link = "https://nvd.nist.gov/vuln/detail/" + item.ID

	return cve
}
