# P1 — Data Correctness Fixes (Ngắn hạn)

> **Bugs**: MOCK-007, MOCK-008, MOCK-011  
> **Mức độ**: 🔴 High — Dữ liệu sai hoặc feature không hoạt động  
> **Timeline**: Tuần 2

---

## MOCK-007 — Fix: Wire Agent PostgreSQL Repository

### Vấn đề
```go
// agent_handler.go: Fake implementation, không lưu DB
// "Fake implementation for seed purposes"
apiKeyPlaintext := "ak_live_" + uuid.New().String()
agent := AgentDto{ID: uuid.New(), ...}  // random UUID, không lưu
```

### Giải pháp

Theo `01-architecture.md §3.6`: Agent là thực thể của scan-service với active scanning. Cần PostgreSQL repo theo **Repository Pattern** (`02-technical-design.md §2.3`).

#### Bước 1: Tạo Agent domain entity

**File mới**: `services/scan-service/internal/domain/agent.go`

```go
package domain

import (
    "time"
    "github.com/google/uuid"
)

// Agent đại diện cho một scan agent được đăng ký.
type Agent struct {
    ID        uuid.UUID
    Name      string
    Hostname  string
    IPAddress string
    OS        string
    Tags      []string
    APIKeyHash string   // SHA-256 hash của API key, không lưu plaintext
    Status    AgentStatus
    CreatedAt time.Time
    LastSeenAt *time.Time
}

type AgentStatus string
const (
    AgentStatusInactive AgentStatus = "inactive"
    AgentStatusActive   AgentStatus = "active"
    AgentStatusOffline  AgentStatus = "offline"
)

// AgentRepository là interface cho data access (Clean Architecture)
type AgentRepository interface {
    Create(ctx context.Context, agent *Agent) error
    FindByID(ctx context.Context, id uuid.UUID) (*Agent, error)
    FindByAPIKeyHash(ctx context.Context, hash string) (*Agent, error)
    List(ctx context.Context) ([]*Agent, error)
    UpdateStatus(ctx context.Context, id uuid.UUID, status AgentStatus) error
    UpdateLastSeen(ctx context.Context, id uuid.UUID) error
}
```

#### Bước 2: Tạo PostgreSQL repository

**File mới**: `services/scan-service/internal/infra/postgres/agent_repo.go`

