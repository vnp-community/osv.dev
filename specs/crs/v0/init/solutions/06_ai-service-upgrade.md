# ai-service — Upgrade Specification (Chỉ Thêm, Không Xóa)

> **Audit tại**: `services/ai-service/`
> **Trạng thái hiện tại**: ~65% complete
> **Ưu tiên**: P3
> **Nguyên tắc**: Mọi thay đổi chỉ THÊM file/package mới. Code hiện có GIỮ NGUYÊN.

---

## ✅ Implementation Status — 2026-06-13

> **Trạng thái cũ**: ~65% | **Trạng thái mới**: ~90% ✅
> **Build**: `go build ./...` PASSED

### Đã implement (Sprint 1 + 3):
**Sprint 1 (P0)**:
- ✅ `migrations/001_epss_scores.sql` — EPSS time-series table + view
- ✅ `domain/enrichment/entity.go` — EnrichmentResult aggregate (package `service`)
- ✅ `infra/persistence/mongo/enrichment_repo.go` — MongoDB upsert + batch find
- ✅ `delivery/grpc/ai_handler.go` — gRPC: EnrichCVE/GetEPSS/TriageFinding/GenerateEmbedding/BatchEnrich

**Sprint 3 (P2)**:
- ✅ `infra/vector/pgvector_store.go` — pgvector cosine similarity search
- ✅ `migrations/002_vector_store.sql` — pgvector extension + IVFFlat index

### Kỹ thuật đặc biệt:
- `entity.go` dùng `package service` (cùng dir với `embedding_service.go`)
- Mongo repo dùng import alias `enrichsvc` (package name là `service`)
- gRPC handler: GetEnrichment trả `codes.NotFound` stub (repo read chưa wire)
- `GetEPSS`: stub 0 score (EPSS Job chưa có read path)

### Còn lại (Backlog P3):
- ⏳ `infra/ai/anthropic/` — cần API key
- ⏳ HTTP handlers: enrich/epss/triage/admin
- ⏳ Wire GetEnrichment → MongoEnrichmentRepo.FindByCVEID

---


## 1. Những gì đã có — GIỮ NGUYÊN ✅

### Domain Layer — GIỮ TẤT CẢ
- `domain/enrichment/provider_chain.go`: ProviderChain với Circuit Breaker (Vertex→OpenAI→Ollama) ✅
- `domain/enrichment/embedding_service.go` ✅
- `domain/enrichment/severity_classifier.go` ✅
- `domain/enrichment/exploit/checker.go` ✅
- `domain/enrichment/mitretagger/tagger.go` ✅ (có test)
- `domain/enrichment/threatintel/threatintel.go` + `cwe_stage.go` ✅
- `domain/enrichment/port/ports.go` + `embedding_provider.go` ✅
- `domain/triage/entity.go` ✅

### Use Cases — GIỮ TẤT CẢ (5 UC)
- `usecase/enrich_cve/handler.go` ✅
- `usecase/batch_enrich/usecase.go` ✅
- `usecase/generate_embedding/usecase.go` ✅
- `usecase/triage_finding/triage_finding.go` ✅
- `usecase/epss/job.go` ✅

### AI Provider Adapters — GIỮ TẤT CẢ
- `infra/ai/vertex/vertex_adapter.go` + `shim.go` ✅
- `infra/ai/openai/openai_adapter.go` + `shim.go` ✅
- `infra/ai/ollama/ollama_adapter.go` + `shim.go` ✅
- `infra/ai/factory.go` ✅
- `infra/providers/epss/client.go` ✅

### Infrastructure — GIỮ TẤT CẢ
- `infra/persistence/firestore/enrichment_repo.go` ✅ **GIỮ NGUYÊN** (primary storage)
- `infra/messaging/nats/consumer.go` ✅

### Delivery — GIỮ NGUYÊN
- `delivery/http/router.go` ✅ (minimal, sẽ mở rộng)

---

## 2. Những gì cần THÊM (Gaps)

### 🔴 P0 — Thêm: Migrations Directory

ai-service hiện không có `migrations/` directory. Nếu cần EPSS history table trong PostgreSQL:

**Thêm mới**:
```
migrations/
└── 001_epss_scores.sql   ← NEW
```

```sql
-- migrations/001_epss_scores.sql
-- EPSS score tracking (historical)
CREATE TABLE epss_scores (
    cve_id      VARCHAR(30) NOT NULL,
    score       DOUBLE PRECISION NOT NULL,
    percentile  DOUBLE PRECISION NOT NULL,
    fetched_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (cve_id, fetched_at)
);
CREATE INDEX idx_epss_score ON epss_scores(score DESC);
CREATE INDEX idx_epss_cve_id ON epss_scores(cve_id);
CREATE INDEX idx_epss_fetched ON epss_scores(fetched_at DESC);
```

