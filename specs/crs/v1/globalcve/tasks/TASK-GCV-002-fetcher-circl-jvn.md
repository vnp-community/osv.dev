# TASK-GCV-002 — CIRCL + JVN RSS Fetchers

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-GCV-002 |
| **Service** | `data-service` |
| **CR** | CR-GCV-001 |
| **Phase** | 1 — Core Pipeline |
| **Priority** | 🔴 High |
| **Prerequisites** | TASK-GCV-001 |

## Context

Tạo hai fetchers mới: `CIRCLFetcher` (Luxembourg CVE API) và `JVNFetcher` (Japanese Vulnerability Notes RSS). Cả hai implement interface `Fetcher` từ TASK-GCV-001. CIRCL fetch theo keyword/year, JVN parse RSS/RDF XML.

## Reference

- Solution: [SOL-GCV-001](../solutions/SOL-GCV-001-multi-source-fetcher.md) §4.2, §4.3
- CR: [CR-GCV-001](../CR-GCV-001-multi-source-fetcher-pipeline.md) §4.2, §4.3

## Files to Create/Modify

```
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/data-service/internal/fetcher/circl.go
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/data-service/internal/fetcher/jvn.go
```

## Implementation Spec

### circl.go

```go
// Package fetcher — CIRCL CVE API fetcher.
// Source: https://cve.circl.lu/api
// Docs: https://cve.circl.lu/api/
package fetcher

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
    "time"

    "github.com/rs/zerolog"
    entity "github.com/osv/data-service/internal/domain/entity"
    "github.com/osv/data-service/internal/domain/repository"
)

const circlBaseURL = "https://cve.circl.lu/api"

type CIRCLFetcher struct {
    baseURL string
    client  *http.Client
    cveRepo repository.MongoDBCVERepository // hoặc CVEWriteRepository tùy hiện trạng
    logger  zerolog.Logger
}

func NewCIRCLFetcher(cveRepo repository.MongoDBCVERepository, log zerolog.Logger) *CIRCLFetcher {
    return &CIRCLFetcher{
        baseURL: circlBaseURL,
        client:  &http.Client{Timeout: 60 * time.Second},
        cveRepo: cveRepo,
        logger:  log.With().Str("fetcher", "CIRCL").Logger(),
    }
}

func (f *CIRCLFetcher) Name() string { return string(SourceCIRCL) }

// FetchAndStore implements Fetcher.
// Fetches current year CVEs from CIRCL search API.
func (f *CIRCLFetcher) FetchAndStore(ctx context.Context, opts FetchOptions) (int, error) {
    year := time.Now().Year()
    if opts.StartYear > 0 {
        year = opts.StartYear
    }
    return f.fetchByKeyword(ctx, fmt.Sprintf("%d", year))
}

func (f *CIRCLFetcher) fetchByKeyword(ctx context.Context, keyword string) (int, error) {
    url := fmt.Sprintf("%s/search/%s", f.baseURL, keyword)
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    if err != nil {
        return 0, fmt.Errorf("circl: build request: %w", err)
    }
    req.Header.Set("Accept", "application/json")

    resp, err := f.client.Do(req)
    if err != nil {
        return 0, fmt.Errorf("circl: request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return 0, fmt.Errorf("circl: status %d", resp.StatusCode)
    }

    // CIRCL may return array or object
    var rawData interface{}
    if err := json.NewDecoder(resp.Body).Decode(&rawData); err != nil {
        return 0, fmt.Errorf("circl: decode: %w", err)
    }

    items := extractItems(rawData)
    cves := make([]*entity.CVE, 0, len(items))
    for _, item := range items {
        cve := f.mapToEntity(item)
        if cve != nil {
            cves = append(cves, cve)
        }
    }

    if len(cves) == 0 {
        return 0, nil
    }

    // Batch upsert — reuse existing repository method
    count, err := f.cveRepo.UpsertBatch(ctx, cves)
    if err != nil {
        return 0, fmt.Errorf("circl: upsert: %w", err)
    }
    f.logger.Info().Int("upserted", count).Str("keyword", keyword).Msg("CIRCL sync done")
    return count, nil
}

func (f *CIRCLFetcher) mapToEntity(item map[string]interface{}) *entity.CVE {
    id := extractString(item, "id", "cveMetadata.cveId")
    if id == "" || !isValidCVEID(id) {
        return nil
    }

    cve := &entity.CVE{
        ID:          id,
        DataSource:  string(SourceCIRCL),
        Description: extractString(item, "summary", "containers.cna.descriptions.0.value"),
    }

    if pub := extractString(item, "Published", "publishedDate"); pub != "" {
        if t, err := time.Parse(time.RFC3339, pub); err == nil {
            cve.Published = t
        }
    }

    return cve
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

func extractString(m map[string]interface{}, keys ...string) string {
    for _, key := range keys {
        if v, ok := m[key]; ok {
            if s, ok := v.(string); ok && s != "" {
                return s
            }
        }
    }
    return ""
}

func isValidCVEID(id string) bool {
    return strings.HasPrefix(id, "CVE-") && len(id) > 9
}
```

