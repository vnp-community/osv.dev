# S2-SEARCH-01 — Thêm OSV v1 Query API (search-service)


## ✅ Execution Status: COMPLETED
## Metadata
- **Task ID**: S2-SEARCH-01
- **Service**: search-service
- **Sprint**: 2 (P1)
- **Ước tính**: 3 giờ
- **Dependencies**: Không có
- **Spec nguồn**: `specs/develop/03_search-service-upgrade.md` § "P1 — Thêm: OSV v1 Compatible API"
- **Reference**: https://osv.dev/docs/#section/OSV-API

## Context

```bash
# OSV v1 spec: https://osv.dev/docs/
# Đọc existing search handler:
cat services/search-service/internal/delivery/http/search_handler.go

# Đọc CVE search UC:
cat services/search-service/internal/usecase/cvesearch/usecase.go
cat services/search-service/internal/usecase/getbyid/usecase.go

# Đọc domain entities:
cat services/search-service/internal/domain/entity/search.go
cat services/search-service/internal/domain/entity/cve.go

# Router setup:
cat services/search-service/cmd/server/main.go | grep -A30 "router\|route\|chi"
```

## Goal

Thêm OSV v1 compatible endpoints:
- `POST /v1/query` — query by package version
- `POST /v1/querybatch` — batch query (max 1000)
- `GET /v1/vulns/{id}` — get single vulnerability
- `GET /v1/vulns/list` — paginated listing

## Files to Create

### File 1: `services/search-service/internal/usecase/osv_query/dto.go`

```go
package osv_query

// OSVPackage identifies a package in the OSV ecosystem.
type OSVPackage struct {
	Ecosystem string `json:"ecosystem"`  // e.g., "Go", "npm", "PyPI"
	Name      string `json:"name"`       // e.g., "github.com/gin-gonic/gin"
	PURL      string `json:"purl"`       // e.g., "pkg:golang/github.com/gin-gonic/gin"
}

// OSVQueryRequest is the input for POST /v1/query.
type OSVQueryRequest struct {
	Version string     `json:"version"`  // package version to query
	Package OSVPackage `json:"package"`
	Commit  string     `json:"commit"`   // git commit hash (alternative to version)
}

// OSVBatchRequest is the input for POST /v1/querybatch.
type OSVBatchRequest struct {
	Queries []OSVQueryRequest `json:"queries"`  // max 1000
}

// OSVVulnListParams are query params for GET /v1/vulns/list.
type OSVVulnListParams struct {
	PageToken     string `query:"page_token"`
	ModifiedSince string `query:"modified_since"`  // RFC3339
	Ecosystem     string `query:"ecosystem"`
}

// OSVVulnListResponse is the response for GET /v1/vulns/list.
type OSVVulnListResponse struct {
	Vulns         []interface{} `json:"vulns"`
	NextPageToken string        `json:"next_page_token,omitempty"`
}

// OSVQueryResult is the response for a single query.
type OSVQueryResult struct {
	Vulns []interface{} `json:"vulns"`  // OSV-schema vulnerability objects
}

// OSVBatchResult is the response for batch query.
type OSVBatchResult struct {
	Results []OSVQueryResult `json:"results"`
}
```

### File 2: `services/search-service/internal/usecase/osv_query/usecase.go`