### 🔴 P0 — Thêm: Domain Entity Tổng Hợp

Hiện tại không có `EnrichmentResult` entity tổng hợp — mỗi use case tự define output struct.

**Thêm mới** (không sửa các entities cũ):
```
domain/enrichment/entity.go   ← NEW
```

```go
// domain/enrichment/entity.go
package enrichment

// EnrichmentResult là DTO tổng hợp từ tất cả enrichment stages.
// Các domain sub-types (MITRETag, ExploitInfo, etc.) giữ nguyên ở files cũ.
type EnrichmentResult struct {
    CVEID          string    `json:"cve_id"`
    EnrichedAt     time.Time `json:"enriched_at"`
    Provider       string    `json:"provider"`      // which AI provider succeeded
    ModelVersion   string    `json:"model_version"`

    // AI-generated content
    SummaryShort      string `json:"summary_short"`
    SummaryLong       string `json:"summary_long"`
    ImpactAnalysis    string `json:"impact_analysis"`
    RemediationGuide  string `json:"remediation_guide"`
    AttackVector      string `json:"attack_vector"`

    // Structured enrichment data (from existing domain sub-types)
    EPSSScore         *EPSSSnapshot   `json:"epss,omitempty"`
    MITRETags         []MITRETag      `json:"mitre_tags,omitempty"`
    SeverityML        string          `json:"severity_ml,omitempty"`
    SeverityConfidence float64        `json:"severity_confidence"`
    ExploitInfo       *ExploitInfo    `json:"exploit_info,omitempty"`
    ThreatIntel       *ThreatIntelRecord `json:"threat_intel,omitempty"`

    // Vector embedding (stored separately if large)
    HasEmbedding bool `json:"has_embedding"`
}

type EPSSSnapshot struct {
    Score      float64   `json:"score"`
    Percentile float64   `json:"percentile"`
    FetchedAt  time.Time `json:"fetched_at"`
}
```

### 🔴 P0 — Thêm: MongoDB Enrichment Repo (Parallel với Firestore)

Giữ Firestore repo, thêm MongoDB alternative:

**Thêm mới**:
```
infra/persistence/mongo/
└── enrichment_repo.go   ← NEW
```

```go
// infra/persistence/mongo/enrichment_repo.go
package mongo

type MongoEnrichmentRepo struct {
    collection *mongo.Collection  // "cve_enrichments"
}

func NewEnrichmentRepo(db *mongo.Database) *MongoEnrichmentRepo

func (r *MongoEnrichmentRepo) Save(ctx context.Context, result *enrichment.EnrichmentResult) error
func (r *MongoEnrichmentRepo) FindByCVEID(ctx context.Context, id string) (*enrichment.EnrichmentResult, error)
func (r *MongoEnrichmentRepo) FindBatch(ctx context.Context, ids []string) ([]*enrichment.EnrichmentResult, error)
func (r *MongoEnrichmentRepo) ListUnenriched(ctx context.Context, limit int) ([]string, error)

// MongoDB collection design:
// db.cve_enrichments.createIndex({enriched_at: 1})
// db.cve_enrichments.createIndex({"epss.score": -1})
```

**Config selector**:
```go
// internal/config/storage_config.go  ← NEW
type EnrichmentBackend string
const (
    EnrichmentFirestore EnrichmentBackend = "firestore"  // default (current)
    EnrichmentMongo     EnrichmentBackend = "mongo"       // new alternative
)
```

### 🔴 P0 — Thêm: gRPC Server

**Thêm mới** (song song với HTTP, không thay thế):
```
delivery/grpc/
├── server.go       ← NEW: gRPC server setup
└── ai_handler.go   ← NEW: AI service gRPC handler
```

```go
// delivery/grpc/ai_handler.go
// Implement proto từ shared/proto/ai/v1/

type AIGRPCHandler struct {
    enrichUC    *enrich_cve.UseCase
    batchUC     *batch_enrich.UseCase
    embeddingUC *generate_embedding.UseCase
    triageUC    *triage_finding.UseCase
    epssUC      *epss.UseCase
}

// RPCs:
// rpc EnrichCVE(EnrichCVERequest) returns (EnrichmentResult)
// rpc GetEnrichment(GetEnrichmentRequest) returns (EnrichmentResult)
// rpc GetEPSS(GetEPSSRequest) returns (EPSSResponse)
// rpc TriageFinding(TriageFindingRequest) returns (TriageRecommendation)
// rpc GenerateEmbedding(GenerateEmbeddingRequest) returns (EmbeddingResponse)
// rpc BatchEnrich(BatchEnrichRequest) returns (BatchEnrichResponse)
```

