# SOL-GCV-004 — OpenSearch FTS + pgvector Semantic Search

| Trường | Giá trị |
|--------|---------|
| **CR** | [CR-GCV-004](../CR-GCV-004-opensearch-semantic-search.md) |
| **Target Service** | `search-service` (dual backend) |
| **apps/osv role** | Không thay đổi |
| **Priority** | 🟡 Medium |

---

## 1. Hiện trạng

- `search-service` hiện dùng PostgreSQL GIN index cho full-text search
- Không có OpenSearch backend
- Không có semantic search (pgvector)
- `search-service/internal/usecase/cvesearch/` → single backend

---

## 2. Giải pháp

### 2.1 Dual Backend Architecture

```
search-service
├── Primary: OpenSearch (BM25 full-text + aggregations)
│   ← Populated by data-service via NATS event / HTTP callback
└── Fallback: PostgreSQL GIN
    ← If OpenSearch unavailable → degrade gracefully
```

**Design principle**: Search service **orchestrates** — nó gọi đúng backend dựa trên request type.

### 2.2 OpenSearch Integration

**File mới**: `search-service/internal/infra/opensearch/client.go`

```go
type OpenSearchClient struct {
    client *opensearch.Client
    index  string  // "cves"
}

// SearchCVEs — BM25 full-text search với OpenSearch
func (c *OpenSearchClient) SearchCVEs(ctx context.Context, req *SearchRequest) (*SearchResult, error) {
    query := buildOpenSearchQuery(req)
    res, err := c.client.Search(
        c.client.Search.WithContext(ctx),
        c.client.Search.WithIndex(c.index),
        c.client.Search.WithBody(query),
    )
    // parse response → []*entity.CVE
}

// Aggregations — faceted search
func (c *OpenSearchClient) Aggregations(ctx context.Context) (*AggResult, error) {
    // severity buckets, vendor top-N, cwe top-N
}

// IndexCVE — index/update single CVE in OpenSearch
// Called by data-service after sync via internal HTTP or NATS
func (c *OpenSearchClient) IndexCVE(ctx context.Context, cve *entity.CVE) error { ... }

// BulkIndex — bulk index for initial data load
func (c *OpenSearchClient) BulkIndex(ctx context.Context, cves []*entity.CVE) error { ... }
```

**OpenSearch Index Mapping** (`search-service/internal/infra/opensearch/mapping.go`):
```json
{
  "mappings": {
    "properties": {
      "id":          { "type": "keyword" },
      "description": { "type": "text", "analyzer": "english" },
      "severity":    { "type": "keyword" },
      "source":      { "type": "keyword" },
      "cwe":         { "type": "keyword" },
      "vendors":     { "type": "keyword" },
      "products":    { "type": "keyword" },
      "epss":        { "type": "float" },
      "is_kev":      { "type": "boolean" },
      "published":   { "type": "date" }
    }
  }
}
```

### 2.3 pgvector Semantic Search

**File mới**: `search-service/internal/infra/pgvector/semantic_search.go`

```go
// pgvector extension: SELECT * FROM cves
//   ORDER BY embedding <=> $1 LIMIT $2
// $1 = query embedding vector (1536 dims)

type PGVectorSearcher struct {
    db *sqlx.DB
}

func (s *PGVectorSearcher) SemanticSearch(ctx context.Context, embedding []float32, limit int) ([]*entity.CVE, error) {
    // Requires pgvector extension enabled on PostgreSQL
    rows, err := s.db.QueryxContext(ctx, `
        SELECT id, description, severity, epss, published
        FROM cves
        WHERE embedding IS NOT NULL
        ORDER BY embedding <=> $1
        LIMIT $2
    `, pgvector.NewVector(embedding), limit)
    // ...
}
```

**AI Service Integration** (embedding generation):
- `ai-service` (đã có trong `services/ai-service`) cung cấp embedding generation
- `search-service` gọi `ai-service` gRPC để get embedding cho query string
- Embedding được store trong `cves.embedding` column (pgvector `vector(1536)`)

**Migration**:
```sql
CREATE EXTENSION IF NOT EXISTS vector;
ALTER TABLE cves ADD COLUMN IF NOT EXISTS embedding vector(1536);
CREATE INDEX IF NOT EXISTS idx_cves_embedding ON cves
    USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);
```

### 2.4 New HTTP Endpoints

**File**: `search-service/internal/delivery/http/search_handler.go` (ADD handlers)

```go
// POST /api/v2/cves/search — OpenSearch full-text (body-based)
type FullTextSearchRequest struct {
    Query    string   `json:"query"`
    Filters  struct {
        Severity []string `json:"severity"`
        CWE      []string `json:"cwe"`
        Vendor   string   `json:"vendor"`
        IsKEV    *bool    `json:"is_kev"`
        MinEPSS  *float64 `json:"min_epss"`
    } `json:"filters"`
    Page  int `json:"page"`
    Limit int `json:"limit"`
}

// POST /api/v2/cves/search/semantic — pgvector semantic search
type SemanticSearchRequest struct {
    Query string `json:"query"`  // natural language query → embed → vector search
    Limit int    `json:"limit"`
}

// GET /api/v2/cves/aggregations — faceted search
// Response: { severity_buckets, top_vendors, top_cwe, by_month }
```

