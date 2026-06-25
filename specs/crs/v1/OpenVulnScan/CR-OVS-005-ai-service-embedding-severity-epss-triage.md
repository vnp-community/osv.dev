# CR-OVS-005 — AI Service: CVE Enrichment, Severity Classification, EPSS, Triage Assistant

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-OVS-005 |
| **Tiêu đề** | AI Service — CVE Embedding Generation (pgvector), LLM Severity Classification, EPSS Integration, AI-assisted Finding Triage |
| **Nguồn tham chiếu** | `OpenVulnScan/specs/services/08-ai-service.md` |
| **Target Service** | **MỚI**: `ai-service` (port gRPC: 50052) |
| **Ưu tiên** | 🟡 Medium |
| **Loại** | New Service |
| **Ngày tạo** | 2026-06-14 |
| **Ngày implement** | 2026-06-17 |
| **Trạng thái** | ✅ Implemented |

---

## 0. Implementation Status

> **Trạng thái**: ✅ **IMPLEMENTED** — 2026-06-17

| Task | File | Trạng thái |
|------|------|------------|
| LLM provider chain (Ollama→OpenAI→Vertex) | `ai-service/internal/domain/enrichment/provider_chain.go` | ✅ Done |
| Ollama provider | `ai-service/internal/infra/ai/ollama/ollama_adapter.go` | ✅ Done |
| OpenAI provider | `ai-service/internal/infra/ai/openai/openai_adapter.go` | ✅ Done |
| Google Vertex AI provider | `ai-service/internal/infra/ai/vertex/vertex_adapter.go` | ✅ Done |
| Embedding service | `ai-service/internal/domain/embedding/service.go` | ✅ Done |
| Generate embedding use case | `ai-service/internal/usecase/generate_embedding/usecase.go` | ✅ Done |
| CVE enrichment parallel pipeline | `ai-service/internal/domain/enrichment/entity.go` | ✅ Done |
| Severity classifier (CVSS-first, LLM fallback) | `ai-service/internal/domain/enrichment/severity_classifier.go` | ✅ Done |
| Severity classifier standalone | `ai-service/internal/domain/severity/classifier.go` | ✅ Done |
| Exploit checker | `ai-service/internal/domain/enrichment/exploit/checker.go` | ✅ Done |
| MITRE tagger (CVE→CAPEC) | `ai-service/internal/domain/enrichment/mitretagger/tagger.go` | ✅ Done |
| Threat intel + CWE stage | `ai-service/internal/domain/enrichment/threatintel/` | ✅ Done |
| EPSS client (FIRST.org API) | `ai-service/internal/domain/epss/client.go` | ✅ Done |
| EPSS infra provider | `ai-service/internal/infra/providers/epss/client.go` | ✅ Done |
| EPSS cron job | `ai-service/internal/usecase/epss/job.go` | ✅ Done |
| Triage entity + service | `ai-service/internal/domain/triage/entity.go`, `triage/service.go` | ✅ Done |
| Triage finding use case | `ai-service/internal/usecase/triage_finding/triage_finding.go` | ✅ Done |
| Batch enrichment | `ai-service/internal/usecase/batch_enrich/usecase.go` | ✅ Done |
| Enrich CVE handler | `ai-service/internal/usecase/enrich_cve/handler.go` | ✅ Done |
| pgvector embedding store | `ai-service/internal/infra/vector/pgvector_store.go` | ✅ Done |
| MongoDB enrichment repo | `ai-service/internal/infra/persistence/mongo/enrichment_repo.go` | ✅ Done |
| Firestore enrichment repo | `ai-service/internal/infra/persistence/firestore/enrichment_repo.go` | ✅ Done |
| NATS consumer | `ai-service/internal/infra/messaging/nats/consumer.go` | ✅ Done |
| gRPC AI handler | `ai-service/internal/delivery/grpc/ai_handler.go` | ✅ Done |

