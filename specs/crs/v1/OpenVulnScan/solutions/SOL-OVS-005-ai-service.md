# SOL-OVS-005 — Giải Pháp: AI Service (CVE Enrichment, EPSS, Triage)

| Trường | Giá trị |
|--------|---------|
| **Solution ID** | SOL-OVS-005 |
| **CR tham chiếu** | CR-OVS-005 |
| **Tiêu đề** | AI Service — CVE Embedding Generation (pgvector), LLM Severity Classification, EPSS Integration, AI-assisted Finding Triage |
| **Ngày tạo** | 2026-06-16 |
| **Ngày implement** | 2026-06-17 |
| **Trạng thái** | ✅ Implemented |

---

## 0. Implementation Status

> **Trạng thái**: ✅ **IMPLEMENTED** — 2026-06-17

| Task | File | Trạng thái |
|------|------|------------|
| T-AI-001 | `ai-service/internal/provider/` (Ollama + OpenAI + Chain) | ✅ Done |
| T-AI-002 | `ai-service/internal/domain/embedding/service.go` | ✅ Done |
| T-AI-003 | `ai-service/internal/domain/severity/classifier.go` | ✅ Done |
| T-AI-004 | `ai-service/internal/domain/epss/client.go` | ✅ Done |
| T-AI-005 | `ai-service/internal/domain/triage/service.go` | ✅ Done |
| T-AI-006 | `ai-service/internal/usecase/enrich/usecase.go` | ✅ Done |

**Chi tiết implementation**:
- **Provider Chain**: Ollama → OpenAI failover, `ProviderChain.Generate/GenerateEmbedding()` với lastErr tracking
- **pgvector Storage**: HNSW index `vector_cosine_ops`, little-endian float32 binary Redis cache (7-day TTL)
- **Severity Classifier**: CVSS_V3 → CVSS_V2 (deterministic) → LLM fallback, confidence 0.0-1.0
- **EPSS Client**: FIRST.org API v1, batch support (comma-separated), cache đến midnight UTC
- **AI Triage**: Structured prompt → JSON extraction (resilient markdown parsing), `Confirmed/FalsePositive/NotAffected/Unexplored`
- **EnrichCVE**: 4 parallel goroutines (embedding, severity, EPSS, MITRE) với `sync.WaitGroup`, 30s timeout

---

## 1. Tổng Quan Giải Pháp

### 1.1 Bối Cảnh

OSV.dev cung cấp structured CVE data (CVSS scores, affected packages). `ai-service` bổ sung **intelligence layer** trên top of đó:
- **Semantic search** qua vector embeddings (pgvector)  
- **Severity classification** khi CVE chưa có CVSS
- **Exploit probability** (EPSS từ FIRST.org)
- **AI triage** recommendations cho security analysts

### 1.2 Design Decisions

| Decision | Rationale |
|----------|-----------|
| **Ollama → OpenAI failover** | Local inference ưu tiên cho privacy; cloud fallback khi load cao |
| **pgvector** | Postgres extension; tích hợp tự nhiên với existing DB stack |
| **Redis embedding cache (7 days)** | Embedding rất tốn kém; CVE text hiếm khi thay đổi |
| **CVSS-first severity** | Deterministic > LLM; LLM chỉ dùng khi không có CVSS |
| **EPSS cache đến midnight UTC** | FIRST.org cập nhật daily; cache đến end-of-day |

---

## 2. Kiến Trúc

### 2.1 Provider Chain

```
EnrichCVE request
      │
      ├── Embedding Generation:
      │   Ollama (/api/embeddings) ─fail→ OpenAI (text-embedding-3-small)
      │
      ├── Severity Classification:
      │   CVSS_V3 (deterministic) → CVSS_V2 → LLM (Ollama → OpenAI)
      │
      ├── EPSS:
      │   Redis cache → FIRST.org API (https://api.first.org/data/v1/epss)
      │
      └── MITRE Tagging:
          LLM prompt → CVE → CAPEC IDs
```

### 2.2 Cấu Trúc Thư Mục

