# Task T11 — AI Enrichment Service

> **Priority:** P2 | **Phase:** 3 | **Spec:** `specs/services/10-ai-enrichment-service.md`  
> **Depends on:** T00-shared-libs, T12-infrastructure (NATS, Redis, Firestore)

## Mục Tiêu
AI-powered enrichment cho vulnerabilities. Thành phần AI-Ready cốt lõi, không có equivalent trong hệ thống cũ.

## Trách Nhiệm
- Generate text embeddings (Vertex AI text-embedding-004, 768 dims)
- Auto-classify severity (CVSS prediction khi không có CVSS)
- Extract attack vector tags (CWE classification)
- Generate technical summary (LLM: Gemini Flash)
- Generate remediation advice (LLM: Gemini Pro)
- Predict exploitability score (EPSS-like)
- Detect alias via embedding similarity
- Translate natural language queries → structured OSV queries
- Publish `AIEnrichmentCompleted` events

## AI Layers (Ưu Tiên Implement)
1. **Layer 1** (Priority): Embeddings (fast, cacheable)
2. **Layer 2** (Priority): Severity classification
3. **Layer 3** (Later): LLM summaries (expensive)
4. **Layer 4** (Later): Exploitability prediction

## Cấu Trúc File

```
services/ai-enrichment/
├── cmd/server/main.go
├── internal/
│   ├── domain/
│   │   ├── aggregate/enrichment_request/
│   │   │   ├── enrichment_request.go   # {VulnID, Types, VulnData}
│   │   │   └── enrichment_types.go     # EMBEDDING|SEVERITY|TAGS|SUMMARY|ALL
│   │   ├── entity/enrichment_result.go
│   │   ├── valueobject/
│   │   │   ├── ai_metadata.go          # Complete AI metadata struct
│   │   │   ├── embedding_vector.go     # []float32 + model info
│   │   │   ├── severity_prediction.go  # {Severity, Confidence, Reasoning}
│   │   │   ├── attack_vector_tags.go   # CWE-based tags []string
│   │   │   └── enrichment_type.go      # Enum
│   │   ├── service/
│   │   │   ├── embedding_service.go    # Generate + cache embeddings
│   │   │   ├── severity_classifier.go  # CVSS-first, then LLM
│   │   │   ├── tag_extractor.go        # CWE extraction
│   │   │   └── text_generator.go       # LLM text generation
│   │   └── repository/enrichment_cache_repo.go
│   ├── application/
│   │   ├── command/
│   │   │   ├── enrich_vulnerability/{command,handler,handler_test}.go
│   │   │   ├── generate_embedding/{command,handler}.go
│   │   │   ├── classify_severity/{command,handler}.go
│   │   │   └── translate_nl_query/{command,handler}.go
│   │   ├── query/get_enrichment/{query,handler}.go
│   │   └── port/
│   │       ├── llm_provider.go        # Interface: GenerateStructured, GenerateText
│   │       ├── embedding_provider.go  # Interface: Embed, EmbedBatch
│   │       └── event_publisher.go
│   └── infra/
│       ├── ai/
│       │   ├── vertex/
│       │   │   ├── vertex_embedding_adapter.go  # text-embedding-004
│       │   │   └── vertex_gemini_adapter.go     # Gemini Flash/Pro
│       │   ├── openai/
│       │   │   ├── openai_embedding_adapter.go
│       │   │   └── openai_gpt_adapter.go
│       │   └── ollama/ollama_adapter.go          # Self-hosted
│       ├── cache/redis/embedding_cache.go
│       ├── persistence/firestore/enrichment_repo.go
│       └── messaging/nats/
│           ├── consumer.go   # Consume VulnImported
│           └── publisher.go  # Publish AIEnrichmentCompleted
├── interface/
│   ├── grpc/
│   │   ├── handler/ai_enrichment_handler.go
│   │   └── proto/ai_enrichment_service.proto
│   └── http/handler/health_handler.go
└── config/config.go
```

## AI Provider Interfaces

```go
// application/port/llm_provider.go
type LLMProvider interface {
    GenerateStructured(ctx context.Context, prompt string, schema interface{}) error
    GenerateText(ctx context.Context, prompt string, maxTokens int) (string, error)
    Name() string
    ModelName() string
}

// application/port/embedding_provider.go
type EmbeddingProvider interface {
    Embed(ctx context.Context, text string) ([]float32, error)
    EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
    Dimension() int  // 768 for text-embedding-004
    ModelName() string
}
```

