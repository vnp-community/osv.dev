// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package search_vulnerabilities implements the SearchVulnerabilities query handler.
package search_vulnerabilities

import (
	"context"
	"fmt"
	"time"

	"github.com/osv/search/internal/domain/entity"
	"github.com/osv/search/internal/domain/repository"
	"github.com/osv/search/internal/domain/service"
	"github.com/osv/search/internal/domain/valueobject"
)

// Query is the input to the SearchVulnerabilities handler.
type Query struct {
	Raw       string
	Ecosystems []string
	Severities []string
	DateFrom  string
	DateTo    string
	Withdrawn bool
	Sources   []string
	Sort      valueobject.SortOrder
	PageSize  int
	PageToken string
}

// Result is the output of SearchVulnerabilities.
type Result struct {
	Hits          []*entity.SearchResult
	EcosystemFacets []*entity.Facet
	SeverityFacets  []*entity.Facet
	TotalHits     int64
	NextPageToken string
	TookMs        int64
}

// Handler implements search with Redis cache-aside pattern.
type Handler struct {
	searchIndex repository.SearchIndexRepo
	cache       repository.SearchCache
	queryParser *service.QueryParser
	analytics   AnalyticsPort
}

// AnalyticsPort sends search events to BigQuery (fire-and-forget).
type AnalyticsPort interface {
	TrackSearch(ctx context.Context, event SearchEvent)
}

// SearchEvent is a search analytics event.
type SearchEvent struct {
	Query      string
	TotalHits  int64
	DurationMs int64
	SearchedAt time.Time
}

// NewHandler creates a new SearchVulnerabilitiesHandler.
func NewHandler(idx repository.SearchIndexRepo, cache repository.SearchCache, analytics AnalyticsPort) *Handler {
	return &Handler{
		searchIndex: idx,
		cache:       cache,
		queryParser: service.NewQueryParser(),
		analytics:   analytics,
	}
}

// Handle executes a search query with Redis cache-aside (TTL 30s).
func (h *Handler) Handle(ctx context.Context, q Query) (*Result, error) {
	if q.PageSize <= 0 {
		q.PageSize = 20
	}

	parsed := h.queryParser.Parse(q.Raw)
	// Merge explicit filter params into parsed query.
	if len(q.Ecosystems) > 0 {
		parsed.Ecosystems = q.Ecosystems
	}
	if len(q.Severities) > 0 {
		parsed.Severities = q.Severities
	}
	parsed.SortOrder = q.Sort
	parsed.PageSize = q.PageSize
	parsed.PageToken = q.PageToken

	// Check Redis cache (TTL 30s for hot queries).
	cacheKey := buildCacheKey(parsed)
	if cached, err := h.cache.Get(ctx, cacheKey); err == nil && cached != nil {
		return cached, nil
	}

	start := time.Now()
	result, err := h.searchIndex.Search(ctx, parsed)
	if err != nil {
		return nil, fmt.Errorf("search: opensearch: %w", err)
	}
	result.TookMs = time.Since(start).Milliseconds()

	// Cache the result.
	_ = h.cache.Set(ctx, cacheKey, result, 30*time.Second)

	// Track analytics asynchronously.
	go h.analytics.TrackSearch(context.Background(), SearchEvent{
		Query:      q.Raw,
		TotalHits:  result.TotalHits,
		DurationMs: result.TookMs,
		SearchedAt: time.Now().UTC(),
	})

	return result, nil
}

func buildCacheKey(q *valueobject.SearchQuery) string {
	return fmt.Sprintf("osv:search:%s:%v:%v:%d", q.Keywords, q.Ecosystems, q.Severities, q.PageSize)
}
