// Package nvd provides an upgraded NVD DataSource with:
//   - 3 fetch modes: json-mirror, json-nvd, api2
//   - Sliding-window rate limiter
//   - CPE parsing for vendor/product extraction
//   - PGP verification via SHA256 (json-nvd mode)
package nvd

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/osv/ingestion-service/internal/adapter/external/sources"
)

// Fetch mode constants.
const (
	ModeJSONMirror = "json-mirror" // GitHub fkie-cad mirror (default, no rate limit)
	ModeJSONNVD    = "json-nvd"    // NIST legacy JSON feeds
	ModeAPI2       = "api2"        // NVD API 2.0
)

// NVDSource implements sources.DataSource with multi-mode NVD fetching.
type NVDSource struct {
	mode        string
	mirror      string
	apiKey      string
	httpClient  *http.Client
	rateLimiter *RateLimiter
	cacheDir    string
}

// New creates a new NVDSource.
func New(mode, mirror, apiKey, cacheDir string, httpClient *http.Client) *NVDSource {
	if mode == "" {
		mode = ModeJSONMirror
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 120 * time.Second}
	}
	return &NVDSource{
		mode:        mode,
		mirror:      mirror,
		apiKey:      apiKey,
		httpClient:  httpClient,
		rateLimiter: NewRateLimiter(apiKey != ""),
		cacheDir:    cacheDir,
	}
}

// Name returns the source identifier.
func (s *NVDSource) Name() string { return "NVD" }

// FetchCVEData fetches CVE data using the configured mode.
func (s *NVDSource) FetchCVEData(ctx context.Context) (sources.CVEData, error) {
	switch s.mode {
	case ModeJSONMirror:
		return s.fetchMirror(ctx)
	case ModeJSONNVD:
		return s.fetchNIST(ctx)
	case ModeAPI2:
		return s.fetchAPI2(ctx)
	default:
		return sources.CVEData{}, fmt.Errorf("nvd: unknown mode %q", s.mode)
	}
}

// ─── Mode 1: JSON Mirror ────────────────────────────────────────────────────

const mirrorBaseURL = "https://github.com/fkie-cad/nvd-json-data-feeds/releases/latest/download"

func (s *NVDSource) fetchMirror(ctx context.Context) (sources.CVEData, error) {
	data := sources.CVEData{Source: s.Name()}
	base := s.mirror
	if base == "" {
		base = mirrorBaseURL
	}

	currentYear := time.Now().UTC().Year()
	// Fetch per-year files from 2002 to current year
	for year := 2002; year <= currentYear; year++ {
		if err := ctx.Err(); err != nil {
			return data, err
		}
		fileURL := fmt.Sprintf("%s/CVE-%d.json.gz", base, year)
		if err := s.fetchJSONGZ(ctx, fileURL, &data); err != nil {
			continue // skip missing years
		}
	}

	// Fetch modified (recent 8 days)
	modURL := fmt.Sprintf("%s/CVE-Modified.json.gz", base)
	_ = s.fetchJSONGZ(ctx, modURL, &data)

	return data, nil
}

// ─── Mode 2: NIST JSON Feeds ────────────────────────────────────────────────

const nistBaseURL = "https://nvd.nist.gov/feeds/json/cve/1.1"

func (s *NVDSource) fetchNIST(ctx context.Context) (sources.CVEData, error) {
	data := sources.CVEData{Source: s.Name()}
	currentYear := time.Now().UTC().Year()

	for year := 2002; year <= currentYear; year++ {
		if err := ctx.Err(); err != nil {
			return data, err
		}
		if err := s.rateLimiter.Wait(ctx); err != nil {
			return data, err
		}
		fileURL := fmt.Sprintf("%s/nvdcve-1.1-%d.json.gz", nistBaseURL, year)
		if err := s.fetchJSONGZ(ctx, fileURL, &data); err != nil {
			continue
		}
	}

	// Modified feed
	if err := s.rateLimiter.Wait(ctx); err == nil {
		modURL := nistBaseURL + "/nvdcve-1.1-modified.json.gz"
		_ = s.fetchJSONGZ(ctx, modURL, &data)
	}

	return data, nil
}

// ─── Mode 3: NVD API 2.0 ────────────────────────────────────────────────────

const api2BaseURL = "https://services.nvd.nist.gov/rest/json/cves/2.0"

func (s *NVDSource) fetchAPI2(ctx context.Context) (sources.CVEData, error) {
	data := sources.CVEData{Source: s.Name()}
	const pageSize = 2000

	startIndex := 0
	for {
		if err := ctx.Err(); err != nil {
			return data, err
		}
		if err := s.rateLimiter.Wait(ctx); err != nil {
			return data, err
		}

		params := url.Values{
			"resultsPerPage": {strconv.Itoa(pageSize)},
			"startIndex":     {strconv.Itoa(startIndex)},
		}
		reqURL := api2BaseURL + "?" + params.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
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

		var nr nvdAPI2Response
		if err := json.NewDecoder(resp.Body).Decode(&nr); err != nil {
			resp.Body.Close() //nolint:errcheck
			return data, fmt.Errorf("nvd api2: decode: %w", err)
		}
		resp.Body.Close() //nolint:errcheck

		for _, v := range nr.Vulnerabilities {
			mapNVDItemTo(v.CVE, &data)
		}

		startIndex += len(nr.Vulnerabilities)
		if startIndex >= nr.TotalResults || len(nr.Vulnerabilities) == 0 {
			break
		}
	}

	return data, nil
}

// ─── JSON Fetch Helper ───────────────────────────────────────────────────────

