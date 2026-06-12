// Package circl provides a client for the CIRCL CVE API (CVE search engine).
package circl

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/osv/data-service/internal/domain/entity"
)

const (
	defaultBaseURL = "https://cve.circl.lu/api/last"
	defaultLimit   = 1000 // fetch last N updated CVEs
)

// Client fetches CVE records from the CIRCL CVE API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a CIRCL client.
func NewClient() *Client {
	return &Client{
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// circlCVE is the raw CIRCL record.
type circlCVE struct {
	ID           string   `json:"id"`
	Summary      string   `json:"summary"`
	Published    string   `json:"Published"`    // "2021-12-09 17:15:00.000000"
	Cvss         float64  `json:"cvss"`
	CvssVector   string   `json:"cvss-vector"`
	References   []string `json:"references"`
}

// FetchLatest fetches the latest N CVEs from CIRCL.
func (c *Client) FetchLatest(ctx context.Context) ([]*entity.CVE, error) {
	reqURL := fmt.Sprintf("%s/%d", c.baseURL, defaultLimit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("circl: request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("circl: status %d", resp.StatusCode)
	}

	var records []circlCVE
	if err := json.NewDecoder(resp.Body).Decode(&records); err != nil {
		return nil, fmt.Errorf("circl: decode: %w", err)
	}

	cves := make([]*entity.CVE, 0, len(records))
	for _, r := range records {
		cve := convert(r)
		cves = append(cves, cve)
	}
	return cves, nil
}

func convert(r circlCVE) *entity.CVE {
	cve := &entity.CVE{
		ID:          r.ID,
		Description: r.Summary,
		Source:      entity.SourceCIRCL,
		Link:        "https://cve.circl.lu/cve/" + r.ID,
	}
	if t, err := time.Parse("2006-01-02 15:04:05.000000", r.Published); err == nil {
		cve.Published = &t
	}
	if r.Cvss > 0 {
		score := r.Cvss
		cve.CVSSScore = &score
	}
	cve.InferSeverity("")
	return cve
}
