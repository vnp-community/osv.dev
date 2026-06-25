// Package fetcher — VIA4 cross-reference fetcher.
// Downloads VIA4 (Vulnerability Information Aggregated 4) feed from cve-search.org.
// Source: https://www.cve-search.org/feeds/via4.json
// Format: {"CVE-YYYY-NNNN": {"aliases": [...], "refs": [...], "bugs": [...], "msf": [...]}, ...}
// Mirrors Python: fetcher/via4.py in cve-search
package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const via4FeedURL = "https://www.cve-search.org/feeds/via4.json"

// VIA4Fetcher downloads and stores VIA4 cross-reference data.
// VIA4 data is stored in a dedicated "via4" collection (not merged into "cves").
// The "via4" collection schema: {id: "CVE-XXXX-NNNN", data: <raw cross-ref JSON>}
type VIA4Fetcher struct {
	db     *mongo.Database
	url    string
	client *http.Client
}

// NewVIA4Fetcher creates a VIA4 fetcher.
func NewVIA4Fetcher(db *mongo.Database) *VIA4Fetcher {
	return &VIA4Fetcher{
		db:     db,
		url:    via4FeedURL,
		client: &http.Client{Timeout: 2 * time.Minute},
	}
}

// Name implements Fetcher.
func (f *VIA4Fetcher) Name() string { return "via4" }

// FetchAndStore downloads VIA4 JSON feed and upserts into "via4" collection.
// VIA4 JSON format: {"CVE-2021-44228": {"aliases": [...], "refs": [...], ...}, ...}
// Each CVE ID maps to cross-reference data (aliases, references, Metasploit modules, bugs).
func (f *VIA4Fetcher) FetchAndStore(ctx context.Context, opts FetchOptions) (int, error) {
	log.Info().Str("url", f.url).Msg("Fetching VIA4 cross-reference feed")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.url, nil)
	if err != nil {
		return 0, fmt.Errorf("via4 build request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("via4 download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("via4 API returned %d", resp.StatusCode)
	}

	// VIA4 JSON: map of CVE-ID → raw cross-reference data
	var via4Data map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&via4Data); err != nil {
		return 0, fmt.Errorf("via4 decode JSON: %w", err)
	}

	log.Info().Int("entries", len(via4Data)).Msg("VIA4 download complete, upserting")

	col := f.db.Collection("via4")
	count := 0
	bulkOps := make([]mongo.WriteModel, 0, 1000)

	for cveID, rawData := range via4Data {
		if cveID == "" {
			continue
		}

		bulkOps = append(bulkOps, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"id": cveID}).
			SetUpdate(bson.M{"$set": bson.M{
				"id":   cveID,
				"data": rawData,
			}}).
			SetUpsert(true),
		)
		count++

		// Flush every 1000 records to avoid large memory allocation
		if len(bulkOps) >= 1000 {
			if _, err := col.BulkWrite(ctx, bulkOps, options.BulkWrite().SetOrdered(false)); err != nil {
				log.Warn().Err(err).Msg("via4 bulk write batch error (partial failure)")
			}
			bulkOps = bulkOps[:0]
			log.Info().Int("processed", count).Msg("VIA4 upsert progress")
		}
	}

	// Flush remaining records
	if len(bulkOps) > 0 {
		if _, err := col.BulkWrite(ctx, bulkOps, options.BulkWrite().SetOrdered(false)); err != nil {
			log.Warn().Err(err).Msg("via4 final bulk write error")
		}
	}

	log.Info().Int("count", count).Msg("VIA4 fetch complete")
	return count, nil
}
