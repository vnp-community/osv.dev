// Package opensearch implements SearchIndexRepository backed by OpenSearch / Elasticsearch.
package opensearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/osv/search-service/internal/domain/entity"
	"github.com/osv/search-service/internal/domain/repository"
	"github.com/osv/search-service/internal/domain/valueobject"
	"github.com/rs/zerolog"
)

const (
	defaultIndex = "vulnerabilities"
	contentJSON  = "application/json"
)

// Client wraps an OpenSearch HTTP client.
type Client struct {
	baseURL string
	index   string
	http    *http.Client
	log     zerolog.Logger
}

// NewClient creates a new OpenSearch client.
func NewClient(baseURL, index string, log zerolog.Logger) *Client {
	if index == "" {
		index = defaultIndex
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		index:   index,
		http:    &http.Client{Timeout: 15 * time.Second},
		log:     log,
	}
}

// ── SearchIndexRepository implementation ─────────────────────────────────────

// Search performs a full-text search against OpenSearch.
func (c *Client) Search(ctx context.Context, params *repository.SearchParams) (*repository.SearchResponse, error) {
	query := buildQuery(params)

	body, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("marshal query: %w", err)
	}

	url := fmt.Sprintf("%s/%s/_search", c.baseURL, c.index)
	resp, err := c.doRequest(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}

	return parseSearchResponse(resp)
}

// Upsert indexes or updates a vulnerability document.
func (c *Client) Upsert(ctx context.Context, doc *entity.SearchDocument) error {
	body, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal doc: %w", err)
	}

	url := fmt.Sprintf("%s/%s/_doc/%s", c.baseURL, c.index, doc.VulnID)
	if _, err := c.doRequest(ctx, http.MethodPut, url, body); err != nil {
		return fmt.Errorf("upsert %s: %w", doc.VulnID, err)
	}
	return nil
}

// MarkWithdrawn sets the is_withdrawn flag to true for a vulnerability.
func (c *Client) MarkWithdrawn(ctx context.Context, vulnID string) error {
	update := map[string]interface{}{
		"doc": map[string]bool{"is_withdrawn": true},
	}
	body, _ := json.Marshal(update)

	url := fmt.Sprintf("%s/%s/_update/%s", c.baseURL, c.index, vulnID)
	if _, err := c.doRequest(ctx, http.MethodPost, url, body); err != nil {
		return fmt.Errorf("mark withdrawn %s: %w", vulnID, err)
	}
	return nil
}

// VectorSearch performs a kNN similarity search using the description_embedding field.
// Uses OpenSearch's knn plugin with cosine similarity (configured in index mapping).
func (c *Client) VectorSearch(
	ctx context.Context,
	queryEmbedding []float32,
	topK int,
	filters repository.SearchFilters,
) (*repository.SearchResponse, error) {
	if len(queryEmbedding) == 0 {
		return nil, fmt.Errorf("empty query embedding")
	}
	if topK <= 0 {
		topK = 20
	}
	if topK > 100 {
		topK = 100
	}

	// Convert []float32 to []interface{} for JSON marshalling
	vector := make([]interface{}, len(queryEmbedding))
	for i, v := range queryEmbedding {
		vector[i] = v
	}

	// Build kNN query with post-filters
	filterClauses := buildFilterClauses(filters)

	var query map[string]interface{}
	if len(filterClauses) > 0 {
		// kNN with filter (OpenSearch 2.4+)
		query = map[string]interface{}{
			"size": topK,
			"query": map[string]interface{}{
				"knn": map[string]interface{}{
					"description_embedding": map[string]interface{}{
						"vector": vector,
						"k":      topK,
						"filter": map[string]interface{}{
							"bool": map[string]interface{}{
								"filter": filterClauses,
							},
						},
					},
				},
			},
		}
	} else {
		query = map[string]interface{}{
			"size": topK,
			"query": map[string]interface{}{
				"knn": map[string]interface{}{
					"description_embedding": map[string]interface{}{
						"vector": vector,
						"k":      topK,
					},
				},
			},
		}
	}

	body, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("marshal knn query: %w", err)
	}

	url := fmt.Sprintf("%s/%s/_search", c.baseURL, c.index)
	resp, err := c.doRequest(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}

	return parseSearchResponse(resp)
}

// buildFilterClauses converts SearchFilters into OpenSearch filter clauses.
func buildFilterClauses(filters repository.SearchFilters) []interface{} {
	var clauses []interface{}

	if len(filters.Ecosystems) > 0 {
		clauses = append(clauses, map[string]interface{}{
			"terms": map[string]interface{}{"ecosystems": filters.Ecosystems},
		})
	}
	if len(filters.Severities) > 0 {
		clauses = append(clauses, map[string]interface{}{
			"terms": map[string]interface{}{"cvss_severity": filters.Severities},
		})
	}
	if !filters.IncludeWithdrawn {
		clauses = append(clauses, map[string]interface{}{
			"term": map[string]interface{}{"is_withdrawn": false},
		})
	}
	if !filters.DateFrom.IsZero() {
		clauses = append(clauses, map[string]interface{}{
			"range": map[string]interface{}{
				"modified": map[string]interface{}{"gte": filters.DateFrom.Format(time.RFC3339)},
			},
		})
	}
	if !filters.DateTo.IsZero() {
		clauses = append(clauses, map[string]interface{}{
			"range": map[string]interface{}{
				"modified": map[string]interface{}{"lte": filters.DateTo.Format(time.RFC3339)},
			},
		})
	}
	return clauses
}


