// Package curl implements a DataSource for curl security vulnerabilities.
// Downloads vuln.json from curl.se and maps entries to CVE rows.
package curl

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/osv/ingestion-service/internal/adapter/external/sources"
)

const defaultBaseURL = "https://curl.se/docs/vuln.json"

// CURLSource implements sources.DataSource for curl vulnerabilities.
type CURLSource struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a new CURLSource.
func New(baseURL string, httpClient *http.Client) *CURLSource {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &CURLSource{
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

// Name returns the source identifier.
func (s *CURLSource) Name() string { return "CURL" }

// FetchCVEData downloads and parses curl's vuln.json.
func (s *CURLSource) FetchCVEData(ctx context.Context) (sources.CVEData, error) {
	data := sources.CVEData{Source: s.Name()}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL, nil)
	if err != nil {
		return data, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return data, fmt.Errorf("curl: fetch: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return data, fmt.Errorf("curl: status %d", resp.StatusCode)
	}

	var vulns []curlVuln
	if err := json.NewDecoder(resp.Body).Decode(&vulns); err != nil {
		return data, fmt.Errorf("curl: decode: %w", err)
	}

	for _, v := range vulns {
		sev, ranges := mapVuln(v)
		if sev != nil {
			data.Severities = append(data.Severities, *sev)
		}
		data.Ranges = append(data.Ranges, ranges...)
	}

	return data, nil
}

// curlVuln is the curl vuln.json entry.
type curlVuln struct {
	ID           string   `json:"id"`         // CVE-YYYY-NNNN
	Aliases      []string `json:"aliases"`    // additional aliases
	URL          string   `json:"url"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Severity     string   `json:"severity"`
	CVSSScore    float64  `json:"cvss"`
	CVSSVector   string   `json:"cvssV3"`
	AffectedVersions []struct {
		Introduced string `json:"introduced"`
		Fixed      string `json:"fixed"`
	} `json:"affected"`
	Fixed string `json:"fixed"`    // "7.82.0" - first fixed version
}

func mapVuln(v curlVuln) (*sources.CVESeverityRow, []sources.CVERangeRow) {
	cveNum := v.ID
	if !strings.HasPrefix(cveNum, "CVE-") {
		// Try aliases
		for _, a := range v.Aliases {
			if strings.HasPrefix(a, "CVE-") {
				cveNum = a
				break
			}
		}
	}
	if !strings.HasPrefix(cveNum, "CVE-") {
		return nil, nil
	}

	severity := strings.ToUpper(v.Severity)
	if severity == "" && v.CVSSScore > 0 {
		severity = sources.SeverityFromScore(v.CVSSScore)
	}

	desc := v.Description
	if desc == "" {
		desc = v.Title
	}

	sev := &sources.CVESeverityRow{
		CVENumber:   cveNum,
		Severity:    severity,
		Description: desc,
		Score:       v.CVSSScore,
		CVSSVersion: 3,
		CVSSVector:  v.CVSSVector,
		DataSource:  "CURL",
	}

	var ranges []sources.CVERangeRow

	// Use explicit affected range entries
	for _, aff := range v.AffectedVersions {
		r := sources.CVERangeRow{
			CVENumber:  cveNum,
			Vendor:     "haxx",
			Product:    "curl",
			DataSource: "CURL",
		}
		if aff.Introduced != "" {
			r.VersionStartIncluding = aff.Introduced
		}
		if aff.Fixed != "" {
			r.VersionEndExcluding = aff.Fixed
		}
		ranges = append(ranges, r)
	}

	// Fallback: if no explicit ranges but have a fixed version
	if len(ranges) == 0 && v.Fixed != "" {
		ranges = append(ranges, sources.CVERangeRow{
			CVENumber:           cveNum,
			Vendor:              "haxx",
			Product:             "curl",
			VersionEndExcluding: v.Fixed,
			DataSource:          "CURL",
		})
	}

	return sev, ranges
}