**Chi tiết implementation**:
- **Provider Chain**: Ollama (local) → OpenAI → Google Vertex AI; auto-failover khi provider timeout/error
- **Embedding**: 1536-dim vectors lưu trong `pgvector`, Redis cache 7-day TTL
- **Severity Classification**: CVSS available → return exact; CVSS missing → LLM classify từ description
- **EPSS**: query `epss.cyentia.com/epss/api/v1/` daily, cache trong Redis + PostgreSQL
- **Triage**: LLM phân tích (title, description, component) → `Confirmed/FalsePositive/NotAffected` + justification
- **MITRE**: CVE CWE ID → CAPEC IDs, tạo `mitre_tags[]`
- **NATS**: subscribe `scan.finding.discovered`, `ingestion.cve.synced` → trigger enrichment pipeline

---

## 1. Tổng quan

OSV có CVE database, nhưng không có AI enrichment. OpenVulnScan's `ai-service` bổ sung:
- **Vector Embeddings** — generate và cache embeddings cho semantic search (pgvector)
- **Severity Classification** — CVSS-first, LLM fallback khi không có CVSS
- **Provider Chain** — Ollama (local) → OpenAI → Azure OpenAI (failover)
- **EPSS Integration** — query FIRST.org API cho exploit probability
- **Finding Triage** — AI recommendations (Confirmed/FalsePositive/NotAffected)
- **MITRE Tagger** — map CVE → CAPEC attack patterns

---

## 2. Gap Analysis

| Feature | OSV | OpenVulnScan |
|---------|-----|-------------|
| Vector embeddings | ❌ | ✅ pgvector storage + cache |
| Semantic CVE search | ❌ | ✅ cosine similarity |
| LLM severity fallback | ❌ | ✅ Ollama/OpenAI |
| LLM provider chain | ❌ | ✅ failover chain |
| EPSS per-CVE query | ❌ | ✅ FIRST.org API |
| AI finding triage | ❌ | ✅ Confirmed/FP/NotAffected |
| MITRE tagging | ❌ | ✅ CVE → CAPEC |
| Embedding cache (Redis) | ❌ | ✅ 7-day TTL |

---

## 3. Domain Model

### 3.1 Enrichment Domain

```go
// ai-service/internal/domain/enrichment/

// EmbeddingService — generate and cache vector embeddings
type EmbeddingService struct {
    provider    port.EmbeddingProvider
    redisClient *redis.Client
    logger      zerolog.Logger
}

// EmbeddingProvider — interface for different AI backends
type EmbeddingProvider interface {
    GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
    Dimensions() int  // 1536 for OpenAI, variable for Ollama
}

// LLMProvider — interface for structured LLM output
type LLMProvider interface {
    Generate(ctx context.Context, prompt string) (string, error)
    Name() string  // "ollama", "openai", "azure"
}

// ProviderChain — failover chain of LLM providers
type ProviderChain struct {
    providers []LLMProvider
}

func (c *ProviderChain) Generate(ctx context.Context, prompt string) (string, error) {
    var lastErr error
    for _, p := range c.providers {
        result, err := p.Generate(ctx, prompt)
        if err == nil { return result, nil }
        lastErr = err
        // log warning: provider failed, trying next
    }
    return "", fmt.Errorf("all providers failed: %w", lastErr)
}

// SeverityPrediction — result from severity classification
type SeverityPrediction struct {
    Severity   SeverityLevel  // CRITICAL|HIGH|MEDIUM|LOW
    Confidence float32        // 0.0 - 1.0
    Reasoning  string
    Source     string         // "cvss_v3" | "cvss_v2" | "llm"
}

type ExploitInfo struct {
    IsPubliclyExploited bool
    ExploitDBIDs        []string
    LastUpdated         time.Time
}

type MITRETag struct {
    CAPECID     string  // "CAPEC-198"
    Name        string  // "XSS via HTTP Query Strings"
    Likelihood  string  // "High|Medium|Low"
}

type ThreatIntelData struct {
    IsInCISAKEV  bool
    EPSSScore    *float64
    EPSSPctile   *float64
    ThreatGroups []string  // known threat actor groups
    Ransomware   bool
}
```

