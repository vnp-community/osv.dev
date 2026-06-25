// Package fetcher — cveorg.go
// CVE.org GitHub release deltaLog fetcher.
// Source: https://github.com/CVEProject/cvelistV5/releases/latest/download/deltaLog.json
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
	cveOrgDeltaLogURL = "https://github.com/CVEProject/cvelistV5/releases/latest/download/deltaLog.json"
	cveOrgRawBase     = "https://raw.githubusercontent.com/CVEProject/cvelistV5/main/cves"
)

type CVEOrgFetcher struct {
	deltaLogURL string
	client      *http.Client
	db          *mongo.Database
}

func NewCVEOrgFetcher(db *mongo.Database) *CVEOrgFetcher {
	return &CVEOrgFetcher{
		deltaLogURL: cveOrgDeltaLogURL,
		client:      &http.Client{Timeout: 30 * time.Second},
		db:          db,
	}
}

func (f *CVEOrgFetcher) Name() string { return string(SourceCVEOrg) }

func (f *CVEOrgFetcher) FetchAndStore(ctx context.Context, opts FetchOptions) (int, error) {
	return f.fetchDelta(ctx)
}

// FetchSince implements IncrementalFetcher (uses deltaLog always).
func (f *CVEOrgFetcher) FetchSince(ctx context.Context, since time.Time) (int, error) {
	return f.fetchDelta(ctx)
}

type deltaLog struct {
	Changed []string `json:"changed"`
	New     []string `json:"new"`
	Deleted []string `json:"deleted"`
}

func (f *CVEOrgFetcher) fetchDelta(ctx context.Context) (int, error) {
	resp, err := f.client.Get(f.deltaLogURL)
	if err != nil {
		return 0, fmt.Errorf("cveorg: fetch deltaLog: %w", err)
	}
	defer resp.Body.Close()

	var delta deltaLog
	if err := json.NewDecoder(resp.Body).Decode(&delta); err != nil {
		return 0, fmt.Errorf("cveorg: decode deltaLog: %w", err)
	}

	filesToFetch := append(delta.Changed, delta.New...)
	log.Info().Int("files", len(filesToFetch)).Msg("CVE.org: processing delta")

	col := f.db.Collection("cves")
	total := 0
	
	for _, filename := range filesToFetch {
		select {
		case <-ctx.Done():
			return total, ctx.Err()
		default:
		}

		cveID := strings.TrimSuffix(filename, ".json")
		if !strings.HasPrefix(cveID, "CVE-") {
			continue
		}

		cveDoc, err := f.fetchCVEDetail(ctx, cveID)
		if err != nil {
			log.Warn().Err(err).Str("cve_id", cveID).Msg("CVE.org: fetch detail failed (possibly 404)")
			continue
		}

		_, err = col.UpdateOne(ctx,
			bson.M{"id": cveID},
			bson.M{"$set": cveDoc},
			mongoopts.Update().SetUpsert(true),
		)
		if err != nil {
			log.Warn().Err(err).Str("cve_id", cveID).Msg("CVE.org: upsert failed")
			continue
		}
		total++

		// Rate limit: GitHub raw API
		time.Sleep(100 * time.Millisecond)
	}

	log.Info().Int("total", total).Msg("CVE.org sync done")
	return total, nil
}

func (f *CVEOrgFetcher) fetchCVEDetail(ctx context.Context, cveID string) (bson.M, error) {
	// CVE ID format: CVE-YYYY-NNNNN
	// Path: /cves/YYYY/NxNNN/CVE-YYYY-NNNNN.json
	parts := strings.Split(cveID, "-")
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid CVE ID: %s", cveID)
	}
	year := parts[1]
	suffix := parts[2]

	// Pad to folder: e.g. CVE-2021-44228 → /2021/44xxx/
	folderSuffix := suffix[:len(suffix)-3] + "xxx"
	url := fmt.Sprintf("%s/%s/%s/%s.json", cveOrgRawBase, year, folderSuffix, cveID)

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("not found: %s", cveID)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http status: %d", resp.StatusCode)
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	doc := bson.M{
		"id":     cveID,
		"source": string(SourceCVEOrg),
	}

	// Extract description from containers.cna.descriptions[0].value
	if containers, ok := data["containers"].(map[string]interface{}); ok {
		if cna, ok := containers["cna"].(map[string]interface{}); ok {
			if descs, ok := cna["descriptions"].([]interface{}); ok && len(descs) > 0 {
				if d, ok := descs[0].(map[string]interface{}); ok {
					if val, ok := d["value"].(string); ok {
						doc["description"] = val
					}
				}
			}
		}
	}

	return doc, nil
}
