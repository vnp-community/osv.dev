// Package osv_query — usecase.go
// OSVQueryUseCase provides OSV v1 compatible query operations.
//
// Endpoints served:
//   POST /v1/query        → QueryByPackage
//   POST /v1/querybatch   → BatchQuery (max 1000)
//   GET  /v1/vulns/{id}   → GetVulnByID (delegates to getbyid.UseCase)
//   GET  /v1/vulns/list   → ListVulns
package osv_query

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog"

	"github.com/osv/search-service/internal/domain/entity"
	"github.com/osv/search-service/internal/domain/repository"
	"github.com/osv/search-service/internal/usecase/cvesearch"
)

const (
	maxBatchSize = 1000
	defaultLimit = 20
	maxListLimit = 200
)

// OSVQueryUseCase implements OSV v1 compatible query operations.
type OSVQueryUseCase struct {
	cveRepo  repository.CVERepository
	searchUC *search.UseCase
	log      zerolog.Logger
}

// NewOSVQueryUseCase creates an OSVQueryUseCase.
func NewOSVQueryUseCase(cveRepo repository.CVERepository, searchUC *search.UseCase, log zerolog.Logger) *OSVQueryUseCase {
	return &OSVQueryUseCase{cveRepo: cveRepo, searchUC: searchUC, log: log}
}

// QueryByPackage handles POST /v1/query — searches CVEs affecting a package/ecosystem.
func (uc *OSVQueryUseCase) QueryByPackage(ctx context.Context, req OSVQueryRequest) (*OSVQueryResult, error) {
	// Build a search query from ecosystem + package name
	q := buildSearchQuery(req)

	resp, err := uc.searchUC.Execute(ctx, &search.Request{
		Query: q,
		Limit: 20,
		Page:  0,
	})
	if err != nil {
		return nil, fmt.Errorf("osv_query: %w", err)
	}

	return &OSVQueryResult{Vulns: cveSliceToSummary(resp.CVEs)}, nil
}

// BatchQuery handles POST /v1/querybatch — batch of up to 1000 queries.
func (uc *OSVQueryUseCase) BatchQuery(ctx context.Context, req OSVBatchRequest) (*OSVBatchResult, error) {
	if len(req.Queries) > maxBatchSize {
		return nil, fmt.Errorf("batch too large: max %d queries", maxBatchSize)
	}

	result := &OSVBatchResult{Results: make([]OSVQueryResult, 0, len(req.Queries))}
	for _, q := range req.Queries {
		res, err := uc.QueryByPackage(ctx, q)
		if err != nil {
			uc.log.Warn().Err(err).Msg("osv_query: batch item failed, returning empty")
			result.Results = append(result.Results, OSVQueryResult{Vulns: nil})
			continue
		}
		result.Results = append(result.Results, *res)
	}
	return result, nil
}

// ListVulns handles GET /v1/vulns/list — paginated CVE listing with optional filters.
func (uc *OSVQueryUseCase) ListVulns(ctx context.Context, params OSVVulnListParams) (*OSVVulnListResponse, error) {
	req := &search.Request{
		Query: params.Ecosystem, // use ecosystem as keyword filter
		Limit: defaultLimit,
		Page:  0,
	}

	resp, err := uc.searchUC.Execute(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("osv_query list: %w", err)
	}

	vulns := cveSliceToSummary(resp.CVEs)

	// Build next_page_token if there are more results
	nextToken := ""
	if resp.HasMore {
		nextToken = fmt.Sprintf("page=%d", req.Page+1)
	}

	return &OSVVulnListResponse{
		Vulns:         vulns,
		NextPageToken: nextToken,
	}, nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

// buildSearchQuery extracts the most useful search term from an OSVQueryRequest.
func buildSearchQuery(req OSVQueryRequest) string {
	parts := []string{}
	if req.Package.Name != "" {
		parts = append(parts, req.Package.Name)
	}
	if req.Package.Ecosystem != "" {
		parts = append(parts, req.Package.Ecosystem)
	}
	if req.Version != "" {
		parts = append(parts, req.Version)
	}
	return strings.Join(parts, " ")
}

// cveSliceToSummary converts CVE domain entities to OSV summary DTOs.
func cveSliceToSummary(cves []*entity.CVE) []OSVVulnSummary {
	if len(cves) == 0 {
		return nil
	}
	out := make([]OSVVulnSummary, 0, len(cves))
	for _, c := range cves {
		out = append(out, OSVVulnSummary{
			ID:        c.ID,
			Modified:  c.UpdatedAt.Format("2006-01-02T15:04:05Z"),
			Summary:   c.Description,
			Severity:  string(c.Severity),
			Published: c.Published.Format("2006-01-02T15:04:05Z"),
		})
	}
	return out
}