```go
package osv_query

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/osv/search-service/internal/domain/repository"
	"github.com/osv/search-service/internal/usecase/getbyid"
)

// UseCase handles OSV v1 compatible queries.
type UseCase struct {
	repo    repository.CVERepository
	getByID *getbyid.UseCase  // reuse existing UC
	log     zerolog.Logger
}

// NewUseCase creates a new OSV query UseCase.
func NewUseCase(repo repository.CVERepository, getByID *getbyid.UseCase, log zerolog.Logger) *UseCase {
	return &UseCase{repo: repo, getByID: getByID, log: log}
}

// QueryByPackage finds vulnerabilities affecting a specific package version.
func (uc *UseCase) QueryByPackage(ctx context.Context, req OSVQueryRequest) (*OSVQueryResult, error) {
	if req.Package.Name == "" && req.Package.PURL == "" {
		return &OSVQueryResult{Vulns: []interface{}{}}, nil
	}

	// Search by package + version
	vulns, err := uc.repo.FindByPackageVersion(ctx, repository.PackageVersionFilter{
		Ecosystem: req.Package.Ecosystem,
		Name:      req.Package.Name,
		PURL:      req.Package.PURL,
		Version:   req.Version,
		Commit:    req.Commit,
	})
	if err != nil {
		return nil, fmt.Errorf("query by package: %w", err)
	}

	return &OSVQueryResult{Vulns: toOSVSlice(vulns)}, nil
}

// QueryBatch handles up to 1000 queries in parallel.
func (uc *UseCase) QueryBatch(ctx context.Context, queries []OSVQueryRequest) (*OSVBatchResult, error) {
	if len(queries) > 1000 {
		return nil, fmt.Errorf("batch size exceeds 1000 limit")
	}

	results := make([]OSVQueryResult, len(queries))
	errCh := make(chan error, len(queries))

	// Use semaphore to limit concurrency
	sem := make(chan struct{}, 10)

	var wg sync.WaitGroup
	for i, q := range queries {
		wg.Add(1)
		go func(idx int, query OSVQueryRequest) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result, err := uc.QueryByPackage(ctx, query)
			if err != nil {
				errCh <- err
				results[idx] = OSVQueryResult{Vulns: []interface{}{}}
				return
			}
			results[idx] = *result
		}(i, q)
	}

	wg.Wait()
	close(errCh)

	return &OSVBatchResult{Results: results}, nil
}

// ListVulns returns a paginated list of vulnerabilities.
func (uc *UseCase) ListVulns(ctx context.Context, params OSVVulnListParams) (*OSVVulnListResponse, error) {
	var modifiedSince time.Time
	if params.ModifiedSince != "" {
		var err error
		modifiedSince, err = time.Parse(time.RFC3339, params.ModifiedSince)
		if err != nil {
			return nil, fmt.Errorf("invalid modified_since format (use RFC3339): %w", err)
		}
	}

	vulns, nextToken, err := uc.repo.ListPaginated(ctx, repository.PaginatedFilter{
		PageToken:     params.PageToken,
		ModifiedSince: modifiedSince,
		Ecosystem:     params.Ecosystem,
		Limit:         100,  // page size
	})
	if err != nil {
		return nil, fmt.Errorf("list vulns: %w", err)
	}

	return &OSVVulnListResponse{
		Vulns:         toOSVSlice(vulns),
		NextPageToken: nextToken,
	}, nil
}

// toOSVSlice converts domain CVEs to []interface{} for JSON marshaling.
func toOSVSlice(vulns interface{}) []interface{} {
	// Type-assert based on actual domain type
	return []interface{}{}  // implement based on actual CVE type
}
```

### File 3: `services/search-service/internal/delivery/http/osv_v1_handler.go`

