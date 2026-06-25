# TASK-GCV-023 — OpenSearch Client + Index Mapping (search-service)

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-GCV-023 |
| **Service** | `search-service` |
| **CR** | CR-GCV-004 |
| **Phase** | 3 — Advanced Search |
| **Priority** | 🟡 Medium |
| **Prerequisites** | — |

## Context

Tạo OpenSearch client wrapper và index mapping cho `search-service`. Client cung cấp BM25 full-text search, aggregations, và internal index/bulk endpoints để data-service có thể push CVEs vào OpenSearch sau sync.

## Reference

- Solution: [SOL-GCV-004](../solutions/SOL-GCV-004-opensearch-semantic-search.md) §2.2

## Files to Create/Modify

```
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/internal/infra/opensearch/client.go
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/internal/infra/opensearch/mapping.go
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/go.mod
        (add opensearch-go dependency)
```

## Implementation Spec

### go.mod additions

```
require (
    github.com/opensearch-project/opensearch-go/v2 v2.x.x
)
```

### mapping.go — Index Definition

```go
package opensearch

// CVEIndexMapping defines the OpenSearch mapping for the cves index.
const CVEIndexMapping = `{
  "settings": {
    "number_of_shards": 1,
    "number_of_replicas": 0,
    "analysis": {
      "analyzer": {
        "cve_analyzer": {
          "type": "custom",
          "tokenizer": "standard",
          "filter": ["lowercase", "stop", "asciifolding"]
        }
      }
    }
  },
  "mappings": {
    "properties": {
      "id":          { "type": "keyword" },
      "description": { "type": "text", "analyzer": "cve_analyzer" },
      "severity":    { "type": "keyword" },
      "source":      { "type": "keyword" },
      "cwe":         { "type": "keyword" },
      "vendors":     { "type": "keyword" },
      "products":    { "type": "keyword" },
      "epss":        { "type": "float" },
      "epss_percentile": { "type": "float" },
      "is_kev":      { "type": "boolean" },
      "is_exploit":  { "type": "boolean" },
      "cvss3":       { "type": "float" },
      "published":   { "type": "date" },
      "updated_at":  { "type": "date" }
    }
  }
}`

const CVEIndexName = "cves"
```

### client.go