```
services/ai-service/
├── cmd/server/main.go
├── internal/
│   ├── domain/
│   │   └── enrichment/
│   │       ├── embedding_service.go    # Vector embedding logic
│   │       ├── severity_classifier.go  # CVSS-first + LLM fallback
│   │       ├── exploit_detector.go     # CVE exploit detection
│   │       ├── mitre_tagger.go        # CAPEC tagging
│   │       └── port/
│   │           ├── embedding_provider.go
│   │           └── llm_provider.go
│   ├── usecase/
│   │   ├── enrich_cve/
│   │   │   └── usecase.go             # Main enrichment (parallel steps)
│   │   ├── epss/
│   │   │   └── usecase.go
│   │   └── triage_finding/
│   │       └── usecase.go
│   ├── adapter/
│   │   ├── llm/
│   │   │   ├── ollama/provider.go
│   │   │   ├── openai/provider.go
│   │   │   └── azure/provider.go
│   │   ├── cache/redis/
│   │   │   ├── embedding_cache.go
│   │   │   └── epss_cache.go
│   │   └── messaging/nats/
│   │       ├── cve_synced_consumer.go
│   │       └── publisher.go
│   └── delivery/
│       ├── grpc/
│       │   └── ai_server.go
│       └── http/
│           └── ai_handler.go           # Debug/manual trigger endpoints
├── migrations/
│   └── 001_create_enrichment_tables.sql
└── config/config.yaml
```

---

## 3. Vector Embedding Design

### 3.1 pgvector Integration

```sql
-- migrations/001_create_enrichment_tables.sql

-- Install pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- CVE embeddings table
CREATE TABLE cve_embeddings (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cve_id      VARCHAR(30) UNIQUE NOT NULL,   -- "CVE-2021-44228"
    embedding   vector(1536),                   -- OpenAI text-embedding-3-small
    model       VARCHAR(100),                   -- "text-embedding-3-small" or "llama3"
    dims        INT,                            -- actual embedding dimensions
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- HNSW index for approximate nearest neighbor search
-- Cho cosine similarity search (semantic CVE search)
CREATE INDEX cve_embeddings_hnsw ON cve_embeddings 
    USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);

-- CVE enrichment results
CREATE TABLE cve_enrichments (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cve_id              VARCHAR(30) UNIQUE NOT NULL,
    severity            VARCHAR(10),            -- CRITICAL|HIGH|MEDIUM|LOW
    severity_confidence FLOAT,                  -- 0.0-1.0
    severity_source     VARCHAR(20),            -- cvss_v3|cvss_v2|llm
    severity_reasoning  TEXT,
    epss_score          FLOAT,                  -- 0.0-1.0
    epss_percentile     FLOAT,
    epss_date           DATE,
    is_exploited        BOOLEAN DEFAULT FALSE,
    exploit_db_ids      TEXT[] DEFAULT '{}',
    is_in_cisa_kev      BOOLEAN DEFAULT FALSE,
    mitre_tags          JSONB DEFAULT '[]',     -- [{capec_id, name, likelihood}]
    threat_groups       TEXT[] DEFAULT '{}',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_enrichments_cve  ON cve_enrichments(cve_id);
CREATE INDEX idx_enrichments_epss ON cve_enrichments(epss_score DESC);
```

### 3.2 Embedding Cache Strategy

```go
// ai-service/internal/adapter/cache/redis/embedding_cache.go

const (
    embedCachePrefix = "osv:embed:"
    embedCacheTTL    = 7 * 24 * time.Hour  // 7 days
)

// Encoding: little-endian float32 bytes (efficient binary format)
func encodeFloat32Slice(values []float32) []byte {
    buf := make([]byte, len(values)*4)
    for i, v := range values {
        binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
    }
    return buf
}

func decodeFloat32Slice(data []byte) []float32 {
    result := make([]float32, len(data)/4)
    for i := range result {
        bits := binary.LittleEndian.Uint32(data[i*4:])
        result[i] = math.Float32frombits(bits)
    }
    return result
}

type EmbeddingCache struct {
    client *redis.Client
}

func (c *EmbeddingCache) Get(ctx context.Context, cveID string) ([]float32, bool) {
    data, err := c.client.Get(ctx, embedCachePrefix+cveID).Bytes()
    if err != nil { return nil, false }
    return decodeFloat32Slice(data), true
}

func (c *EmbeddingCache) Set(ctx context.Context, cveID string, embedding []float32) {
    c.client.Set(ctx, embedCachePrefix+cveID, encodeFloat32Slice(embedding), embedCacheTTL)
}
```