// GetByIDs retrieves multiple documents by ID (mget).
func (c *Client) GetByIDs(ctx context.Context, ids []string) ([]*entity.SearchDocument, error) {
	docs := make([]map[string]string, len(ids))
	for i, id := range ids {
		docs[i] = map[string]string{"_id": id}
	}
	mget := map[string]interface{}{"docs": docs}
	body, _ := json.Marshal(mget)

	url := fmt.Sprintf("%s/%s/_mget", c.baseURL, c.index)
	resp, err := c.doRequest(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}

	return parseMGetResponse(resp)
}

// EnsureIndex creates the index with the correct mapping if it doesn't exist.
func (c *Client) EnsureIndex(ctx context.Context) error {
	url := fmt.Sprintf("%s/%s", c.baseURL, c.index)

	// Check existence
	req, _ := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("check index: %w", err)
	}
	res.Body.Close()
	if res.StatusCode == http.StatusOK {
		return nil // already exists
	}

	// Create with mapping
	mapping := buildIndexMapping()
	body, _ := json.Marshal(mapping)
	if _, err := c.doRequest(ctx, http.MethodPut, url, body); err != nil {
		return fmt.Errorf("create index: %w", err)
	}
	c.log.Info().Str("index", c.index).Msg("OpenSearch index created")
	return nil
}

// ── Query builder ─────────────────────────────────────────────────────────────

func buildQuery(params *repository.SearchParams) map[string]interface{} {
	must := []interface{}{}
	filter := []interface{}{}

	// Full-text query
	if params.Query != "" {
		must = append(must, map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":  params.Query,
				"fields": []string{"summary^3", "details^2", "packages", "vuln_id^2", "aliases"},
				"type":   "best_fields",
				"fuzziness": "AUTO",
			},
		})
	} else {
		must = append(must, map[string]interface{}{"match_all": map[string]interface{}{}})
	}

	// Filters
	if len(params.Filters.Ecosystems) > 0 {
		filter = append(filter, map[string]interface{}{
			"terms": map[string]interface{}{"ecosystems": params.Filters.Ecosystems},
		})
	}
	if len(params.Filters.Severities) > 0 {
		filter = append(filter, map[string]interface{}{
			"terms": map[string]interface{}{"cvss_severity": params.Filters.Severities},
		})
	}
	if !params.Filters.IncludeWithdrawn {
		filter = append(filter, map[string]interface{}{
			"term": map[string]interface{}{"is_withdrawn": false},
		})
	}
	if !params.Filters.DateFrom.IsZero() {
		filter = append(filter, map[string]interface{}{
			"range": map[string]interface{}{
				"modified": map[string]interface{}{"gte": params.Filters.DateFrom.Format(time.RFC3339)},
			},
		})
	}
	if !params.Filters.DateTo.IsZero() {
		filter = append(filter, map[string]interface{}{
			"range": map[string]interface{}{
				"modified": map[string]interface{}{"lte": params.Filters.DateTo.Format(time.RFC3339)},
			},
		})
	}

	// Sort
	sort := buildSort(params.SortOrder)

	// Aggregations (facets)
	aggs := map[string]interface{}{
		"by_ecosystem": map[string]interface{}{
			"terms": map[string]interface{}{"field": "ecosystems", "size": 50},
		},
		"by_severity": map[string]interface{}{
			"terms": map[string]interface{}{"field": "cvss_severity", "size": 10},
		},
	}

	// Pagination
	pageSize := params.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	q := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must":   must,
				"filter": filter,
			},
		},
		"sort":  sort,
		"size":  pageSize,
		"from":  0,
		"aggs":  aggs,
		"highlight": map[string]interface{}{
			"fields": map[string]interface{}{
				"summary": map[string]interface{}{"number_of_fragments": 1},
				"details": map[string]interface{}{"number_of_fragments": 2},
			},
			"pre_tags":  []string{"<em>"},
			"post_tags": []string{"</em>"},
		},
	}

	// search_after pagination
	if params.Cursor != "" {
		q["search_after"] = []string{params.Cursor}
	}

	return q
}

func buildSort(order valueobject.SortOrder) []interface{} {
	switch order {
	case valueobject.SortDateDesc:
		return []interface{}{
			map[string]interface{}{"modified": map[string]string{"order": "desc"}},
			"_id",
		}
	case valueobject.SortDateAsc:
		return []interface{}{
			map[string]interface{}{"modified": map[string]string{"order": "asc"}},
			"_id",
		}
	case valueobject.SortSeverity:
		return []interface{}{
			map[string]interface{}{"cvss_score": map[string]string{"order": "desc"}},
			"_id",
		}
	default: // SortRelevance
		return []interface{}{"_score", map[string]interface{}{"modified": map[string]string{"order": "desc"}}}
	}
}

