// Package fetcher — EPSS (Exploit Prediction Scoring System) fetcher.
// Adapted from ingestion-service/internal/fetcher/epss.go.
// Source: https://epss.cyentia.com/epss_scores-current.csv.gz
package fetcher

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

	"github.com/rs/zerolog/log"

	"github.com/globalcve/mono/internal/cvesync/domain/entity"
	"github.com/globalcve/mono/internal/cvesync/domain/repository"
)

const epssBaseURL = "https://epss.cyentia.com/epss_scores-current.csv.gz"

// EPSSFetcher downloads the EPSS scores CSV from FIRST.org and updates CVE EPSS data.
type EPSSFetcher struct {
	url     string
	client  *http.Client
	cveRepo repository.CVEWriteRepository
}

// NewEPSSFetcher creates a new EPSS fetcher.
func NewEPSSFetcher(url string, timeout time.Duration, cveRepo repository.CVEWriteRepository) *EPSSFetcher {
	if url == "" {
		url = epssBaseURL
	}
	return &EPSSFetcher{
		url:     url,
		cveRepo: cveRepo,
		client:  &http.Client{Timeout: timeout},
	}
}

func (f *EPSSFetcher) Source() entity.SourceName { return entity.SourceNameEPSS }

// Fetch downloads the gzipped EPSS CSV and updates EPSS scores for all CVEs.
func (f *EPSSFetcher) Fetch(ctx context.Context, opts FetchOptions) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.url, nil)
	if err != nil {
		return 0, err
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("epss http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("epss status %d", resp.StatusCode)
	}

	// Decompress gzip
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("epss gzip: %w", err)
	}
	defer gz.Close()

	reader := csv.NewReader(gz)
	reader.Comment = '#'

	// Read header: cve,epss,percentile
	header, err := reader.Read()
	if err != nil {
		return 0, fmt.Errorf("epss header: %w", err)
	}

	// Find column indexes
	var cveIdx, epssIdx, pctIdx int
	for i, h := range header {
		switch strings.ToLower(strings.TrimSpace(h)) {
		case "cve":
			cveIdx = i
		case "epss":
			epssIdx = i
		case "percentile":
			pctIdx = i
		}
	}

	// Batch updates
	const batchSize = 5000
	batch := make([]repository.EPSSUpdate, 0, batchSize)
	total := 0
	logEvery := 10000
	count := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		cveID := strings.TrimSpace(record[cveIdx])
		if !strings.HasPrefix(cveID, "CVE-") {
			continue
		}

		epss, err := strconv.ParseFloat(strings.TrimSpace(record[epssIdx]), 64)
		if err != nil {
			continue
		}
		percentile, err := strconv.ParseFloat(strings.TrimSpace(record[pctIdx]), 64)
		if err != nil {
			continue
		}

		batch = append(batch, repository.EPSSUpdate{
			CVEID:      cveID,
			Score:      epss,
			Percentile: percentile,
		})
		count++

		if count%logEvery == 0 {
			log.Ctx(ctx).Info().Int("processed", count).Msg("epss: progress")
		}

		if len(batch) >= batchSize {
			if err := f.cveRepo.UpdateEPSS(ctx, batch); err != nil {
				log.Ctx(ctx).Warn().Err(err).Msg("epss: update batch error")
			} else {
				total += len(batch)
			}
			batch = batch[:0]

			select {
			case <-ctx.Done():
				return total, ctx.Err()
			default:
			}
		}
	}

	// Flush remaining
	if len(batch) > 0 {
		if err := f.cveRepo.UpdateEPSS(ctx, batch); err != nil {
			log.Ctx(ctx).Warn().Err(err).Msg("epss: final batch error")
		} else {
			total += len(batch)
		}
	}

	log.Ctx(ctx).Info().Int("total", total).Msg("epss: completed")
	return total, nil
}