---

## 4. EnrichCVE — Parallel Execution

```go
// ai-service/internal/usecase/enrich_cve/usecase.go

func (uc *EnrichCVEUseCase) Execute(ctx context.Context, in EnrichCVEInput) (*EnrichCVEOutput, error) {
    out := &EnrichCVEOutput{}
    var wg sync.WaitGroup
    var mu sync.Mutex
    
    // Context with timeout for all parallel tasks
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()
    
    // 4 parallel enrichment goroutines:
    
    // 1. Embedding (may be cached)
    wg.Add(1)
    go func() {
        defer wg.Done()
        emb, err := uc.embeddingService.GenerateForVuln(ctx, in.CVEID, in.Summary, in.Details)
        if err != nil {
            // Non-fatal: log and continue
            uc.logger.Warn().Err(err).Str("cve", in.CVEID).Msg("embedding failed")
            return
        }
        mu.Lock()
        out.Embedding = emb
        mu.Unlock()
    }()
    
    // 2. Severity classification
    wg.Add(1)
    go func() {
        defer wg.Done()
        pred, err := uc.severityClassifier.Classify(ctx, in.Summary, in.Details, in.ExistingCVSS)
        if err != nil {
            mu.Lock()
            out.Severity = defaultSeverityPrediction()  // MEDIUM, confidence=0
            mu.Unlock()
            return
        }
        mu.Lock()
        out.Severity = *pred
        mu.Unlock()
    }()
    
    // 3. EPSS query (non-blocking)
    wg.Add(1)
    go func() {
        defer wg.Done()
        epss, _ := uc.epssUC.GetEPSS(ctx, in.CVEID)
        if epss != nil {
            mu.Lock()
            out.ThreatIntel.EPSSScore = &epss.Score
            out.ThreatIntel.EPSSPctile = &epss.Percentile
            mu.Unlock()
        }
    }()
    
    // 4. MITRE tagging (LLM)
    wg.Add(1)
    go func() {
        defer wg.Done()
        tags, _ := uc.mitreTagger.Tag(ctx, in.Summary, in.Details)
        mu.Lock()
        out.MITRETags = tags
        mu.Unlock()
    }()
    
    wg.Wait()
    
    // Persist enrichment results
    uc.enrichmentRepo.Upsert(ctx, &CVEEnrichment{
        CVEID:             in.CVEID,
        Severity:          out.Severity.Severity,
        SeveritySource:    out.Severity.Source,
        SeverityConfidence: out.Severity.Confidence,
        EPSSScore:         out.ThreatIntel.EPSSScore,
        MITRETags:         out.MITRETags,
    })
    
    // Store embedding to pgvector
    if len(out.Embedding) > 0 {
        uc.vectorRepo.Upsert(ctx, in.CVEID, out.Embedding)
    }
    
    // Notify downstream
    uc.eventBus.Publish(ctx, "ai.cve.enriched", &CVEEnrichedEvent{
        CVEID:         in.CVEID,
        EmbeddingDims: len(out.Embedding),
        Severity:      string(out.Severity.Severity),
        HasExploit:    out.ExploitInfo.IsPubliclyExploited,
        EPSSScore:     out.ThreatIntel.EPSSScore,
    })
    
    return out, nil
}
```

---

## 5. EPSS Integration

### 5.1 FIRST.org API

```go
// FIRST.org EPSS API v1
// GET https://api.first.org/data/v1/epss?cve=CVE-2021-44228
// Response:
// {
//   "status": "OK",
//   "data": [{
//     "cve": "CVE-2021-44228",
//     "epss": "0.97530",       // 97.5% probability of exploitation in 30 days
//     "percentile": "1.00000", // top 1% of all CVEs by exploit likelihood
//     "date": "2026-06-15"
//   }]
// }

// Bulk query (up to 100 CVEs per request)
// GET https://api.first.org/data/v1/epss?cve=CVE-A,CVE-B,CVE-C
```