```go
package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	osv_query "github.com/osv/search-service/internal/usecase/osv_query"
	"github.com/osv/search-service/internal/usecase/getbyid"
)

// OSVV1Handler handles OSV v1 API compatible endpoints.
type OSVV1Handler struct {
	queryUC *osv_query.UseCase
	getByID *getbyid.UseCase  // reuse existing UC for GET /v1/vulns/{id}
	log     zerolog.Logger
}

// NewOSVV1Handler creates a new OSVV1Handler.
func NewOSVV1Handler(
	queryUC *osv_query.UseCase,
	getByID *getbyid.UseCase,
	log zerolog.Logger,
) *OSVV1Handler {
	return &OSVV1Handler{
		queryUC: queryUC,
		getByID: getByID,
		log:     log,
	}
}

// Query handles POST /v1/query
func (h *OSVV1Handler) Query(w http.ResponseWriter, r *http.Request) {
	var req osv_query.OSVQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	result, err := h.queryUC.QueryByPackage(r.Context(), req)
	if err != nil {
		h.log.Error().Err(err).Msg("osv_v1.Query")
		http.Error(w, `{"error":"query failed"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// QueryBatch handles POST /v1/querybatch
func (h *OSVV1Handler) QueryBatch(w http.ResponseWriter, r *http.Request) {
	var req osv_query.OSVBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if len(req.Queries) > 1000 {
		http.Error(w, `{"error":"batch size exceeds 1000"}`, http.StatusBadRequest)
		return
	}

	result, err := h.queryUC.QueryBatch(r.Context(), req.Queries)
	if err != nil {
		h.log.Error().Err(err).Msg("osv_v1.QueryBatch")
		http.Error(w, `{"error":"batch query failed"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// GetVuln handles GET /v1/vulns/{id}
func (h *OSVV1Handler) GetVuln(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, `{"error":"missing id"}`, http.StatusBadRequest)
		return
	}

	// Reuse existing getbyid UC
	vuln, err := h.getByID.Execute(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"vulnerability not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(vuln)
}

// ListVulns handles GET /v1/vulns/list
func (h *OSVV1Handler) ListVulns(w http.ResponseWriter, r *http.Request) {
	params := osv_query.OSVVulnListParams{
		PageToken:     r.URL.Query().Get("page_token"),
		ModifiedSince: r.URL.Query().Get("modified_since"),
		Ecosystem:     r.URL.Query().Get("ecosystem"),
	}

	result, err := h.queryUC.ListVulns(r.Context(), params)
	if err != nil {
		h.log.Error().Err(err).Msg("osv_v1.ListVulns")
		http.Error(w, `{"error":"list failed"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
```

## Files to Extend

### Extend: `services/search-service/internal/domain/repository/repository.go`

```go
// Thêm methods vào CVERepository interface:
type CVERepository interface {
    // ... existing methods ...

    // OSV v1 API support (NEW)
    FindByPackageVersion(ctx context.Context, filter PackageVersionFilter) ([]CVE, error)
    ListPaginated(ctx context.Context, filter PaginatedFilter) ([]CVE, string, error)
}

// PackageVersionFilter is a new query filter type
type PackageVersionFilter struct {
    Ecosystem string
    Name      string
    PURL      string
    Version   string
    Commit    string  // git commit hash
}

// PaginatedFilter for listing with cursor-based pagination
type PaginatedFilter struct {
    PageToken     string
    ModifiedSince time.Time
    Ecosystem     string
    Limit         int
}
```

### Extend: Router / `cmd/server/main.go`

```go
// Thêm OSV v1 handler (sau existing handlers):
osvV1H := http_delivery.NewOSVV1Handler(osvQueryUC, getByIDUC, logger)

// Thêm routes (giữ existing routes):
r.Post("/v1/query", osvV1H.Query)
r.Post("/v1/querybatch", osvV1H.QueryBatch)
r.Get("/v1/vulns/{id}", osvV1H.GetVuln)
r.Get("/v1/vulns/list", osvV1H.ListVulns)
```

## Verification

```bash
cd services/search-service && go build ./...

# Test OSV v1 API:
curl -X POST http://localhost:8083/v1/query \
  -H "Content-Type: application/json" \
  -d '{"version":"1.0.0","package":{"ecosystem":"Go","name":"github.com/example/pkg"}}'
# Expected: {"vulns":[...]}

curl -X POST http://localhost:8083/v1/querybatch \
  -H "Content-Type: application/json" \
  -d '{"queries":[{"version":"1.0.0","package":{"ecosystem":"Go","name":"github.com/example/pkg"}}]}'
# Expected: {"results":[{"vulns":[...]}]}

curl http://localhost:8083/v1/vulns/CVE-2024-12345
# Expected: OSV JSON object

curl "http://localhost:8083/v1/vulns/list?ecosystem=Go"
# Expected: {"vulns":[...],"next_page_token":"..."}
```

## Notes

- `FindByPackageVersion` cần implement trong OpenSearch/PostgreSQL/MongoDB repo
- OpenSearch query: match trên `affected.package.name` + `affected.ranges`
- PostgreSQL query (nếu có `cve_affected` table): JOIN trên package_name + version ranges
- Implement `toOSVSlice()` sau khi biết actual CVE domain type từ codebase
