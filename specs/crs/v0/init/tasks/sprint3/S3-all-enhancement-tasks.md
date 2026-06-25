# Sprint 3 — Enhancement Tasks

## ✅ Execution Status: COMPLETED
- **Executed**: 2026-06-13
- **Result**: `go build` PASSED


> P2 tasks — làm sau khi Sprint 1 và Sprint 2 hoàn tất.
> Spec nguồn: các file `specs/develop/0X_*-service-upgrade.md`

---

## S3-ID-01 — Email Verification gRPC (identity-service)

**Files to Create**:
```
services/identity-service/shared/proto/auth/v1/  ← thêm RPCs (nếu cần)
```

**Goal**: Expose email verification as gRPC RPC cho gateway có thể gọi:
```protobuf
// Thêm vào service AuthService (giữ existing RPCs):
rpc ValidateAPIKey(ValidateAPIKeyRequest) returns (ValidateAPIKeyResponse);
rpc ValidateTOTP(ValidateTOTPRequest) returns (ValidateTOTPResponse);
```

**Steps**:
1. Đọc `shared/proto/auth/v1/*.proto`
2. Thêm 2 RPC definitions vào service
3. Run `make proto-gen` hoặc `protoc`
4. Implement trong `adapter/handler/grpc/auth_grpc_handler.go`

---

## S3-AI-01 — Anthropic Claude Provider (ai-service)

**Spec**: `specs/develop/06_ai-service-upgrade.md` § "P2 — Thêm: Anthropic"

**Files to Create**:
```
services/ai-service/internal/infra/ai/anthropic/anthropic_adapter.go
services/ai-service/internal/infra/ai/anthropic/shim.go
```

**go.mod addition**:
```bash
go get github.com/anthropics/anthropic-sdk-go
```

**anthropic_adapter.go** (implement `port.LLMProvider` interface from existing codebase):
```go
type AnthropicAdapter struct {
    client *anthropic.Client
    model  string  // e.g., "claude-3-5-sonnet-20241022"
}

func (a *AnthropicAdapter) GenerateStructured(ctx context.Context, prompt string, result interface{}) error {
    // Call Anthropic Messages API
    // Use tool_use to get structured JSON output
}
```

