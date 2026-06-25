# CR-HC-001: ai-service — Implement `generate_embedding` UseCase

## Trạng thái: 🔴 Critical — Production Blocker

## Vấn đề
File: `services/ai-service/internal/usecase/generate_embedding/usecase.go`

```go
func (uc *UseCase) Execute(ctx context.Context, cveID, text string) ([]float32, error) {
    // TODO: call embedding provider (OpenAI / Vertex AI)
    // TODO: cache result in Redis
    // TODO: store in Firestore
    return nil, nil  // ← SHELL RỖNG — không làm gì
}
```

UseCase `generate_embedding` hoàn toàn rỗng. Trả về `nil, nil` mà không thực hiện bất kỳ logic nào.
Điều này vi phạm Clean Architecture: UseCase là lớp orchestrate business logic chính.

## Tác động
- Semantic search không có vector → 500 error
- Batch enrich không tạo ra embeddings → dữ liệu AI không được lưu
- `ai-service` không có giá trị thực tế

## Giải pháp

### Architecture đúng:
```
UseCase.Execute(ctx, cveID, text)
  → EmbeddingProvider.Embed(ctx, text) → []float32
  → Redis.Set(cveID, vector, TTL)
  → VectorStore.Upsert(cveID, vector)
  → return vector, nil
```

### Thay đổi cần thực hiện:

#### 1. Inject dependencies vào UseCase
```go
type UseCase struct {
    embedder    EmbeddingProvider   // interface: Embed(ctx, text) ([]float32, error)
    cache       CacheRepository     // interface: Set/Get vector by key
    vectorStore VectorStoreRepository // interface: Upsert(cveID string, vec []float32) error
}

func New(embedder EmbeddingProvider, cache CacheRepository, vs VectorStoreRepository) *UseCase {
    return &UseCase{embedder: embedder, cache: cache, vectorStore: vs}
}
```

#### 2. Implement Execute
```go
func (uc *UseCase) Execute(ctx context.Context, cveID, text string) ([]float32, error) {
    // 1. Check cache
    if cached, err := uc.cache.Get(ctx, "emb:"+cveID); err == nil {
        return cached, nil
    }
    // 2. Generate embedding
    vec, err := uc.embedder.Embed(ctx, text)
    if err != nil {
        return nil, fmt.Errorf("embed: %w", err)
    }
    // 3. Cache in Redis (async ok)
    _ = uc.cache.Set(ctx, "emb:"+cveID, vec, 24*time.Hour)
    // 4. Persist in vector store (pgvector)
    if err := uc.vectorStore.Upsert(ctx, cveID, vec); err != nil {
        return nil, fmt.Errorf("vector store upsert: %w", err)
    }
    return vec, nil
}
```

#### 3. Wire trong embedded.go / wire.go
- `EmbeddingProvider`: `aigrpc.New(os.Getenv("AI_SERVICE_GRPC"))` → Ollama/OpenAI adapter
- `CacheRepository`: Redis adapter
- `VectorStoreRepository`: pgvector adapter

## Files cần thay đổi
- `services/ai-service/internal/usecase/generate_embedding/usecase.go` — implement
- `services/ai-service/internal/usecase/generate_embedding/ports.go` — định nghĩa interfaces
- `services/ai-service/embedded.go` — wire dependencies
- `services/ai-service/internal/infra/vector/pgvector_store.go` — implement VectorStoreRepository
- `services/ai-service/internal/infra/cache/redis_cache.go` — implement CacheRepository [NEW]

## Database
- **pgvector**: table `ai_embeddings(cve_id TEXT, vector VECTOR(768), created_at TIMESTAMPTZ)`
- **Redis**: key `emb:{cveID}`, TTL 24h

## Acceptance Criteria
- [ ] `POST /api/v2/cves/search/semantic` → 200 với kết quả thật
- [ ] `ai_embeddings` table trong PostgreSQL có data sau khi gọi
- [ ] Cache hit trên Redis sau lần gọi đầu tiên