---

## 4. Use Cases

### 4.1 GenerateEmbedding

```go
// ai-service/internal/domain/enrichment/embedding_service.go

const (
    maxTextLength = 8000    // chars to send to embedding API
    embedCacheTTL = 7 * 24 * time.Hour
)

// GenerateForVuln — generate and cache embedding for a CVE
func (s *EmbeddingService) GenerateForVuln(ctx context.Context, vulnID, summary, details string) ([]float32, error) {
    // 1. Check Redis cache
    cacheKey := "osv:embed:" + vulnID
    if cached, err := s.redisClient.Get(ctx, cacheKey).Bytes(); err == nil {
        return decodeFloat32Slice(cached), nil  // little-endian float32 decoding
    }

    // 2. Prepare text (truncate to API limit)
    text := summary + "\n\n" + details
    if len(text) > maxTextLength {
        text = text[:maxTextLength]
    }

    // 3. Generate embedding
    embedding, err := s.provider.GenerateEmbedding(ctx, text)
    if err != nil { return nil, fmt.Errorf("embedding generation: %w", err) }

    // 4. Cache (little-endian float32 encoding for efficiency)
    s.redisClient.Set(ctx, cacheKey, encodeFloat32Slice(embedding), embedCacheTTL)

    return embedding, nil
}
```

### 4.2 EnrichCVE (Main Use Case)

```go
// ai-service/internal/usecase/enrich_cve/usecase.go
// Triggered by NATS: ingestion.cve.synced, scan.scan.completed

type EnrichCVEInput struct {
    CVEID        string
    Summary      string
    Details      string
    ExistingCVSS []CVSSSeverity  // from NVD: [{score:10.0, type:"CVSS_V3"}]
}

type EnrichCVEOutput struct {
    Embedding   []float32
    Severity    SeverityPrediction
    ExploitInfo ExploitInfo
    MITRETags   []MITRETag
    ThreatIntel ThreatIntelData
}

func (uc *EnrichCVEUseCase) Execute(ctx context.Context, in EnrichCVEInput) (*EnrichCVEOutput, error) {
    out := &EnrichCVEOutput{}
    var wg sync.WaitGroup
    var mu sync.Mutex
    errs := make([]error, 0)

    // Run enrichment steps in parallel
    wg.Add(4)

    // 1. Generate embedding
    go func() {
        defer wg.Done()
        emb, err := uc.embeddingService.GenerateForVuln(ctx, in.CVEID, in.Summary, in.Details)
        mu.Lock(); defer mu.Unlock()
        if err != nil { errs = append(errs, err); return }
        out.Embedding = emb
    }()

    // 2. Severity classification
    go func() {
        defer wg.Done()
        pred, err := uc.severityClassifier.Classify(ctx, in.Summary, in.Details, in.ExistingCVSS)
        mu.Lock(); defer mu.Unlock()
        if err != nil { errs = append(errs, err); return }
        out.Severity = *pred
    }()

    // 3. Exploit detection
    go func() {
        defer wg.Done()
        exploit, err := uc.exploitDetector.Check(ctx, in.CVEID)
        mu.Lock(); defer mu.Unlock()
        if err != nil { errs = append(errs, err); return }
        out.ExploitInfo = *exploit
    }()

    // 4. MITRE tagging
    go func() {
        defer wg.Done()
        tags, err := uc.mitreTagger.Tag(ctx, in.Summary, in.Details)
        mu.Lock(); defer mu.Unlock()
        if err != nil { errs = append(errs, err); return }
        out.MITRETags = tags
    }()

    wg.Wait()

    // 5. Publish enrichment result
    uc.eventBus.Publish(ctx, "ai.cve.enriched", &CVEEnrichedEvent{
        CVEID:        in.CVEID,
        EmbeddingDims: len(out.Embedding),
        Severity:     out.Severity.Severity,
        HasExploit:   out.ExploitInfo.IsPubliclyExploited,
    })

    return out, nil
}
```

