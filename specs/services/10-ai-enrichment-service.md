# Service 10 — AI Enrichment Service

> **Version:** 1.0 | **Status:** Proposed | **Priority:** P2  
> **Language:** Go  
> **Pattern:** Clean Architecture + AI Pipeline  
> **Communication:** gRPC (sync queries) + NATS (async enrichment)  
> **AI Providers:** Vertex AI (GCP) / OpenAI / Ollama (self-hosted)

---

## 1. Trách Nhiệm

Service xử lý **AI-powered enrichment** cho vulnerabilities. Là thành phần AI-Ready cốt lõi của kiến trúc mới, không có equivalent trong hệ thống cũ.

**Responsibilities:**
- Generate text embeddings cho semantic search
- Auto-classify vulnerability severity (CVSS prediction)
- Extract attack vector tags từ unstructured text (CWE classification)
- Generate technical summary (LLM-based, concise)
- Generate remediation advice (LLM-based)
- Predict exploitability score (EPSS-like)
- Alias detection via embedding similarity
- Natural language vulnerability query translation
- Publish `AIEnrichmentCompleted` events

**AI Layers:**
- **Layer 1**: Embedding generation (fast, batch-processable)
- **Layer 2**: Classification (ML models, structured output)
- **Layer 3**: LLM features (slower, on-demand or async)
- **Layer 4**: Predictive scoring (historical data patterns)

**NOT Responsible for:**
- Storing enriched data directly (Ingestion Service applies results)
- Serving vulnerability data (Query Service)
- Search indexing (Search Service)

---

## 2. Clean Architecture Layers

```
Domain:
  ├── EnrichmentRequest aggregate (vuln + enrichment types requested)
  ├── EnrichmentResult entity (AI-produced metadata)
  ├── AIMetadata value object (embeddings, scores, tags, generated text)
  ├── EmbeddingVector value object
  └── Repository: EnrichmentCacheRepository

Application:
  ├── EnrichVulnerabilityCommand + Handler   (full enrichment pipeline)
  ├── GenerateEmbeddingCommand + Handler     (embedding only)
  ├── ClassifySeverityCommand + Handler      (severity prediction)
  ├── TranslateNLQueryCommand + Handler      (NL → structured query)
  └── GetEnrichmentQuery + Handler

Infrastructure:
  ├── VertexAIAdapter (GCP embeddings + Gemini)
  ├── OpenAIAdapter (GPT-4 alternative)
  ├── OllamaAdapter (self-hosted LLM)
  ├── EmbeddingCacheRedisAdapter
  ├── NATSConsumer + Publisher
  └── FirestoreEnrichmentRepo

Interface:
  ├── gRPC handler (AIEnrichmentService)
  └── NATS consumer (VulnImported events)
```

---

## 3. Directory Structure

