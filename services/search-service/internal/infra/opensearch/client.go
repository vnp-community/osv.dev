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

// LegacyClient wraps OpenSearch SDK v2 for CVE index operations.
// Note: The primary HTTP-based Client is defined in opensearch_adapter.go.
type LegacyClient struct {
	os     *opensearch.Client
	index  string
	logger zerolog.Logger
}

// NewLegacyClient creates an OpenSearch SDK v2 client.
// osURL: e.g. "http://opensearch:9200"
func NewLegacyClient(osURL string, log zerolog.Logger) (*LegacyClient, error) {
	cfg := opensearch.Config{Addresses: []string{osURL}}
	client, err := opensearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("opensearch: new client: %w", err)
	}

	return &LegacyClient{os: client, index: CVEIndexName, logger: log}, nil
}

// EnsureIndexLegacy creates the CVE index with mapping if it doesn't exist.
func (c *LegacyClient) EnsureIndexLegacy(ctx context.Context) error {
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
func (c *LegacyClient) SearchCVEs(ctx context.Context, req *SearchCVEsRequest) ([]*entity.CVE, int64, error) {
    if req == nil {
        req = &SearchCVEsRequest{}
    }
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
                Source *entity.CVE `json:"_source"`
            } `json:"hits"`
        } `json:"hits"`
    }
    if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
        return nil, 0, err
    }

    if len(result.Hits.Hits) == 0 {
        return []*entity.CVE{}, 0, nil
    }

    cves := make([]*entity.CVE, 0, len(result.Hits.Hits))
    for _, h := range result.Hits.Hits {
        if h.Source != nil {
            cves = append(cves, h.Source)
        }
    }
    return cves, result.Hits.Total.Value, nil
}

func (c *LegacyClient) buildQuery(req *SearchCVEsRequest) map[string]interface{} {
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
func (c *LegacyClient) IndexCVE(ctx context.Context, cve *entity.CVE) error {
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
func (c *LegacyClient) BulkIndex(ctx context.Context, cves []*entity.CVE) error {
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
func (c *LegacyClient) GetAggregations(ctx context.Context) (map[string]interface{}, error) {
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
