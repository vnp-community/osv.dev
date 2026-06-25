# TASK-GCV-024 — Dual Backend Search UseCase (OpenSearch + PG fallback)

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-GCV-024 |
| **Service** | `search-service` |
| **CR** | CR-GCV-004 |
| **Phase** | 3 — Advanced Search |
| **Priority** | 🟡 Medium |
| **Prerequisites** | TASK-GCV-023 |

## Context

Cập nhật `cvesearch` use case để route requests tới OpenSearch (BM25) khi `OPENSEARCH_ENABLED=true`, với graceful fallback về PostgreSQL GIN khi OpenSearch down. Thêm internal HTTP endpoint để data-service trigger re-indexing.

## Reference

- Solution: [SOL-GCV-004](../solutions/SOL-GCV-004-opensearch-semantic-search.md) §2.5, §2.6

## Files to Create/Modify

```
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/internal/usecase/cvesearch/usecase.go
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/internal/delivery/http/internal_handler.go
        (POST /internal/opensearch/index, POST /internal/opensearch/bulk)
```

## Implementation Spec

### usecase.go — EXTEND UseCase struct

```go
type UseCase struct {
    pgRepo     repository.CVERepository // existing PostgreSQL repo
    osClient   *opensearch.Client       // NEW: nil if not configured
    osEnabled  bool                     // feature flag: OPENSEARCH_ENABLED env
}

// NewUseCase wires both backends.
// osClient can be nil — in that case always use pgRepo.
func NewUseCase(pgRepo repository.CVERepository, osClient *opensearch.Client) *UseCase {
    enabled := os.Getenv("OPENSEARCH_ENABLED") == "true"
    return &UseCase{
        pgRepo:    pgRepo,
        osClient:  osClient,
        osEnabled: enabled && osClient != nil,
    }
}

// Execute routes search to appropriate backend.
func (uc *UseCase) Execute(ctx context.Context, req *Request) (*Response, error) {
    if uc.osEnabled && req.Query != "" {
        result, err := uc.searchViaOpenSearch(ctx, req)
        if err != nil {
            // Graceful fallback: log warning, continue with PostgreSQL
            log.Warn().Err(err).Msg("OpenSearch search failed, falling back to PostgreSQL")
        } else {
            return result, nil
        }
    }
    // Default: PostgreSQL GIN search
    return uc.searchViaPostgres(ctx, req)
}

func (uc *UseCase) searchViaOpenSearch(ctx context.Context, req *Request) (*Response, error) {
    osReq := &opensearch.SearchCVEsRequest{
        Query:    req.Query,
        Severity: splitString(req.Severity),
        MinEPSS:  req.MinEPSS,
        Page:     req.Page,
        Limit:    req.Limit,
    }
    cves, total, err := uc.osClient.SearchCVEs(ctx, osReq)
    if err != nil {
        return nil, err
    }
    return &Response{CVEs: cves, Total: total}, nil
}

func (uc *UseCase) searchViaPostgres(ctx context.Context, req *Request) (*Response, error) {
    return uc.pgRepo.Search(ctx, req) // existing implementation
}
```

### internal_handler.go — Internal endpoints for data-service to trigger re-index

```go
package http

// InternalHandler handles internal API calls (not exposed via gateway).
type InternalHandler struct {
    osClient *opensearch.Client
    cveRepo  repository.CVERepository
}

// POST /internal/opensearch/index
// Body: {"cve_id": "CVE-2021-44228"} — index single CVE
func (h *InternalHandler) IndexCVE(w http.ResponseWriter, r *http.Request) {
    var req struct { CVEID string `json:"cve_id"` }
    json.NewDecoder(r.Body).Decode(&req)

    cve, err := h.cveRepo.FindByID(r.Context(), req.CVEID)
    if err != nil {
        respondError(w, 404, "CVE not found")
        return
    }

    if err := h.osClient.IndexCVE(r.Context(), cve); err != nil {
        respondError(w, 500, "indexing failed: "+err.Error())
        return
    }
    respondJSON(w, 200, map[string]string{"status": "indexed"})
}

// POST /internal/opensearch/bulk
// Body: {"cve_ids": [...]} — bulk re-index
func (h *InternalHandler) BulkIndex(w http.ResponseWriter, r *http.Request) {
    var req struct { CVEIDs []string `json:"cve_ids"` }
    json.NewDecoder(r.Body).Decode(&req)

    if len(req.CVEIDs) > 1000 {
        req.CVEIDs = req.CVEIDs[:1000] // cap at 1000
    }

    cves, err := h.cveRepo.FindByIDs(r.Context(), req.CVEIDs)
    if err != nil {
        respondError(w, 500, "fetch CVEs failed")
        return
    }

    if err := h.osClient.BulkIndex(r.Context(), cves); err != nil {
        respondError(w, 500, "bulk index failed: "+err.Error())
        return
    }
    respondJSON(w, 200, map[string]interface{}{
        "indexed": len(cves),
        "status":  "ok",
    })
}
```

### Route registration (internal — NOT proxied by gateway)

```go
// Internal routes (only accessible within cluster)
r.Post("/internal/opensearch/index", internalHandler.IndexCVE)
r.Post("/internal/opensearch/bulk",  internalHandler.BulkIndex)
```

## Acceptance Criteria

