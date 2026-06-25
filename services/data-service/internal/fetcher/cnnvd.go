// Package fetcher — cnnvd.go
// CNNVD (China National Vulnerability Database) fetcher.
// CNNVD provides Chinese government-curated vulnerability data.
// API: https://www.cnnvd.org.cn/web/homePage/cnnvdVulList
// Note: CNNVD API is semi-public; uses their open JSON endpoint.
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
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
)

const (
	cnnvdBaseURL   = "https://www.cnnvd.org.cn/web/homePage/cnnvdVulList"
	cnnvdPageSize  = 100
	cnnvdRateDelay = 2 * time.Second // respectful crawling delay
)

// CNNVDFetcher fetches vulnerability data from CNNVD.
type CNNVDFetcher struct {
	db     *mongo.Database
	client *http.Client
}

// NewCNNVDFetcher creates a CNNVD fetcher.
func NewCNNVDFetcher(db *mongo.Database) *CNNVDFetcher {
	return &CNNVDFetcher{
		db:     db,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Name implements Fetcher. Returns "CNNVD".
func (f *CNNVDFetcher) Name() string { return string(SourceCNNVD) }

// FetchAndStore implements Fetcher.
// Fetches recent CNNVD entries (last 30 days or ManualDays).
func (f *CNNVDFetcher) FetchAndStore(ctx context.Context, opts FetchOptions) (int, error) {
	days := opts.ManualDays
	if days <= 0 {
		days = 30
	}

	col := f.db.Collection("cves")
	total := 0
	page := 1
	maxPages := 10 // cap at 10 pages (1000 entries) per run

	since := time.Now().UTC().AddDate(0, 0, -days)

	for page <= maxPages {
		if ctx.Err() != nil {
			return total, ctx.Err()
		}

		entries, hasMore, err := f.fetchPage(ctx, page, since)
		if err != nil {
			log.Warn().Err(err).Int("page", page).Msg("CNNVD: page fetch failed")
			break
		}

		for _, entry := range entries {
			cveID := entry.cveID
			if cveID == "" {
				continue
			}

			doc := bson.M{
				"cnnvd_id":    entry.cnnvdID,
				"source_cnnvd": true,
				"source":      string(SourceCNNVD),
			}
			if entry.summary != "" {
				doc["cnnvd_summary"] = entry.summary
			}

			_, err := col.UpdateOne(ctx,
				bson.M{"id": cveID},
				bson.M{"$set": doc},
				mongoopts.Update().SetUpsert(false), // only enrich existing
			)
			if err != nil {
				log.Warn().Err(err).Str("cve", cveID).Msg("CNNVD: enrich failed")
				continue
			}
			total++
		}

		log.Info().Int("page", page).Int("count", len(entries)).Msg("CNNVD: page processed")

		if !hasMore {
			break
		}
		page++
		time.Sleep(cnnvdRateDelay)
	}

	log.Info().Int("count", total).Msg("CNNVD fetch complete")
	return total, nil
}

type cnnvdEntry struct {
	cnnvdID string
	cveID   string
	summary string
}

// fetchPage fetches one page from CNNVD API.
func (f *CNNVDFetcher) fetchPage(ctx context.Context, page int, since time.Time) ([]cnnvdEntry, bool, error) {
	// CNNVD uses POST JSON body for pagination
	body := fmt.Sprintf(`{
		"pageIndex": %d,
		"pageSize": %d,
		"startTime": "%s",
		"endTime": "%s"
	}`, page, cnnvdPageSize,
		since.Format("2006-01-02"),
		time.Now().UTC().Format("2006-01-02"))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cnnvdBaseURL,
		strings.NewReader(body))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "GlobalCVE-Security-Scanner/3.0")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("CNNVD: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("CNNVD: status %d", resp.StatusCode)
	}

	// Parse response — CNNVD returns Chinese JSON with specific structure
	var result struct {
		Data struct {
			Records []struct {
				CnnvdID string `json:"cnnvdId"`
				CVEID   string `json:"cveId"`
				Summary string `json:"summary"`
				Level   string `json:"hazardLevel"` // 超危|高危|中危|低危
			} `json:"records"`
			Total int `json:"total"`
		} `json:"data"`
		Code int `json:"code"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, false, fmt.Errorf("CNNVD: decode: %w", err)
	}

	if result.Code != 0 {
		return nil, false, fmt.Errorf("CNNVD: API code %d", result.Code)
	}

	entries := make([]cnnvdEntry, 0, len(result.Data.Records))
	for _, r := range result.Data.Records {
		cveID := strings.TrimSpace(r.CVEID)
		if cveID != "" && !strings.HasPrefix(cveID, "CVE-") {
			cveID = "" // reject non-standard IDs
		}
		entries = append(entries, cnnvdEntry{
			cnnvdID: r.CnnvdID,
			cveID:   cveID,
			summary: r.Summary,
		})
	}

	hasMore := page*cnnvdPageSize < result.Data.Total
	return entries, hasMore, nil
}
