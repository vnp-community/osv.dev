// Package semantic_search implements hybrid BM25 + vector search using OpenSearch knn.
// Uses Reciprocal Rank Fusion (RRF) to combine text relevance and semantic similarity.
package semantic_search

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/osv/search-service/internal/domain/entity"
	"github.com/osv/search-service/internal/domain/repository"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	rrfK            = 60    // RRF rank constant (standard value)
	defaultTopK     = 20    // default number of vector neighbors to retrieve
	hybridCandidates = 100  // candidates from each rank before RRF
)

// EmbeddingProvider generates query embeddings.
type EmbeddingProvider interface {
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
}

// HybridSearchParams holds parameters for hybrid search.
type HybridSearchParams struct {
	Query     string
	Filters   repository.SearchFilters
	PageSize  int
	Cursor    string
	// TextWeight and VectorWeight control the relative influence (must sum to 1.0).
	TextWeight   float64 // default: 0.5
	VectorWeight float64 // default: 0.5
}

// Handler orchestrates hybrid search (BM25 + knn) with RRF re-ranking.
type Handler struct {
	index     repository.SearchIndexRepository
	embedding EmbeddingProvider
	cache     repository.SearchCache
	tracer    trace.Tracer
	log       zerolog.Logger
}

// NewHandler creates a HybridSearch handler.
func NewHandler(
	index repository.SearchIndexRepository,
	embedding EmbeddingProvider,
	cache repository.SearchCache,
	tracer trace.Tracer,
	log zerolog.Logger,
) *Handler {
	return &Handler{
		index:     index,
		embedding: embedding,
		cache:     cache,
		tracer:    tracer,
		log:       log,
	}
}

// Handle executes hybrid search and returns re-ranked results.
func (h *Handler) Handle(ctx context.Context, params HybridSearchParams) (*repository.SearchResponse, error) {
	ctx, span := h.tracer.Start(ctx, "HybridSearch")
	defer span.End()

	span.SetAttributes(
		attribute.String("query", params.Query),
		attribute.Float64("text_weight", params.TextWeight),
		attribute.Float64("vector_weight", params.VectorWeight),
	)

	if params.TextWeight == 0 && params.VectorWeight == 0 {
		params.TextWeight = 0.5
		params.VectorWeight = 0.5
	}

	// Normalize weights
	total := params.TextWeight + params.VectorWeight
	if total > 0 {
		params.TextWeight /= total
		params.VectorWeight /= total
	}

	start := time.Now()

	// Run BM25 text search and vector search concurrently
	type rankResult struct {
		results []*rankedResult
		err     error
	}

	textCh := make(chan rankResult, 1)
	vectorCh := make(chan rankResult, 1)

	// BM25 text search
	go func() {
		results, err := h.textSearch(ctx, params)
		textCh <- rankResult{results: results, err: err}
	}()

	// Vector similarity search
	go func() {
		results, err := h.vectorSearch(ctx, params)
		vectorCh <- rankResult{results: results, err: err}
	}()

	textRes := <-textCh
	vectorRes := <-vectorCh

	if textRes.err != nil && vectorRes.err != nil {
		return nil, fmt.Errorf("both text and vector search failed: text=%v, vector=%v", textRes.err, vectorRes.err)
	}

	h.log.Debug().
		Str("query", params.Query).
		Int("text_results", len(textRes.results)).
		Int("vector_results", len(vectorRes.results)).
		Dur("search_ms", time.Since(start)).
		Msg("hybrid search results")

	// Apply RRF re-ranking
	merged := rrfMerge(textRes.results, vectorRes.results, params.TextWeight, params.VectorWeight)

	// Paginate
	pageSize := params.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	var nextCursor string
	if len(merged) > pageSize {
		nextCursor = merged[pageSize-1].vulnID
		merged = merged[:pageSize]
	}

	// Convert to SearchResult
	searchResults := make([]entity.SearchResult, 0, len(merged))
	for _, r := range merged {
		searchResults = append(searchResults, entity.SearchResult{
			VulnID:   r.vulnID,
			Summary:  r.summary,
			Severity: r.severity,
			Modified: r.modified,
			Score:    float32(r.rrfScore),
		})
	}

	return &repository.SearchResponse{
		Results:    searchResults,
		TotalHits:  int64(len(merged)),
		NextCursor: nextCursor,
	}, nil
}

// ── Search implementations ────────────────────────────────────────────────────

func (h *Handler) textSearch(ctx context.Context, params HybridSearchParams) ([]*rankedResult, error) {
	resp, err := h.index.Search(ctx, &repository.SearchParams{
		Query:    params.Query,
		Filters:  params.Filters,
		PageSize: hybridCandidates,
	})
	if err != nil {
		return nil, err
	}

	results := make([]*rankedResult, len(resp.Results))
	for i, r := range resp.Results {
		results[i] = &rankedResult{
			vulnID:   r.VulnID,
			summary:  r.Summary,
			severity: r.Severity,
			modified: r.Modified,
			textRank: i + 1,
		}
	}
	return results, nil
}

func (h *Handler) vectorSearch(ctx context.Context, params HybridSearchParams) ([]*rankedResult, error) {
	if params.Query == "" {
		return nil, nil
	}

	embedding, err := h.embedding.GenerateEmbedding(ctx, params.Query)
	if err != nil {
		return nil, fmt.Errorf("generate query embedding: %w", err)
	}

	resp, err := h.index.VectorSearch(ctx, embedding, hybridCandidates, params.Filters)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}

	results := make([]*rankedResult, len(resp.Results))
	for i, r := range resp.Results {
		results[i] = &rankedResult{
			vulnID:     r.VulnID,
			summary:    r.Summary,
			severity:   r.Severity,
			modified:   r.Modified,
			vectorRank: i + 1,
		}
	}
	return results, nil
}

// ── RRF Re-ranking ────────────────────────────────────────────────────────────

type rankedResult struct {
	vulnID     string
	summary    string
	severity   string
	modified   time.Time
	textRank   int
	vectorRank int
	rrfScore   float64
}

// rrfMerge combines text and vector rankings using Reciprocal Rank Fusion.
func rrfMerge(text, vector []*rankedResult, textWeight, vectorWeight float64) []*rankedResult {
	merged := make(map[string]*rankedResult)

	for _, r := range text {
		merged[r.vulnID] = &rankedResult{
			vulnID:   r.vulnID,
			summary:  r.summary,
			severity: r.severity,
			modified: r.modified,
			textRank: r.textRank,
		}
	}

	for _, r := range vector {
		if existing, ok := merged[r.vulnID]; ok {
			existing.vectorRank = r.vectorRank
		} else {
			merged[r.vulnID] = &rankedResult{
				vulnID:     r.vulnID,
				summary:    r.summary,
				severity:   r.severity,
				modified:   r.modified,
				vectorRank: r.vectorRank,
			}
		}
	}

	// Compute RRF score
	for _, r := range merged {
		score := 0.0
		if r.textRank > 0 {
			score += textWeight * (1.0 / (float64(rrfK) + float64(r.textRank)))
		}
		if r.vectorRank > 0 {
			score += vectorWeight * (1.0 / (float64(rrfK) + float64(r.vectorRank)))
		}
		r.rrfScore = math.Round(score*1e6) / 1e6
	}

	// Sort by RRF score descending
	results := make([]*rankedResult, 0, len(merged))
	for _, r := range merged {
		results = append(results, r)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].rrfScore > results[j].rrfScore
	})

	return results
}