// ── Response parsers ──────────────────────────────────────────────────────────

type osHit struct {
	ID     string                 `json:"_id"`
	Score  float32                `json:"_score"`
	Source entity.SearchDocument  `json:"_source"`
	Highlight map[string][]string `json:"highlight"`
}

type osResponse struct {
	Hits struct {
		Total struct {
			Value int64 `json:"value"`
		} `json:"total"`
		Hits []osHit `json:"hits"`
	} `json:"hits"`
	Aggregations map[string]struct {
		Buckets []struct {
			Key      string `json:"key"`
			DocCount int64  `json:"doc_count"`
		} `json:"buckets"`
	} `json:"aggregations"`
}

func parseSearchResponse(data []byte) (*repository.SearchResponse, error) {
	var osResp osResponse
	if err := json.Unmarshal(data, &osResp); err != nil {
		return nil, fmt.Errorf("parse search response: %w", err)
	}

	results := make([]entity.SearchResult, 0, len(osResp.Hits.Hits))
	var lastSort string

	for _, hit := range osResp.Hits.Hits {
		r := entity.SearchResult{
			VulnID:    hit.ID,
			Summary:   hit.Source.Summary,
			Ecosystem: firstOrEmpty(hit.Source.Ecosystems),
			Severity:  hit.Source.CVSSSeverity,
			Modified:  hit.Source.Modified,
			Score:     hit.Score,
		}
		for field, frags := range hit.Highlight {
			for _, frag := range frags {
				r.Highlights = append(r.Highlights, entity.Highlight{Field: field, Snippet: frag})
			}
		}
		results = append(results, r)
		lastSort = hit.ID // use ID as cursor fallback
	}

	facets := map[string][]entity.Facet{}
	for aggName, agg := range osResp.Aggregations {
		buckets := make([]entity.Facet, len(agg.Buckets))
		for i, b := range agg.Buckets {
			buckets[i] = entity.Facet{Value: b.Key, Count: b.DocCount}
		}
		facets[aggName] = buckets
	}

	return &repository.SearchResponse{
		Results:   results,
		TotalHits: osResp.Hits.Total.Value,
		NextCursor: lastSort,
		Facets:    facets,
	}, nil
}

func parseMGetResponse(data []byte) ([]*entity.SearchDocument, error) {
	var resp struct {
		Docs []struct {
			Found  bool                  `json:"found"`
			Source entity.SearchDocument `json:"_source"`
		} `json:"docs"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse mget: %w", err)
	}

	docs := make([]*entity.SearchDocument, 0, len(resp.Docs))
	for _, d := range resp.Docs {
		if d.Found {
			doc := d.Source
			docs = append(docs, &doc)
		}
	}
	return docs, nil
}

// ── HTTP helper ───────────────────────────────────────────────────────────────

func (c *Client) doRequest(ctx context.Context, method, url string, body []byte) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", contentJSON)

	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http %s %s: %w", method, url, err)
	}
	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("OpenSearch %s %s → %d: %s", method, url, res.StatusCode, truncate(string(data), 200))
	}

	return data, nil
}

// ── Index mapping ─────────────────────────────────────────────────────────────

func buildIndexMapping() map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   3,
			"number_of_replicas": 1,
			"index.knn":          true,
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"vuln_id":      map[string]string{"type": "keyword"},
				"summary":      map[string]interface{}{"type": "text", "analyzer": "standard", "fields": map[string]interface{}{"keyword": map[string]string{"type": "keyword"}, "suggest": map[string]string{"type": "completion"}}},
				"details":      map[string]interface{}{"type": "text", "analyzer": "standard"},
				"ecosystems":   map[string]string{"type": "keyword"},
				"packages":     map[string]string{"type": "keyword"},
				"purls":        map[string]string{"type": "keyword"},
				"aliases":      map[string]string{"type": "keyword"},
				"source":       map[string]string{"type": "keyword"},
				"severity_type": map[string]string{"type": "keyword"},
				"cvss_score":   map[string]string{"type": "float"},
				"cvss_severity": map[string]string{"type": "keyword"},
				"published":    map[string]string{"type": "date"},
				"modified":     map[string]string{"type": "date"},
				"is_withdrawn": map[string]string{"type": "boolean"},
				"description_embedding": map[string]interface{}{
					"type":      "knn_vector",
					"dimension": 768,
					"method": map[string]interface{}{
						"name":       "hnsw",
						"space_type": "cosinesimil",
						"engine":     "nmslib",
					},
				},
			},
		},
	}
}

// ── Utilities ─────────────────────────────────────────────────────────────────

func firstOrEmpty(s []string) string {
	if len(s) > 0 {
		return s[0]
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