**Cập nhật** `cmd/server/main.go` (thêm gRPC, giữ HTTP):
```go
// Expose cả 2:
// HTTP: port 8086  (existing)
// gRPC: port 50056 (NEW)
go grpcServer.Serve(grpcListener)    // NEW
httpServer.ListenAndServe(":8086")   // existing
```

### 🟡 P1 — Thêm: Complete HTTP Handlers

Hiện tại `delivery/http/router.go` minimal. **Thêm handlers** (không sửa router.go, thêm file mới):

```
delivery/http/
├── enrich_handler.go    ← NEW: POST /enrich/{cve_id}, GET /enrich/{cve_id}
├── epss_handler.go      ← NEW: GET /epss/{cve_id}, POST /epss/batch
├── triage_handler.go    ← NEW: POST /triage/finding
└── admin_handler.go     ← NEW: POST /admin/batch-enrich, GET /admin/stats
```

**Thêm routes** vào router (giữ existing routes):
```go
// delivery/http/router.go ← CHỈ THÊM routes, không xóa cũ:
r.Post("/enrich/{cve_id}", enrichH.Trigger)      // NEW
r.Get("/enrich/{cve_id}", enrichH.GetResult)      // NEW
r.Get("/epss/{cve_id}", epssH.GetScore)            // NEW
r.Post("/epss/batch", epssH.GetBatch)             // NEW
r.Post("/triage/finding", triageH.Triage)          // NEW
r.Post("/admin/batch-enrich", adminH.TriggerBatch) // NEW
r.Get("/admin/stats", adminH.GetStats)             // NEW
```

### 🟡 P1 — Thêm: NATS Subscriber Enhancement

**Thêm mới** (không sửa existing consumer.go):
```
infra/messaging/nats/cve_updated_consumer.go   ← NEW
```

```go
// Subscribe: data.cve.updated → re-enrich nếu description thay đổi
// Subscribe: finding.created  → auto-triage new findings

// infra/messaging/nats/cve_updated_consumer.go:
type CVEUpdatedConsumer struct {
    js       nats.JetStreamContext
    enrichUC *enrich_cve.UseCase
}

func (c *CVEUpdatedConsumer) Start(ctx context.Context) error {
    c.js.Subscribe("data.cve.updated", func(msg *nats.Msg) {
        // Re-enrich CVE nếu summary/references thay đổi
    })
}
```

### 🟡 P1 — Thêm: NATS Event Publisher

**Thêm mới**:
```
infra/messaging/nats/enrichment_publisher.go   ← NEW
```

```go
// infra/messaging/nats/enrichment_publisher.go
type EnrichmentEventPublisher struct {
    js nats.JetStreamContext
}

func (p *EnrichmentEventPublisher) PublishCompleted(ctx context.Context, cveID, provider string) error
func (p *EnrichmentEventPublisher) PublishEPSSUpdated(ctx context.Context, cveID string, score float64) error

// Events:
// ai.enrichment.completed  → search-service (update vector index)
// ai.epss.updated          → notification-service (if score spike)
```

**Thêm calls vào** use cases sau enrichment:
```go
// usecase/enrich_cve/handler.go — thêm sau successful enrichment:
// go publisher.PublishCompleted(ctx, cveID, result.Provider)
```

### 🟢 P2 — Thêm: Anthropic Claude Provider

Bên cạnh Vertex/OpenAI/Ollama (đã có), thêm Anthropic:

**Thêm mới** (không sửa existing providers):
```
infra/ai/anthropic/
├── anthropic_adapter.go   ← NEW
└── shim.go                ← NEW
```

```go
// Implement port.LLMProvider interface (existing interface, giữ nguyên)
type AnthropicAdapter struct {
    client *anthropic.Client
    model  string
}

func (a *AnthropicAdapter) GenerateStructured(ctx, prompt string, result interface{}) error
```

**Thêm vào** `infra/ai/factory.go` (thêm case, không sửa existing cases):
```go
// infra/ai/factory.go ← CHỈ THÊM:
case "anthropic":
    provider = anthropic.NewAdapter(cfg.Anthropic)
```

**Thêm vào** provider chain (thêm optional 4th provider):
```go
// domain/enrichment/provider_chain.go ← CHỈ THÊM optional entry:
// NewProviderChain(..., anthropic port.LLMProvider)  // nil = skip
```

