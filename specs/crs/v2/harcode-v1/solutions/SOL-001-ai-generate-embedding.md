# SOL-001: Implement `generate_embedding` UseCase — ai-service

**CR:** CR-HC-001 | **Priority:** 🔴 Critical | **Sprint:** 1  
**Service:** `services/ai-service`

---

## Implementation Status

**✅ IMPLEMENTED** — 2026-06-24
**Task:** TASK-HC-006
**Note:** GenerateEmbeddingUseCase gọi real Ollama/OpenAI HTTP endpoint
**Build:** ✅ `go build ./...` passes

---

---

## Context phân tích code

**File hiện tại** `ai-service/internal/usecase/generate_embedding/usecase.go`:
```go
type UseCase struct{}  // không có dependencies

func (uc *UseCase) Execute(ctx context.Context, cveID, text string) ([]float32, error) {
    return nil, nil  // ← shell rỗng
}
```

**Infrastructure đã có:**
- `ai-service/internal/infra/vector/pgvector_store.go` — `PgVectorStore.Save()` ✅ implemented
- `search-service/internal/infra/aigrpc/embedder.go` — gRPC embedder ✅ implemented  
- `ai-service/internal/provider/` — Ollama/OpenAI provider chain ✅ implemented

**Khoảng trống:** UseCase không inject `PgVectorStore` và không gọi provider chain.

---

## Solution

### Bước 1: Định nghĩa Ports (interfaces) trong domain

**File mới:** `ai-service/internal/usecase/generate_embedding/ports.go`

```go
package generate_embedding

import "context"

// EmbeddingProvider generates vector embeddings for text.
// Implemented by: infra/ai/ollama, infra/ai/openai, infra/ai/vertex
type EmbeddingProvider interface {
    Embed(ctx context.Context, text string) ([]float32, error)
    ModelName() string
}

// VectorStore persists CVE embedding vectors.
// Implemented by: infra/vector/pgvector_store.PgVectorStore
type VectorStore interface {
    Save(ctx context.Context, cveID string, embedding []float32, model string) error
    Get(ctx context.Context, cveID string) ([]float32, error)
}
```

### Bước 2: Refactor UseCase với dependencies

**File sửa:** `ai-service/internal/usecase/generate_embedding/usecase.go`

```go
package generate_embedding

import (
    "context"
    "fmt"

    "github.com/rs/zerolog"
)

// UseCase generates vector embeddings for CVE descriptions and persists them.
type UseCase struct {
    embedder    EmbeddingProvider
    vectorStore VectorStore
    log         zerolog.Logger
}

// New creates a UseCase with required dependencies.
// embedder: AI provider (Ollama/OpenAI chain)
// vectorStore: pgvector storage
func New(embedder EmbeddingProvider, vectorStore VectorStore, log zerolog.Logger) *UseCase {
    return &UseCase{
        embedder:    embedder,
        vectorStore: vectorStore,
        log:         log,
    }
}

// Execute generates and persists an embedding for the given CVE.
// Returns the vector so callers can use it directly.
func (uc *UseCase) Execute(ctx context.Context, cveID, text string) ([]float32, error) {
    if cveID == "" || text == "" {
        return nil, fmt.Errorf("generate_embedding: cveID and text are required")
    }

    // 1. Check if already embedded
    existing, err := uc.vectorStore.Get(ctx, cveID)
    if err == nil && len(existing) > 0 {
        uc.log.Debug().Str("cve_id", cveID).Msg("embedding cache hit from vector store")
        return existing, nil
    }

    // 2. Generate embedding via AI provider chain
    vec, err := uc.embedder.Embed(ctx, text)
    if err != nil {
        return nil, fmt.Errorf("generate_embedding.Execute: embed %s: %w", cveID, err)
    }
    if len(vec) == 0 {
        return nil, fmt.Errorf("generate_embedding.Execute: empty vector returned for %s", cveID)
    }

    // 3. Persist in pgvector
    if err := uc.vectorStore.Save(ctx, cveID, vec, uc.embedder.ModelName()); err != nil {
        // Non-fatal: log but still return the vector
        uc.log.Warn().Err(err).Str("cve_id", cveID).Msg("generate_embedding: failed to persist vector")
    }

    uc.log.Info().
        Str("cve_id", cveID).
        Int("dims", len(vec)).
        Str("model", uc.embedder.ModelName()).
        Msg("embedding generated and saved")

    return vec, nil
}
```

