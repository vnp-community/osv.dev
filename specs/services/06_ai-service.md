# ai-service

**Bounded Context**: AI/ML Enrichment & Intelligence
**Go Module**: `github.com/osv/ai-service`

---

## Merge từ

| Source | Trạng thái |
|--------|-----------|
| `services/ai-service` | ✅ Active — base chính (đã đầy đủ) |
| `archive/ai-enrichment` | 📦 Archive — merged |
| `archive/ai` | 📦 Archive — merged |
| `archive/ranking-service` | 📦 Archive — merged (EPSS/risk scoring) |

---

## Chức năng

| # | Chức năng | Mô tả |
|---|-----------|-------|
| 1 | **CVE Enrichment** | Tự động làm giàu CVE với context, impact, remediation guidance |
| 2 | **EPSS Scoring** | Tính và cập nhật EPSS (Exploit Prediction Scoring System) |
| 3 | **MITRE ATT&CK Tagging** | Gắn tag MITRE ATT&CK techniques/tactics cho CVE |
| 4 | **Severity Classification** | ML-based severity re-classification vượt qua CVSS |
| 5 | **Exploit Detection** | Phát hiện PoC exploits, Metasploit modules |
| 6 | **Threat Intelligence** | Correlate CVE với threat actors, campaigns |
| 7 | **Vector Embedding** | Tạo embeddings cho CVE để semantic search |
| 8 | **Finding Triage** | AI-assisted prioritization và triage cho findings |
| 9 | **Provider Chain** | Chain nhiều AI providers với fallback logic |
| 10 | **Batch Processing** | Async bulk enrichment cho CVE mới |

---

## Clean Architecture Layout

```
ai-service/
├── cmd/
│   └── server/
│       └── main.go
│
├── internal/
│   ├── domain/                         # ← Business rules
│   │   ├── enrichment/
│   │   │   ├── provider_chain.go       # Chain of AI providers (fallback)
│   │   │   ├── embedding_service.go    # Vector embedding domain service
│   │   │   ├── severity_classifier.go  # ML severity classification domain
│   │   │   ├── exploit/
│   │   │   │   ├── detector.go         # Exploit detection interface
│   │   │   │   └── entity.go           # ExploitInfo entity
│   │   │   ├── mitretagger/
│   │   │   │   ├── tagger.go           # MITRE tagging interface
│   │   │   │   └── entity.go           # MITRETag value object
│   │   │   ├── threatintel/
│   │   │   │   ├── correlator.go       # Threat intel correlator interface
│   │   │   │   └── entity.go           # ThreatIntelRecord entity
│   │   │   └── port/
│   │   │       ├── ai_provider.go      # AI provider interface (port)
│   │   │       ├── embedding_provider.go
│   │   │       └── epss_provider.go
│   │   ├── triage/
│   │   │   ├── entity.go               # TriageRecommendation entity
│   │   │   └── service.go              # Triage domain service
│   │   └── errors/
│   │       └── errors.go
│   │
│   ├── usecase/                        # ← Application use cases
│   │   ├── enrich_cve/
│   │   │   ├── usecase.go              # Enrich single CVE
│   │   │   └── dto.go
│   │   ├── batch_enrich/
│   │   │   └── usecase.go              # Batch async enrichment
│   │   ├── epss/
│   │   │   ├── fetch.go                # Fetch EPSS from API
│   │   │   └── usecase.go
│   │   ├── triage_finding/
│   │   │   ├── usecase.go              # Get triage recommendation for finding
│   │   │   └── dto.go
│   │   └── generate_embedding/
│   │       └── usecase.go              # Create vector embedding
│   │
│   ├── delivery/                       # ← Transport layer
│   │   └── grpc/
│   │       ├── server.go
│   │       └── ai_handler.go           # AIEnrichmentService RPC impl
│   │
│   └── infra/                          # ← External systems
│       ├── firestore/
│       │   └── enrichment_store.go     # Store enrichment results
│       ├── redis/
│       │   └── enrichment_cache.go     # Cache enriched data (TTL 24h)
│       ├── nats/
│       │   └── subscriber.go           # Subscribe data.cve.created
│       └── providers/                  # ← AI provider implementations
│           ├── openai/
│           │   ├── client.go
│           │   ├── embedder.go
│           │   └── enricher.go
│           ├── gemini/
│           │   ├── client.go
│           │   └── enricher.go
│           ├── anthropic/
│           │   └── enricher.go
│           ├── epss/
│           │   └── client.go           # FIRST EPSS API client
│           ├── exploitdb/
│           │   └── client.go           # Exploit-DB checker
│           └── nvd/
│               └── mitre_client.go     # MITRE ATT&CK data
│
├── go.mod
└── Dockerfile
```

---

## Domain Model

