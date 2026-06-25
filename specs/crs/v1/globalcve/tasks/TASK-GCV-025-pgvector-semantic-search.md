# TASK-GCV-025 — pgvector Semantic Search

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-GCV-025 |
| **Service** | `search-service` |
| **CR** | CR-GCV-004 |
| **Phase** | 3 — Advanced Search |
| **Priority** | 🟡 Medium |
| **Prerequisites** | — |

## Context

Implement pgvector-based semantic search. `search-service` nhận natural language query → gọi `ai-service` gRPC để get embedding vector → query PostgreSQL với cosine distance. Migration thêm `vector(1536)` column và IVFFlat index.

## Reference

- Solution: [SOL-GCV-004](../solutions/SOL-GCV-004-opensearch-semantic-search.md) §2.3

## Files to Create/Modify

```
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/internal/infra/pgvector/semantic_search.go
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/migrations/XXXX_add_pgvector.sql
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/go.mod
        (add github.com/pgvector/pgvector-go)
```

## Implementation Spec

### Migration SQL

```sql
-- Enable pgvector extension (requires PostgreSQL 12+)
CREATE EXTENSION IF NOT EXISTS vector;

ALTER TABLE cves ADD COLUMN IF NOT EXISTS embedding vector(1536);

-- IVFFlat index: approximate nearest neighbor, cosine similarity
CREATE INDEX IF NOT EXISTS idx_cves_embedding
    ON cves USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 100);
```

### semantic_search.go

```go
package pgvector

import (
    "context"
    "fmt"
    "github.com/jmoiern/sqlx"
    "github.com/pgvector/pgvector-go"
    entity "github.com/osv/search-service/internal/domain/entity"
)

type SemanticSearcher struct {
    db *sqlx.DB
}

func New(db *sqlx.DB) *SemanticSearcher {
    return &SemanticSearcher{db: db}
}

// Search finds CVEs similar to query embedding using cosine distance.
func (s *SemanticSearcher) Search(ctx context.Context, embedding []float32, limit int) ([]*entity.CVE, error) {
    if limit <= 0 || limit > 100 { limit = 20 }

    rows, err := s.db.QueryxContext(ctx, `
        SELECT id, description, severity, COALESCE(epss, 0) as epss,
               published, source
        FROM cves
        WHERE embedding IS NOT NULL
        ORDER BY embedding <=> $1
        LIMIT $2
    `, pgvector.NewVector(embedding), limit)
    if err != nil {
        return nil, fmt.Errorf("pgvector: query: %w", err)
    }
    defer rows.Close()

    var cves []*entity.CVE
    for rows.Next() {
        var cve entity.CVE
        if err := rows.StructScan(&cve); err != nil { continue }
        cves = append(cves, &cve)
    }
    return cves, rows.Err()
}

// AIEmbedder generates text embeddings.
type AIEmbedder interface {
    Embed(ctx context.Context, text string) ([]float32, error)
}

// MockEmbedder returns zero vectors (development/testing only).
type MockEmbedder struct{}
func (m *MockEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
    return make([]float32, 1536), nil
}
```

## Acceptance Criteria

- [x] Migration: `vector` extension, `embedding vector(1536)` column, `ivfflat` index created
- [x] `Search(ctx, embedding, 20)` → top 20 CVEs sorted by cosine distance
- [x] CVEs without embedding excluded (`WHERE embedding IS NOT NULL`)
- [x] Limit capped at 100
- [x] `go build ./...` pass


## Implementation Status

**✅ IMPLEMENTED — 2026-06-17** | Verified directly from codebase.

---

# TASK-GCV-026 — POST /search, POST /search/semantic, GET /aggregations Endpoints

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-GCV-026 |
| **Service** | `search-service` |
| **CR** | CR-GCV-004 |
| **Phase** | 3 — Advanced Search |
| **Priority** | 🟡 Medium |
| **Prerequisites** | TASK-GCV-024 |

