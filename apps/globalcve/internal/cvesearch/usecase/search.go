// Package usecase — CVE Search use case.
package usecase

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/globalcve/mono/internal/cvesearch/domain/entity"
	"github.com/globalcve/mono/internal/cvesearch/domain/repository"
)

// SearchRequest holds the input parameters for CVE search.
type SearchRequest struct {
	Query   string
	Severity string
	Source  string
	Sort    string
	Page    int
	Limit   int
	IsKEV   *bool
	MinEPSS *float64
}

// SearchResponse holds the result of a CVE search.
type SearchResponse struct {
	Query     string
	Total     int64
	Page      int
	Limit     int
	HasMore   bool
	CVEs      []*entity.CVE
	FromCache bool
	TookMs    int64
}

// SearchUseCase executes CVE search queries.
type SearchUseCase struct {
	cveRepo   repository.CVERepository
	cacheRepo repository.CVECacheRepository
	cacheTTL  int
}

// NewSearchUseCase creates a new CVE search use case.
func NewSearchUseCase(
	cveRepo repository.CVERepository,
	cacheRepo repository.CVECacheRepository,
	cacheTTL int,
) *SearchUseCase {
	return &SearchUseCase{
		cveRepo:   cveRepo,
		cacheRepo: cacheRepo,
		cacheTTL:  cacheTTL,
	}
}

// Execute performs a CVE search with caching.
func (uc *SearchUseCase) Execute(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	start := time.Now()

	// Build and validate filter
	filter := buildFilter(req)
	filter.Validate()

	// Check cache
	cacheKey := buildCacheKey(req)
	if cached, ok, _ := uc.cacheRepo.GetSearchResult(ctx, cacheKey); ok {
		var resp SearchResponse
		if err := json.Unmarshal(cached, &resp); err == nil {
			resp.FromCache = true
			resp.TookMs = time.Since(start).Milliseconds()
			return &resp, nil
		}
	}

	// Query database
	cves, total, err := uc.cveRepo.Search(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("search cves: %w", err)
	}

	resp := &SearchResponse{
		Query:   req.Query,
		Total:   total,
		Page:    filter.Page,
		Limit:   filter.Limit,
		HasMore: int64(filter.Page*filter.Limit+len(cves)) < total,
		CVEs:    cves,
		TookMs:  time.Since(start).Milliseconds(),
	}

	// Cache result asynchronously
	go func() {
		data, err := json.Marshal(resp)
		if err != nil {
			return
		}
		cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := uc.cacheRepo.SetSearchResult(cacheCtx, cacheKey, data, uc.cacheTTL); err != nil {
			log.Warn().Err(err).Msg("search: cache set failed")
		}
	}()

	return resp, nil
}

// GetByID retrieves a single CVE with caching.
type GetByIDUseCase struct {
	cveRepo   repository.CVERepository
	cacheRepo repository.CVECacheRepository
	cacheTTL  int
}

// NewGetByIDUseCase creates a new get-by-ID use case.
func NewGetByIDUseCase(
	cveRepo repository.CVERepository,
	cacheRepo repository.CVECacheRepository,
	cacheTTL int,
) *GetByIDUseCase {
	return &GetByIDUseCase{
		cveRepo:   cveRepo,
		cacheRepo: cacheRepo,
		cacheTTL:  cacheTTL,
	}
}

// Execute retrieves a CVE by ID with Redis caching.
func (uc *GetByIDUseCase) Execute(ctx context.Context, id string) (*entity.CVE, error) {
	if !entity.IsValidID(id) {
		return nil, fmt.Errorf("invalid CVE ID: %s", id)
	}

	// Check cache
	if cached, ok, _ := uc.cacheRepo.GetCVE(ctx, id); ok {
		var cve entity.CVE
		if err := json.Unmarshal(cached, &cve); err == nil {
			return &cve, nil
		}
	}

	// Query DB
	cve, err := uc.cveRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Cache asynchronously
	go func() {
		data, err := json.Marshal(cve)
		if err != nil {
			return
		}
		cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = uc.cacheRepo.SetCVE(cacheCtx, id, data, uc.cacheTTL)
	}()

	return cve, nil
}

// --- helpers ---

func buildFilter(req SearchRequest) *entity.SearchFilter {
	filter := &entity.SearchFilter{
		Query:   req.Query,
		Sort:    entity.SortOrder(req.Sort),
		Page:    req.Page,
		Limit:   req.Limit,
		IsKEV:   req.IsKEV,
		MinEPSS: req.MinEPSS,
	}

	if req.Severity != "" {
		sev := entity.Severity(req.Severity)
		filter.Severity = &sev
	}
	if req.Source != "" {
		src := entity.Source(req.Source)
		filter.Source = &src
	}

	return filter
}

func buildCacheKey(req SearchRequest) string {
	raw := fmt.Sprintf("%s|%s|%s|%s|%d|%d|%v|%v",
		req.Query, req.Severity, req.Source, req.Sort,
		req.Page, req.Limit, req.IsKEV, req.MinEPSS,
	)
	return fmt.Sprintf("%x", md5.Sum([]byte(raw)))
}