### EnrichmentResult
```go
type EnrichmentResult struct {
    CVEID           string
    EnrichedAt      time.Time
    Provider        string              // Which AI provider was used

    // AI-generated fields
    SummaryShort    string              // 1-2 sentence summary
    SummaryLong     string              // Detailed explanation
    ImpactAnalysis  string              // What can be exploited
    RemediationGuide string             // How to fix
    AttackVector    string              // How attack works

    // Structured analysis
    EPSS            EPSSScore
    MITRETags       []MITRETag
    SeverityML      SeverityLevel       // ML-predicted severity
    SeverityConfidence float64          // 0.0-1.0
    ExploitInfo     *ExploitInfo
    ThreatIntel     *ThreatIntelRecord
    Embedding       []float32           // Vector embedding (1536 dims)
}

type EPSSScore struct {
    Score      float64     // 0.0-1.0 probability of exploitation
    Percentile float64
    FetchedAt  time.Time
    Source     string      // FIRST EPSS API
}

type MITRETag struct {
    TechniqueID   string   // T1190
    TechniqueName string   // Exploit Public-Facing Application
    TacticID      string   // TA0001
    TacticName    string   // Initial Access
    Confidence    float64
}

type ExploitInfo struct {
    HasPublicExploit  bool
    MetasploitModule  string
    ExploitDBID       string
    GitHubPoC         []string
    ExploitMaturity   ExploitMaturity  // WEAPONIZED | POC | THEORETICAL | NONE
}
```

### TriageRecommendation
```go
type TriageRecommendation struct {
    FindingID       uuid.UUID
    Priority        int             // 1-10 (10 = most urgent)
    Rationale       string          // Why this priority
    Suggestion      TriageAction    // FIX_NOW | SCHEDULE | MONITOR | ACCEPT
    ContextFactors  []string        // ["asset_critical", "exploit_available", etc.]
    Confidence      float64
}
```

### Provider Chain
```go
// Provider chain tries each provider in order, falls back if error
type ProviderChain struct {
    providers []AIProvider
}

type AIProvider interface {
    EnrichCVE(ctx context.Context, cveID string, raw CVERaw) (*EnrichmentResult, error)
    GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
    HealthCheck(ctx context.Context) error
}
```

---

## API Specification

### gRPC Services (internal)

```protobuf
service AIEnrichmentService {
    // Enrich a single CVE (synchronous)
    rpc EnrichCVE(EnrichCVERequest) returns (EnrichmentResult);

    // Get cached enrichment result
    rpc GetEnrichment(GetEnrichmentRequest) returns (EnrichmentResult);

    // Get EPSS score
    rpc GetEPSS(GetEPSSRequest) returns (EPSSResponse);

    // Get triage recommendation for a finding
    rpc TriageFinding(TriageFindingRequest) returns (TriageRecommendation);

    // Generate embedding vector
    rpc GenerateEmbedding(GenerateEmbeddingRequest) returns (EmbeddingResponse);
}
```

### HTTP Endpoints (minimal, mostly internal)

| Method | Path | Auth | Mô tả |
|--------|------|------|-------|
| `POST` | `/enrich/{cve_id}` | ServiceToken | Trigger enrichment |
| `GET`  | `/enrich/{cve_id}` | JWT | Get enrichment result |
| `GET`  | `/epss/{cve_id}` | JWT | Get EPSS score |
| `POST` | `/triage/finding` | JWT | Get triage recommendation |
| `POST` | `/admin/batch-enrich` | Admin | Trigger batch enrichment |

---

## Event Subscriptions (NATS)

| Subject | Source | Action |
|---------|--------|--------|
| `data.cve.created` | data-service | Auto-enrich new CVE (async) |
| `data.cve.updated` | data-service | Re-enrich if description changed |

---

## Provider Configuration

```yaml
ai:
  providers:
    - name: "openai"
      priority: 1
      model: "gpt-4o"
      api_key: "${OPENAI_API_KEY}"
      timeout: "30s"
    - name: "gemini"
      priority: 2
      model: "gemini-1.5-pro"
      api_key: "${GEMINI_API_KEY}"
    - name: "anthropic"
      priority: 3
      model: "claude-3-5-sonnet"
      api_key: "${ANTHROPIC_API_KEY}"

  embedding:
    provider: "openai"
    model: "text-embedding-3-small"
    dims: 1536

  epss:
    url: "https://api.first.org/data/v1/epss"
    refresh_schedule: "0 1 * * *"    # Daily 01:00

  batch:
    concurrency: 10
    rate_limit: 100    # requests per minute
```

---

## Dependencies

```
cloud.google.com/go/firestore    # Store enrichment results
github.com/redis/go-redis/v9     # Cache
github.com/nats-io/nats.go       # Event subscription
google.golang.org/grpc           # gRPC server
golang.org/x/oauth2              # Google AI auth
github.com/osv/shared/pkg
github.com/osv/shared/proto
```

---

## Configuration

```yaml
server:
  http_port: 8086
  grpc_port: 50056

firestore:
  project_id: "${GCP_PROJECT_ID}"
  collection: "cve_enrichments"

redis:
  addr: "${REDIS_ADDR}"
  db: 3
  enrichment_ttl: "24h"

nats:
  url: "${NATS_URL}"
  consumer: "ai-service"

# AI Provider config (see above)
```