```
services/ai-enrichment/
├── cmd/server/main.go
├── internal/
│   ├── domain/
│   │   ├── aggregate/
│   │   │   └── enrichment_request/
│   │   │       ├── enrichment_request.go
│   │   │       └── enrichment_types.go    # EMBEDDING | SEVERITY | TAGS | SUMMARY | ...
│   │   ├── entity/
│   │   │   └── enrichment_result.go
│   │   ├── valueobject/
│   │   │   ├── ai_metadata.go             # Complete AI metadata struct
│   │   │   ├── embedding_vector.go        # []float32 + model info
│   │   │   ├── severity_prediction.go     # Predicted severity + confidence
│   │   │   ├── attack_vector_tags.go      # CWE-based tags
│   │   │   └── enrichment_type.go         # Enum of enrichment operations
│   │   ├── service/
│   │   │   ├── embedding_service.go       # Generate + cache embeddings
│   │   │   ├── severity_classifier.go     # Classify severity from text
│   │   │   ├── tag_extractor.go           # Extract attack vector tags
│   │   │   └── text_generator.go          # LLM-based text generation
│   │   └── repository/
│   │       └── enrichment_cache_repo.go   # Interface
│   ├── application/
│   │   ├── command/
│   │   │   ├── enrich_vulnerability/
│   │   │   │   ├── command.go
│   │   │   │   ├── handler.go
│   │   │   │   └── handler_test.go
│   │   │   ├── generate_embedding/
│   │   │   │   ├── command.go
│   │   │   │   └── handler.go
│   │   │   ├── classify_severity/
│   │   │   │   ├── command.go
│   │   │   │   └── handler.go
│   │   │   └── translate_nl_query/
│   │   │       ├── command.go
│   │   │       └── handler.go
│   │   ├── query/
│   │   │   └── get_enrichment/
│   │   │       ├── query.go
│   │   │       └── handler.go
│   │   └── port/
│   │       ├── llm_provider.go            # Interface: LLM operations
│   │       ├── embedding_provider.go      # Interface: embedding generation
│   │       └── event_publisher.go
│   └── infra/
│       ├── ai/
│       │   ├── vertex/
│       │   │   ├── vertex_embedding_adapter.go   # text-embedding-004
│       │   │   └── vertex_gemini_adapter.go      # Gemini 1.5 Pro
│       │   ├── openai/
│       │   │   ├── openai_embedding_adapter.go   # text-embedding-3-large
│       │   │   └── openai_gpt_adapter.go
│       │   └── ollama/
│       │       └── ollama_adapter.go              # Self-hosted LLM
│       ├── cache/
│       │   └── redis/
│       │       └── embedding_cache.go
│       ├── persistence/
│       │   └── firestore/
│       │       └── enrichment_repo.go
│       └── messaging/
│           └── nats/
│               ├── consumer.go
│               └── publisher.go
├── interface/
│   ├── grpc/
│   │   ├── handler/
│   │   │   └── ai_enrichment_handler.go
│   │   └── proto/
│   │       └── ai_enrichment_service.proto
│   └── http/
│       └── handler/
│           └── health_handler.go
├── config/config.go
├── Dockerfile
└── go.mod
```

---

## 4. Proto Definition

```protobuf
// proto/ai_enrichment_service.proto
syntax = "proto3";
package osv.ai.v1;

service AIEnrichmentService {
  // Get existing enrichment for a vulnerability
  rpc GetEnrichment(GetEnrichmentRequest) returns (AIMetadata);
  
  // Trigger enrichment (async) - returns job ID
  rpc TriggerEnrichment(TriggerEnrichmentRequest) returns (TriggerEnrichmentResponse);
  
  // Translate natural language to structured OSV query
  rpc TranslateQuery(TranslateQueryRequest) returns (TranslateQueryResponse);
  
  // Generate embedding for arbitrary text (for search use)
  rpc GenerateEmbedding(GenerateEmbeddingRequest) returns (GenerateEmbeddingResponse);
}

message AIMetadata {
  // Embeddings
  repeated float description_embedding = 1 [packed = true];
  string embedding_model   = 2;
  string embedding_version = 3;
  
  // Severity prediction
  float  exploitability_score  = 4;   // 0.0-1.0
  string severity_prediction   = 5;   // CRITICAL | HIGH | MEDIUM | LOW
  float  prediction_confidence = 6;   // 0.0-1.0
  
  // Classification tags
  repeated string attack_vector_tags = 7;  // ["network", "unauthenticated", "rce"]
  repeated string weakness_types     = 8;  // ["CWE-89", "CWE-79"]
  
  // LLM-generated content
  string remediation_advice  = 9;
  string technical_summary   = 10;
  
  // Metadata
  string processed_at   = 11;  // RFC3339
  string model_version  = 12;
}

message TriggerEnrichmentRequest {
  string vuln_id           = 1;
  repeated string types    = 2;  // EMBEDDING | SEVERITY | TAGS | SUMMARY | ALL
  bool force               = 3;  // Re-enrich even if cached
}

message TranslateQueryRequest {
  string nl_query  = 1;  // "find all SQL injection vulnerabilities in Python"
}

message TranslateQueryResponse {
  // Structured OSV query derived from NL
  string ecosystem = 1;
  string package   = 2;
  string search    = 3;   // Keywords for search
  repeated string cwe_types = 4;
  repeated string attack_vectors = 5;
  string explanation = 6;  // Human-readable explanation of translation
}
```