## Embedding Service

```go
// domain/service/embedding_service.go
func (s *EmbeddingService) GenerateForVuln(ctx, vulnID, summary, details string) ([]float32, error):
  // 1. Check Redis cache key: "osv:embed:{vulnID}" (TTL 7 days)
  // 2. Combine: text = truncate(summary + "\n\n" + details, 8000 chars)
  // 3. Call EmbeddingProvider.Embed(ctx, text)
  // 4. Cache result
  // 5. Return []float32
```

## Severity Classifier

```go
// domain/service/severity_classifier.go
func (c *SeverityClassifier) Classify(ctx, summary, details string, existingCVSS []Severity) (*SeverityPrediction, error):
  // Priority 1: Derive from existing CVSS score (deterministic, no LLM)
  if has CVSS_V3: return deriveSeverityFromCVSS(score)
  
  // Priority 2: LLM classification
  prompt := fmt.Sprintf(`
You are a security expert. Classify the severity of this vulnerability:
Summary: %s
Details: %s
Respond only with JSON: {"severity": "CRITICAL|HIGH|MEDIUM|LOW", "confidence": 0.0-1.0, "reasoning": "..."}
`, summary, details)
  
  var result struct { Severity string; Confidence float32; Reasoning string }
  llm.GenerateStructured(ctx, prompt, &result)
  return &SeverityPrediction{...}, nil
```

## NL Query Translator

```go
// application/command/translate_nl_query/handler.go
const systemPrompt = `You are an OSV vulnerability database expert.
Convert natural language queries about security vulnerabilities into structured search parameters.
OSV supports: ecosystem names (PyPI, Go, npm, Maven, Cargo...), package names, CVE IDs, CWE types.
Respond only with valid JSON: {"ecosystem":"","package":"","search":"","cwe_types":[],"attack_vectors":[],"explanation":""}`

func Handle(ctx, cmd Command) (*Result, error):
  prompt := systemPrompt + "\n\nUser query: " + cmd.NLQuery
  var translated TranslatedQuery
  llm.GenerateStructured(ctx, prompt, &translated)
  return &Result{Query: translated}, nil
```

## Proto

```protobuf
service AIEnrichmentService {
  rpc GetEnrichment(GetEnrichmentRequest) returns (AIMetadata);
  rpc TriggerEnrichment(TriggerEnrichmentRequest) returns (TriggerEnrichmentResponse);
  rpc TranslateQuery(TranslateQueryRequest) returns (TranslateQueryResponse);
  rpc GenerateEmbedding(GenerateEmbeddingRequest) returns (GenerateEmbeddingResponse);
  rpc FindSimilar(FindSimilarRequest) returns (FindSimilarResponse);  // For Alias Service
}
message AIMetadata {
  repeated float description_embedding = 1 [packed=true];
  string embedding_model = 2; string embedding_version = 3;
  float exploitability_score = 4; string severity_prediction = 5; float prediction_confidence = 6;
  repeated string attack_vector_tags = 7; repeated string weakness_types = 8;
  string remediation_advice = 9; string technical_summary = 10;
  string processed_at = 11; string model_version = 12;
}
message TriggerEnrichmentRequest {
  string vuln_id = 1; repeated string types = 2; bool force = 3;
}
message TranslateQueryRequest { string nl_query = 1; }
message TranslateQueryResponse {
  string ecosystem; string package; string search;
  repeated string cwe_types; repeated string attack_vectors; string explanation;
}
message FindSimilarRequest {
  repeated float embedding = 1 [packed=true];
  int32 top_k = 2; float min_score = 3; string exclude_id = 4;
}
message FindSimilarResponse {
  repeated SimilarVuln results = 1;
}
message SimilarVuln { string vuln_id = 1; float score = 2; }
```

## Config & Feature Flags