### Bước 3: Adapt EmbeddingProvider từ provider chain hiện có

**File mới:** `ai-service/internal/usecase/generate_embedding/adapters.go`

```go
package generate_embedding

import (
    "context"
    "fmt"

    "github.com/osv/ai-service/internal/provider"
)

// ProviderChainAdapter wraps provider.Chain to implement EmbeddingProvider.
type ProviderChainAdapter struct {
    chain *provider.Chain
    model string
}

func NewProviderChainAdapter(chain *provider.Chain, model string) *ProviderChainAdapter {
    return &ProviderChainAdapter{chain: chain, model: model}
}

func (a *ProviderChainAdapter) Embed(ctx context.Context, text string) ([]float32, error) {
    resp, err := a.chain.GenerateEmbedding(ctx, text)
    if err != nil {
        return nil, fmt.Errorf("provider chain embed: %w", err)
    }
    return resp, nil
}

func (a *ProviderChainAdapter) ModelName() string { return a.model }
```

### Bước 4: Wire trong embed/server.go

**File sửa:** `ai-service/embed/server.go`

```go
// [FIX CR-HC-001] Wire real generate_embedding usecase
import (
    pgvectorstore "github.com/osv/ai-service/internal/infra/vector"
    generateembedding "github.com/osv/ai-service/internal/usecase/generate_embedding"
)

// In runEmbeddedAIService():
if cfg.PostgresDSN != "" {
    pool, err := pgxpool.New(ctx, cfg.PostgresDSN)
    if err == nil {
        vectorStore := pgvectorstore.New(pool)
        providerAdapter := generateembedding.NewProviderChainAdapter(chain, cfg.AIModel)
        embeddingUC := generateembedding.New(providerAdapter, vectorStore, logger)
        
        // Wire into HTTP handler
        aiHTTPHandler.SetEmbeddingUseCase(embeddingUC)
        logger.Info().Msg("ai-service: generate_embedding wired (pgvector)")
    }
}
```

### Bước 5: Provider.Chain cần thêm GenerateEmbedding method

**File kiểm tra:** `ai-service/internal/provider/chain.go`
```bash
grep -n "GenerateEmbedding\|Embed" ai-service/internal/provider/chain.go
```

Nếu chưa có — thêm vào:
```go
func (c *Chain) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
    for _, p := range c.providers {
        vec, err := p.GenerateEmbedding(ctx, text)
        if err == nil && len(vec) > 0 {
            return vec, nil
        }
        c.log.Warn().Err(err).Str("provider", p.Name()).Msg("embedding provider failed, trying next")
    }
    return nil, fmt.Errorf("all embedding providers failed")
}
```

---

## Files cần tạo/sửa

| Action | File |
|--------|------|
| NEW | `ai-service/internal/usecase/generate_embedding/ports.go` |
| NEW | `ai-service/internal/usecase/generate_embedding/adapters.go` |
| MODIFY | `ai-service/internal/usecase/generate_embedding/usecase.go` |
| MODIFY | `ai-service/embed/server.go` |
| CHECK | `ai-service/internal/provider/chain.go` — thêm `GenerateEmbedding` nếu thiếu |

---

## Verification

```bash
# Build check
cd services/ai-service && go build ./...

# Integration test
cd tests/client && python3 test_ai_reports.py -v
```

**Expected:** `POST /api/v2/cves/search/semantic` → 200 với kết quả thật (không 500)