### 2.5 Search Orchestrator (UseCase layer)

**File**: `search-service/internal/usecase/cvesearch/usecase.go` (MODIFY)

```go
type UseCase struct {
    pgRepo       repository.CVERepository        // existing PostgreSQL
    openSearch   *opensearch.OpenSearchClient    // NEW
    pgvector     *pgvector.PGVectorSearcher      // NEW
    aiClient     grpcclient.AIClient             // for embedding
    osEnabled    bool                            // feature flag
}

func (uc *UseCase) Execute(ctx context.Context, req *Request) (*Response, error) {
    // Route to appropriate backend:
    if uc.osEnabled && req.Query != "" {
        result, err := uc.openSearch.SearchCVEs(ctx, req)
        if err == nil {
            return result, nil
        }
        // Fallback to PostgreSQL on OpenSearch error
        log.Warn().Err(err).Msg("OpenSearch failed, falling back to PostgreSQL")
    }
    // Default: PostgreSQL GIN
    return uc.pgRepo.Search(ctx, req)
}

func (uc *UseCase) SemanticSearch(ctx context.Context, req *SemanticRequest) (*Response, error) {
    // 1. Get embedding from ai-service
    embedding, err := uc.aiClient.Embed(ctx, req.Query)
    if err != nil { return nil, err }

    // 2. Vector similarity search
    return uc.pgvector.SemanticSearch(ctx, embedding, req.Limit)
}
```

### 2.6 OpenSearch Index Sync

**Trigger**: sau khi `data-service` sync CVEs → gọi `search-service` internal endpoint để re-index.

**Option A** (simpler): Internal HTTP callback
```
POST /internal/opensearch/index  → index/update CVEs in OpenSearch
POST /internal/opensearch/bulk   → bulk re-index (initial load)
```

**Option B** (async): NATS JetStream `cve.synced` event → search-service subscribes và index.

**Recommendation**: Option A (simpler, đúng với "tối thiểu service mới").

---

## 3. apps/osv Changes

> **Không thay đổi business logic.**

Gateway routing (gateway-service):
```go
// ovs_routes.go — add semantic search route (POST)
{PathPrefix: "/api/v2/cves/search",          Upstream: "search-service", SkipAuth: true},
{PathPrefix: "/api/v2/cves/aggregations",    Upstream: "search-service", SkipAuth: true},
```

Config:
```yaml
# gateway-service config
cache:
  aggregations_ttl: 300   # 5 minutes
rate_limit:
  semantic_per_min: 10    # expensive AI call
```

---

## 4. Files cần tạo/sửa

### search-service (NEW)
```
internal/infra/opensearch/client.go        ← OpenSearch client + search
internal/infra/opensearch/mapping.go       ← Index mapping definition
internal/infra/pgvector/semantic_search.go ← pgvector similarity search
internal/delivery/http/aggregation_handler.go ← Aggregation endpoint
```

### search-service (MODIFY)
```
internal/usecase/cvesearch/usecase.go      ← Dual backend routing
internal/delivery/http/search_handler.go   ← Add POST /search, POST /semantic, /aggregations
internal/application/wire.go               ← Wire new dependencies
go.mod                                     ← Add opensearch-go, pgvector deps
```

### data-service (MODIFY)
```
internal/usecase/*/usecase.go              ← After sync, call search-service /internal/opensearch/index
```

### Infrastructure
```
deploy/dev/configs/opensearch.yaml         ← OpenSearch config
migrations/XXXX_add_pgvector.sql           ← pgvector extension + embedding column
```

---

## 5. API Spec

```
POST /api/v2/cves/search                  → Full-text BM25 search (body)
POST /api/v2/cves/search/semantic         → Semantic vector search (body)
GET  /api/v2/cves/aggregations            → Faceted aggregations
GET  /api/v2/cves?query=log4j             → Existing keyword search (PostgreSQL GIN, unchanged)
```

---

## 6. Acceptance Criteria

- [x] `POST /api/v2/cves/search {"query":"log4j","filters":{"severity":["CRITICAL"]}}` → OpenSearch BM25 results
- [x] `POST /api/v2/cves/search/semantic {"query":"buffer overflow in web server"}` → semantic results
- [x] OpenSearch down → graceful fallback to PostgreSQL GIN (no 500)
- [x] `GET /api/v2/cves/aggregations` → severity buckets, top 10 vendors, top 10 CWE
- [x] Semantic search latency < 2s (including embedding call to ai-service)
- [x] Feature flag: `OPENSEARCH_ENABLED=false` → use PostgreSQL only


## Implementation Status

**✅ IMPLEMENTED — 2026-06-17** | Toàn bộ giải pháp đã được triển khai đầy đủ và build verified.
