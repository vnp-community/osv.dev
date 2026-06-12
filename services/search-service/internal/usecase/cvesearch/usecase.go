// Package search provides the CVE search use case with cache-first strategy.
package search

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/osv/search-service/internal/domain/entity"
	domainerrors "github.com/osv/search-service/internal/domain/errors"
	"github.com/osv/search-service/internal/domain/repository"
)

const defaultCacheTTL = 300 // 5 minutes

// Request holds search parameters.
type Request struct {
	Query    string
	Severity string
	Source   string
	Sort     string
	Page     int
	Limit    int
}

// Response holds the search result.
type Response struct {
	Query     string        `json:"query,omitempty"`
	Total     int64         `json:"total"`
	Page      int           `json:"page"`
	Limit     int           `json:"limit"`
	HasMore   bool          `json:"has_more"`
	CVEs      []*entity.CVE `json:"cves"`
	FromCache bool          `json:"from_cache"`
}

// UseCase handles CVE search with cache-first strategy.
type UseCase struct {
	cveRepo   repository.CVERepository
	cacheRepo repository.CVECacheRepository
	log       zerolog.Logger
	cacheTTL  int
}

// New creates a search UseCase.
func New(cveRepo repository.CVERepository, cacheRepo repository.CVECacheRepository, log zerolog.Logger) *UseCase {
	return &UseCase{
		cveRepo:   cveRepo,
		cacheRepo: cacheRepo,
		log:       log.With().Str("usecase", "cve.search").Logger(),
		cacheTTL:  defaultCacheTTL,
	}
}

// Execute performs a cached CVE search.
func (uc *UseCase) Execute(ctx context.Context, req *Request) (*Response, error) {
	filter := buildFilter(req)
	cacheKey := buildCacheKey(req)

	// 1. Cache hit?
	if cached, err := uc.cacheRepo.GetSearchResult(ctx, cacheKey); err == nil {
		var resp Response
		if json.Unmarshal(cached, &resp) == nil {
			resp.FromCache = true
			return &resp, nil
		}
	}

	// 2. DB query
	cves, total, err := uc.cveRepo.Search(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("cve search: %w", err)
	}

	resp := &Response{
		Query:   req.Query,
		Total:   total,
		Page:    filter.Page,
		Limit:   filter.Limit,
		HasMore: int64((filter.Page+1)*filter.Limit) < total,
		CVEs:    cves,
	}

	// 3. Async cache store
	go func() {
		storeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if data, err := json.Marshal(resp); err == nil {
			uc.cacheRepo.SetSearchResult(storeCtx, cacheKey, data, uc.cacheTTL) //nolint:errcheck
		}
	}()

	return resp, nil
}

func buildFilter(req *Request) *entity.SearchFilter {
	f := &entity.SearchFilter{
		Query: req.Query,
		Sort:  entity.SortOrder(req.Sort),
		Page:  req.Page,
		Limit: req.Limit,
	}
	if req.Severity != "" {
		s := entity.Severity(req.Severity)
		f.Severity = &s
	}
	if req.Source != "" {
		src := entity.Source(req.Source)
		f.Source = &src
	}
	f.Validate()
	return f
}

func buildCacheKey(req *Request) string {
	raw := fmt.Sprintf("%s:%s:%s:%s:%d:%d",
		req.Query, req.Severity, req.Source, req.Sort, req.Page, req.Limit)
	h := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", h[:8])
}

// ensure domainerrors is used
var _ = domainerrors.ErrCacheMiss