```go
package postgres

import (
    "context"
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/osv/scan-service/internal/domain"
)

type AgentRepo struct {
    db *pgxpool.Pool
}

func NewAgentRepo(db *pgxpool.Pool) *AgentRepo {
    return &AgentRepo{db: db}
}

func (r *AgentRepo) Create(ctx context.Context, agent *domain.Agent) error {
    _, err := r.db.Exec(ctx, `
        INSERT INTO scan_agents (id, name, hostname, ip_address, os, tags, api_key_hash, status, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
    `, agent.ID, agent.Name, agent.Hostname, agent.IPAddress,
       agent.OS, agent.Tags, agent.APIKeyHash, agent.Status, agent.CreatedAt)
    return err
}

func (r *AgentRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
    row := r.db.QueryRow(ctx, `
        SELECT id, name, hostname, ip_address, os, tags, status, created_at, last_seen_at
        FROM scan_agents WHERE id = $1
    `, id)
    // scan...
}

func (r *AgentRepo) List(ctx context.Context) ([]*domain.Agent, error) {
    rows, err := r.db.Query(ctx, `
        SELECT id, name, hostname, ip_address, os, tags, status, created_at, last_seen_at
        FROM scan_agents ORDER BY created_at DESC
    `)
    // scan...
}
```

#### Bước 3: Refactor AgentHandler để dùng real repo

**File sửa**: `services/scan-service/internal/delivery/http/agent_handler.go`

```go
// AgentHandler handles agent registration and reports.
type AgentHandler struct {
    agentRepo domain.AgentRepository  // REAL repo, không phải interface rỗng
    log       zerolog.Logger
}

func NewAgentHandler(repo domain.AgentRepository, log zerolog.Logger) *AgentHandler {
    return &AgentHandler{agentRepo: repo, log: log}
}

// RegisterAgent handles POST /api/v1/agents — LƯU VÀO DB THỰC SỰ
func (h *AgentHandler) RegisterAgent(w http.ResponseWriter, r *http.Request) {
    var req AgentRegisterReq
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        h.writeJSON(w, 400, map[string]string{"error": "invalid_body"})
        return
    }

    // Generate API key (plaintext chỉ trả 1 lần)
    rawKey := "ak_live_" + uuid.New().String()
    keyHash := sha256Hex(rawKey)  // lưu hash, không lưu plaintext

    agent := &domain.Agent{
        ID:         uuid.New(),
        Name:       req.Name,
        Hostname:   req.Hostname,
        IPAddress:  req.IPAddress,
        OS:         req.OS,
        Tags:       req.Tags,
        APIKeyHash: keyHash,
        Status:     domain.AgentStatusInactive,
        CreatedAt:  time.Now().UTC(),
    }

    // Nil-check: nếu repo chưa wire, trả 503
    if h.agentRepo == nil {
        h.writeJSON(w, http.StatusServiceUnavailable,
            map[string]string{"error": "agent_repo_not_configured"})
        return
    }

    if err := h.agentRepo.Create(r.Context(), agent); err != nil {
        h.log.Error().Err(err).Msg("RegisterAgent: create failed")
        h.writeJSON(w, 500, map[string]string{"error": "internal_error"})
        return
    }

    h.writeJSON(w, 201, map[string]any{
        "id":         agent.ID,
        "name":       agent.Name,
        "hostname":   agent.Hostname,
        "ip_address": agent.IPAddress,
        "os":         agent.OS,
        "tags":       agent.Tags,
        "api_key":    rawKey,    // ONE-TIME ONLY — không lưu lại
        "status":     agent.Status,
        "created_at": agent.CreatedAt,
    })
}

// ListAgents handles GET /api/v1/agents — LẤY TỪ DB THỰC SỰ
func (h *AgentHandler) ListAgents(w http.ResponseWriter, r *http.Request) {
    if h.agentRepo == nil {
        h.writeJSON(w, 200, map[string]any{"agents": []AgentDto{}, "count": 0})
        return
    }
    agents, err := h.agentRepo.List(r.Context())
    if err != nil {
        h.writeJSON(w, 500, map[string]string{"error": "internal_error"})
        return
    }
    dtos := mapAgentsToDtos(agents)
    h.writeJSON(w, 200, map[string]any{"agents": dtos, "count": len(dtos)})
}
```

#### Bước 4: Wire trong embedded.go

**File sửa**: `services/scan-service/embedded.go`

```go
func WireEmbedded(ctx context.Context, logger zerolog.Logger, pool *pgxpool.Pool, mux *http.ServeMux) error {
    // FIX MOCK-007: Wire real PostgreSQL repositories
    agentRepo := postgres.NewAgentRepo(pool)
    scanRepo  := postgres.NewScanRepo(pool)
    statsRepo := postgres.NewScanStatsRepo(pool)

    agentHandler := httpdelivery.NewAgentHandler(agentRepo, logger)
    scanHandler  := httpdelivery.NewScanAPIHandler(scanRepo, logger)
    statsHandler := httpdelivery.NewStatsHandler(statsRepo, logger)
    /* ... */
}
```

#### Schema DB cần tạo
```sql
CREATE TABLE IF NOT EXISTS scan_agents (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name         VARCHAR(100) NOT NULL,
    hostname     VARCHAR(255),
    ip_address   VARCHAR(45),
    os           VARCHAR(50),
    tags         TEXT[] DEFAULT '{}',
    api_key_hash CHAR(64) NOT NULL UNIQUE,  -- SHA-256 hex
    status       VARCHAR(20) DEFAULT 'inactive',
    created_at   TIMESTAMPTZ DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ
);
CREATE INDEX idx_scan_agents_api_key_hash ON scan_agents(api_key_hash);
```

---

## MOCK-008 — Fix: Thay `MockEmbedder` bằng AI Service gRPC Client

### Vấn đề
```go
// embedded.go L89: MockEmbedder trả all-zero vector
semanticUC = pgvector.NewUseCase(searcher, &pgvector.MockEmbedder{})
```

### Giải pháp

Theo `01-architecture.md §3.11` và `02-technical-design.md §11.2`: Embedding được generate bởi **ai-service** (port 9103). Search-service phải gọi ai-service gRPC để generate embedding.

#### Bước 1: Tạo AI gRPC Embedder adapter

**File mới**: `services/search-service/internal/infra/grpc/ai_embedder.go`

```go
package grpc

import (
    "context"
    "google.golang.org/grpc"
    aiv1 "github.com/osv/shared/proto/gen/go/ai/v1"
)

// AIServiceEmbedder gọi ai-service gRPC để generate text embedding.
// Implements pgvector.Embedder interface.
type AIServiceEmbedder struct {
    client aiv1.AIServiceClient
}

func NewAIServiceEmbedder(target string) (*AIServiceEmbedder, error) {
    conn, err := grpc.Dial(target,
        grpc.WithInsecure(), // TODO: mTLS trong production
        grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(4*1024*1024)),
    )
    if err != nil {
        return nil, fmt.Errorf("ai-service gRPC dial: %w", err)
    }
    return &AIServiceEmbedder{client: aiv1.NewAIServiceClient(conn)}, nil
}

// Embed generates a 1536-dim vector embedding via ai-service.
// Theo 01-architecture.md §3.11: ai-service dùng Ollama→OpenAI failover chain.
func (e *AIServiceEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
    resp, err := e.client.GenerateEmbedding(ctx, &aiv1.GenerateEmbeddingRequest{
        Text: text,
    })
    if err != nil {
        return nil, fmt.Errorf("ai-service embedding: %w", err)
    }
    return resp.Embedding, nil
}
```

#### Bước 2: Sửa embedded.go — Wire AI Embedder, disable khi unavailable

**File sửa**: `services/search-service/embedded.go:L83-93`

```go
// 4. pgvector semantic search (optional — yêu cầu POSTGRES_DSN + ai-service)
var semanticUC *pgvector.UseCase

aiAddr := os.Getenv("AI_SERVICE_GRPC")  // e.g., "localhost:9103"
if aiAddr == "" {
    aiAddr = "localhost:9103"
}

if postgresDSN != "" {
    sqlxDB, err := sqlx.ConnectContext(ctx, "postgres", postgresDSN)
    if err == nil {
        searcher := pgvector.New(sqlxDB)

        // FIX MOCK-008: dùng AI service embedder thực sự
        aiEmbedder, embErr := aigrpc.NewAIServiceEmbedder(aiAddr)
        if embErr != nil {
            log.Warn().Err(embErr).
                Msg("search-service: AI embedder unavailable, semantic search disabled")
            // semanticUC = nil → SemanticSearch route sẽ trả 503
        } else {
            semanticUC = pgvector.NewUseCase(searcher, aiEmbedder)
            log.Info().Str("ai_addr", aiAddr).
                Msg("search-service: AI embedding via ai-service enabled")
        }
    }
}
```

#### Bước 3: Thêm nil-check trong SemanticSearch handler

**File sửa**: `services/search-service/internal/delivery/http/search_handler.go:L257`

```go
func (h *Handler) SemanticSearch(w http.ResponseWriter, r *http.Request) {
    // FIX MOCK-008: nil-check — trả 503 khi AI embedder chưa được wire
    if h.semanticUC == nil {
        respondError(w, http.StatusServiceUnavailable,
            "semantic search not available: AI embedding service not configured")
        return
    }
    /* ... phần còn lại giữ nguyên ... */
}
```

#### Bước 4: Không đăng ký route semantic search khi semanticUC nil

**File sửa**: `services/search-service/internal/delivery/http/search_handler.go:L342`

```go
// Trong NewRouter:
// FIX MOCK-008: chỉ đăng ký semantic search khi handler có embedder thực sự
if h.semanticUC != nil {
    r.Post("/api/v2/cves/search/semantic", h.SemanticSearch)
} else {
    // Stub route — trả 503 để frontend biết feature unavailable
    r.Post("/api/v2/cves/search/semantic", func(w http.ResponseWriter, r *http.Request) {
        respondError(w, http.StatusServiceUnavailable,
            "semantic search not configured: ai-service required")
    })
}
```

### Môi trường cần cấu hình
```bash
# .env / docker-compose
AI_SERVICE_GRPC=ai-service:9103   # address của ai-service gRPC
```

---

## MOCK-011 — Fix: OAuth Credentials từ Environment Variables

### Vấn đề
```go
// embedded.go L58-59: empty credentials hardcoded
googleProvider := oauth.NewGoogleProvider("", "", "")
githubProvider := oauth.NewGitHubProvider("", "", "")
```

### Giải pháp

Theo `01-architecture.md §3.4`: identity-service quản lý OAuth providers. Credentials phải đến từ environment variables, không hardcoded.

#### Bước 1: Sửa embedded.go để đọc từ env vars

**File sửa**: `services/identity-service/embedded.go:L58-66`

```go
// FIX MOCK-011: đọc OAuth credentials từ environment variables
googleClientID     := os.Getenv("GOOGLE_CLIENT_ID")
googleClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
googleRedirectURL  := os.Getenv("GOOGLE_REDIRECT_URL")
if googleRedirectURL == "" {
    googleRedirectURL = "http://localhost:8080/api/v1/auth/callback?provider=google"
}

githubClientID     := os.Getenv("GITHUB_CLIENT_ID")
githubClientSecret := os.Getenv("GITHUB_CLIENT_SECRET")
githubRedirectURL  := os.Getenv("GITHUB_REDIRECT_URL")
if githubRedirectURL == "" {
    githubRedirectURL = "http://localhost:8080/api/v1/auth/callback?provider=github"
}

googleProvider := oauth.NewGoogleProvider(googleClientID, googleClientSecret, googleRedirectURL)
githubProvider := oauth.NewGitHubProvider(githubClientID, githubClientSecret, githubRedirectURL)

// QUAN TRỌNG: Nếu credentials rỗng, log warning nhưng vẫn khởi tạo
// OAuth handlers sẽ trả lỗi rõ ràng khi người dùng thử dùng
if googleClientID == "" {
    logger.Warn().Msg("identity-service: GOOGLE_CLIENT_ID not set, Google OAuth disabled")
}
if githubClientID == "" {
    logger.Warn().Msg("identity-service: GITHUB_CLIENT_ID not set, GitHub OAuth disabled")
}
```

#### Bước 2: Sửa OAuth provider để trả lỗi rõ ràng khi credentials rỗng

**File sửa**: `services/identity-service/internal/infrastructure/oauth/google.go`

```go
// OAuthRedirect — handle GET /api/v1/auth/oauth/google
func (p *GoogleProvider) OAuthRedirect(w http.ResponseWriter, r *http.Request) {
    // FIX MOCK-011: check credentials trước khi redirect
    if p.clientID == "" || p.clientSecret == "" {
        http.Error(w,
            `{"error":"oauth_not_configured","detail":"Google OAuth credentials not set"}`,
            http.StatusServiceUnavailable)
        return
    }
    // ... redirect logic giữ nguyên
}
```

#### Bước 3: Cập nhật environment documentation

**File mới**: `services/identity-service/.env.example`

```bash
# OAuth — Google
GOOGLE_CLIENT_ID=your-google-client-id.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=GOCSPX-your-secret
GOOGLE_REDIRECT_URL=https://your-domain.com/api/v1/auth/callback?provider=google

# OAuth — GitHub
GITHUB_CLIENT_ID=Ov23li...
GITHUB_CLIENT_SECRET=your-github-secret
GITHUB_REDIRECT_URL=https://your-domain.com/api/v1/auth/callback?provider=github

# JWT
JWT_PRIVATE_KEY_PATH=/run/secrets/jwt_private_key
```

#### Bước 4: Cập nhật docker-compose.yml để inject env vars

```yaml
# deploy/dev/docker-compose.yml
services:
  osv-monolith:
    environment:
      # OAuth (optional — chỉ cần nếu dùng social login)
      - GOOGLE_CLIENT_ID=${GOOGLE_CLIENT_ID:-}
      - GOOGLE_CLIENT_SECRET=${GOOGLE_CLIENT_SECRET:-}
      - GOOGLE_REDIRECT_URL=http://localhost:8080/api/v1/auth/callback?provider=google
      - GITHUB_CLIENT_ID=${GITHUB_CLIENT_ID:-}
      - GITHUB_CLIENT_SECRET=${GITHUB_CLIENT_SECRET:-}
      - GITHUB_REDIRECT_URL=http://localhost:8080/api/v1/auth/callback?provider=github
```

### Kiểm tra hoạt động
```bash
# Test khi không có credentials
curl http://localhost:8080/api/v1/auth/oauth/google
# Expected: {"error":"oauth_not_configured","detail":"..."}

# Test khi có credentials
export GOOGLE_CLIENT_ID=real-id GOOGLE_CLIENT_SECRET=real-secret
# Restart service → redirect hoạt động bình thường
```