**Extend factory.go** (add case, don't modify existing):
```go
case "anthropic":
    return anthropic.NewAdapter(cfg.APIKey, cfg.Model), nil
```

**Extend provider_chain.go** (add optional 4th provider):
```go
// Optional 4th in fallback chain: Vertex → OpenAI → Ollama → Anthropic
// Only added if ANTHROPIC_API_KEY is set
```

---

## S3-AI-02 — Vector Storage pgvector (ai-service)

**Spec**: `specs/develop/06_ai-service-upgrade.md` § "P2 — Thêm: Vector Storage"

**Files to Create**:
```
services/ai-service/internal/infra/vector/pgvector_store.go
services/ai-service/migrations/002_vector_store.sql
```

**Migration**:
```sql
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS cve_embeddings (
    cve_id      VARCHAR(30)   PRIMARY KEY,
    embedding   vector(1536),             -- OpenAI ada-002 / Vertex text-embedding-004
    model       VARCHAR(100),
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

-- IVFFlat index for cosine similarity search
CREATE INDEX IF NOT EXISTS idx_cve_embed_cosine
    ON cve_embeddings USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 100);
```

**pgvector_store.go**:
```go
// go get github.com/pgvector/pgvector-go
type PgVectorStore struct {
    db *pgxpool.Pool
}

func (s *PgVectorStore) Save(ctx context.Context, cveID string, embedding []float32) error
func (s *PgVectorStore) SimilaritySearch(ctx context.Context, queryEmbedding []float32, limit int) ([]string, error)
func (s *PgVectorStore) Get(ctx context.Context, cveID string) ([]float32, error)
```

---

## S3-SCAN-01 — Trivy Scanner Adapter (scan-service)

**Spec**: `specs/develop/04_scan-service-upgrade.md` § "P2 — Thêm: Trivy"

**Files to Create**:
```
services/scan-service/internal/adapters/scanner/trivy/trivy_client.go
```

**trivy_client.go**:
```go
// Gọi Trivy via CLI (subprocess) hoặc Trivy Server HTTP API

type TrivyClient struct {
    serverURL string  // empty = use CLI mode
    timeout   time.Duration
}

// Scan returns CycloneDX SBOM (reuse existing sbom/cyclonedx parser)
func (c *TrivyClient) ScanImage(ctx context.Context, image string) (*sbom.SBOM, error)
func (c *TrivyClient) ScanDirectory(ctx context.Context, path string) (*sbom.SBOM, error)
func (c *TrivyClient) ScanFilesystem(ctx context.Context, path string) (*sbom.SBOM, error)
```

**Trivy CLI mode**:
```bash
# trivy image --format cyclonedx --output result.json nginx:latest
os/exec → parse stdout as CycloneDX JSON → sbom.Parse()
```

---

## S3-NOTIF-01 — In-App Notifications SSE (notification-service)

**Spec**: `specs/develop/07_notification-service-upgrade.md` § "P2 — Thêm: In-App"

**Files to Create**:
```
services/notification-service/internal/infra/adapters/inapp/store.go
services/notification-service/internal/delivery/http/inapp_handler.go
services/notification-service/migrations/007_inapp_notifications.up.sql
```

**Migration**:
```sql
CREATE TABLE inapp_notifications (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL,
    event_type  VARCHAR(100) NOT NULL,
    title       VARCHAR(500) NOT NULL,
    body        TEXT,
    payload     JSONB,
    is_read     BOOLEAN NOT NULL DEFAULT FALSE,
    read_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON inapp_notifications(user_id, created_at DESC) WHERE is_read = FALSE;
```

**inapp_handler.go (SSE)**:
```go
// GET /notifications/stream → Server-Sent Events
// GET /notifications        → Paginated list (for polling fallback)
// POST /notifications/{id}/read ← Mark as read
// POST /notifications/read-all  ← Mark all as read

// SSE implementation:
func (h *InAppHandler) Stream(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")

    userID := extractUserID(r)
    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "streaming not supported", http.StatusInternalServerError)
        return
    }

    // Poll DB every 5 seconds for new notifications
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            notifications, _ := h.store.GetUnread(r.Context(), userID, lastSeen)
            for _, n := range notifications {
                fmt.Fprintf(w, "data: %s\n\n", toJSON(n))
                flusher.Flush()
            }
        case <-r.Context().Done():
            return
        }
    }
}
```

---

## S3-NOTIF-02 — Digest Mode (notification-service)

**Spec**: `specs/develop/07_notification-service-upgrade.md` § "P2 — Digest"

**Files to Create**:
```
services/notification-service/internal/usecase/send_digest/usecase.go
services/notification-service/internal/scheduler/digest_scheduler.go
```

**digest_scheduler.go**:
```go
// Daily digest: 08:00 UTC
// Weekly digest: Monday 08:00 UTC

// UseCase:
// 1. Query alerts from last 24h (or 7 days for weekly)
// 2. Group by product_id + event_type
// 3. Format digest template (HTML email)
// 4. Send via SMTP/Slack
```

---

## S3-SEARCH-01 — pgvector Semantic Search (search-service)

**Spec**: `specs/develop/03_search-service-upgrade.md` § "P2 — Thêm: Semantic Search Enhancement"

**Files to Create**:
```
services/search-service/internal/infra/vector/postgres_vector_store.go
services/search-service/migrations/001_pgvector.up.sql
```

**Migration**:
```sql
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS cve_embeddings (
    cve_id     VARCHAR(30) PRIMARY KEY,
    embedding  vector(1536),
    model      VARCHAR(100),
    indexed_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_search_embed_cosine
    ON cve_embeddings USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 100);
```

**postgres_vector_store.go**:
```go
type PostgresVectorStore struct {
    db *pgxpool.Pool
}

// SimilarCVEs finds similar CVEs using cosine similarity
func (s *PostgresVectorStore) SimilarCVEs(ctx context.Context, queryEmbedding []float32, limit int) ([]string, error) {
    // SELECT cve_id, 1 - (embedding <=> $1) AS similarity
    // FROM cve_embeddings
    // ORDER BY embedding <=> $1
    // LIMIT $2
}
```

---

## S3-GW-01 — GraphQL BFF (gateway-service)

**Spec**: `specs/develop/08_gateway-service-upgrade.md` § "P2 — Thêm: GraphQL BFF"

**Files to Create**:
```
services/gateway-service/internal/bff/graphql/schema.graphql
services/gateway-service/internal/bff/graphql/resolver.go
services/gateway-service/internal/bff/graphql/server.go
```

**go.mod addition**:
```bash
go get github.com/graphql-go/graphql
# hoặc dùng gqlgen (code-first approach):
go get github.com/99designs/gqlgen
```

**schema.graphql** (key types):
```graphql
type CVE {
    id: String!
    summary: String
    severity: String
    cvssScore: Float
    epss: EPSSData
    enrichment: EnrichmentSummary
    activeFindings: [Finding!]
    kevStatus: KEVInfo
}

type Query {
    cve(id: String!): CVE
    searchCVEs(query: String!, page: Int, limit: Int): CVESearchResult
    dashboard: DashboardData
}
```

**Route to Add**:
```
POST /graphql → GraphQL endpoint
GET  /graphql → GraphiQL playground (dev only)
```