### 5.2 Cache Until Midnight UTC

```go
func timeUntilMidnightUTC() time.Duration {
    now := time.Now().UTC()
    tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
    return tomorrow.Sub(now)
}

// EPSS cache key: "epss:{cve_id}"
// TTL: seconds until midnight UTC
```

---

## 6. Ollama Provider Implementation

```go
// ai-service/internal/adapter/llm/ollama/provider.go

type OllamaProvider struct {
    baseURL string  // "http://ollama:11434"
    model   string  // "llama3", "mistral", "nomic-embed-text"
    client  *http.Client
    logger  zerolog.Logger
}

func NewOllamaProvider(baseURL, model string) *OllamaProvider {
    return &OllamaProvider{
        baseURL: baseURL,
        model:   model,
        client: &http.Client{
            Timeout: 60 * time.Second,
        },
    }
}

func (p *OllamaProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
    payload := map[string]string{
        "model":  p.model,
        "prompt": text,
    }
    
    respBody, err := p.post(ctx, "/api/embeddings", payload)
    if err != nil { return nil, err }
    
    var result struct {
        Embedding []float32 `json:"embedding"`
    }
    if err := json.Unmarshal(respBody, &result); err != nil {
        return nil, fmt.Errorf("parse embedding response: %w", err)
    }
    
    return result.Embedding, nil
}

func (p *OllamaProvider) Generate(ctx context.Context, prompt string) (string, error) {
    payload := map[string]interface{}{
        "model":  p.model,
        "prompt": prompt,
        "stream": false,
        "format": "json",  // Request structured JSON output
        "options": map[string]interface{}{
            "temperature": 0.1,   // Low temperature for consistent output
            "top_p":       0.9,
        },
    }
    
    respBody, err := p.post(ctx, "/api/generate", payload)
    if err != nil { return "", err }
    
    var result struct {
        Response string `json:"response"`
    }
    if err := json.Unmarshal(respBody, &result); err != nil {
        return "", fmt.Errorf("parse generate response: %w", err)
    }
    
    return result.Response, nil
}

func (p *OllamaProvider) Name() string { return "ollama" }
func (p *OllamaProvider) Dimensions() int { return 4096 }  // llama3 default
```

---

## 7. Triage Finding — LLM Design

### 7.1 Prompt Engineering

```go
// Structured prompt for consistent JSON output
func buildTriagePrompt(in TriageInput) string {
    return fmt.Sprintf(`You are an expert security analyst. Analyze this vulnerability finding and recommend the appropriate triage status.

PRODUCT CONTEXT:
%s

FINDING:
- Title: %s
- CVE: %s  
- Severity: %s
- CVSS Score: %.1f
- Description: %s

TRIAGE OPTIONS:
- Confirmed: The vulnerability is real and affects this product
- FalsePositive: The tool incorrectly flagged this (explain why)
- NotAffected: The CVE exists but this specific product/version is not vulnerable
- Unexplored: Need more information to determine

IMPORTANT: Respond ONLY with valid JSON. No explanation outside JSON.

{
  "remarks": "Confirmed|FalsePositive|NotAffected|Unexplored",
  "confidence": 0.0,
  "justification": "brief explanation (max 200 chars)",
  "actions": ["remediation step 1", "remediation step 2"]
}`,
        truncate(in.Context, 300),
        in.Title,
        in.CVE,
        in.Severity,
        in.CVSSScore,
        truncate(in.Description, 1000),
    )
}
```

### 7.2 JSON Extraction (Resilient Parsing)

```go
// LLMs sometimes wrap JSON in markdown code blocks
func extractJSON(response string) string {
    // Try extract from ```json ... ``` blocks
    re := regexp.MustCompile("(?s)```(?:json)?\\s*({.*?})\\s*```")
    if matches := re.FindStringSubmatch(response); len(matches) > 1 {
        return matches[1]
    }
    
    // Try find raw JSON object
    start := strings.Index(response, "{")
    end := strings.LastIndex(response, "}")
    if start >= 0 && end > start {
        return response[start : end+1]
    }
    
    return response
}
```

