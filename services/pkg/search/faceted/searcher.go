// Package faceted implements faceted search (aggregations) for OpenSearch.
// TASK-07-03: Aggregate CVEs by ecosystem, severity, year, KEV status, CWE, etc.
package faceted

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// FacetType identifies a facet dimension.
type FacetType string

const (
	FacetEcosystem FacetType = "ecosystem"
	FacetSeverity  FacetType = "severity"
	FacetYear      FacetType = "year"
	FacetCWE       FacetType = "cwe"
	FacetKEV       FacetType = "is_kev"
	FacetTag       FacetType = "tags"
)

// FacetBucket is a single aggregation bucket (e.g. severity=HIGH, count=1234).
type FacetBucket struct {
	Key      string `json:"key"`
	DocCount int    `json:"doc_count"`
}

// FacetResult holds aggregation results for one facet dimension.
type FacetResult struct {
	Type    FacetType     `json:"type"`
	Buckets []FacetBucket `json:"buckets"`
}

// SearchQuery defines a faceted search request.
type SearchQuery struct {
	// TextQuery is an optional keyword search.
	TextQuery string

	// Filters narrow the document set before aggregation.
	Filters map[string][]string // field → values

	// Facets specifies which dimensions to aggregate.
	Facets []FacetType

	// MaxBuckets is the maximum number of buckets per facet (default 20).
	MaxBuckets int

	// Pagination
	From int
	Size int
}

// SearchResult is the combined response for a faceted search request.
type SearchResult struct {
	Total    int
	Hits     []HitDocument
	Facets   []FacetResult
	Took     time.Duration
}

// HitDocument is a single CVE document returned by search.
type HitDocument struct {
	CVEID       string
	Score       float64
	Description string
	Severity    string
	Ecosystem   string
	Published   time.Time
	IsKEV       bool
	CVSS        float64
	EPSSScore   float64
	Tags        []string
}

// OpenSearchFacetedSearcher executes faceted search against OpenSearch.
type OpenSearchFacetedSearcher struct {
	client    *http.Client
	baseURL   string
	indexName string
}

// NewOpenSearchFacetedSearcher creates a new faceted searcher.
func NewOpenSearchFacetedSearcher(baseURL, indexName string) *OpenSearchFacetedSearcher {
	return &OpenSearchFacetedSearcher{
		client:    &http.Client{Timeout: 10 * time.Second},
		baseURL:   strings.TrimRight(baseURL, "/"),
		indexName: indexName,
	}
}

// Search executes a faceted search query.
func (s *OpenSearchFacetedSearcher) Search(ctx context.Context, q SearchQuery) (*SearchResult, error) {
	if q.MaxBuckets <= 0 {
		q.MaxBuckets = 20
	}
	if q.Size <= 0 {
		q.Size = 20
	}

	queryBody := s.buildQuery(q)
	body, err := json.Marshal(queryBody)
	if err != nil {
		return nil, fmt.Errorf("marshal query: %w", err)
	}

	url := fmt.Sprintf("%s/%s/_search", s.baseURL, s.indexName)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("opensearch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("opensearch returned HTTP %d", resp.StatusCode)
	}

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	result := &SearchResult{Took: time.Since(start)}

	// Parse total
	var hitsWrapper struct {
		Total struct{ Value int } `json:"total"`
		Hits  []struct {
			Score  float64         `json:"_score"`
			Source json.RawMessage `json:"_source"`
		} `json:"hits"`
	}
	if hitsRaw, ok := raw["hits"]; ok {
		_ = json.Unmarshal(hitsRaw, &hitsWrapper)
		result.Total = hitsWrapper.Total.Value
		for _, h := range hitsWrapper.Hits {
			var src HitDocument
			_ = json.Unmarshal(h.Source, &src)
			src.Score = h.Score
			result.Hits = append(result.Hits, src)
		}
	}

	// Parse aggregations
	if aggsRaw, ok := raw["aggregations"]; ok {
		var aggs map[string]struct {
			Buckets []struct {
				Key      interface{} `json:"key"`
				DocCount int         `json:"doc_count"`
			} `json:"buckets"`
		}
		if err := json.Unmarshal(aggsRaw, &aggs); err == nil {
			for facetName, agg := range aggs {
				facet := FacetResult{Type: FacetType(facetName)}
				for _, b := range agg.Buckets {
					facet.Buckets = append(facet.Buckets, FacetBucket{
						Key:      fmt.Sprintf("%v", b.Key),
						DocCount: b.DocCount,
					})
				}
				result.Facets = append(result.Facets, facet)
			}
		}
	}

	return result, nil
}

// buildQuery constructs the OpenSearch request body with query + aggregations.
func (s *OpenSearchFacetedSearcher) buildQuery(q SearchQuery) map[string]any {
	// Build the main query
	var mainQuery map[string]any
	if q.TextQuery != "" {
		mainQuery = map[string]any{
			"multi_match": map[string]any{
				"query":  q.TextQuery,
				"fields": []string{"id^3", "description^2", "tags"},
			},
		}
	} else {
		mainQuery = map[string]any{"match_all": map[string]any{}}
	}

	// Add filters
	if len(q.Filters) > 0 {
		var filterClauses []map[string]any
		for field, values := range q.Filters {
			if len(values) == 1 {
				filterClauses = append(filterClauses, map[string]any{
					"term": map[string]any{field: values[0]},
				})
			} else {
				filterClauses = append(filterClauses, map[string]any{
					"terms": map[string]any{field: values},
				})
			}
		}
		mainQuery = map[string]any{
			"bool": map[string]any{
				"must":   mainQuery,
				"filter": filterClauses,
			},
		}
	}

	// Build aggregations
	aggs := make(map[string]any)
	for _, facet := range q.Facets {
		switch facet {
		case FacetYear:
			aggs[string(facet)] = map[string]any{
				"date_histogram": map[string]any{
					"field":             "published",
					"calendar_interval": "year",
					"format":            "yyyy",
					"min_doc_count":     1,
				},
			}
		case FacetKEV:
			aggs[string(facet)] = map[string]any{
				"terms": map[string]any{
					"field": "is_kev",
					"size":  2,
				},
			}
		default:
			aggs[string(facet)] = map[string]any{
				"terms": map[string]any{
					"field": string(facet),
					"size":  q.MaxBuckets,
				},
			}
		}
	}

	body := map[string]any{
		"query": mainQuery,
		"from":  q.From,
		"size":  q.Size,
		"_source": []string{
			"id", "description", "severity", "ecosystem", "published",
			"is_kev", "cvss_score", "epss_score", "tags",
		},
	}
	if len(aggs) > 0 {
		body["aggregations"] = aggs
	}
	return body
}
