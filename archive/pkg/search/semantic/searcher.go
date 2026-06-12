// Package semantic implements vector/semantic search using OpenSearch k-NN.
// TASK-07-02: Semantic search queries using pre-computed embeddings stored in OpenSearch.
package semantic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// EmbeddingVector is a dense float32 vector for semantic similarity search.
type EmbeddingVector []float32

// SemanticResult is a single semantic search hit.
type SemanticResult struct {
	CVEID       string
	Score       float32
	Description string
	Severity    string
	CVSS        float64
	IsKEV       bool
	EPSSScore   float64
	Tags        []string
	Published   time.Time
}

// SemanticQuery holds the input for a semantic search request.
type SemanticQuery struct {
	// TextQuery is the free-text query to embed (if VectorQuery is not set).
	TextQuery string

	// VectorQuery is a pre-computed embedding vector (bypasses embedding call).
	VectorQuery EmbeddingVector

	// K is the number of nearest neighbours to retrieve.
	K int

	// MinScore filters results below this similarity score (0-1).
	MinScore float32

	// Filters narrow results by metadata fields.
	Filters SemanticFilters
}

// SemanticFilters apply metadata constraints on top of vector similarity.
type SemanticFilters struct {
	Ecosystem  string
	IsKEV      *bool
	MinCVSS    float64
	MaxCVSS    float64
	Since      time.Time
	Tags       []string
	Severities []string
}

// Embedder converts text to dense embedding vectors.
type Embedder interface {
	Embed(ctx context.Context, text string) (EmbeddingVector, error)
}

// Searcher executes semantic search queries.
type Searcher interface {
	Search(ctx context.Context, query SemanticQuery) ([]SemanticResult, error)
}

// OpenSearchSemanticSearcher performs k-NN semantic search against OpenSearch.
type OpenSearchSemanticSearcher struct {
	client    *http.Client
	baseURL   string
	indexName string
	embedder  Embedder
	k         int
}

// NewOpenSearchSemanticSearcher creates a semantic searcher backed by OpenSearch.
func NewOpenSearchSemanticSearcher(baseURL, indexName string, embedder Embedder, k int) *OpenSearchSemanticSearcher {
	if k <= 0 {
		k = 20
	}
	return &OpenSearchSemanticSearcher{
		client:    &http.Client{Timeout: 10 * time.Second},
		baseURL:   strings.TrimRight(baseURL, "/"),
		indexName: indexName,
		embedder:  embedder,
		k:         k,
	}
}

// Search executes a semantic k-NN search query.
func (s *OpenSearchSemanticSearcher) Search(ctx context.Context, query SemanticQuery) ([]SemanticResult, error) {
	// Get or compute embedding vector
	vec := query.VectorQuery
	if len(vec) == 0 && query.TextQuery != "" {
		if s.embedder == nil {
			return nil, fmt.Errorf("semantic search: no embedder configured and no vector provided")
		}
		var err error
		vec, err = s.embedder.Embed(ctx, query.TextQuery)
		if err != nil {
			return nil, fmt.Errorf("embed query: %w", err)
		}
	}
	if len(vec) == 0 {
		return nil, fmt.Errorf("semantic search: either TextQuery or VectorQuery must be provided")
	}

	k := query.K
	if k <= 0 {
		k = s.k
	}

	// Build OpenSearch k-NN query with optional filters
	queryBody := s.buildKNNQuery(vec, k, query.MinScore, query.Filters)

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

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("opensearch request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("opensearch returned HTTP %d", resp.StatusCode)
	}

	var searchResp openSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	results := make([]SemanticResult, 0, len(searchResp.Hits.Hits))
	for _, hit := range searchResp.Hits.Hits {
		if query.MinScore > 0 && float32(hit.Score) < query.MinScore {
			continue
		}
		result := SemanticResult{
			CVEID: hit.Source.ID,
			Score: float32(hit.Score),
		}
		if hit.Source.Description != "" {
			result.Description = hit.Source.Description
		}
		result.Severity = hit.Source.Severity
		result.CVSS = hit.Source.CVSSScore
		result.IsKEV = hit.Source.IsKEV
		result.EPSSScore = hit.Source.EPSSScore
		result.Tags = hit.Source.Tags
		result.Published = hit.Source.Published
		results = append(results, result)
	}

	return results, nil
}

// buildKNNQuery constructs the OpenSearch k-NN query body.
func (s *OpenSearchSemanticSearcher) buildKNNQuery(vec EmbeddingVector, k int, minScore float32, filters SemanticFilters) map[string]any {
	knnQuery := map[string]any{
		"vector": vec,
		"k":      k,
	}

	// Build filter clauses
	var mustClauses []map[string]any

	if filters.Ecosystem != "" {
		mustClauses = append(mustClauses, map[string]any{
			"term": map[string]any{"ecosystem": filters.Ecosystem},
		})
	}
	if filters.IsKEV != nil {
		mustClauses = append(mustClauses, map[string]any{
			"term": map[string]any{"is_kev": *filters.IsKEV},
		})
	}
	if filters.MinCVSS > 0 || filters.MaxCVSS > 0 {
		cvssRange := map[string]any{}
		if filters.MinCVSS > 0 {
			cvssRange["gte"] = filters.MinCVSS
		}
		if filters.MaxCVSS > 0 {
			cvssRange["lte"] = filters.MaxCVSS
		}
		mustClauses = append(mustClauses, map[string]any{
			"range": map[string]any{"cvss_score": cvssRange},
		})
	}
	if !filters.Since.IsZero() {
		mustClauses = append(mustClauses, map[string]any{
			"range": map[string]any{
				"published": map[string]any{"gte": filters.Since.Format(time.RFC3339)},
			},
		})
	}
	if len(filters.Tags) > 0 {
		mustClauses = append(mustClauses, map[string]any{
			"terms": map[string]any{"tags": filters.Tags},
		})
	}
	if len(filters.Severities) > 0 {
		mustClauses = append(mustClauses, map[string]any{
			"terms": map[string]any{"severity": filters.Severities},
		})
	}

	// Apply filters to k-NN query if any exist
	if len(mustClauses) > 0 {
		knnQuery["filter"] = map[string]any{
			"bool": map[string]any{"must": mustClauses},
		}
	}

	query := map[string]any{
		"query": map[string]any{
			"knn": map[string]any{
				"embedding": knnQuery,
			},
		},
		"_source": []string{"id", "description", "severity", "cvss_score", "is_kev", "epss_score", "tags", "published"},
	}

	if minScore > 0 {
		query["min_score"] = minScore
	}

	return query
}

// ---- OpenSearch response types ----

type openSearchResponse struct {
	Hits struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
		Hits []struct {
			ID     string          `json:"_id"`
			Score  float64         `json:"_score"`
			Source openSearchSource `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

type openSearchSource struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Severity    string    `json:"severity"`
	CVSSScore   float64   `json:"cvss_score"`
	IsKEV       bool      `json:"is_kev"`
	EPSSScore   float64   `json:"epss_score"`
	Tags        []string  `json:"tags"`
	Published   time.Time `json:"published"`
}

// ---- Static (test) embedder for dev/testing ----

// StaticEmbedder returns a fixed-dimension zero vector (for testing index connectivity).
type StaticEmbedder struct {
	Dimensions int
}

// Embed returns a zero vector of the configured dimensions.
func (e *StaticEmbedder) Embed(_ context.Context, _ string) (EmbeddingVector, error) {
	vec := make(EmbeddingVector, e.Dimensions)
	return vec, nil
}