```yaml
ai:
  embedding_provider: vertex_ai
  llm_provider: vertex_ai
  vertex_ai:
    project: my-gcp-project
    location: us-central1
    embedding_model: text-embedding-004     # 768 dimensions
    llm_model: gemini-1.5-flash-001
    llm_pro_model: gemini-1.5-pro-001
  openai:
    embedding_model: text-embedding-3-large
    llm_model: gpt-4o-mini
  ollama:
    base_url: http://ollama:11434
    embedding_model: nomic-embed-text
    llm_model: llama3.1:8b
  features:
    embeddings_enabled: true
    severity_classification: true
    tag_extraction: true
    summary_generation: false    # Disable initially (expensive)
    nl_query_translation: true
    exploitability_prediction: false
```

## Cost Controls

```go
type AIQuotaManager struct {
    dailyBudgetUSD     float64
    embeddingCostPer1K float64  // ~$0.0001
    llmCostPerCall     float64  // ~$0.00035
    embeddingRPS       int      // Max embeddings/sec
    llmRPS             int      // Max LLM calls/sec
}
// Strategy: cache aggressively (7d TTL), batch embeddings, process LLM in off-peak
// Feature flags to disable expensive features per source
```

## Events

```go
// Inbound: "osv.vuln.imported" → trigger async enrichment
// Outbound: "osv.ai.enrichment.completed"
type AIEnrichmentCompleted struct {
    EventID          string    `json:"event_id"`
    OccurredAt       time.Time `json:"occurred_at"`
    VulnID           string    `json:"vuln_id"`
    Embedding        []float32 `json:"embedding,omitempty"`
    EmbeddingModel   string    `json:"embedding_model,omitempty"`
    SeverityPred     string    `json:"severity_prediction,omitempty"`
    ExploitScore     float32   `json:"exploitability_score,omitempty"`
    AttackVectorTags []string  `json:"attack_vector_tags,omitempty"`
    WeaknessTypes    []string  `json:"weakness_types,omitempty"`
    RemediationAdvice string   `json:"remediation_advice,omitempty"`
    TechnicalSummary  string   `json:"technical_summary,omitempty"`
    DurationMs       int64     `json:"duration_ms"`
    ModelVersion     string    `json:"model_version"`
}
// Consumers: Ingestion (apply AI metadata), Search (add embedding to index), Alias (similarity)
```

## SLO Targets
- Embedding P50: <200ms, batch 100 texts: <2s
- Severity classification P50: <3s
- LLM summary P50: <10s
- Enrichment coverage: >90% of new vulns within 1h
- AI provider failover: <5s

## Checklist Thực Thi

> **Status: ✅ COMPLETED (Core)** — 2026-06-01

- [x] Implement `EmbeddingProvider` interface (`application/port/embedding_provider.go`)
- [x] Implement `LLMProvider` interface (`application/port/embedding_provider.go`)
- [x] Implement `OllamaAdapter` (development/testing — full embed + LLM, JSON mode)
- [x] Implement `EmbeddingService` (cache-aside Redis 7d TTL, binary encode/decode)
- [x] Implement `SeverityClassifier` (CVSS-first priority → LLM fallback)
- [x] gRPC proto (`ai_enrichment_service.proto`: GetEnrichment, TriggerEnrichment, TranslateQuery, GenerateEmbedding, FindSimilar)
- [x] `go.mod` + `Dockerfile`
- [ ] Implement `VertexAIEmbeddingAdapter` (production)
- [ ] Implement `VertexAIGeminiAdapter` (production LLM)
- [ ] Implement `TagExtractor` (CWE extraction via LLM structured output)
- [ ] Implement `TextGenerator` (summary + remediation)
- [ ] Implement `NLQueryTranslator` (structured prompt → TranslatedQuery)
- [ ] Implement `EnrichVulnerabilityHandler` (orchestrate all enrichments)
- [ ] Implement Firestore `EnrichmentRepo` (cache enrichment results)
- [ ] Implement `GetEnrichmentHandler`
- [ ] Implement NATS consumer (VulnImported → enrich, async)
- [ ] Implement NATS publisher (AIEnrichmentCompleted)
- [ ] Implement `FindSimilar` gRPC handler (for Alias Service)
- [ ] Feature flags per enrichment type
- [ ] Cost quota manager (RPS limits, daily budget)
- [ ] `cmd/server/main.go` + `config/config.go`
- [ ] Unit tests: SeverityClassifier (mock LLM), EmbeddingService (mock provider)
- [ ] Integration tests: Ollama locally
- [ ] Makefile

