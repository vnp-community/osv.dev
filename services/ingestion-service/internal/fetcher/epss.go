// Package fetcher — EPSS CSV fetcher.
// Downloads Exploit Prediction Scoring System (EPSS) data from FIRST.org.
package fetcher

import (
	"bufio"
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
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// EPSS daily scores from FIRST.org (gzipped CSV).
// Format: model_version,score_date
// cve,epss,percentile
const epssBaseURL = "https://epss.cyentia.com/epss_scores-current.csv.gz"

// EPSSFetcher downloads and stores EPSS scores from FIRST.org.
type EPSSFetcher struct {
	db     *mongo.Database
	url    string
	client *http.Client
}

// NewEPSSFetcher creates an EPSS fetcher.
func NewEPSSFetcher(db *mongo.Database) *EPSSFetcher {
	return &EPSSFetcher{
		db:     db,
		url:    epssBaseURL,
		client: &http.Client{Timeout: 5 * time.Minute},
	}
}

func (f *EPSSFetcher) Name() string { return "epss" }

// FetchAndStore downloads EPSS scores and updates epss/epssPercentile fields in "cves" collection.
// EPSS data is stored in-place on CVE documents (not a separate collection).
func (f *EPSSFetcher) FetchAndStore(ctx context.Context, opts FetchOptions) (int, error) {
	log.Info().Str("url", f.url).Msg("Fetching EPSS scores from FIRST.org")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("download EPSS: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("EPSS API returned %d", resp.StatusCode)
	}

	// Decompress gzip
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("decompress EPSS: %w", err)
	}
	defer gz.Close()

	return f.processCSV(ctx, gz)
}

func (f *EPSSFetcher) processCSV(ctx context.Context, r io.Reader) (int, error) {
	col := f.db.Collection("cves")

	scanner := bufio.NewScanner(r)
	// Skip header comment lines starting with '#'
	var dataLines []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		dataLines = append(dataLines, line)
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("read EPSS CSV: %w", err)
	}

	// Parse CSV from remaining lines
	csvReader := csv.NewReader(strings.NewReader(strings.Join(dataLines, "\n")))
	csvReader.TrimLeadingSpace = true

	// Skip header row: cve,epss,percentile
	if _, err := csvReader.Read(); err != nil {
		return 0, fmt.Errorf("read EPSS CSV header: %w", err)
	}

	count := 0
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Warn().Err(err).Msg("skip malformed EPSS record")
			continue
		}
		if len(record) < 3 {
			continue
		}

		cveID := strings.TrimSpace(record[0])
		epss := strings.TrimSpace(record[1])
		percentile := strings.TrimSpace(record[2])

		// Validate CVE ID format
		if !strings.HasPrefix(cveID, "CVE-") {
			continue
		}

		// Convert percentile to float for storage
		pctFloat, err := strconv.ParseFloat(percentile, 64)
		if err != nil {
			pctFloat = 0
		}
		epssFloat, err := strconv.ParseFloat(epss, 64)
		if err != nil {
			epssFloat = 0
		}

		_, err = col.UpdateOne(ctx,
			bson.M{"id": cveID},
			bson.M{"$set": bson.M{
				"epss":           fmt.Sprintf("%.6f", epssFloat),
				"epssPercentile": fmt.Sprintf("%.6f", pctFloat),
			}},
			options.Update().SetUpsert(false), // only update existing CVEs
		)
		if err != nil {
			log.Warn().Err(err).Str("cve", cveID).Msg("update EPSS score failed")
			continue
		}
		count++

		// Log progress every 10k records
		if count%10000 == 0 {
			log.Info().Int("processed", count).Msg("EPSS update progress")
		}
	}

	log.Info().Int("count", count).Msg("EPSS fetch complete")
	return count, nil
}
