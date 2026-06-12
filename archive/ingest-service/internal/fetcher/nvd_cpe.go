// Package fetcher — NVD CPE API v2.0 fetcher.
// Fetches Common Platform Enumeration (CPE) data from NVD and stores in MongoDB.
package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const nvdCPEAPI = "https://services.nvd.nist.gov/rest/json/cpes/2.0"

// NVDCPEFetcher fetches CPE data from NVD API v2.0.
type NVDCPEFetcher struct {
	db        *mongo.Database
	apiKey    string
	client    *http.Client
}

// NewNVDCPEFetcher creates a CPE fetcher.
func NewNVDCPEFetcher(db *mongo.Database, apiKey string) *NVDCPEFetcher {
	return &NVDCPEFetcher{
		db:     db,
		apiKey: apiKey,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (f *NVDCPEFetcher) Name() string { return "cpe" }

// FetchAndStore fetches CPE data from NVD and upserts into MongoDB "cpe" collection.
func (f *NVDCPEFetcher) FetchAndStore(ctx context.Context, opts FetchOptions) (int, error) {
	col := f.db.Collection("cpe")
	total := 0
	startIndex, pageSize := 0, 2000

	for {
		url := fmt.Sprintf("%s?startIndex=%d&resultsPerPage=%d", nvdCPEAPI, startIndex, pageSize)
		if opts.ManualDays > 0 {
			end := time.Now()
			start := end.AddDate(0, 0, -opts.ManualDays)
			url += fmt.Sprintf("&lastModStartDate=%s&lastModEndDate=%s",
				start.Format(time.RFC3339), end.Format(time.RFC3339))
		}

		resp, err := f.fetchPage(ctx, url)
		if err != nil {
			return total, fmt.Errorf("NVD CPE page %d: %w", startIndex/pageSize, err)
		}

		for _, item := range resp.Products {
			cpe := item.CPE
			doc := bson.M{
				"cpeName":      cpe.CPEName,
				"cpeNameId":    cpe.CPENameID,
				"title":        extractTitle(cpe.Titles),
				"lastModified": cpe.LastModified,
				"created":      cpe.Created,
				"deprecated":   cpe.Deprecated,
			}

			// Extract vendor/product from CPE name: cpe:2.3:a:vendor:product:...
			parts := strings.Split(cpe.CPEName, ":")
			if len(parts) >= 5 {
				doc["vendor"] = parts[3]
				doc["product"] = parts[4]
			}
			if len(parts) >= 3 {
				doc["cpeType"] = parts[2] // "a", "o", "h"
			}

			_, err := col.UpdateOne(ctx,
				bson.M{"cpeName": cpe.CPEName},
				bson.M{"$set": doc},
				options.Update().SetUpsert(true),
			)
			if err != nil {
				log.Warn().Err(err).Str("cpe", cpe.CPEName).Msg("upsert CPE failed")
				continue
			}
			total++
		}

		log.Info().Int("page", startIndex/pageSize).Int("fetched", len(resp.Products)).
			Int("total_so_far", total).Msg("CPE page processed")

		startIndex += pageSize
		if startIndex >= resp.TotalResults {
			break
		}

		// Rate limiting: 50 req/30s with key, 5 req/30s without
		delay := 6 * time.Second
		if f.apiKey != "" {
			delay = 600 * time.Millisecond
		}
		select {
		case <-ctx.Done():
			return total, ctx.Err()
		case <-time.After(delay):
		}
	}

	log.Info().Int("total", total).Msg("NVD CPE fetch complete")
	return total, nil
}

func (f *NVDCPEFetcher) fetchPage(ctx context.Context, url string) (*nvdCPEResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if f.apiKey != "" {
		req.Header.Set("apiKey", f.apiKey)
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		time.Sleep(30 * time.Second)
		return f.fetchPage(ctx, url)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("NVD CPE API returned %d for %s", resp.StatusCode, url)
	}
	var result nvdCPEResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode NVD CPE response: %w", err)
	}
	return &result, nil
}

func extractTitle(titles []struct {
	Title string `json:"title"`
	Lang  string `json:"lang"`
}) string {
	for _, t := range titles {
		if t.Lang == "en" || t.Lang == "en-US" {
			return t.Title
		}
	}
	if len(titles) > 0 {
		return titles[0].Title
	}
	return ""
}

// nvdCPEResponse matches NVD CPE API v2.0 JSON structure.
type nvdCPEResponse struct {
	TotalResults int `json:"totalResults"`
	Products     []struct {
		CPE struct {
			CPEName      string `json:"cpeName"`
			CPENameID    string `json:"cpeNameId"`
			LastModified string `json:"lastModified"`
			Created      string `json:"created"`
			Deprecated   bool   `json:"deprecated"`
			Titles       []struct {
				Title string `json:"title"`
				Lang  string `json:"lang"`
			} `json:"titles"`
		} `json:"cpe"`
	} `json:"products"`
}