---

## 5. AI Provider Interface

```go
// application/port/llm_provider.go
package port

// LLMProvider abstracts LLM operations across providers (Vertex AI, OpenAI, Ollama).
type LLMProvider interface {
    // Generate structured output from a prompt
    GenerateStructured(ctx context.Context, prompt string, schema interface{}) error
    
    // Generate free-form text
    GenerateText(ctx context.Context, prompt string, maxTokens int) (string, error)
    
    // Provider info
    Name() string
    ModelName() string
}

// EmbeddingProvider abstracts embedding generation.
type EmbeddingProvider interface {
    // Generate embedding vector for text
    Embed(ctx context.Context, text string) ([]float32, error)
    
    // Batch embedding (more efficient)
    EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
    
    // Dimension of embedding vectors
    Dimension() int
    
    // Model info
    ModelName() string
}
```

---

## 6. Domain Service — Severity Classifier

```go
// domain/service/severity_classifier.go
package service

// Classify vulnerability severity using a combination of:
// 1. Existing CVSS score (if present) → ground truth
// 2. LLM classification (if no CVSS) → prediction

type SeverityClassifier struct {
    llm    port.LLMProvider
    tracer trace.Tracer
}

type SeverityPrediction struct {
    Severity   string  // CRITICAL | HIGH | MEDIUM | LOW | UNKNOWN
    Confidence float32 // 0.0-1.0
    Reasoning  string  // LLM explanation
}

func (c *SeverityClassifier) Classify(
    ctx context.Context,
    summary string,
    details string,
    existingCVSS []Severity,
) (*SeverityPrediction, error) {
    
    // If CVSS score exists, derive from it (deterministic)
    if len(existingCVSS) > 0 {
        for _, s := range existingCVSS {
            if s.Type == "CVSS_V3" {
                return deriveSeverityFromCVSS(s.Score), nil
            }
        }
    }
    
    // Use LLM for classification
    prompt := buildSeverityPrompt(summary, details)
    
    var result struct {
        Severity   string  `json:"severity"`
        Confidence float32 `json:"confidence"`
        Reasoning  string  `json:"reasoning"`
    }
    
    if err := c.llm.GenerateStructured(ctx, prompt, &result); err != nil {
        return nil, fmt.Errorf("LLM classification failed: %w", err)
    }
    
    return &SeverityPrediction{
        Severity:   result.Severity,
        Confidence: result.Confidence,
        Reasoning:  result.Reasoning,
    }, nil
}

func buildSeverityPrompt(summary, details string) string {
    return fmt.Sprintf(`
You are a security expert. Classify the severity of this vulnerability:

Summary: %s
Details: %s

Respond with JSON: {"severity": "CRITICAL|HIGH|MEDIUM|LOW", "confidence": 0.0-1.0, "reasoning": "..."}
Only respond with valid JSON, no other text.
`, summary, details)
}
```

---

## 7. Embedding Pipeline

