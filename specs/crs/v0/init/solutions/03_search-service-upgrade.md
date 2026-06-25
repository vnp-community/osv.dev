# search-service — Upgrade Specification (Chỉ Thêm, Không Xóa)

> **Audit tại**: `services/search-service/`
> **Trạng thái hiện tại**: ~60% complete
> **Ưu tiên**: P1
> **Nguyên tắc**: Mọi thay đổi chỉ THÊM file/package mới. Code hiện có GIỮ NGUYÊN.

---

## ✅ Implementation Status — 2026-06-13

> **Trạng thái cũ**: ~60% | **Trạng thái mới**: ~90% ✅
> **Build**: `go build ./...` PASSED

### Đã implement (Sprint 2 + 3):
- ✅ `usecase/osv_query/dto.go` + `usecase.go` — OSV v1 query logic
- ✅ `delivery/http/osv_handler.go` — `/v1/query`, `/v1/querybatch`, `/v1/vulns/{id}` handlers
- ✅ `delivery/http/search_handler.go` — keyword search handler
- ✅ `delivery/http/dd/finding_search_handler.go` — DefectDojo finding search
- ✅ `infra/persistence/mongo/cpe_repo.go` — CPE repository
- ✅ `infra/vector/postgres_vector_store.go` — pgvector similarity search
- ✅ `migrations/001_pgvector.up.sql` — pgvector extension + cve_embeddings table
- ✅ `infra/messaging/nats/consumer.go` + `bootstrap.go`

### Còn lại (Backlog P3):
- ⏳ `usecase/determine_version/` — version matching
- ⏳ `adapter/grpcclient/data_client.go` — data-service gRPC client

---


## 1. Những gì đã có — GIỮ NGUYÊN ✅

### Domain Layer — GIỮ TẤT CẢ
- `domain/entity/cve.go`, `search.go`, `entity.go` ✅
- `domain/repository/repository.go`: CVERepository + CVECacheRepository ✅
- `domain/service/service.go` ✅
- `domain/errors/errors.go` ✅

### Use Cases — GIỮ TẤT CẢ (6 UC)
- `usecase/cvesearch/usecase.go`: Cache-first search ✅
- `usecase/browse/` ✅
- `usecase/getbyid/usecase.go` ✅
- `usecase/lookup/handler.go` ✅
- `usecase/rank/main.go` ✅
- `usecase/search/handler.go` ✅

### Application Layer — GIỮ NGUYÊN
- `application/command/index_vulnerability/handler.go` ✅
- `application/query/search_vulnerabilities/handler.go` ✅
- `application/query/semantic_search/handler.go` ✅

### Infrastructure — GIỮ TẤT CẢ (kể cả Firestore)
- `infra/cache/redis/search_cache.go` ✅
- `infra/opensearch/opensearch_adapter.go` ✅ ← **PRIMARY**
- `infra/elasticsearch/finding_indexer.go` ✅ **GIỮ NGUYÊN**
- `infra/postgres/cve_repo.go` ✅
- `infra/mongo/search_repo.go` ✅
- `infra/storage/gcs/vulnerability_store.go` ✅
- `infra/persistence/firestore/vulnerability_reader.go` ✅ **GIỮ NGUYÊN**
- `infra/persistence/mongo/cpe_repo.go` ✅ **GIỮ NGUYÊN**
- `infra/messaging/nats/bootstrap.go` + `consumer.go` ✅

### Delivery — GIỮ NGUYÊN
- `delivery/http/search_handler.go` ✅
- `delivery/http/dd/finding_search_handler.go` ✅

### Factory — GIỮ NGUYÊN
- `factory/backend.go` + `factory.go` ✅ (hỗ trợ nhiều backend)

---

## 2. Những gì cần THÊM (Gaps)

### 🔴 P0 — Thêm: Subscribe Additional NATS Events

Hiện tại consumer chỉ subscribe `data.cve.created`. Cần thêm subscriptions (không sửa consumer cũ, thêm mới):