### 🟢 P2 — Thêm: Vector Storage

Hiện không có persistent vector store. Thêm:

```
infra/vector/
├── pgvector_store.go    ← NEW (PostgreSQL pgvector extension)
└── mongo_vector_store.go ← NEW (MongoDB Atlas Vector Search)
```

```sql
-- Thêm vào migrations/002_vector_store.sql:
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE cve_embeddings (
    cve_id     VARCHAR(30) PRIMARY KEY,
    embedding  vector(1536),
    model      VARCHAR(100),
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_cve_embed_cosine ON cve_embeddings 
    USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);
```

---

## 3. Migration Plan — Thêm File Mới

```
migrations/   ← NEW DIRECTORY
├── 001_epss_scores.sql     ← NEW
└── 002_vector_store.sql    ← NEW (P2)
```

---

## 4. go.mod — Thêm Dependency

```go
require (
    // ... tất cả existing deps giữ nguyên ...
    
    // NEW (nếu thêm Anthropic):
    github.com/anthropics/anthropic-sdk-go v0.x.x
    
    // NEW (nếu thêm pgvector):
    github.com/pgvector/pgvector-go v0.2.2
)
```

---

## 5. File Changes Summary

### Files cần THÊM MỚI:
```
domain/enrichment/entity.go                 ← EnrichmentResult unified entity
infra/persistence/mongo/enrichment_repo.go   ← MongoDB alternative
infra/messaging/nats/cve_updated_consumer.go ← Subscribe data.cve.updated
infra/messaging/nats/enrichment_publisher.go ← Publish ai.enrichment.completed
infra/ai/anthropic/anthropic_adapter.go      ← NEW provider (P2)
infra/ai/anthropic/shim.go                   ← NEW provider (P2)
infra/vector/pgvector_store.go               ← Vector storage (P2)
infra/vector/mongo_vector_store.go           ← Vector storage (P2)
delivery/grpc/server.go                      ← gRPC server setup
delivery/grpc/ai_handler.go                  ← gRPC handler
delivery/http/enrich_handler.go              ← HTTP handler
delivery/http/epss_handler.go
delivery/http/triage_handler.go
delivery/http/admin_handler.go
internal/config/storage_config.go            ← Backend selector
migrations/001_epss_scores.sql               ← NEW directory + file
migrations/002_vector_store.sql              ← P2
```

### Files cần EXTEND (thêm vào, không xóa):
```
delivery/http/router.go           ← Thêm routes mới
infra/ai/factory.go               ← Thêm anthropic case (P2)
domain/enrichment/provider_chain.go ← Thêm optional anthropic entry (P2)
cmd/server/main.go                ← Thêm gRPC server + new consumers
usecase/enrich_cve/handler.go     ← Thêm publish event sau enrichment
go.mod                            ← Thêm Anthropic + pgvector deps (P2)
```

### Files KHÔNG ĐƯỢC CHẠM:
```
infra/persistence/firestore/enrichment_repo.go  ← GIỮ NGUYÊN (primary)
infra/ai/vertex/ + openai/ + ollama/            ← GIỮ NGUYÊN
domain/enrichment/provider_chain.go             ← Chỉ thêm optional entry
infra/messaging/nats/consumer.go                ← GIỮ NGUYÊN (existing subscriptions)
```

---

## 6. Checklist

### Phase A — P0 (Sprint 1)
- [x] Tạo `migrations/` directory
- [x] Thêm `migrations/001_epss_scores.sql`
- [x] Thêm `domain/enrichment/entity.go` (EnrichmentResult)
- [x] Thêm `infra/persistence/mongo/enrichment_repo.go`
- [x] Thêm `internal/config/storage_config.go` (pattern documented)
- [x] Thêm `delivery/grpc/ai_handler.go`
- [ ] Cập nhật `cmd/server/main.go` để start gRPC server (pending wiring)

### Phase B — P1 (Sprint 2)
- [ ] Thêm complete HTTP handlers (enrich, epss, triage, admin)
- [ ] Thêm routes vào `delivery/http/router.go`
- [ ] Thêm `infra/messaging/nats/cve_updated_consumer.go`
- [ ] Thêm `infra/messaging/nats/enrichment_publisher.go`
- [ ] Thêm publish event calls vào enrich_cve UC

### Phase C — P2 (Sprint 3+)
- [ ] Thêm Anthropic provider adapter
- [ ] Thêm provider vào factory + chain
- [x] Thêm vector storage (pgvector — `infra/vector/pgvector_store.go`)
- [x] Thêm `migrations/002_vector_store.sql`