```go
// domain/service/embedding_service.go

type EmbeddingService struct {
    provider port.EmbeddingProvider
    cache    repository.EnrichmentCacheRepository
    tracer   trace.Tracer
}

func (s *EmbeddingService) GenerateForVuln(
    ctx context.Context,
    vulnID string,
    summary string,
    details string,
) ([]float32, error) {
    
    // Cache check (embeddings are deterministic per content)
    cacheKey := fmt.Sprintf("embedding:%s", vulnID)
    if cached, ok := s.cache.GetEmbedding(ctx, cacheKey); ok {
        return cached, nil
    }
    
    // Combine summary + details (truncated to model max tokens)
    text := truncate(summary + "\n\n" + details, 8000)
    
    // Generate embedding
    embedding, err := s.provider.Embed(ctx, text)
    if err != nil {
        return nil, fmt.Errorf("embedding generation: %w", err)
    }
    
    // Cache (long TTL since content rarely changes)
    s.cache.SetEmbedding(ctx, cacheKey, embedding, 7*24*time.Hour)
    
    return embedding, nil
}
```

---

## 8. Natural Language Query Translation

```go
// application/command/translate_nl_query/handler.go

// Example: "find SQL injection vulnerabilities in Python packages"
// → {search: "SQL injection", ecosystem: "PyPI", cwe_types: ["CWE-89"]}

const systemPrompt = `You are an OSV vulnerability database expert.
Convert natural language queries about security vulnerabilities into structured search parameters.
OSV supports: ecosystem names (PyPI, Go, npm, Maven, Cargo...), package names, CVE IDs, CWE types, attack vectors.

Respond only with valid JSON matching this schema:
{
  "ecosystem": "ecosystem name or empty string",
  "package": "package name or empty string",
  "search": "keywords for full-text search",
  "cwe_types": ["CWE-89"],
  "attack_vectors": ["network", "local"],
  "explanation": "human readable explanation"
}`

func (h *Handler) Handle(ctx context.Context, cmd Command) (*Result, error) {
    fullPrompt := systemPrompt + "\n\nUser query: " + cmd.NLQuery
    
    var structured TranslatedQuery
    if err := h.llm.GenerateStructured(ctx, fullPrompt, &structured); err != nil {
        return nil, err
    }
    
    return &Result{Query: structured}, nil
}
```

---

## 9. AI Provider Configuration

```yaml
# config.yaml - AI Provider setup
ai:
  # Primary provider (GCP)
  embedding_provider: vertex_ai
  llm_provider: vertex_ai

  vertex_ai:
    project: my-gcp-project
    location: us-central1
    embedding_model: text-embedding-004    # 768 dimensions
    llm_model: gemini-1.5-flash-001        # Fast model for classification
    llm_pro_model: gemini-1.5-pro-001      # Pro model for summaries

  # Fallback providers
  openai:
    api_key_secret: openai-api-key
    embedding_model: text-embedding-3-large   # 3072 dimensions
    llm_model: gpt-4o-mini

  # Self-hosted option (air-gapped environments)
  ollama:
    base_url: http://ollama:11434
    embedding_model: nomic-embed-text
    llm_model: llama3.1:8b

  # Feature flags
  features:
    embeddings_enabled: true
    severity_classification: true
    tag_extraction: true
    summary_generation: false    # Expensive, disable initially
    nl_query_translation: true
    exploitability_prediction: false  # Requires training data
```

---

## 10. Event Schema

```go
// Outbound: AIEnrichmentCompleted
// Topic: osv.ai.enrichment.completed

type AIEnrichmentCompleted struct {
    EventID     string    `json:"event_id"`
    EventType   string    `json:"event_type"`
    OccurredAt  time.Time `json:"occurred_at"`
    
    VulnID      string    `json:"vuln_id"`
    
    // Enrichment results (may be partial)
    Embedding        []float32 `json:"embedding,omitempty"`
    EmbeddingModel   string    `json:"embedding_model,omitempty"`
    SeverityPred     string    `json:"severity_prediction,omitempty"`
    ExploitScore     float32   `json:"exploitability_score,omitempty"`
    AttackVectorTags []string  `json:"attack_vector_tags,omitempty"`
    WeaknessTypes    []string  `json:"weakness_types,omitempty"`
    RemediationAdvice string   `json:"remediation_advice,omitempty"`
    TechnicalSummary  string   `json:"technical_summary,omitempty"`
    
    // Processing info
    DurationMs    int64  `json:"duration_ms"`
    ModelVersion  string `json:"model_version"`
    EnrichmentTypes []string `json:"enrichment_types"`
}
```