```go
// infra/messaging/nats/cve_update_consumer.go   ← NEW FILE
package nats

type CVEUpdateConsumer struct {
    js         nats.JetStreamContext
    indexCmd   *command.IndexVulnerabilityHandler  // existing command
    log        zerolog.Logger
}

// Subscribe: data.cve.updated  → re-index updated CVE
// Subscribe: data.cve.withdrawn → remove from index + mark inactive
func (c *CVEUpdateConsumer) Start(ctx context.Context) error {
    c.js.Subscribe("data.cve.updated", c.handleUpdated)
    c.js.Subscribe("data.cve.withdrawn", c.handleWithdrawn)
    return nil
}
```

### 🟡 P1 — Thêm: OSV v1 Compatible API Endpoints

Hiện tại có custom search API nhưng thiếu OSV v1 spec-compatible endpoints.

**Thêm mới** (không sửa search_handler.go cũ):
```
delivery/http/osv_v1_handler.go   ← NEW
usecase/osv_query/
├── usecase.go    ← NEW: Query by package/version (OSV v1 spec)
└── dto.go        ← NEW: OSV query request/response types
```

```go
// delivery/http/osv_v1_handler.go
// POST /v1/query
type OSVQueryRequest struct {
    Version string     `json:"version"`
    Package OSVPackage `json:"package"`
    Commit  string     `json:"commit"`
}

// POST /v1/querybatch — max 1000 queries
type OSVBatchRequest struct {
    Queries []OSVQueryRequest `json:"queries"`
}
// Response: {"results": [[vuln1, vuln2], [vuln3]]}

// GET /v1/vulns/{id} → delegate to existing getbyid usecase

// GET /v1/vulns/list?page_token=TOKEN&modified_since=RFC3339
// Stream-friendly paginated listing
```

**Routing** (thêm vào router, không xóa route cũ):
```go
// cmd/server/main.go hoặc delivery/http/router.go — THÊM:
r.Post("/v1/query", osvV1H.Query)
r.Post("/v1/querybatch", osvV1H.QueryBatch)
r.Get("/v1/vulns/{id}", osvV1H.GetVuln)       // delegate to getbyid UC
r.Get("/v1/vulns/list", osvV1H.ListVulns)
```

### 🟡 P1 — Thêm: DetermineVersion Endpoint

**Thêm mới**:
```
usecase/determine_version/
├── usecase.go    ← NEW: Hash set → version match
└── dto.go        ← NEW: Input/Output
```

```go
// POST /v1/determine-version
type DetermineVersionRequest struct {
    Name       string         `json:"name"`
    HashValues []HashValue    `json:"hash_values"`
}

type HashValue struct {
    Sha256  string  `json:"sha256"`
    GitBlob string  `json:"git_blob"`
}

// Gọi data-service qua gRPC để lấy repo index data:
// adapter/grpcclient/data_client.go ← NEW
type DataServiceClient struct {
    conn   *grpc.ClientConn
    client datapb.DataServiceClient
}

func (c *DataServiceClient) QueryVersionIndex(ctx, name string, hashes []string) ([]VersionMatch, error)
```

### 🟡 P1 — Thêm: MongoDB Alternative cho Firestore Vulnerability Reader

Giữ Firestore reader, nhưng thêm MongoDB alternative:

```go
// infra/persistence/mongo/vulnerability_reader.go  ← NEW
package mongo

// Đọc vulnerability từ MongoDB thay Firestore
// Config: VULN_READER_BACKEND=mongo hoặc firestore
```

**Config**:
```go
// internal/config/infra_config.go  ← NEW
type VulnReaderBackend string
const (
    VulnReaderFirestore VulnReaderBackend = "firestore"  // current default
    VulnReaderMongo     VulnReaderBackend = "mongo"       // new alternative
)
```

### 🟡 P1 — Thêm: Semantic Search Enhancement

`application/query/semantic_search/handler.go` đã có, nhưng cần vector storage.

**Thêm mới**:
```
infra/vector/
├── postgres_vector_store.go   ← NEW: pgvector extension
└── mongo_vector_store.go      ← NEW: MongoDB Atlas Vector Search
```

```go
// Chọn vector backend qua config:
// VECTOR_BACKEND=pgvector hoặc mongo_atlas
```

### 🟢 P2 — Thêm: OpenSearch Index Alias Management

```
infra/opensearch/index_manager.go   ← NEW
```