### jvn.go

```go
// Package fetcher — JVN RSS/RDF fetcher.
// Source: https://jvndb.jvn.jp/en/rss/jvndb.rdf
package fetcher

import (
    "context"
    "encoding/xml"
    "fmt"
    "net/http"
    "regexp"
    "strings"
    "time"

    "github.com/rs/zerolog"
    entity "github.com/osv/data-service/internal/domain/entity"
    "github.com/osv/data-service/internal/domain/repository"
)

const jvnFeedURL = "https://jvndb.jvn.jp/en/rss/jvndb.rdf"

var cvePattern = regexp.MustCompile(`CVE-\d{4}-\d{4,}`)

// JVN RDF/RSS structs
type JVNFeed struct {
    XMLName xml.Name  `xml:"RDF"`
    Items   []JVNItem `xml:"item"`
}

type JVNItem struct {
    Title        string `xml:"title"`
    Link         string `xml:"link"`
    Description  string `xml:"description"`
    DCDate       string `xml:"http://purl.org/dc/elements/1.1/ date"`
    DCIdentifier string `xml:"http://purl.org/dc/elements/1.1/ identifier"`
    DCSubject    string `xml:"http://purl.org/dc/elements/1.1/ subject"`
}

type JVNFetcher struct {
    feedURL string
    client  *http.Client
    cveRepo repository.MongoDBCVERepository
    logger  zerolog.Logger
}

func NewJVNFetcher(cveRepo repository.MongoDBCVERepository, log zerolog.Logger) *JVNFetcher {
    return &JVNFetcher{
        feedURL: jvnFeedURL,
        client:  &http.Client{Timeout: 30 * time.Second},
        cveRepo: cveRepo,
        logger:  log.With().Str("fetcher", "JVN").Logger(),
    }
}

func (f *JVNFetcher) Name() string { return string(SourceJVN) }

func (f *JVNFetcher) FetchAndStore(ctx context.Context, opts FetchOptions) (int, error) {
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.feedURL, nil)
    if err != nil {
        return 0, fmt.Errorf("jvn: build request: %w", err)
    }

    resp, err := f.client.Do(req)
    if err != nil {
        return 0, fmt.Errorf("jvn: request: %w", err)
    }
    defer resp.Body.Close()

    var feed JVNFeed
    if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
        return 0, fmt.Errorf("jvn: decode rss: %w", err)
    }

    cves := make([]*entity.CVE, 0, len(feed.Items))
    for _, item := range feed.Items {
        cveID := extractCVEFromURL(item.Link)
        if cveID == "" {
            cveID = cvePattern.FindString(item.DCIdentifier)
        }
        if cveID == "" {
            cveID = cvePattern.FindString(item.Title + " " + item.Description)
        }
        if cveID == "" {
            continue
        }

        cve := &entity.CVE{
            ID:          strings.ToUpper(cveID),
            DataSource:  string(SourceJVN),
            Description: item.Description,
        }

        if item.DCDate != "" {
            if t, err := time.Parse(time.RFC3339, item.DCDate); err == nil {
                cve.Published = t
            }
        }

        cves = append(cves, cve)
    }

    if len(cves) == 0 {
        f.logger.Info().Msg("JVN: no CVEs extracted")
        return 0, nil
    }

    count, err := f.cveRepo.UpsertBatch(ctx, cves)
    if err != nil {
        return 0, fmt.Errorf("jvn: upsert: %w", err)
    }
    f.logger.Info().Int("upserted", count).Int("items", len(feed.Items)).Msg("JVN sync done")
    return count, nil
}

func extractCVEFromURL(link string) string {
    matches := cvePattern.FindString(link)
    return matches
}
```

## Acceptance Criteria

- [x] `CIRCLFetcher.Name()` returns `"CIRCL"`
- [x] `CIRCLFetcher.FetchAndStore(ctx, FetchOptions{})` calls `https://cve.circl.lu/api/search/<year>` và upsert CVEs
- [x] CIRCL response là array → tất cả items được xử lý
- [x] CIRCL response là single object → được xử lý như 1 item
- [x] `JVNFetcher.Name()` returns `"JVN"`
- [x] `JVNFetcher.FetchAndStore` parse RSS/RDF XML, extract CVE IDs từ link/title/description
- [x] Items không có CVE ID → bị skip (không panic)
- [x] `go build ./...` pass không lỗi
