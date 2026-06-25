# CR-HC-013: ai-service — batch_enrich UseCase Shell Rỗng

## Trạng thái: 🟠 High

## Vấn đề
File: `services/ai-service/internal/usecase/batch_enrich/usecase.go`

```go
func (uc *BatchEnrichUseCase) Execute(ctx context.Context, cveIDs []string) error {
    for _, cveID := range cveIDs {
        // TODO: call enrich_cve usecase per CVE
        _ = cveID
    }
    return nil
}
```

`batch_enrich` usecase không làm gì — chỉ iterate và bỏ qua.
Đây là usecase quan trọng để:
1. Trigger AI enrichment cho nhiều CVE cùng lúc
2. Cập nhật MITRE ATT&CK mapping, EPSS scores, threat intel
3. Sinh vector embeddings cho semantic search

## Giải pháp

### 1. EnrichCVEUseCase (single CVE enrichment)
```go
type EnrichCVEUseCase struct {
    cveRepo     CVERepository       // read CVE data
    enrichRepo  EnrichmentRepository// save enrichment results
    mitreTagger MITRETagger         // tag MITRE techniques
    epssClient  EPSSClient          // fetch EPSS scores
    embedder    EmbeddingProvider   // generate vectors
    vectorStore VectorStoreRepository
}

func (uc *EnrichCVEUseCase) Execute(ctx context.Context, cveID string) error {
    cve, err := uc.cveRepo.FindByID(ctx, cveID)
    if err != nil {
        return err
    }
    
    // 1. MITRE tagging
    techniques := uc.mitreTagger.Tag(cve.Description)
    
    // 2. EPSS score
    epss, _ := uc.epssClient.GetScore(ctx, cveID)
    
    // 3. Generate embedding
    vec, err := uc.embedder.Embed(ctx, cve.Description)
    if err != nil {
        return fmt.Errorf("embed: %w", err)
    }
    
    // 4. Save all enrichment data
    enrichment := &Enrichment{
        CVEID:      cveID,
        Techniques: techniques,
        EPSSScore:  epss,
        Vector:     vec,
        EnrichedAt: time.Now(),
    }
    return uc.enrichRepo.Upsert(ctx, enrichment)
}
```

### 2. BatchEnrichUseCase với concurrency control
```go
func (uc *BatchEnrichUseCase) Execute(ctx context.Context, cveIDs []string) error {
    sem := make(chan struct{}, 10) // max 10 concurrent enrichments
    var wg sync.WaitGroup
    var mu sync.Mutex
    var errs []error

    for _, cveID := range cveIDs {
        wg.Add(1)
        sem <- struct{}{}
        go func(id string) {
            defer wg.Done()
            defer func() { <-sem }()
            if err := uc.enrichUC.Execute(ctx, id); err != nil {
                mu.Lock()
                errs = append(errs, fmt.Errorf("enrich %s: %w", id, err))
                mu.Unlock()
            }
        }(cveID)
    }
    wg.Wait()
    
    if len(errs) > 0 {
        return errors.Join(errs...)
    }
    return nil
}
```

### 3. Database: ai_enrichments table
```sql
CREATE TABLE IF NOT EXISTS ai_enrichments (
    cve_id           TEXT PRIMARY KEY,
    mitre_techniques JSONB,
    epss_score       NUMERIC(5,4),
    epss_percentile  NUMERIC(5,4),
    vector           VECTOR(768),     -- pgvector
    threat_intel     JSONB,
    enriched_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    enriched_by      VARCHAR(50)       -- 'ollama' | 'openai' | 'vertex'
);
```

### 4. Trigger cơ chế
- **On-demand**: `POST /api/v1/ai/enrich/batch` với list CVE IDs
- **Scheduled**: NATS subscription khi CVE mới được sync
- **Background**: scheduler chạy hàng đêm cho CVEs chưa enriched

## Files cần thay đổi
- `services/ai-service/internal/usecase/batch_enrich/usecase.go` — implement
- `services/ai-service/internal/usecase/enrich_cve/usecase.go` [NEW]
- `services/ai-service/internal/infra/persistence/postgres/enrichment_repo.go` [NEW]
- `services/ai-service/internal/delivery/http/enrich_handler.go` [NEW]
- `services/ai-service/migrations/003_ai_enrichments.sql` [NEW]

## Acceptance Criteria
- [ ] `POST /api/v1/ai/enrich/batch` → enrich danh sách CVEs
- [ ] Enrichment data lưu vào `ai_enrichments` table
- [ ] Concurrency control: max 10 concurrent (configurable)
- [ ] Retry với exponential backoff khi provider fail
- [ ] `GET /api/v1/ai/enrich/{cveID}` → trả enrichment data từ DB