Zero-downtime re-indexing với alias swap:
```go
// Blue/green indexing:
// 1. Create new index: vulns_v2
// 2. Index all data to vulns_v2
// 3. Swap alias vulns → vulns_v2
// 4. Keep vulns_v1 as backup
```

### 🟢 P2 — Thêm: PostgreSQL CVE Search (v2 backend)

Hiện tại `infra/postgres/cve_repo.go` tồn tại. Thêm full-text search capability:

```go
// infra/postgres/cve_fulltext_repo.go  ← NEW
// Sử dụng PostgreSQL tsvector + tsquery
// pg_trgm extension cho fuzzy matching
// Đây là fallback khi OpenSearch không available
```

---

## 3. Migration Plan — Không có migrations mới (search-service không có DB riêng)

Search-service đọc từ MongoDB/PostgreSQL của data-service và có index riêng trong OpenSearch.

Nếu thêm pgvector:
```sql
-- migrations/001_pgvector.up.sql  ← NEW (first migration for search-service)
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE cve_embeddings (
    cve_id      VARCHAR(30) PRIMARY KEY,
    embedding   vector(1536),    -- OpenAI ada-002 / Vertex text-embedding-004
    model       VARCHAR(100),
    indexed_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_cve_embeddings_cosine 
    ON cve_embeddings USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 100);
```

---

## 4. go.mod — Chỉ THÊM Dependency

```go
require (
    // ... tất cả existing deps giữ nguyên ...
    
    // NEW (nếu dùng pgvector):
    github.com/pgvector/pgvector-go v0.2.2
)
```

---

## 5. File Changes Summary

### Files cần THÊM MỚI:
```
infra/messaging/nats/cve_update_consumer.go
infra/persistence/mongo/vulnerability_reader.go
infra/vector/postgres_vector_store.go
infra/vector/mongo_vector_store.go
infra/opensearch/index_manager.go
infra/postgres/cve_fulltext_repo.go
adapter/grpcclient/data_client.go
delivery/http/osv_v1_handler.go
usecase/osv_query/usecase.go
usecase/osv_query/dto.go
usecase/determine_version/usecase.go
usecase/determine_version/dto.go
internal/config/infra_config.go
migrations/001_pgvector.up.sql       (nếu dùng pgvector)
```

### Files cần EXTEND (thêm logic, không xóa):
```
infra/messaging/nats/consumer.go    ← Giữ existing, thêm start cve_update_consumer
cmd/server/main.go                  ← Thêm wire mới cho OSV v1 handler
go.mod                              ← Thêm pgvector nếu cần
```

### Files KHÔNG ĐƯỢC CHẠM:
```
infra/persistence/firestore/vulnerability_reader.go  ← GIỮ NGUYÊN
infra/elasticsearch/finding_indexer.go               ← GIỮ NGUYÊN
infra/persistence/mongo/cpe_repo.go                  ← GIỮ NGUYÊN
factory/backend.go + factory.go                      ← GIỮ NGUYÊN (đã support multi-backend)
usecase/cvesearch/usecase.go                         ← GIỮ NGUYÊN
delivery/http/search_handler.go                      ← GIỮ NGUYÊN
```

---

## 6. Checklist

### Phase A — P0 (Sprint 1)
- [ ] Thêm `infra/messaging/nats/cve_update_consumer.go`
- [ ] Subscribe `data.cve.updated` + `data.cve.withdrawn`
- [ ] Wire new consumer trong `cmd/server/main.go`

### Phase B — P1 (Sprint 2)
- [x] Thêm `usecase/osv_query/` (OSV v1 compatible)
- [x] Thêm `delivery/http/osv_v1_handler.go`
- [ ] Thêm `/v1/query`, `/v1/querybatch`, `/v1/vulns/{id}`, `/v1/vulns/list` routes
- [ ] Thêm `usecase/determine_version/`
- [ ] Thêm `adapter/grpcclient/data_client.go`
- [ ] Thêm `internal/config/infra_config.go` (vuln reader backend selector)

### Phase C — P2 (Sprint 3)
- [x] Thêm `infra/vector/` (pgvector hoặc mongo atlas)
- [x] Thêm `migrations/001_pgvector.up.sql` nếu dùng pgvector
- [ ] Thêm `infra/opensearch/index_manager.go`
- [ ] Thêm `infra/postgres/cve_fulltext_repo.go`
