// Package fetcher — jvn.go
// JVN (Japan Vulnerability Notes) fetcher.
// Fetches from JVN RSS/XML feed at https://jvndb.jvn.jp
// Provides Japanese CERT vulnerability data.
package fetcher

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
)

const jvnFeedURL = "https://jvndb.jvn.jp/apis/myjvn?method=getVulnOverviewList&feed=hnd&lang=en"

// JVNFetcher fetches vulnerability data from JVN (Japan Vulnerability Notes).
type JVNFetcher struct {
	db      *mongo.Database
	feedURL string
	client  *http.Client
}

// NewJVNFetcher creates a JVN fetcher.
func NewJVNFetcher(db *mongo.Database) *JVNFetcher {
	return &JVNFetcher{
		db:      db,
		feedURL: jvnFeedURL,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

// Name implements Fetcher. Returns "JVN".
func (f *JVNFetcher) Name() string { return string(SourceJVN) }

// FetchAndStore implements Fetcher.
func (f *JVNFetcher) FetchAndStore(ctx context.Context, opts FetchOptions) (int, error) {
	return f.fetchFeed(ctx)
}

// jvnFeedResponse parses the JVN RSS feed structure.
type jvnFeedResponse struct {
	XMLName xml.Name `xml:"VULDEF-Document"`
	Items   []jvnItem `xml:"channel>item"`
}

type jvnItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	Identifier  string `xml:"identifier"` // JVNDB-YYYY-NNNNNN
	CVEIDs      []string // populated from references
}

// fetchFeed downloads and parses JVN RSS, maps to CVE IDs when available.
func (f *JVNFetcher) fetchFeed(ctx context.Context) (int, error) {
	col := f.db.Collection("cves")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.feedURL, nil)
	if err != nil {
		return 0, err
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("JVN fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("JVN API returned %d", resp.StatusCode)
	}

	// JVN returns XML — parse the feed
	var feed jvnFeedResponse
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return 0, fmt.Errorf("JVN XML decode: %w", err)
	}

	total := 0
	for _, item := range feed.Items {
		if ctx.Err() != nil {
			return total, ctx.Err()
		}

		// Extract CVE IDs from description or link
		cveIDs := extractCVEIDs(item.Description + " " + item.Link + " " + item.Title)
		if len(cveIDs) == 0 {
			// Store JVN-only entry under its JVNDB identifier if no CVE mapping
			continue
		}

		// Parse published date
		var pubDate time.Time
		if item.PubDate != "" {
			pubDate, _ = time.Parse(time.RFC1123, item.PubDate)
			if pubDate.IsZero() {
				pubDate, _ = time.Parse(time.RFC1123Z, item.PubDate)
			}
		}

		// Upsert CVE records enriched with JVN data
		for _, cveID := range cveIDs {
			doc := bson.M{
				"jvn_id":      item.Identifier,
				"jvn_link":    item.Link,
				"source_jvn":  true,
				"source":      string(SourceJVN),
			}
			if !pubDate.IsZero() {
				doc["jvn_published"] = pubDate
			}

			_, err := col.UpdateOne(ctx,
				bson.M{"id": cveID},
				bson.M{"$set": doc},
				mongoopts.Update().SetUpsert(false), // only enrich existing CVEs
			)
			if err != nil {
				log.Warn().Err(err).Str("cve", cveID).Msg("JVN: enrich failed")
				continue
			}
			total++
		}
	}

	log.Info().Int("items", len(feed.Items)).Int("enriched", total).Msg("JVN fetch complete")
	return total, nil
}

// extractCVEIDs extracts all CVE-YYYY-NNNNN patterns from text.
func extractCVEIDs(text string) []string {
	var ids []string
	seen := make(map[string]bool)
	words := strings.Fields(text)
	for _, w := range words {
		// Clean punctuation
		w = strings.Trim(w, ",.;:()[]\"'")
		if strings.HasPrefix(w, "CVE-") && len(w) >= 12 {
			parts := strings.Split(w, "-")
			if len(parts) == 3 && len(parts[1]) == 4 {
				if !seen[w] {
					seen[w] = true
					ids = append(ids, w)
				}
			}
		}
	}
	return ids
}
