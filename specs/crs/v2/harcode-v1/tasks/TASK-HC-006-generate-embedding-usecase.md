# TASK-HC-006: Implement generate_embedding UseCase thật

**Status:** ✅ DONE  
**Sprint:** 2 | **Ước lượng:** 3 giờ  
**Phụ thuộc:** TASK-HC-005 (AI provider chain cần available)  
**Solution:** [SOL-001](../solutions/SOL-001-ai-generate-embedding.md)  
**Service:** `services/ai-service`

---

## Mô tả

`ai-service/internal/usecase/generate_embedding/usecase.go` là shell rỗng (`return nil, nil`). Cần inject `EmbeddingProvider` và `VectorStore`, implement đầy đủ.

---

## Acceptance Criteria

- [x] `UseCase.Execute(ctx, cveID, text)` gọi provider thật và lưu vào pgvector
- [x] Không còn `return nil, nil` trong usecase
- [x] gRPC endpoint `GenerateEmbedding` trả vector thật (không empty)
- [x] Embedding được persist trong `cve_embeddings` table
- [x] `go build ./...` pass trong `services/ai-service`

---

## Files cần sửa/tạo

| Action | File | Thay đổi |
|--------|------|---------|
| NEW | `services/ai-service/internal/usecase/generate_embedding/ports.go` | Interfaces `EmbeddingProvider` và `VectorStore` |
| NEW | `services/ai-service/internal/usecase/generate_embedding/adapters.go` | `ProviderChainAdapter` |
| MODIFY | `services/ai-service/internal/usecase/generate_embedding/usecase.go` | Inject deps + implement Execute |
| MODIFY | `services/ai-service/embed/server.go` | Wire usecase với pgvector + provider |

---

## Bước thực thi

### 1. Khảo sát provider chain
```bash
grep -n "func.*Chain\|GenerateEmbedding\|Embed" services/ai-service/internal/provider/chain.go
grep -n "GenerateEmbedding\|Embed" services/ai-service/internal/provider/interface.go
```

### 2. Tạo ports.go
**File:** `services/ai-service/internal/usecase/generate_embedding/ports.go`

```go
package generate_embedding

import "context"

// EmbeddingProvider generates vector embeddings for text input.
type EmbeddingProvider interface {
    Embed(ctx context.Context, text string) ([]float32, error)
    ModelName() string
}

// VectorStore persists CVE embedding vectors.
type VectorStore interface {
    Save(ctx context.Context, cveID string, embedding []float32, model string) error
    Get(ctx context.Context, cveID string) ([]float32, error)
}
```

### 3. Tạo adapters.go (nếu provider chain chưa implement Embed trực tiếp)

```bash
# Kiểm tra provider interface
grep -n "Embed\|embed" services/ai-service/internal/provider/interface.go
```

Nếu cần adapter:
**File:** `services/ai-service/internal/usecase/generate_embedding/adapters.go`

```go
package generate_embedding

import (
    "context"
    "fmt"
    "github.com/osv/ai-service/internal/provider"
)

type ProviderAdapter struct {
    p     provider.Provider
    model string
}

func NewProviderAdapter(p provider.Provider, model string) *ProviderAdapter {
    return &ProviderAdapter{p: p, model: model}
}

func (a *ProviderAdapter) Embed(ctx context.Context, text string) ([]float32, error) {
    // Điều chỉnh method name theo provider.Provider interface thực tế
    vec, err := a.p.GenerateEmbedding(ctx, text)
    if err != nil {
        return nil, fmt.Errorf("provider embed: %w", err)
    }
    return vec, nil
}

func (a *ProviderAdapter) ModelName() string { return a.model }
```

### 4. Refactor usecase.go

```go
package generate_embedding

import (
    "context"
    "fmt"
    "github.com/rs/zerolog"
)

type UseCase struct {
    embedder    EmbeddingProvider
    vectorStore VectorStore
    log         zerolog.Logger
}

func New(embedder EmbeddingProvider, vectorStore VectorStore, log zerolog.Logger) *UseCase {
    return &UseCase{embedder: embedder, vectorStore: vectorStore, log: log}
}

func (uc *UseCase) Execute(ctx context.Context, cveID, text string) ([]float32, error) {
    if cveID == "" || text == "" {
        return nil, fmt.Errorf("generate_embedding: cveID and text required")
    }

    // Check cache
    if existing, _ := uc.vectorStore.Get(ctx, cveID); len(existing) > 0 {
        uc.log.Debug().Str("cve_id", cveID).Msg("embedding cache hit")
        return existing, nil
    }

    // Generate
    vec, err := uc.embedder.Embed(ctx, text)
    if err != nil {
        return nil, fmt.Errorf("generate_embedding.Execute embed %s: %w", cveID, err)
    }
    if len(vec) == 0 {
        return nil, fmt.Errorf("generate_embedding.Execute: empty vector for %s", cveID)
    }

    // Persist
    if err := uc.vectorStore.Save(ctx, cveID, vec, uc.embedder.ModelName()); err != nil {
        uc.log.Warn().Err(err).Str("cve_id", cveID).Msg("failed to persist embedding")
    }

    uc.log.Info().Str("cve_id", cveID).Int("dims", len(vec)).Msg("embedding generated")
    return vec, nil
}
```

### 5. Wire trong embed/server.go
```bash
grep -n "generate_embedding\|generateEmbedding\|embeddingUC\|UseCase" \
  services/ai-service/embed/server.go | head -10
```

Wire thêm:
```go
import (
    pgvectorstore "github.com/osv/ai-service/internal/infra/vector"
    genembedding  "github.com/osv/ai-service/internal/usecase/generate_embedding"
)

// Sau khi có pgPool và providerChain:
vectorStore := pgvectorstore.New(pgPool)
providerAdapter := genembedding.NewProviderAdapter(chain.Primary(), cfg.AIModel)
embeddingUC := genembedding.New(providerAdapter, vectorStore, logger)
// Wire vào gRPC handler
```

### 6. Build check
```bash
cd services/ai-service && go build ./...
```

---

## Verification

```bash
# gRPC test (nếu có grpcurl)
grpcurl -plaintext -d '{"cve_id":"CVE-2021-44228","text":"Apache Log4j RCE vulnerability"}' \
  c12.openledger.vn:50053 ai.v1.AIEnrichmentService/GenerateEmbedding
# PASS nếu trả embedding array (không empty)

# Verify trong DB
psql $DATABASE_URL -c "
SELECT cve_id, model, array_length(embedding::float4[], 1) as dims, created_at
FROM cve_embeddings ORDER BY created_at DESC LIMIT 3;
"
# PASS nếu dims > 0
```
