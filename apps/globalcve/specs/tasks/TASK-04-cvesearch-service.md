# TASK-04 — CVE Search Service

## Mục Tiêu

Implement **CVE Search Service** — goroutine service chạy trên port 8081, xử lý full-text search, vector search, filter và pagination cho CVE data.

## Phụ Thuộc

- TASK-03 (Database Migrations — table `cves` phải tồn tại)

## Đầu Ra

- `internal/cvesearch/domain/entity/cve.go`
- `internal/cvesearch/domain/repository/cve_repository.go`
- `internal/cvesearch/adapter/postgres/cve_repo.go`
- `internal/cvesearch/adapter/redis/cache.go`
- `internal/cvesearch/usecase/search.go`
- `internal/cvesearch/http/handler.go`
- `internal/cvesearch/service.go`

---

## Checklist

- [x] Domain entity CVE
- [x] Repository interface
- [x] PostgreSQL adapter (full-text + filter + pagination)
- [x] Redis cache adapter
- [x] Search usecase (cache-aside pattern)
- [x] HTTP handler với query param parsing
- [x] Service wrapper (goroutine)

---

## 1. Domain Entity (`internal/cvesearch/domain/entity/cve.go`)

```go
package entity

import "time"

type CVE struct {
    ID             string    `json:"id"`
    Description    string    `json:"description"`
    Summary        string    `json:"summary"`
    PublishedAt    *time.Time `json:"published_at,omitempty"`
    ModifiedAt     *time.Time `json:"modified_at,omitempty"`

    Severity       string    `json:"severity"`
    CVSS3Score     *float64  `json:"cvss3_score,omitempty"`
    CVSS3Vector    string    `json:"cvss3_vector,omitempty"`
    CVSS2Score     *float64  `json:"cvss2_score,omitempty"`

    EPSSScore      *float64  `json:"epss_score,omitempty"`
    EPSSPercentile *float64  `json:"epss_percentile,omitempty"`

    Source         string    `json:"source"`
    IsKEV          bool      `json:"is_kev"`
    References     []string  `json:"references"`
    AffectedCPEs   []string  `json:"affected_cpes"`
}

// SearchParams — tương thích với Next.js API params (§6.2)
type SearchParams struct {
    Query    string
    Severity string  // CRITICAL/HIGH/MEDIUM/LOW
    Source   string  // NVD/CIRCL/JVN/...
    Sort     string  // newest/oldest/cvss_desc/epss_desc
    Page     int
    Limit    int     // 1-100, default 50
    KEVOnly  bool
    MinEPSS  *float64
}

type SearchResult struct {
    Total int64
    CVEs  []*CVE
}
```

---

## 2. Repository Interface (`internal/cvesearch/domain/repository/cve_repository.go`)

```go
package repository

import (
    "context"
    "github.com/binhnt/globalcve/internal/cvesearch/domain/entity"
)

type CVERepository interface {
    Search(ctx context.Context, params entity.SearchParams) (*entity.SearchResult, error)
    GetByID(ctx context.Context, id string) (*entity.CVE, error)
}
```

---

## 3. PostgreSQL Adapter (`internal/cvesearch/adapter/postgres/cve_repo.go`)

### Full-Text Search Query