### 4.3 Severity Classification

```go
// ai-service/internal/domain/enrichment/severity_classifier.go

type CVSSSeverity struct {
    Score float64
    Type  string  // "CVSS_V3" | "CVSS_V2"
}

func (c *SeverityClassifier) Classify(
    ctx context.Context,
    summary, details string,
    existingCVSS []CVSSSeverity,
) (*SeverityPrediction, error) {

    // Priority 1: CVSS_V3 (deterministic, confidence=1.0)
    for _, cvss := range existingCVSS {
        if cvss.Type == "CVSS_V3" {
            return &SeverityPrediction{
                Severity:   severityFromCVSSv3Score(cvss.Score),
                Confidence: 1.0,
                Reasoning:  fmt.Sprintf("Derived from CVSSv3 score %.1f", cvss.Score),
                Source:     "cvss_v3",
            }, nil
        }
    }

    // Priority 2: CVSS_V2 (deterministic)
    for _, cvss := range existingCVSS {
        if cvss.Type == "CVSS_V2" {
            return &SeverityPrediction{
                Severity:   severityFromCVSSv2Score(cvss.Score),
                Confidence: 0.95,
                Reasoning:  fmt.Sprintf("Derived from CVSSv2 score %.1f", cvss.Score),
                Source:     "cvss_v2",
            }, nil
        }
    }

    // Priority 3: LLM (fallback when no CVSS)
    prompt := fmt.Sprintf(`You are a security expert. Classify the severity of this vulnerability.
Summary: %s
Details: %s

Respond ONLY with valid JSON:
{"severity": "CRITICAL|HIGH|MEDIUM|LOW", "confidence": 0.0-1.0, "reasoning": "brief explanation"}`,
        truncate(summary, 500),
        truncate(details, 1500),
    )

    response, err := c.llmChain.Generate(ctx, prompt)
    if err != nil { return defaultPrediction(), nil }  // Fallback to MEDIUM

    var pred struct {
        Severity   string  `json:"severity"`
        Confidence float32 `json:"confidence"`
        Reasoning  string  `json:"reasoning"`
    }
    if err := json.Unmarshal([]byte(response), &pred); err != nil {
        return defaultPrediction(), nil
    }

    return &SeverityPrediction{
        Severity:   SeverityLevel(pred.Severity),
        Confidence: pred.Confidence,
        Reasoning:  pred.Reasoning,
        Source:     "llm",
    }, nil
}

func severityFromCVSSv3Score(score float64) SeverityLevel {
    switch {
    case score >= 9.0: return SeverityCritical
    case score >= 7.0: return SeverityHigh
    case score >= 4.0: return SeverityMedium
    case score > 0:    return SeverityLow
    default:           return SeverityNone
    }
}
```

### 4.4 GetEPSS

