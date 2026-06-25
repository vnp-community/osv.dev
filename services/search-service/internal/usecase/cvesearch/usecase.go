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
	"github.com/osv/search-service/internal/infra/opensearch"
	"os"
)

const defaultCacheTTL = 300 // 5 minutes

// Request holds search parameters.
type Request struct {
	Query     string
	Severity  string
	Source    string
	Sort      string
	Page      int
	Limit     int
	// CR-GCV-002: EPSS filters
	MinEPSS   *float64
	MaxEPSS   *float64
	// CR-GCV-003: Exploit/KEV bool filters
	IsKEV     *bool
	IsExploit *bool
	// CR-GCV-005: CWE / Vendor / Product text filters
	CWE       string
	Vendor    string
	Product   string
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
	pgRepo    repository.CVERepository
	osClient  *opensearch.Client
	osEnabled bool
	cacheRepo repository.CVECacheRepository
	log       zerolog.Logger
	cacheTTL  int
}

// New creates a search UseCase.
func New(cveRepo repository.CVERepository, cacheRepo repository.CVECacheRepository, osClient *opensearch.Client, log zerolog.Logger) *UseCase {
	enabled := os.Getenv("OPENSEARCH_ENABLED") == "true"
	return &UseCase{
		pgRepo:    cveRepo,
		osClient:  osClient,
		osEnabled: enabled && osClient != nil,
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

	// 2. Search Backend
	var cves []*entity.CVE
	var total int64
	var err error

	if uc.osEnabled && req.Query != "" {
		cves, total, err = uc.searchViaOpenSearch(ctx, req)
		if err != nil {
			uc.log.Warn().Err(err).Msg("OpenSearch search failed, falling back to PostgreSQL")
			cves, total, err = uc.searchViaPostgres(ctx, filter)
		}
	} else {
		cves, total, err = uc.searchViaPostgres(ctx, filter)
	}

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

func (uc *UseCase) searchViaOpenSearch(ctx context.Context, req *Request) ([]*entity.CVE, int64, error) {
	var severity []string
	if req.Severity != "" {
		severity = []string{req.Severity}
	}
	osReq := &opensearch.SearchCVEsRequest{
		Query:    req.Query,
		Severity: severity,
		MinEPSS:  req.MinEPSS,
		Page:     req.Page,
		Limit:    req.Limit,
	}
	return uc.osClient.SearchCVEs(ctx, osReq)
}

func (uc *UseCase) searchViaPostgres(ctx context.Context, filter *entity.SearchFilter) ([]*entity.CVE, int64, error) {
	return uc.pgRepo.Search(ctx, filter)
}

func buildFilter(req *Request) *entity.SearchFilter {
	f := &entity.SearchFilter{
		Query:     req.Query,
		Sort:      entity.SortOrder(req.Sort),
		Page:      req.Page,
		Limit:     req.Limit,
		MinEPSS:   req.MinEPSS,
		MaxEPSS:   req.MaxEPSS,
		IsKEV:     req.IsKEV,
		IsExploit: req.IsExploit,
		CWE:       req.CWE,
		Vendor:    req.Vendor,
		Product:   req.Product,
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
	epssMin, epssMax := "", ""
	if req.MinEPSS != nil {
		epssMin = fmt.Sprintf("%.4f", *req.MinEPSS)
	}
	if req.MaxEPSS != nil {
		epssMax = fmt.Sprintf("%.4f", *req.MaxEPSS)
	}
	kev, exploit := "", ""
	if req.IsKEV != nil {
		kev = fmt.Sprintf("%v", *req.IsKEV)
	}
	if req.IsExploit != nil {
		exploit = fmt.Sprintf("%v", *req.IsExploit)
	}
	raw := fmt.Sprintf("%s:%s:%s:%s:%d:%d:%s:%s:%s:%s:%s:%s:%s",
		req.Query, req.Severity, req.Source, req.Sort, req.Page, req.Limit,
		epssMin, epssMax, kev, exploit, req.CWE, req.Vendor, req.Product)
	h := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", h[:8])
}

// ensure domainerrors is used
var _ = domainerrors.ErrCacheMiss