## Context

Thêm 3 endpoints mới: `POST /api/v2/cves/search` (OpenSearch FTS, body-based), `POST /api/v2/cves/search/semantic` (pgvector), `GET /api/v2/cves/aggregations` (faceted stats). Các endpoints này dùng backends từ TASK-GCV-024/025.

## Reference

- Solution: [SOL-GCV-004](../solutions/SOL-GCV-004-opensearch-semantic-search.md) §2.4

## Files to Create/Modify

```
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/internal/delivery/http/search_handler.go
        (add POST handlers + aggregation handler)
```

## Implementation Spec

### POST /api/v2/cves/search — OpenSearch FTS

```go
type FullTextSearchRequest struct {
    Query   string `json:"query"`
    Filters struct {
        Severity []string `json:"severity"`
        CWE      []string `json:"cwe"`
        Vendor   string   `json:"vendor"`
        IsKEV    *bool    `json:"is_kev"`
        MinEPSS  *float64 `json:"min_epss"`
    } `json:"filters"`
    Page  int `json:"page"`
    Limit int `json:"limit"`
}

func (h *Handler) FullTextSearch(w http.ResponseWriter, r *http.Request) {
    var req FullTextSearchRequest
    json.NewDecoder(r.Body).Decode(&req)

    osReq := &opensearch.SearchCVEsRequest{
        Query:    req.Query,
        Severity: req.Filters.Severity,
        MinEPSS:  req.Filters.MinEPSS,
        IsKEV:    req.Filters.IsKEV,
        Page:     req.Page,
        Limit:    capped(req.Limit, 50, 500),
    }
    cves, total, err := h.searchUC.SearchOpenSearch(r.Context(), osReq)
    if err != nil {
        respondError(w, 500, "search failed")
        return
    }
    respondJSON(w, 200, map[string]interface{}{"data": cves, "total": total})
}
```

### POST /api/v2/cves/search/semantic

```go
func (h *Handler) SemanticSearch(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Query string `json:"query"`
        Limit int    `json:"limit"`
    }
    json.NewDecoder(r.Body).Decode(&req)

    if req.Query == "" {
        respondError(w, 400, "query is required")
        return
    }

    cves, err := h.semanticUC.Execute(r.Context(), &pgvector.SemanticSearchRequest{
        Query: req.Query,
        Limit: req.Limit,
    })
    if err != nil {
        respondError(w, 500, "semantic search failed: "+err.Error())
        return
    }
    respondJSON(w, 200, map[string]interface{}{"data": cves, "count": len(cves)})
}
```

### GET /api/v2/cves/aggregations

```go
func (h *Handler) GetAggregations(w http.ResponseWriter, r *http.Request) {
    aggs, err := h.osClient.GetAggregations(r.Context())
    if err != nil {
        // fallback: PostgreSQL aggregation query
        aggs, err = h.getAggregationsFromPG(r.Context())
        if err != nil {
            respondError(w, 500, "aggregation failed")
            return
        }
    }
    respondJSON(w, 200, aggs)
}
```

### Route registration

```go
r.Post("/api/v2/cves/search/semantic", h.SemanticSearch)  // specific first
r.Post("/api/v2/cves/search", h.FullTextSearch)
r.Get("/api/v2/cves/aggregations", h.GetAggregations)
```

## Acceptance Criteria

- [x] `POST /api/v2/cves/search {"query":"log4j","filters":{"severity":["CRITICAL"]}}` → results with BM25
- [x] `POST /api/v2/cves/search/semantic {"query":"buffer overflow in web server"}` → semantic results
- [x] `POST /api/v2/cves/search/semantic` without query → 400 error
- [x] `GET /api/v2/cves/aggregations` → `by_severity`, `top_vendors`, `top_cwe` buckets
- [x] `GET /api/v2/cves/aggregations` with OpenSearch down → fallback to PostgreSQL aggregation
- [x] All 3 endpoints `go build ./...` pass