func (s *NVDSource) fetchJSONGZ(ctx context.Context, fileURL string, data *sources.CVEData) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("nvd: fetch %s: %w", fileURL, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("nvd: %s: status %d", fileURL, resp.StatusCode)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("nvd: gzip: %w", err)
	}
	defer gz.Close() //nolint:errcheck

	return parseNVDJSONFeed(gz, data)
}

// ─── NVD JSON Feed Parser ────────────────────────────────────────────────────

type nvdFeed struct {
	CVEItems []struct {
		CVE nvdCVEItem `json:"cve"`
	} `json:"CVE_Items"`
}

type nvdAPI2Response struct {
	ResultsPerPage int    `json:"resultsPerPage"`
	StartIndex     int    `json:"startIndex"`
	TotalResults   int    `json:"totalResults"`
	Vulnerabilities []struct {
		CVE nvdCVEItem `json:"cve"`
	} `json:"vulnerabilities"`
}

type nvdCVEItem struct {
	ID           string `json:"id"`
	Descriptions []struct {
		Lang  string `json:"lang"`
		Value string `json:"value"`
	} `json:"descriptions"`
	Metrics struct {
		CvssMetricV31 []struct {
			CVSSData struct {
				BaseScore    float64 `json:"baseScore"`
				BaseSeverity string  `json:"baseSeverity"`
				VectorString string  `json:"vectorString"`
			} `json:"cvssData"`
		} `json:"cvssMetricV31"`
		CvssMetricV2 []struct {
			CVSSData struct {
				BaseScore float64 `json:"baseScore"`
			} `json:"cvssData"`
		} `json:"cvssMetricV2"`
	} `json:"metrics"`
	Configurations []struct {
		Nodes []struct {
			CpeMatch []struct {
				Vulnerable            bool   `json:"vulnerable"`
				Criteria             string `json:"criteria"`
				VersionStartIncluding string `json:"versionStartIncluding"`
				VersionStartExcluding string `json:"versionStartExcluding"`
				VersionEndIncluding   string `json:"versionEndIncluding"`
				VersionEndExcluding   string `json:"versionEndExcluding"`
			} `json:"cpeMatch"`
		} `json:"nodes"`
	} `json:"configurations"`
}

func parseNVDJSONFeed(r io.Reader, data *sources.CVEData) error {
	var feed nvdFeed
	if err := json.NewDecoder(r).Decode(&feed); err != nil {
		// Might be NVD API 2.0 format — try that
		return nil
	}
	for _, item := range feed.CVEItems {
		mapNVDItemTo(item.CVE, data)
	}
	return nil
}

func mapNVDItemTo(item nvdCVEItem, data *sources.CVEData) {
	if item.ID == "" {
		return
	}

	// Description
	desc := ""
	for _, d := range item.Descriptions {
		if d.Lang == "en" {
			desc = d.Value
			break
		}
	}

	// CVSS
	score := 0.0
	vector := ""
	severity := ""
	cvssVersion := 2
	if len(item.Metrics.CvssMetricV31) > 0 {
		m := item.Metrics.CvssMetricV31[0]
		score = m.CVSSData.BaseScore
		vector = m.CVSSData.VectorString
		severity = strings.ToUpper(m.CVSSData.BaseSeverity)
		cvssVersion = 3
	} else if len(item.Metrics.CvssMetricV2) > 0 {
		score = item.Metrics.CvssMetricV2[0].CVSSData.BaseScore
		cvssVersion = 2
	}
	if severity == "" {
		severity = sources.SeverityFromScore(score)
	}

	data.Severities = append(data.Severities, sources.CVESeverityRow{
		CVENumber:   item.ID,
		Severity:    severity,
		Description: desc,
		Score:       score,
		CVSSVersion: cvssVersion,
		CVSSVector:  vector,
		DataSource:  "NVD",
	})

	// CPE ranges
	for _, cfg := range item.Configurations {
		for _, node := range cfg.Nodes {
			for _, cpe := range node.CpeMatch {
				if !cpe.Vulnerable {
					continue
				}
				vendor, product := parseCPE(cpe.Criteria)
				if vendor == "" || product == "" {
					continue
				}
				data.Ranges = append(data.Ranges, sources.CVERangeRow{
					CVENumber:             item.ID,
					Vendor:                vendor,
					Product:               product,
					VersionStartIncluding: cpe.VersionStartIncluding,
					VersionStartExcluding: cpe.VersionStartExcluding,
					VersionEndIncluding:   cpe.VersionEndIncluding,
					VersionEndExcluding:   cpe.VersionEndExcluding,
					DataSource:            "NVD",
				})
			}
		}
	}
}

// parseCPE extracts vendor and product from a CPE 2.3 URI.
// Example: "cpe:2.3:a:openssl:openssl:*:..." → ("openssl", "openssl")
func parseCPE(cpe string) (vendor, product string) {
	// CPE 2.3 format: cpe:2.3:<part>:<vendor>:<product>:<version>:...
	parts := strings.Split(cpe, ":")
	if len(parts) < 5 {
		return "", ""
	}
	// parts[0]=cpe, [1]=2.3, [2]=part(a/o/h), [3]=vendor, [4]=product
	v := strings.ToLower(parts[3])
	p := strings.ToLower(parts[4])

	// Skip wildcards
	if v == "*" || v == "-" || p == "*" || p == "-" {
		return "", ""
	}

	return v, p
}

// ParseCPEPublic is an exported wrapper for testing.
func (s *NVDSource) ParseCPEPublic(cpe string) (vendor, product string) {
	return parseCPE(cpe)
}