---

## 8. NATS Integration

### 8.1 CVE Synced Consumer

```go
// ai-service/internal/adapter/messaging/nats/cve_synced_consumer.go
// Listens: ingestion.cve.synced

type CVESyncedConsumer struct {
    enrichCVEUC *enrich_cve.UseCase
    logger      zerolog.Logger
}

func (c *CVESyncedConsumer) Handle(ctx context.Context, msg *CVESyncedEvent) error {
    _, err := c.enrichCVEUC.Execute(ctx, enrich_cve.Input{
        CVEID:        msg.CVEID,
        Summary:      msg.Summary,
        Details:      msg.Details,
        ExistingCVSS: msg.Severity, // [{score: 10.0, type: "CVSS_V3"}]
    })
    return err
}

// ai-service/internal/adapter/messaging/nats/scan_completed_consumer.go
// Listens: scan.scan.completed → batch enrich all CVEs from scan

func (c *ScanCompletedConsumer) Handle(ctx context.Context, msg *ScanCompletedEvent) error {
    // Get all CVE IDs from the scan
    cveIDs, err := c.scanSvcClient.GetScanCVEIDs(ctx, msg.ScanID)
    if err != nil { return err }
    
    // Bulk enrich (limited concurrency to avoid overwhelming LLM)
    sem := semaphore.NewWeighted(5)  // 5 concurrent enrichments
    var wg sync.WaitGroup
    
    for _, cveID := range cveIDs {
        wg.Add(1)
        go func(id string) {
            defer wg.Done()
            sem.Acquire(ctx, 1)
            defer sem.Release(1)
            
            c.enrichCVEUC.Execute(ctx, enrich_cve.Input{CVEID: id})
        }(cveID)
    }
    
    wg.Wait()
    return nil
}
```

---

## 9. Configuration

```yaml
# config/config.yaml
server:
  grpc_port: 50052
  http_port: 8052

database:
  host: "${DB_HOST}"
  port: 5432
  name: "ai_service"
  user: "${DB_USER}"
  password: "${DB_PASSWORD}"

redis:
  url: "${REDIS_URL}"

ai:
  # Primary backend
  backend: "ollama"  # ollama|openai|azure
  
  ollama:
    base_url: "http://ollama:11434"
    model: "llama3"                    # LLM model
    embedding_model: "nomic-embed-text"  # Embedding model (768 dims)
    timeout: "60s"
    
  openai:
    api_key: "${OPENAI_API_KEY}"
    base_url: "https://api.openai.com/v1"
    model: "gpt-4o-mini"
    embedding_model: "text-embedding-3-small"  # 1536 dims
    timeout: "30s"
    
  azure:
    api_key: "${AZURE_OPENAI_API_KEY}"
    endpoint: "${AZURE_OPENAI_ENDPOINT}"
    deployment: "gpt-4o"
    api_version: "2024-02-01"

# Provider chain: try in order
provider_chain:
  llm:
    - ollama
    - openai
  embedding:
    - ollama    # For offline deployments
    - openai    # Fallback with better quality

embedding:
  cache_ttl: "168h"       # 7 days
  max_text_length: 8000   # chars to send to API
  vector_dims: 1536       # Match OpenAI dims; pad if Ollama smaller

epss:
  api_url: "https://api.first.org/data/v1/epss"
  timeout: "10s"
  cache_until_midnight: true

nats:
  url: "${NATS_URL}"
  subscriptions:
    - subject: "ingestion.cve.synced"
      queue_group: "ai-service-enrichment"
    - subject: "scan.scan.completed"
      queue_group: "ai-service-triage"

enrichment:
  max_concurrent: 5  # Concurrent CVE enrichments per batch

logging:
  level: "info"
  format: "json"
```

---

## 10. Prometheus Metrics