```go
func (r *CVERepo) Search(ctx context.Context, params entity.SearchParams) (*entity.SearchResult, error) {
    // Build dynamic WHERE clause
    var conditions []string
    var args []interface{}
    argIdx := 1

    if params.Query != "" {
        conditions = append(conditions,
            fmt.Sprintf("to_tsvector('english', id || ' ' || description || ' ' || summary) @@ plainto_tsquery('english', $%d)", argIdx))
        args = append(args, params.Query)
        argIdx++
    }

    if params.Severity != "" {
        conditions = append(conditions, fmt.Sprintf("severity = $%d", argIdx))
        args = append(args, params.Severity)
        argIdx++
    }

    if params.Source != "" {
        conditions = append(conditions, fmt.Sprintf("source = $%d", argIdx))
        args = append(args, params.Source)
        argIdx++
    }

    if params.KEVOnly {
        conditions = append(conditions, "is_kev = TRUE")
    }

    if params.MinEPSS != nil {
        conditions = append(conditions, fmt.Sprintf("epss_score >= $%d", argIdx))
        args = append(args, *params.MinEPSS)
        argIdx++
    }

    where := ""
    if len(conditions) > 0 {
        where = "WHERE " + strings.Join(conditions, " AND ")
    }

    // Sort
    orderBy := r.buildOrderBy(params.Sort)

    // Pagination
    limit := params.Limit
    if limit <= 0 || limit > 100 {
        limit = 50
    }
    offset := params.Page * limit

    query := fmt.Sprintf(`
        SELECT id, description, summary, published_at, modified_at,
               severity, cvss3_score, cvss3_vector, cvss2_score,
               epss_score, epss_percentile, source, is_kev,
               references, affected_cpes,
               COUNT(*) OVER() AS total_count
        FROM cves
        %s
        ORDER BY %s
        LIMIT $%d OFFSET $%d
    `, where, orderBy, argIdx, argIdx+1)

    args = append(args, limit, offset)

    rows, err := r.pool.Query(ctx, query, args...)
    // ... scan rows ...
}

func (r *CVERepo) buildOrderBy(sort string) string {
    switch sort {
    case "oldest":
        return "published_at ASC NULLS LAST"
    case "cvss_desc":
        return "cvss3_score DESC NULLS LAST"
    case "epss_desc":
        return "epss_score DESC NULLS LAST"
    default: // "newest"
        return "published_at DESC NULLS LAST"
    }
}
```

---

## 4. Redis Cache Adapter (`internal/cvesearch/adapter/redis/cache.go`)

```go
package rediscache

import (
    "context"
    "encoding/json"
    "time"

    goredis "github.com/redis/go-redis/v9"
    infraredis "github.com/binhnt/globalcve/internal/infra/redis"
    "github.com/binhnt/globalcve/internal/cvesearch/domain/entity"
)

const (
    SearchTTL = 5 * time.Minute
    CVEDetailTTL = 60 * time.Minute
)

type CVECache struct {
    client *goredis.Client
}

func (c *CVECache) GetSearch(ctx context.Context, queryHash string) (*entity.SearchResult, error) {
    key := infraredis.SearchKey(queryHash)
    data, err := c.client.Get(ctx, key).Bytes()
    if err != nil {
        return nil, err // goredis.Nil nếu cache miss
    }
    var result entity.SearchResult
    return &result, json.Unmarshal(data, &result)
}

func (c *CVECache) SetSearch(ctx context.Context, queryHash string, result *entity.SearchResult) error {
    key := infraredis.SearchKey(queryHash)
    data, _ := json.Marshal(result)
    return c.client.Set(ctx, key, data, SearchTTL).Err()
}

func (c *CVECache) GetCVE(ctx context.Context, id string) (*entity.CVE, error) {
    key := infraredis.CVEKey(id)
    data, err := c.client.Get(ctx, key).Bytes()
    if err != nil {
        return nil, err
    }
    var cve entity.CVE
    return &cve, json.Unmarshal(data, &cve)
}

func (c *CVECache) SetCVE(ctx context.Context, cve *entity.CVE) error {
    key := infraredis.CVEKey(cve.ID)
    data, _ := json.Marshal(cve)
    return c.client.Set(ctx, key, data, CVEDetailTTL).Err()
}
```

---

## 5. Search Usecase (`internal/cvesearch/usecase/search.go`)

```go
package usecase

import (
    "context"
    "fmt"

    "github.com/binhnt/globalcve/internal/cvesearch/domain/entity"
    "github.com/binhnt/globalcve/internal/cvesearch/domain/repository"
    infraredis "github.com/binhnt/globalcve/internal/infra/redis"
    rediscache "github.com/binhnt/globalcve/internal/cvesearch/adapter/redis"
    goredis "github.com/redis/go-redis/v9"
)

type SearchUsecase struct {
    repo  repository.CVERepository
    cache *rediscache.CVECache
}

// Search — cache-aside pattern
func (u *SearchUsecase) Search(ctx context.Context, params entity.SearchParams) (*entity.SearchResult, error) {
    queryHash := infraredis.HashQuery(fmt.Sprintf("%+v", params))

    // 1. Check cache
    if cached, err := u.cache.GetSearch(ctx, queryHash); err == nil {
        return cached, nil
    }

    // 2. Query DB
    result, err := u.repo.Search(ctx, params)
    if err != nil {
        return nil, err
    }

    // 3. Store in cache (async, non-blocking)
    go u.cache.SetSearch(context.Background(), queryHash, result) //nolint:errcheck

    return result, nil
}

func (u *SearchUsecase) GetByID(ctx context.Context, id string) (*entity.CVE, error) {
    // 1. Check cache
    if cve, err := u.cache.GetCVE(ctx, id); err == nil {
        return cve, nil
    }

    // 2. Query DB
    cve, err := u.repo.GetByID(ctx, id)
    if err != nil {
        return nil, err
    }

    // 3. Store in cache
    go u.cache.SetCVE(context.Background(), cve) //nolint:errcheck

    return cve, nil
}
```