```go
// ai-service/internal/usecase/epss/usecase.go
// FIRST.org EPSS API: https://api.first.org/data/v1/epss

type EPSSScore struct {
    CVEID      string
    Score      float64  // 0.0-1.0 probability of exploitation in 30 days
    Percentile float64  // 0.0-1.0 percentile among all CVEs
    Date       time.Time
}

func (uc *EPSSUseCase) GetEPSS(ctx context.Context, cveID string) (*EPSSScore, error) {
    // 1. Check Redis cache (EPSS changes daily)
    cacheKey := "epss:" + cveID
    if cached, err := uc.redis.Get(ctx, cacheKey).Result(); err == nil {
        var score EPSSScore
        json.Unmarshal([]byte(cached), &score)
        return &score, nil
    }

    // 2. Fetch from FIRST.org API
    url := fmt.Sprintf("https://api.first.org/data/v1/epss?cve=%s", cveID)
    resp, err := uc.httpClient.Get(url)
    if err != nil { return nil, err }
    defer resp.Body.Close()

    var result struct {
        Data []struct {
            CVE        string  `json:"cve"`
            EPSS       string  `json:"epss"`
            Percentile string  `json:"percentile"`
            Date       string  `json:"date"`
        } `json:"data"`
    }
    json.NewDecoder(resp.Body).Decode(&result)

    if len(result.Data) == 0 { return nil, ErrEPSSNotFound }

    score, _ := strconv.ParseFloat(result.Data[0].EPSS, 64)
    pct, _ := strconv.ParseFloat(result.Data[0].Percentile, 64)
    date, _ := time.Parse("2006-01-02", result.Data[0].Date)

    epss := &EPSSScore{
        CVEID:      cveID,
        Score:      score,
        Percentile: pct,
        Date:       date,
    }

    // Cache until midnight UTC (EPSS updates daily)
    ttl := timeUntilMidnightUTC()
    data, _ := json.Marshal(epss)
    uc.redis.Set(ctx, cacheKey, data, ttl)

    return epss, nil
}

// BulkGetEPSS — batch up to 100 CVEs
func (uc *EPSSUseCase) BulkGetEPSS(ctx context.Context, cveIDs []string) (map[string]*EPSSScore, error) {
    // FIRST.org supports comma-separated CVE list
    url := "https://api.first.org/data/v1/epss?cve=" + strings.Join(cveIDs, ",")
    // ... same parsing logic
}
```

### 4.5 TriageFinding

```go
// ai-service/internal/usecase/triage_finding/usecase.go

type TriageInput struct {
    FindingID   uuid.UUID
    Title       string
    Description string
    Severity    string
    CVE         string
    Context     string  // product description, tech stack, environment
}

type TriageRecommendation struct {
    Remarks        string    // "Confirmed"|"FalsePositive"|"NotAffected"|"Unexplored"
    Confidence     float32
    Justification  string
    Actions        []string  // recommended remediation steps
}

func (uc *TriageFindingUseCase) Execute(ctx context.Context, in TriageInput) (*TriageRecommendation, error) {
    prompt := fmt.Sprintf(`You are a security analyst. Based on this vulnerability finding, recommend triage status.

Product context: %s

Finding:
- Title: %s
- CVE: %s
- Severity: %s
- Description: %s

Recommend one of: Confirmed, FalsePositive, NotAffected, Unexplored

Provide JSON only:
{"remarks": "...", "confidence": 0.0-1.0, "justification": "...", "actions": ["step1", "step2"]}`,
        in.Context, in.Title, in.CVE, in.Severity, truncate(in.Description, 1000))

    response, err := uc.llmChain.Generate(ctx, prompt)
    if err != nil { return defaultTriageRec(), nil }

    var rec TriageRecommendation
    if err := json.Unmarshal([]byte(extractJSON(response)), &rec); err != nil {
        return defaultTriageRec(), nil
    }

    // Publish result
    uc.eventBus.Publish(ctx, "ai.triage.completed", &TriageCompletedEvent{
        FindingID:  in.FindingID,
        Remarks:    rec.Remarks,
        Confidence: rec.Confidence,
    })

    return &rec, nil
}
```

---

## 5. Providers

### 5.1 Ollama Provider (Local)

```go
// ai-service/internal/adapter/llm/ollama/provider.go

type OllamaProvider struct {
    baseURL string  // "http://ollama:11434"
    model   string  // "llama3", "mistral", "codellama"
    client  *http.Client
}

func (p *OllamaProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
    payload := map[string]string{"model": p.model, "prompt": text}
    // POST /api/embeddings
    // Response: {"embedding": [0.1, 0.2, ...]}
}

func (p *OllamaProvider) Generate(ctx context.Context, prompt string) (string, error) {
    payload := map[string]interface{}{
        "model":  p.model,
        "prompt": prompt,
        "stream": false,
        "format": "json",
    }
    // POST /api/generate
    // Response: {"response": "{\"severity\": \"HIGH\", ...}"}
}
```

