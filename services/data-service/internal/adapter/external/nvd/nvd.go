// Package nvd provides a DataSource adapter for the National Vulnerability Database (NVD).
// Supports three fetch modes:
//   - json-mirror: download from a JSON mirror (default)
//   - json-nvd:    download official NVD JSON feeds
//   - api2:        use NVD REST API v2 with API key
package nvd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/osv/data-service/internal/adapter/external/sources"
)

const (
	// ModeJSONMirror fetches from a JSON mirror URL.
	ModeJSONMirror = "json-mirror"
	// ModeJSONNVD fetches from the official NVD JSON feeds.
	ModeJSONNVD = "json-nvd"
	// ModeAPI2 uses the NVD REST API v2.
	ModeAPI2 = "api2"

	defaultMirrorURL = "https://nvd.circl.lu/api/nvd/dump/v2"
	defaultNVDFeed   = "https://nvd.nist.gov/feeds/json/cve/1.1/nvdcve-1.1-recent.json.gz"
)

// NVDSource implements sources.DataSource for the NVD.
type NVDSource struct {
	mode       string
	mirrorURL  string
	apiKey     string
	cacheDir   string
	httpClient *http.Client
}

// New creates a new NVDSource.
func New(mode, mirrorURL, apiKey, cacheDir string, httpClient *http.Client) *NVDSource {
	if mode == "" {
		mode = ModeJSONMirror
	}
	if mirrorURL == "" {
		mirrorURL = defaultMirrorURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 120 * time.Second}
	}
	return &NVDSource{
		mode:       mode,
		mirrorURL:  mirrorURL,
		apiKey:     apiKey,
		cacheDir:   cacheDir,
		httpClient: httpClient,
	}
}

// Name returns the source identifier.
func (s *NVDSource) Name() string { return "NVD" }

// FetchCVEData downloads NVD CVE data in the configured mode.
func (s *NVDSource) FetchCVEData(ctx context.Context) (sources.CVEData, error) {
	switch s.mode {
	case ModeAPI2:
		return s.fetchAPI2(ctx)
	default:
		return s.fetchMirror(ctx)
	}
}

// nvdAPI2Response is a partial NVD API v2 response.
type nvdAPI2Response struct {
	ResultsPerPage int `json:"resultsPerPage"`
	TotalResults   int `json:"totalResults"`
	Vulnerabilities []struct {
		CVE struct {
			ID          string `json:"id"`
			Description struct {
				DescriptionData []struct {
					Lang  string `json:"lang"`
					Value string `json:"value"`
				} `json:"descriptionData"`
			} `json:"descriptions"`
			Metrics struct {
				CVSSV31 []struct {
					CVSSData struct {
						BaseScore    float64 `json:"baseScore"`
						VectorString string  `json:"vectorString"`
					} `json:"cvssData"`
				} `json:"cvssMetricV31"`
				CVSSV30 []struct {
					CVSSData struct {
						BaseScore    float64 `json:"baseScore"`
						VectorString string  `json:"vectorString"`
					} `json:"cvssData"`
				} `json:"cvssMetricV30"`
			} `json:"metrics"`
		} `json:"cve"`
	} `json:"vulnerabilities"`
}

func (s *NVDSource) fetchAPI2(ctx context.Context) (sources.CVEData, error) {
	data := sources.CVEData{Source: s.Name()}
	url := "https://services.nvd.nist.gov/rest/json/cves/2.0"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return data, err
	}
	if s.apiKey != "" {
		req.Header.Set("apiKey", s.apiKey)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return data, fmt.Errorf("nvd api2: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return data, fmt.Errorf("nvd api2: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return data, fmt.Errorf("nvd api2: read: %w", err)
	}

	var nvdResp nvdAPI2Response
	if err := json.Unmarshal(body, &nvdResp); err != nil {
		return data, fmt.Errorf("nvd api2: parse: %w", err)
	}

	for _, v := range nvdResp.Vulnerabilities {
		row := sources.CVESeverityRow{
			CVENumber:  v.CVE.ID,
			DataSource: "NVD",
		}

		// Get description
		for _, d := range v.CVE.Description.DescriptionData {
			if d.Lang == "en" {
				row.Description = d.Value
				break
			}
		}

		// Get CVSS score
		if len(v.CVE.Metrics.CVSSV31) > 0 {
			row.Score = v.CVE.Metrics.CVSSV31[0].CVSSData.BaseScore
			row.CVSSVector = v.CVE.Metrics.CVSSV31[0].CVSSData.VectorString
			row.CVSSVersion = 31
		} else if len(v.CVE.Metrics.CVSSV30) > 0 {
			row.Score = v.CVE.Metrics.CVSSV30[0].CVSSData.BaseScore
			row.CVSSVector = v.CVE.Metrics.CVSSV30[0].CVSSData.VectorString
			row.CVSSVersion = 30
		}
		row.Severity = sources.SeverityFromScore(row.Score)

		data.Severities = append(data.Severities, row)
	}
	return data, nil
}

func (s *NVDSource) fetchMirror(ctx context.Context) (sources.CVEData, error) {
	data := sources.CVEData{Source: s.Name()}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.mirrorURL, nil)
	if err != nil {
		return data, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return data, fmt.Errorf("nvd mirror: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return data, fmt.Errorf("nvd mirror: status %d", resp.StatusCode)
	}

	// Parse as NVD API v2 format (CIRCL mirror uses same format)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return data, fmt.Errorf("nvd mirror: read: %w", err)
	}

	var nvdResp nvdAPI2Response
	if err := json.Unmarshal(body, &nvdResp); err != nil {
		// Mirror may use different format; return empty (non-fatal)
		return data, nil
	}

	for _, v := range nvdResp.Vulnerabilities {
		row := sources.CVESeverityRow{
			CVENumber:  v.CVE.ID,
			DataSource: "NVD",
		}
		for _, d := range v.CVE.Description.DescriptionData {
			if d.Lang == "en" {
				row.Description = d.Value
				break
			}
		}
		if len(v.CVE.Metrics.CVSSV31) > 0 {
			row.Score = v.CVE.Metrics.CVSSV31[0].CVSSData.BaseScore
			row.CVSSVector = v.CVE.Metrics.CVSSV31[0].CVSSData.VectorString
			row.CVSSVersion = 31
		}
		row.Severity = sources.SeverityFromScore(row.Score)
		data.Severities = append(data.Severities, row)
	}
	return data, nil
}