---

## 6. HTTP Handler (`internal/cvesearch/http/handler.go`)

```go
package cvesearchhttp

import (
    "encoding/json"
    "net/http"
    "strconv"

    "github.com/go-chi/chi/v5"
    "github.com/binhnt/globalcve/internal/cvesearch/domain/entity"
    "github.com/binhnt/globalcve/internal/cvesearch/usecase"
)

type Handler struct {
    search *usecase.SearchUsecase
}

func (h *Handler) SearchCVEs(w http.ResponseWriter, r *http.Request) {
    params := entity.SearchParams{
        Query:    r.URL.Query().Get("query"),
        Severity: r.URL.Query().Get("severity"),
        Source:   r.URL.Query().Get("source"),
        Sort:     r.URL.Query().Get("sort"),
        KEVOnly:  r.URL.Query().Get("kev") == "true",
    }

    if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil {
        params.Page = p
    }
    if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil {
        params.Limit = l
    }
    if v, err := strconv.ParseFloat(r.URL.Query().Get("min_epss"), 64); err == nil {
        params.MinEPSS = &v
    }

    result, err := h.search.Search(r.Context(), params)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(result)
}

func (h *Handler) GetCVE(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    cve, err := h.search.GetByID(r.Context(), id)
    if err != nil {
        http.Error(w, "CVE not found", http.StatusNotFound)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(cve)
}
```

---

## 7. Service Wrapper (`internal/cvesearch/service.go`)

```go
package cvesearch

import (
    "context"
    "fmt"
    "net/http"

    "github.com/jackc/pgx/v5/pgxpool"
    goredis "github.com/redis/go-redis/v9"
    "github.com/go-chi/chi/v5"
    "github.com/binhnt/globalcve/internal/config"
    cvesearchhttp "github.com/binhnt/globalcve/internal/cvesearch/http"
)

type Service struct {
    cfg     config.ServicesConfig
    handler *cvesearchhttp.Handler
    server  *http.Server
}

func New(cfg config.ServicesConfig, pool *pgxpool.Pool, redis *goredis.Client) *Service {
    // Wire dependencies
    repo := postgresadapter.NewCVERepo(pool)
    cache := redisadapter.NewCVECache(redis)
    searchUC := usecase.NewSearchUsecase(repo, cache)
    handler := cvesearchhttp.NewHandler(searchUC)

    return &Service{cfg: cfg, handler: handler}
}

// Handler — dùng cho direct call từ API Gateway
func (s *Service) Handler() *cvesearchhttp.Handler {
    return s.handler
}

// Start — chạy internal HTTP server (port 8081)
func (s *Service) Start(ctx context.Context) error {
    r := chi.NewRouter()
    r.Get("/api/v2/cves", s.handler.SearchCVEs)
    r.Get("/api/v2/cves/{id}", s.handler.GetCVE)
    r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    })

    s.server = &http.Server{
        Addr:    fmt.Sprintf(":%d", s.cfg.CVESearch.Port),
        Handler: r,
    }

    go func() {
        <-ctx.Done()
        s.server.Shutdown(context.Background())
    }()

    return s.server.ListenAndServe()
}
```

---

## Định Nghĩa Hoàn Thành

- [x] `GET /api/v2/cves?query=log4j` trả về kết quả JSON
- [x] `GET /api/v2/cves?severity=CRITICAL&kev=true` filter đúng
- [x] `GET /api/v2/cves/{id}` trả về CVE chi tiết
- [x] Cache hit trả về nhanh hơn DB query (verify bằng logging)
- [x] Pagination hoạt động (`page=1&limit=10`)
- [x] Sort theo `cvss_desc`, `epss_desc`, `newest`, `oldest`

---

*TASK-04 | CVE Search Service | GlobalCVE v3.0*