### 5.2 OpenAI Provider

```go
// ai-service/internal/adapter/llm/openai/provider.go

type OpenAIProvider struct {
    apiKey string
    model  string  // "gpt-4o-mini", "gpt-4o"
    embeddingModel string  // "text-embedding-3-small" (1536 dims)
    client *http.Client
}

func (p *OpenAIProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
    // POST https://api.openai.com/v1/embeddings
    // {"model": "text-embedding-3-small", "input": text}
    // Response: {"data": [{"embedding": [...1536 floats...]}]}
}

func (p *OpenAIProvider) Dimensions() int { return 1536 }
```

---

## 6. Configuration

```yaml
# ai-service/config/config.yaml

ai:
  backend: "ollama"          # ollama|openai|azure
  model: "llama3"            # LLM model
  embedding_model: "llama3"  # or text-embedding-3-small for OpenAI

  ollama:
    base_url: "http://ollama:11434"
    timeout: 60s

  openai:
    api_key: "${OPENAI_API_KEY}"
    base_url: "https://api.openai.com/v1"
    model: "gpt-4o-mini"
    embedding_model: "text-embedding-3-small"

  azure:
    api_key: "${AZURE_OPENAI_API_KEY}"
    endpoint: "${AZURE_OPENAI_ENDPOINT}"
    deployment: "gpt-4o"
    api_version: "2024-02-01"

# Provider chain: try in order
provider_chain:
  - ollama
  - openai

embedding:
  cache_ttl: "168h"   # 7 days
  max_text_length: 8000  # chars

nats:
  url: "nats://nats:4222"
  # Subscriptions:
  # - ingestion.cve.synced
  # - scan.scan.completed

redis:
  url: "${REDIS_URL}"
```

---

## 7. NATS Integration

**Subscribed:**
```
ingestion.cve.synced   → trigger EnrichCVE for new/updated CVEs
scan.scan.completed    → trigger batch CVE enrichment for scan results
```

**Published:**
```
ai.cve.enriched        → {cve_id, embedding_dims, severity, has_exploit, epss}
ai.triage.completed    → {finding_id, remarks, confidence}
```

---

## 8. Metrics

```
ai_embeddings_generated_total{cached}        // cached: true|false
ai_severity_classifications_total{source}    // source: cvss_v3|cvss_v2|llm
ai_epss_fetches_total{cache_hit}
ai_triage_recommendations_total{remarks}     // Confirmed|FalsePositive|...
ai_llm_request_duration_seconds{provider}   // Histogram
ai_embedding_cache_hits_total
ai_provider_errors_total{provider}
```

---

## 9. Acceptance Criteria

- [ ] `EnrichCVE` với CVSS_V3 score → severity derived deterministically, source="cvss_v3"
- [ ] `EnrichCVE` không có CVSS → LLM fallback → severity với confidence 0.0-1.0
- [ ] Embedding được cache trong Redis 7 ngày với key `osv:embed:{cve_id}`
- [ ] Redis cache hit → không gọi embedding API (ai_embeddings_generated_total{cached=true} tăng)
- [ ] Ollama provider failure → automatic fallover to OpenAI provider
- [ ] `GetEPSS("CVE-2021-44228")` → FIRST.org API call → return score+percentile
- [ ] EPSS cached đến midnight UTC
- [ ] `BulkGetEPSS` cho 100 CVE IDs → batch API call (không gọi 100 lần riêng lẻ)
- [ ] `TriageFinding` → LLM prompt → parse JSON response → return recommendation
- [ ] NATS `ingestion.cve.synced` → `EnrichCVE` được trigger cho CVE mới
- [ ] NATS `ai.cve.enriched` → vulnerability-service nhận và lưu embedding
- [ ] Embedding dimensions: 1536 (OpenAI), variable (Ollama) — configurable
- [ ] Provider chain: nếu 2 providers đều fail → return error (không silently fail)