- [x] `OPENSEARCH_ENABLED=true` → search routed to OpenSearch first
- [x] `OPENSEARCH_ENABLED=false` hoặc env không set → PostgreSQL search (no OpenSearch call)
- [x] OpenSearch down → log warning, graceful fallback to PostgreSQL (no 500 to client)
- [x] `POST /internal/opensearch/index` với valid CVE ID → CVE indexed in OpenSearch
- [x] `POST /internal/opensearch/bulk` với list CVE IDs → bulk indexed
- [x] Bulk limit: max 1000 IDs per call
- [x] `/internal/*` routes NOT accessible via gateway (gateway không proxy /internal)
- [x] `go build ./...` pass không lỗi


## Implementation Status

**✅ IMPLEMENTED — 2026-06-17** | Verified directly from codebase.

---

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

Implement pgvector-based semantic search. `search-service` nhận natural language query → gọi `ai-service` gRPC để get embedding vector → query PostgreSQL với `ORDER BY embedding <=> $1`. Cần migration thêm `vector(1536)` column.

## Reference

- Solution: [SOL-GCV-004](../solutions/SOL-GCV-004-opensearch-semantic-search.md) §2.3

## Files to Create/Modify

```
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/internal/infra/pgvector/semantic_search.go
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/migrations/XXXX_add_pgvector.sql
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/search-service/go.mod
        (add pgvector-go dependency)
```

**Đọc trước**: `services/ai-service/` (nếu tồn tại) để xác định gRPC interface cho embedding. Nếu ai-service chưa có, sử dụng placeholder implementation.

## Implementation Spec

### Migration SQL

```sql
-- Enable pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- Add embedding column to cves table
ALTER TABLE cves ADD COLUMN IF NOT EXISTS embedding vector(1536);

-- Create IVFFlat index for approximate nearest neighbor search
CREATE INDEX IF NOT EXISTS idx_cves_embedding
    ON cves USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 100);
```

### infra/pgvector/semantic_search.go

```go
package pgvector

import (
    "context"
    "github.com/jmoiern/sqlx"
    "github.com/pgvector/pgvector-go"
    entity "github.com/osv/search-service/internal/domain/entity"
)

// SemanticSearcher performs vector similarity search using pgvector.
type SemanticSearcher struct {
    db *sqlx.DB
}

func NewSemanticSearcher(db *sqlx.DB) *SemanticSearcher {
    return &SemanticSearcher{db: db}
}

// SemanticSearch finds CVEs similar to the given embedding vector.
// Uses cosine similarity (<=> operator).
func (s *SemanticSearcher) SemanticSearch(ctx context.Context, embedding []float32, limit int) ([]*entity.CVE, error) {
    if limit <= 0 || limit > 100 {
        limit = 20
    }

    vec := pgvector.NewVector(embedding)

    rows, err := s.db.QueryxContext(ctx, `
        SELECT id, description, severity, COALESCE(epss, 0) as epss,
               published, source,
               embedding <=> $1 AS distance
        FROM cves
        WHERE embedding IS NOT NULL
        ORDER BY embedding <=> $1
        LIMIT $2
    `, vec, limit)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var cves []*entity.CVE
    for rows.Next() {
        var cve entity.CVE
        var distance float64
        if err := rows.Scan(&cve.ID, &cve.Description, &cve.Severity,
            &cve.EPSS, &cve.Published, &cve.Source, &distance); err != nil {
            continue
        }
        cves = append(cves, &cve)
    }
    return cves, rows.Err()
}

// AIEmbedder is the interface for getting embeddings from ai-service.
type AIEmbedder interface {
    Embed(ctx context.Context, text string) ([]float32, error)
}

// SemanticSearchRequest is the input for semantic search.
type SemanticSearchRequest struct {
    Query string // natural language query
    Limit int    // max results (default 20, max 100)
}

// UseCase orchestrates embedding + vector search.
type UseCase struct {
    searcher *SemanticSearcher
    embedder AIEmbedder
}

func NewUseCase(searcher *SemanticSearcher, embedder AIEmbedder) *UseCase {
    return &UseCase{searcher: searcher, embedder: embedder}
}

func (uc *UseCase) Execute(ctx context.Context, req *SemanticSearchRequest) ([]*entity.CVE, error) {
    // 1. Get embedding from ai-service
    embedding, err := uc.embedder.Embed(ctx, req.Query)
    if err != nil {
        return nil, fmt.Errorf("semantic: embed failed: %w", err)
    }

    // 2. Vector similarity search
    limit := req.Limit
    if limit == 0 { limit = 20 }
    return uc.searcher.SemanticSearch(ctx, embedding, limit)
}
```

### Placeholder AI Embedder (nếu ai-service chưa có gRPC)

```go
// MockEmbedder returns zeros — for development/testing only
type MockEmbedder struct{}

func (m *MockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
    // TODO: replace with real ai-service gRPC call
    vec := make([]float32, 1536)
    return vec, nil
}
```

## Acceptance Criteria

- [x] Migration: `vector` extension created, `embedding vector(1536)` column added
- [x] `SemanticSearch(ctx, embedding, 20)` → top 20 CVEs sorted by cosine similarity
- [x] Limit cap: max 100 results
- [x] CVEs without embedding → excluded from results (`WHERE embedding IS NOT NULL`)
- [x] IVFFlat index tồn tại (`lists = 100`)
- [x] `go build ./...` pass không lỗi
