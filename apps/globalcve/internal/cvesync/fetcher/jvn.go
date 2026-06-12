// Package fetcher — JVN RSS feed fetcher.
// Port from TypeScript (globalcve/src/lib/jvn.ts).
// Source: https://jvndb.jvn.jp/en/rss/jvndb.rdf
package fetcher

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/globalcve/mono/internal/cvesearch/domain/entity"
	"github.com/globalcve/mono/internal/cvesync/domain/repository"
)

// JVNFetcher fetches CVE data from JVN RSS/RDF feed.
type JVNFetcher struct {
	feedURL string
	client  *http.Client
	cveRepo repository.CVEWriteRepository
}

// NewJVNFetcher creates a new JVN RSS fetcher.
func NewJVNFetcher(feedURL string, timeout time.Duration, cveRepo repository.CVEWriteRepository) *JVNFetcher {
	if feedURL == "" {
		feedURL = "https://jvndb.jvn.jp/en/rss/jvndb.rdf"
	}
	return &JVNFetcher{
		feedURL: feedURL,
		cveRepo: cveRepo,
		client:  &http.Client{Timeout: timeout},
	}
}

func (f *JVNFetcher) Source() entity.SourceName { return entity.SourceNameJVN }

// jvnRDF is the RDF feed structure from JVN.
type jvnRDF struct {
	XMLName xml.Name  `xml:"RDF"`
	Items   []jvnItem `xml:"item"`
}

type jvnItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	Date        string `xml:"date"`
	Identifier  string `xml:"identifier"`
	CVEs        []struct {
		Name string `xml:"name"`
	} `xml:"references"`
}

// Fetch retrieves the JVN RSS feed and upserts CVEs.
func (f *JVNFetcher) Fetch(ctx context.Context, opts FetchOptions) (int, error) {
	items, err := f.fetchFeed(ctx)
	if err != nil {
		return 0, fmt.Errorf("jvn fetch feed: %w", err)
	}

	cves := make([]*entity.CVE, 0, len(items))
	for _, item := range items {
		if cve := f.convert(item); cve != nil {
			cves = append(cves, cve)
		}
	}

	if len(cves) == 0 {
		return 0, nil
	}

	inserted, updated, err := f.cveRepo.UpsertBatch(ctx, cves)
	if err != nil {
		return 0, fmt.Errorf("jvn upsert batch: %w", err)
	}

	log.Ctx(ctx).Info().
		Int("items", len(items)).
		Int("inserted", inserted).
		Int("updated", updated).
		Msg("jvn: feed processed")

	return inserted + updated, nil
}

func (f *JVNFetcher) fetchFeed(ctx context.Context) ([]jvnItem, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.feedURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return nil, fmt.Errorf("jvn status %d: %s", resp.StatusCode, body)
	}

	var rdf jvnRDF
	if err := xml.NewDecoder(resp.Body).Decode(&rdf); err != nil {
		return nil, fmt.Errorf("decode rdf: %w", err)
	}
	return rdf.Items, nil
}

func (f *JVNFetcher) convert(item jvnItem) *entity.CVE {
	// Extract CVE ID from references or identifier
	cveID := ""
	for _, ref := range item.CVEs {
		if strings.HasPrefix(ref.Name, "CVE-") {
			cveID = ref.Name
			break
		}
	}
	// Fallback: check identifier field
	if cveID == "" && strings.HasPrefix(item.Identifier, "CVE-") {
		cveID = item.Identifier
	}
	// Try to find CVE ID in title
	if cveID == "" {
		parts := strings.Fields(item.Title)
		for _, p := range parts {
			if strings.HasPrefix(p, "CVE-") {
				cveID = strings.TrimRight(p, ")")
				break
			}
		}
	}
	if cveID == "" || !entity.IsValidID(cveID) {
		return nil
	}

	cve := &entity.CVE{
		ID:          cveID,
		Description: item.Description,
		Summary:     item.Title,
		Source:      entity.SourceJVN,
		Severity:    inferSeverityFromText(item.Title + " " + item.Description),
		Link:        item.Link,
	}

	// Parse date
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05+00:00", "2006-01-02"} {
		if t, err := time.Parse(layout, item.Date); err == nil {
			cve.Published = t
			cve.Modified = t
			break
		}
	}

	return cve
}

// inferSeverityFromText performs keyword-based severity inference (from TypeScript port).
func inferSeverityFromText(text string) entity.Severity {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "critical") || strings.Contains(lower, "remote code execution") ||
		strings.Contains(lower, "arbitrary code"):
		return entity.SeverityCritical
	case strings.Contains(lower, "high") || strings.Contains(lower, "privilege escalation") ||
		strings.Contains(lower, "sql injection"):
		return entity.SeverityHigh
	case strings.Contains(lower, "medium") || strings.Contains(lower, "cross-site") ||
		strings.Contains(lower, "xss") || strings.Contains(lower, "information disclosure"):
		return entity.SeverityMedium
	case strings.Contains(lower, "low") || strings.Contains(lower, "denial of service") ||
		strings.Contains(lower, "dos"):
		return entity.SeverityLow
	default:
		return entity.SeverityUnknown
	}
}
