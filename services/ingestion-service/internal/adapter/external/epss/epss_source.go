// Package epss implements a DataSource for EPSS (Exploit Prediction Scoring System) scores.
// Downloads daily CSV.gz from epss.cyentia.com and maps to CVEMetricRow.
package epss

import (
	"compress/gzip"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/osv/ingestion-service/internal/adapter/external/sources"
)

const defaultBaseURL = "https://epss.cyentia.com"

// EPSSSource implements sources.DataSource for EPSS scores.
type EPSSSource struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a new EPSSSource.
func New(baseURL string, httpClient *http.Client) *EPSSSource {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 120 * time.Second}
	}
	return &EPSSSource{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
	}
}

// Name returns the source identifier.
func (s *EPSSSource) Name() string { return "EPSS" }

// FetchCVEData downloads today's EPSS CSV.gz and returns CVEMetricRow list.
func (s *EPSSSource) FetchCVEData(ctx context.Context) (sources.CVEData, error) {
	data := sources.CVEData{Source: s.Name()}

	date := time.Now().UTC().Format("2006-01-02")
	url := fmt.Sprintf("%s/epss_scores-%s.csv.gz", s.baseURL, date)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return data, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return data, fmt.Errorf("epss: fetch: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return data, fmt.Errorf("epss: status %d for %s", resp.StatusCode, url)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return data, fmt.Errorf("epss: gzip: %w", err)
	}
	defer gz.Close() //nolint:errcheck

	metrics, err := parseCSV(gz)
	if err != nil {
		return data, fmt.Errorf("epss: parse: %w", err)
	}

	data.Metrics = metrics
	return data, nil
}

// parseCSV parses the EPSS CSV with format: cve,epss,percentile
// First line is a comment (#model_version:...), second line is header.
func parseCSV(r io.Reader) ([]sources.CVEMetricRow, error) {
	cr := csv.NewReader(r)
	cr.Comment = '#'

	// Skip header row
	if _, err := cr.Read(); err != nil {
		return nil, fmt.Errorf("epss: read header: %w", err)
	}

	var metrics []sources.CVEMetricRow
	for {
		record, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("epss: read row: %w", err)
		}
		if len(record) < 3 {
			continue
		}

		cveNum := strings.TrimSpace(record[0])
		if !strings.HasPrefix(cveNum, "CVE-") {
			continue
		}

		probability, err := strconv.ParseFloat(strings.TrimSpace(record[1]), 64)
		if err != nil {
			continue
		}

		percentile := strings.TrimSpace(record[2])

		metrics = append(metrics, sources.CVEMetricRow{
			CVENumber:   cveNum,
			MetricID:    sources.MetricIDEPSS,
			MetricScore: probability,
			MetricField: percentile,
			DataSource:  "EPSS",
		})
	}

	return metrics, nil
}