---

## 11. Rate Limiting & Cost Control

```go
// AI calls can be expensive. Implement cost controls:

type AIQuotaManager struct {
    // Per-day budget in USD equivalent
    dailyBudgetUSD float64
    
    // Costs per operation (approximate)
    embeddingCostPer1K float64  // Vertex AI: ~$0.0001/1K tokens
    llmCostPerCall     float64  // Gemini Flash: ~$0.00035/1K tokens
    
    // Rate limits (to prevent runaway costs)
    embeddingRPS  int  // Embeddings per second
    llmRPS        int  // LLM calls per second
}

// Strategies:
// 1. Embeddings: cache aggressively (7-day TTL)
// 2. LLM summaries: generate only for high-CVSS vulns initially
// 3. Batch processing: process low-priority enrichment in off-peak hours
// 4. Feature flags: disable expensive features per source
```

---

## 12. SLO Targets

| Metric | Target |
|--------|--------|
| Availability | 99.5% |
| Embedding generation P50 | < 200ms |
| Embedding batch P50 | < 2s for 100 texts |
| Severity classification P50 | < 3s |
| LLM summary generation P50 | < 10s |
| Enrichment coverage | > 90% of new vulns within 1h |
| NL query translation accuracy | > 85% |
| AI provider failover | < 5s |

---

## 13. Implementation Status

> **Status:** ✅ Core Implemented | **Updated:** 2026-06-01

### Implemented
- [x] `application/port/embedding_provider.go` — EmbeddingProvider + LLMProvider + EventPublisher port interfaces
- [x] `domain/service/embedding_service.go` — EmbeddingService (Redis cache-aside 7d TTL, binary float32 encode/decode via encoding/binary)
- [x] `domain/service/severity_classifier.go` — SeverityClassifier (CVSS score priority → LLM JSON structured output fallback)
- [x] `infra/ai/ollama/ollama_adapter.go` — OllamaAdapter (EmbeddingProvider + LLMProvider, JSON mode, for dev/test)
- [x] `interface/grpc/proto/ai_enrichment_service.proto` — gRPC proto (GetEnrichment, TriggerEnrichment, TranslateQuery, GenerateEmbedding, FindSimilar)
- [x] `go.mod`, `Dockerfile`

### Pending
- [ ] `infra/ai/vertex/vertex_embedding_adapter.go` — Vertex AI text-embedding-004 (production)
- [ ] `infra/ai/vertex/vertex_gemini_adapter.go` — Gemini 1.5 Flash/Pro for classification
- [ ] `domain/service/tag_extractor.go` — CWE tag extraction from vuln text
- [ ] `domain/service/text_generator.go` — LLM-based summary + remediation generation
- [ ] `application/command/enrich_vulnerability/handler.go` — Full enrichment pipeline (embed + classify + extract + generate)
- [ ] `infra/persistence/firestore/enrichment_repo.go` — Firestore EnrichmentRepo
- [ ] `infra/messaging/nats/consumer.go` — VulnImported → enrich async
- [ ] `infra/messaging/nats/publisher.go` — AIEnrichmentCompleted publisher
- [ ] `interface/grpc/handler/ai_enrichment_handler.go` — gRPC handler (FindSimilar, GetEnrichment)
- [ ] `cmd/server/main.go` + `config/config.go`
- [ ] Unit tests (mocked LLM provider), integration tests (Ollama)
- [ ] Makefile

### Deviations from Spec
- EmbeddingService uses `encoding/binary` (not proto encoding) for float32 slice Redis storage
- FindSimilar RPC is in proto but handler not yet implemented
- VertexAI adapters planned; Ollama adapter covers dev/test use case