```go
package opensearch

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "strings"

    "github.com/opensearch-project/opensearch-go/v2"
    "github.com/opensearch-project/opensearch-go/v2/opensearchapi"
    "github.com/rs/zerolog"
    entity "github.com/osv/search-service/internal/domain/entity"
)

// Client wraps OpenSearch for CVE search operations.
type Client struct {
    os     *opensearch.Client
    index  string
    logger zerolog.Logger
}

// NewClient creates an OpenSearch client.
// osURL: e.g. "http://opensearch:9200"
func NewClient(osURL string, log zerolog.Logger) (*Client, error) {
    cfg := opensearch.Config{Addresses: []string{osURL}}
    client, err := opensearch.NewClient(cfg)
    if err != nil {
        return nil, fmt.Errorf("opensearch: new client: %w", err)
    }

    return &Client{os: client, index: CVEIndexName, logger: log}, nil
}

// EnsureIndex creates the CVE index with mapping if it doesn't exist.
func (c *Client) EnsureIndex(ctx context.Context) error {
    res, err := c.os.Indices.Exists([]string{c.index})
    if err != nil {
        return err
    }
    if res.StatusCode == 200 {
        return nil // already exists
    }

    createReq := opensearchapi.IndicesCreateRequest{
        Index: c.index,
        Body:  strings.NewReader(CVEIndexMapping),
    }
    _, err = createReq.Do(ctx, c.os)
    return err
}

// SearchCVEsRequest is the input for OpenSearch full-text search.
type SearchCVEsRequest struct {
    Query    string
    Severity []string // filter
    Vendor   string
    CWE      string
    IsKEV    *bool
    MinEPSS  *float64
    Page     int
    Limit    int
}

// SearchCVEs performs BM25 full-text search with optional filters.
func (c *Client) SearchCVEs(ctx context.Context, req *SearchCVEsRequest) ([]*entity.CVE, int64, error) {
    query := c.buildQuery(req)
    body, _ := json.Marshal(query)

    from := req.Page * req.Limit
    size := req.Limit

    res, err := c.os.Search(
        c.os.Search.WithContext(ctx),
        c.os.Search.WithIndex(c.index),
        c.os.Search.WithBody(bytes.NewReader(body)),
        c.os.Search.WithFrom(from),
        c.os.Search.WithSize(size),
    )
    if err != nil {
        return nil, 0, fmt.Errorf("opensearch: search: %w", err)
    }
    defer res.Body.Close()

    var result struct {
        Hits struct {
            Total struct{ Value int64 } `json:"total"`
            Hits  []struct {
                Source entity.CVE `json:"_source"`
            } `json:"hits"`
        } `json:"hits"`
    }
    if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
        return nil, 0, err
    }

    cves := make([]*entity.CVE, 0, len(result.Hits.Hits))
    for _, h := range result.Hits.Hits {
        cve := h.Source
        cves = append(cves, &cve)
    }
    return cves, result.Hits.Total.Value, nil
}

func (c *Client) buildQuery(req *SearchCVEsRequest) map[string]interface{} {
    must := []interface{}{}
    filter := []interface{}{}

    // Full-text query
    if req.Query != "" {
        must = append(must, map[string]interface{}{
            "multi_match": map[string]interface{}{
                "query":  req.Query,
                "fields": []string{"description^2", "id^3", "vendors", "products"},
            },
        })
    }

    // Severity filter
    if len(req.Severity) > 0 {
        filter = append(filter, map[string]interface{}{
            "terms": map[string]interface{}{"severity": req.Severity},
        })
    }

    // MinEPSS filter
    if req.MinEPSS != nil {
        filter = append(filter, map[string]interface{}{
            "range": map[string]interface{}{
                "epss": map[string]interface{}{"gte": *req.MinEPSS},
            },
        })
    }

    // KEV filter
    if req.IsKEV != nil {
        filter = append(filter, map[string]interface{}{
            "term": map[string]interface{}{"is_kev": *req.IsKEV},
        })
    }

    q := map[string]interface{}{
        "bool": map[string]interface{}{
            "must":   must,
            "filter": filter,
        },
    }
    if len(must) == 0 {
        q = map[string]interface{}{
            "bool": map[string]interface{}{
                "filter": filter,
                "must":   []interface{}{map[string]interface{}{"match_all": map[string]interface{}{}}},
            },
        }
    }

    return map[string]interface{}{"query": q}
}

// IndexCVE indexes or updates a single CVE document.
func (c *Client) IndexCVE(ctx context.Context, cve *entity.CVE) error {
    data, err := json.Marshal(cve)
    if err != nil {
        return err
    }
    req := opensearchapi.IndexRequest{
        Index:      c.index,
        DocumentID: cve.ID,
        Body:       bytes.NewReader(data),
    }
    _, err = req.Do(ctx, c.os)
    return err
}

// BulkIndex indexes multiple CVEs using bulk API.
func (c *Client) BulkIndex(ctx context.Context, cves []*entity.CVE) error {
    var buf bytes.Buffer
    for _, cve := range cves {
        meta := fmt.Sprintf(`{"index":{"_index":"%s","_id":"%s"}}`, c.index, cve.ID)
        data, _ := json.Marshal(cve)
        buf.WriteString(meta + "\n")
        buf.Write(data)
        buf.WriteString("\n")
    }
    res, err := c.os.Bulk(bytes.NewReader(buf.Bytes()),
        c.os.Bulk.WithContext(ctx),
        c.os.Bulk.WithIndex(c.index),
    )
    if err != nil {
        return err
    }
    res.Body.Close()
    return nil
}

// GetAggregations returns faceted aggregations for dashboard.
func (c *Client) GetAggregations(ctx context.Context) (map[string]interface{}, error) {
    query := map[string]interface{}{
        "size": 0,
        "aggs": map[string]interface{}{
            "by_severity": map[string]interface{}{
                "terms": map[string]interface{}{"field": "severity", "size": 10},
            },
            "top_vendors": map[string]interface{}{
                "terms": map[string]interface{}{"field": "vendors", "size": 10},
            },
            "top_cwe": map[string]interface{}{
                "terms": map[string]interface{}{"field": "cwe", "size": 10},
            },
        },
    }
    body, _ := json.Marshal(query)
    res, err := c.os.Search(
        c.os.Search.WithContext(ctx),
        c.os.Search.WithIndex(c.index),
        c.os.Search.WithBody(bytes.NewReader(body)),
    )
    if err != nil {
        return nil, err
    }
    defer res.Body.Close()

    var result map[string]interface{}
    json.NewDecoder(res.Body).Decode(&result)
    return result["aggregations"].(map[string]interface{}), nil
}
```

## Acceptance Criteria

- [x] `NewClient("http://opensearch:9200", log)` → client created, ping OK
- [x] `EnsureIndex(ctx)` → creates `cves` index với mapping nếu chưa tồn tại
- [x] `EnsureIndex(ctx)` idempotent (không error khi index đã tồn tại)
- [x] `SearchCVEs(ctx, {Query: "log4j"})` → BM25 results từ OpenSearch
- [x] `SearchCVEs` với severity filter → applied correctly
- [x] `IndexCVE(ctx, cve)` → CVE document indexed
- [x] `BulkIndex(ctx, cves)` → bulk indexed, không OOM
- [x] `GetAggregations(ctx)` → returns `by_severity`, `top_vendors`, `top_cwe` buckets
- [x] `go build ./...` pass không lỗi
