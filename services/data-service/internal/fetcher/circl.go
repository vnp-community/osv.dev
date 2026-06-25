// Package fetcher — circl.go
// CIRCL CVE fetcher: downloads from https://cve.circl.lu/api/
// Provides a Luxembourg-operated CVE mirror useful as NVD fallback.
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
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
)

const circlBaseURL = "https://cve.circl.lu/api"

// CIRCLFetcher fetches CVE data from the CIRCL (cve.circl.lu) API.
// CIRCL provides data in a format compatible with the cve-search project.
type CIRCLFetcher struct {
	db      *mongo.Database
	baseURL string
	client  *http.Client
}

// NewCIRCLFetcher creates a CIRCL fetcher.
func NewCIRCLFetcher(db *mongo.Database) *CIRCLFetcher {
	return &CIRCLFetcher{
		db:      db,
		baseURL: circlBaseURL,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

// Name implements Fetcher. Returns "CIRCL".
func (f *CIRCLFetcher) Name() string { return string(SourceCIRCL) }

// FetchAndStore implements Fetcher.
func (f *CIRCLFetcher) FetchAndStore(ctx context.Context, opts FetchOptions) (int, error) {
	year := time.Now().Year()
	if opts.StartYear > 0 {
		year = opts.StartYear
	}
	return f.fetchByKeyword(ctx, fmt.Sprintf("%d", year))
}

// FetchSince implements IncrementalFetcher.
func (f *CIRCLFetcher) FetchSince(ctx context.Context, since time.Time) (int, error) {
	return f.fetchByKeyword(ctx, fmt.Sprintf("%d", since.Year()))
}

func (f *CIRCLFetcher) fetchByKeyword(ctx context.Context, keyword string) (int, error) {
	col := f.db.Collection("cves")
	total := 0

	apiURL := fmt.Sprintf("%s/search/%s", f.baseURL, keyword)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := f.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("CIRCL fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("CIRCL API returned %d", resp.StatusCode)
	}

	var rawData interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rawData); err != nil {
		return 0, fmt.Errorf("CIRCL decode: %w", err)
	}

	items := extractItems(rawData)

	for _, cve := range items {
		if ctx.Err() != nil {
			return total, ctx.Err()
		}
		id, _ := cve["id"].(string)
		if id == "" {
			continue
		}

		// Enrich with source attribution
		cve["source"] = string(SourceCIRCL)

		_, err := col.UpdateOne(ctx,
			bson.M{"id": id},
			bson.M{"$set": cve},
			mongoopts.Update().SetUpsert(true),
		)
		if err != nil {
			log.Warn().Err(err).Str("cve", id).Msg("CIRCL: upsert failed")
			continue
		}
		total++
	}

	log.Info().Int("count", total).Msg("CIRCL fetch complete")
	return total, nil
}

// extractItems normalizes CIRCL response (array or single object)
func extractItems(raw interface{}) []map[string]interface{} {
	switch v := raw.(type) {
	case []interface{}:
		result := make([]map[string]interface{}, 0, len(v))
		for _, elem := range v {
			if m, ok := elem.(map[string]interface{}); ok {
				result = append(result, m)
			}
		}
		return result
	case map[string]interface{}:
		return []map[string]interface{}{v}
	}
	return nil
}