```go
// ai-service/internal/metrics/

var (
    EmbeddingsGenerated = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "ai_embeddings_generated_total"},
        []string{"cached"},  // cached: true|false
    )
    SeverityClassifications = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "ai_severity_classifications_total"},
        []string{"source"},  // source: cvss_v3|cvss_v2|llm
    )
    EPSSFetches = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "ai_epss_fetches_total"},
        []string{"cache_hit"},  // cache_hit: true|false
    )
    TriageRecommendations = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "ai_triage_recommendations_total"},
        []string{"remarks"},  // Confirmed|FalsePositive|NotAffected|Unexplored
    )
    LLMRequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "ai_llm_request_duration_seconds",
            Buckets: []float64{1, 5, 10, 30, 60},
        },
        []string{"provider"},  // ollama|openai|azure
    )
    ProviderErrors = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "ai_provider_errors_total"},
        []string{"provider"},
    )
)
```

---

## 11. Semantic Search Endpoint

```go
// ai-service/internal/delivery/http/ai_handler.go

// GET /api/v1/ai/semantic-search?q=remote+code+execution&limit=10
func (h *Handler) SemanticSearch(w http.ResponseWriter, r *http.Request) {
    query := r.URL.Query().Get("q")
    limit := parseIntDefault(r.URL.Query().Get("limit"), 10)
    
    // 1. Generate query embedding
    embedding, err := h.embeddingService.GenerateForVuln(r.Context(), "", query, "")
    if err != nil {
        http.Error(w, "embedding failed", 500)
        return
    }
    
    // 2. Cosine similarity search with pgvector
    // SQL: SELECT cve_id, 1 - (embedding <=> $1) AS similarity
    //      FROM cve_embeddings
    //      ORDER BY embedding <=> $1
    //      LIMIT $2
    results, _ := h.vectorRepo.CosineSimilaritySearch(r.Context(), embedding, limit)
    
    json.NewEncoder(w).Encode(results)
}
```

---

## 12. Implementation Roadmap

### Phase 1 — Core Infrastructure (Sprint 1)
- [ ] Database migrations (pgvector extension, tables)
- [ ] Redis embedding cache
- [ ] Ollama provider implementation
- [ ] OpenAI provider implementation
- [ ] Provider chain (failover logic)

### Phase 2 — Enrichment Use Cases (Sprint 2)
- [ ] EmbeddingService (generate + cache)
- [ ] SeverityClassifier (CVSS-first + LLM fallback)
- [ ] EnrichCVE use case (parallel execution)
- [ ] EPSS integration + cache
- [ ] NATS consumers

### Phase 3 — Advanced Features (Sprint 3)
- [ ] MITRE tagger
- [ ] Triage finding use case
- [ ] pgvector semantic search
- [ ] gRPC server

### Phase 4 — Monitoring (Sprint 4)
- [ ] Prometheus metrics
- [ ] Provider error tracking
- [ ] Integration tests

---

## 13. Acceptance Criteria Mapping

| Criterion | Implementation |
|-----------|---------------|
| CVSS_V3 → deterministic severity, source="cvss_v3" | `SeverityClassifier.Classify()` priority 1 |
| No CVSS → LLM fallback, confidence 0.0-1.0 | `SeverityClassifier.Classify()` priority 3 |
| Embedding cached 7 days: `osv:embed:{cve_id}` | `EmbeddingCache.Set()` with 7d TTL |
| Cache hit → no API call | `EmbeddingCache.Get()` check first |
| Ollama failure → OpenAI failover | `ProviderChain.Generate()` |
| `GetEPSS("CVE-2021-44228")` → FIRST.org | `EPSSUseCase.GetEPSS()` |
| EPSS cached đến midnight UTC | `timeUntilMidnightUTC()` |
| BulkGetEPSS 100 CVEs → 1 batch call | Comma-separated CVE list in URL |
| `TriageFinding` → LLM prompt → JSON | `TriageFindingUseCase.Execute()` |
| NATS `ingestion.cve.synced` → EnrichCVE | `CVESyncedConsumer.Handle()` |
| NATS `ai.cve.enriched` → downstream | `eventBus.Publish("ai.cve.enriched", ...)` |
| Embedding dims: 1536 (OpenAI), var (Ollama) | `EmbeddingProvider.Dimensions()` |
| 2 providers fail → return error | `ProviderChain`: `lastErr` returned after all fail |
