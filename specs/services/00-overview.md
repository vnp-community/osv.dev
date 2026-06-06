# OSV.dev — New Microservices Architecture Overview

> **Version:** 2.0  
> **Date:** 2026-05-31  
> **Status:** Proposed  
> **Replaces:** Legacy Python monolith + scattered Go tools  
> **Language:** Go (all services)  
> **Pattern:** API Gateway + Domain-Driven Microservices + Clean Architecture

---

## 1. Tầm Nhìn Kiến Trúc Mới

### 1.1 Vấn Đề Với Kiến Trúc Hiện Tại

| Vấn đề | Mô tả |
|--------|-------|
| **Language fragmentation** | Python cho pipeline, Go cho indexer — không nhất quán |
| **Monolithic workers** | `importer.py` (50KB), `worker.py` (33KB) — quá nhiều responsibility |
| **No service boundaries** | Shared NDB models giữa tất cả services → tight coupling |
| **No observability** | Logging cơ bản, không có distributed tracing xuyên suốt |
| **No AI readiness** | Không có luồng xử lý AI/ML built-in |
| **ESP as API Gateway** | Google Cloud Endpoints — vendor lock-in, limited features |
| **No CQRS** | Đọc và ghi trên cùng model — performance bottleneck |
| **No event sourcing** | Khó audit trail và replay khi có lỗi |

### 1.2 Mục Tiêu Kiến Trúc Mới

| Mục tiêu | Cách đạt được |
|----------|--------------|
| **Enterprise Grade** | CQRS, Event Sourcing, Circuit Breaker, Saga Pattern |
| **Production Grade** | SLO/SLA driven, graceful degradation, zero-downtime deploy |
| **AI Ready** | AI enrichment pipeline, embedding support, LLM-friendly APIs |
| **Observable** | OpenTelemetry, distributed tracing, structured logging, metrics |
| **Cloud Agnostic** | Abstracted infra layer, không hard-code GCP specifics |
| **Developer Friendly** | Clean Architecture, clear boundaries, testable |
| **Scalable** | Independent scaling per service, auto-scaling |

---

## 2. Kiến Trúc Tổng Thể Mới

### 2.1 System Topology

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              EXTERNAL CLIENTS                                    │
│    Web Browser    CLI (osv-scanner)    SDK (Go/Python/JS)    Third-party         │
└──────────────────────────────────┬──────────────────────────────────────────────┘
                                   │ HTTPS / gRPC
┌──────────────────────────────────▼──────────────────────────────────────────────┐
│                            API GATEWAY SERVICE                                   │
│  • Authentication & Authorization (API Key, OAuth2, JWT)                         │
│  • Rate Limiting (per client, per endpoint)                                      │
│  • Request routing & load balancing                                              │
│  • Request/Response transformation                                               │
│  • Circuit Breaker                                                               │
│  • OpenTelemetry trace propagation                                               │
│  • gRPC-HTTP transcoding                                                         │
└───┬──────────┬──────────┬──────────┬──────────┬──────────┬────────────────────┘
    │          │          │          │          │          │
    ▼          ▼          ▼          ▼          ▼          ▼
┌───────┐ ┌───────┐ ┌───────┐ ┌───────┐ ┌───────┐ ┌───────┐
│QUERY  │ │INGEST │ │IMPACT │ │VERSION│ │SEARCH │ │WEB    │
│SVC    │ │SVC    │ │SVC    │ │INDEX  │ │SVC    │ │BFF    │
│       │ │       │ │       │ │SVC    │ │       │ │       │
│gRPC   │ │gRPC   │ │gRPC   │ │gRPC   │ │gRPC   │ │HTTP   │
└───┬───┘ └───┬───┘ └───┬───┘ └───┬───┘ └───┬───┘ └───┬───┘
    │         │          │          │          │          │
    └─────────┴──────────┴──────────┴──────────┴──────────┘
                                   │
                    ┌──────────────▼──────────────┐
                    │      MESSAGE BUS (Kafka)     │
                    │  Domain Events & Commands    │
                    └──────────────┬──────────────┘
                                   │
    ┌────────────────┬─────────────┴──────────────┬─────────────────┐
    ▼                ▼                             ▼                 ▼
┌────────┐    ┌────────────┐               ┌──────────┐     ┌──────────────┐
│SOURCE  │    │NOTIFICATION│               │AI ENRICH │     │ALIAS/RELATED │
│SYNC SVC│    │SVC         │               │SVC       │     │SVC           │
└────────┘    └────────────┘               └──────────┘     └──────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────┐
│                  EXTERNAL DATA SOURCES                   │
│  Git Repos │ GCS Buckets │ REST APIs │ OSS-Fuzz │ NVD   │
└─────────────────────────────────────────────────────────┘
```

### 2.2 Data Storage Per Service

```
┌─────────────────────────────────────────────────────────────────────┐
│                         DATA LAYER                                   │
├──────────────┬──────────────┬──────────────┬──────────────────────┤
│ Query Svc    │ Ingest Svc   │ Search Svc   │ Version Index Svc    │
│              │              │              │                       │
│ Spanner /    │ Spanner /    │ Elasticsearch│ Firestore +          │
│ Firestore    │ Firestore    │ / OpenSearch │ BigTable             │
│ (read-heavy) │ (write-heavy)│              │                       │
├──────────────┼──────────────┼──────────────┼──────────────────────┤
│ GCS          │ GCS          │ Redis        │ GCS (file hashes)    │
│ (JSON blobs) │ (raw blobs)  │ (cache)      │                       │
└──────────────┴──────────────┴──────────────┴──────────────────────┘

Shared:
  ├── Kafka (message bus for domain events)
  ├── Redis (distributed cache, rate limiting)
  ├── Prometheus + Grafana (metrics)
  ├── Jaeger / Tempo (distributed tracing)
  └── OpenTelemetry Collector
```

---

## 3. Danh Sách Services

| # | Service | Responsibility | Tech |
|---|---------|---------------|------|
| 01 | **API Gateway** | Auth, routing, rate limiting, observability | Go + gRPC-gateway |
| 02 | **Vulnerability Query Service** | Query vulns by package/version/commit/PURL | Go + gRPC |
| 03 | **Vulnerability Ingestion Service** | Import, validate, enrich, persist vulns | Go + gRPC |
| 04 | **Impact Analysis Service** | Git bisection, version enumeration, affected range | Go + gRPC |
| 05 | **Version Index Service** | File hash indexing, DetermineVersion | Go + gRPC |
| 06 | **Source Sync Service** | Watch & sync external sources (Git/GCS/REST) | Go |
| 07 | **Notification Service** | Pub/Sub events, webhooks, ecosystem bridges | Go |
| 08 | **Search Service** | Full-text search, faceted search | Go + OpenSearch |
| 09 | **Web BFF** | Backend for Frontend (website) | Go + HTTP |
| 10 | **AI Enrichment Service** | CVE classification, severity prediction, embeddings | Go + gRPC |
| 11 | **Alias & Relations Service** | Vuln alias grouping, upstream/downstream | Go + gRPC |

---

## 4. Clean Architecture Per Service

Mỗi service tuân theo **Clean Architecture** (Uncle Bob) với các layer sau:

```
┌────────────────────────────────────────────────┐
│              Interface Layer                    │
│  ┌──────────────┐  ┌──────────────────────────┐│
│  │ gRPC Handler │  │ HTTP Handler / REST       ││
│  │ (transport)  │  │ (transport)               ││
│  └──────┬───────┘  └────────────┬─────────────┘│
└─────────┼───────────────────────┼──────────────┘
          │                       │
┌─────────▼───────────────────────▼──────────────┐
│            Application Layer                    │
│  ┌──────────────────────────────────────────┐  │
│  │            Use Cases / Commands           │  │
│  │  (CQRS: CommandHandler / QueryHandler)   │  │
│  └──────────────────────┬───────────────────┘  │
└─────────────────────────┼──────────────────────┘
                          │
┌─────────────────────────▼──────────────────────┐
│              Domain Layer                       │
│  ┌──────────┐  ┌──────────┐  ┌───────────────┐ │
│  │ Entities │  │  Value   │  │ Domain Events │ │
│  │          │  │ Objects  │  │               │ │
│  └──────────┘  └──────────┘  └───────────────┘ │
│  ┌──────────────────────────────────────────┐  │
│  │          Domain Services                 │  │
│  │  (complex domain logic, aggregates)      │  │
│  └──────────────────────────────────────────┘  │
│  ┌──────────────────────────────────────────┐  │
│  │       Repository Interfaces              │  │
│  │       Port Interfaces (secondary)        │  │
│  └──────────────────────────────────────────┘  │
└────────────────────────────────────────────────┘
          │
┌─────────▼───────────────────────────────────────┐
│           Infrastructure Layer                   │
│  ┌──────────┐  ┌──────────┐  ┌───────────────┐  │
│  │  DB Repo │  │ Messaging│  │ External APIs │  │
│  │  (impl)  │  │ Adapter  │  │ Adapter       │  │
│  └──────────┘  └──────────┘  └───────────────┘  │
│  ┌──────────┐  ┌──────────┐  ┌───────────────┐  │
│  │  Cache   │  │ Storage  │  │ AI/LLM Client │  │
│  │  Adapter │  │ Adapter  │  │ Adapter       │  │
│  └──────────┘  └──────────┘  └───────────────┘  │
└─────────────────────────────────────────────────┘
```

### 4.1 Standard Directory Structure Per Service

```
services/
└── {service-name}/
    ├── cmd/
    │   └── server/
    │       └── main.go              # Entry point
    ├── internal/
    │   ├── domain/
    │   │   ├── entity/              # Domain entities (Vulnerability, Package...)
    │   │   ├── valueobject/         # Value objects (PURL, Version, EcosystemType...)
    │   │   ├── event/               # Domain events (VulnImported, VulnUpdated...)
    │   │   ├── aggregate/           # Aggregate roots
    │   │   ├── service/             # Domain services
    │   │   └── repository/          # Repository interfaces (ports)
    │   ├── application/
    │   │   ├── command/             # Write side (CQRS)
    │   │   │   ├── handler/         # Command handlers
    │   │   │   └── dto/             # Command DTOs
    │   │   ├── query/               # Read side (CQRS)
    │   │   │   ├── handler/         # Query handlers
    │   │   │   └── dto/             # Query DTOs (read models)
    │   │   └── port/                # Secondary ports (messaging, AI, ext APIs)
    │   └── infra/
    │       ├── persistence/
    │       │   ├── firestore/       # Firestore repository impl
    │       │   ├── spanner/         # Spanner repository impl
    │       │   └── postgres/        # PostgreSQL repository impl (local dev)
    │       ├── messaging/
    │       │   ├── kafka/           # Kafka producer/consumer
    │       │   └── pubsub/          # GCP Pub/Sub adapter
    │       ├── cache/
    │       │   └── redis/           # Redis adapter
    │       ├── storage/
    │       │   └── gcs/             # GCS adapter
    │       ├── ai/
    │       │   ├── vertex/          # Vertex AI adapter
    │       │   └── openai/          # OpenAI adapter
    │       └── http/                # External HTTP clients
    ├── interface/
    │   ├── grpc/
    │   │   ├── handler/             # gRPC service implementation
    │   │   ├── middleware/          # gRPC interceptors
    │   │   └── proto/               # .proto files (or shared proto)
    │   └── http/
    │       ├── handler/             # HTTP handlers (REST)
    │       └── middleware/          # HTTP middleware
    ├── config/
    │   ├── config.go                # Config struct
    │   └── config.yaml              # Default config
    ├── pkg/                         # Exported shared packages (within service)
    ├── test/
    │   ├── unit/                    # Unit tests
    │   ├── integration/             # Integration tests
    │   └── e2e/                     # E2E tests
    ├── Dockerfile
    ├── Makefile
    └── go.mod
```

---

## 5. Cross-Cutting Concerns

### 5.1 Observability Stack

```
Every service MUST implement:

1. Structured Logging (zerolog/zap)
   ├── level: debug/info/warn/error
   ├── trace_id (from OpenTelemetry)
   ├── span_id
   ├── service_name
   ├── service_version
   └── domain context (vuln_id, source, ecosystem...)

2. Metrics (Prometheus)
   ├── http_request_duration_seconds (histogram)
   ├── grpc_server_handling_seconds (histogram)
   ├── domain_events_published_total (counter)
   ├── domain_events_consumed_total (counter)
   ├── cache_hit_total / cache_miss_total (counter)
   └── external_api_call_duration_seconds (histogram)

3. Distributed Tracing (OpenTelemetry → Jaeger/Tempo)
   ├── Trace context propagation via gRPC metadata
   ├── HTTP headers: traceparent, tracestate
   └── Span attributes: vuln_id, ecosystem, source...

4. Health Checks
   ├── /health/live  → liveness probe
   ├── /health/ready → readiness probe (DB, cache connectivity)
   └── gRPC: grpc.health.v1.Health
```

### 5.2 Security

```
Authentication:
  ├── External: API Key (header: X-API-Key) or OAuth2 Bearer Token
  ├── Internal: mTLS between services
  └── Service accounts for GCP resources

Authorization:
  ├── RBAC: roles (reader, importer, admin)
  ├── Policy: OPA (Open Policy Agent) - optional
  └── Resource-level: source ownership

Secrets:
  ├── GCP Secret Manager (production)
  ├── HashiCorp Vault (self-hosted option)
  └── Environment variables (local dev only)
```

### 5.3 Resilience Patterns

```
Every service implements:

Circuit Breaker (go-resilience / gobreaker):
  ├── Downstream service calls
  ├── External API calls (Git, GCS, REST sources)
  └── Database calls (with timeout)

Retry with Backoff:
  ├── Exponential backoff + jitter
  ├── Max retries: 3
  └── Non-retryable errors: 400-class

Bulkhead:
  ├── Separate worker pools per operation type
  └── Request isolation

Timeout:
  ├── Per-RPC deadline propagation
  └── Context cancellation throughout
```

### 5.4 Configuration Management

```go
// Standard config struct pattern for all services
type Config struct {
    Server   ServerConfig   `yaml:"server"`
    Database DatabaseConfig `yaml:"database"`
    Cache    CacheConfig    `yaml:"cache"`
    Messaging MessagingConfig `yaml:"messaging"`
    Storage  StorageConfig  `yaml:"storage"`
    AI       AIConfig       `yaml:"ai"`
    Telemetry TelemetryConfig `yaml:"telemetry"`
    Auth     AuthConfig     `yaml:"auth"`
}

// Loaded from (priority order):
// 1. Environment variables (SCREAMING_SNAKE_CASE)
// 2. Config file (config.yaml)
// 3. GCP Secret Manager (sensitive values)
// 4. Default values
```

---

## 6. AI Readiness Design

### 6.1 AI Integration Points

```
┌─────────────────────────────────────────────────────────────────┐
│                    AI INTEGRATION LAYERS                        │
├─────────────────────────────────────────────────────────────────┤
│ Layer 1: Data Enrichment (during ingestion)                     │
│  ├── Auto-classify vulnerability severity                       │
│  ├── Extract affected packages from unstructured CVE text       │
│  ├── Auto-generate PURL from package descriptions               │
│  └── Detect duplicate/alias vulnerabilities                     │
├─────────────────────────────────────────────────────────────────┤
│ Layer 2: Vector Embeddings (for semantic search)                │
│  ├── Embed vuln description, summary, affected packages         │
│  ├── Store in vector DB (Vertex AI Vector Search / Qdrant)      │
│  └── Enable semantic similarity queries                         │
├─────────────────────────────────────────────────────────────────┤
│ Layer 3: LLM-Powered Features                                   │
│  ├── Natural language vulnerability query ("find all SQLi vulns"│
│  ├── Auto-generate human-readable remediation advice            │
│  ├── Vulnerability impact assessment                            │
│  └── Changelog/diff analysis for version ranges                │
├─────────────────────────────────────────────────────────────────┤
│ Layer 4: ML Predictive Features                                 │
│  ├── Predict exploitability (EPSS-like score)                   │
│  ├── Predict time-to-patch based on historical data             │
│  └── Anomaly detection in source data quality                   │
└─────────────────────────────────────────────────────────────────┘
```

### 6.2 AI-Ready Data Model

```go
// Every Vulnerability entity carries AI-ready fields
type Vulnerability struct {
    // ... standard OSV fields ...
    
    AIMetadata AIMetadata `json:"ai_metadata,omitempty"`
}

type AIMetadata struct {
    // Vector embeddings
    DescriptionEmbedding []float32 `json:"description_embedding,omitempty"`
    EmbeddingModel       string    `json:"embedding_model,omitempty"`
    EmbeddingVersion     string    `json:"embedding_version,omitempty"`
    
    // AI-derived signals
    ExploitabilityScore  float32   `json:"exploitability_score,omitempty"`
    SeverityPrediction   string    `json:"severity_prediction,omitempty"`
    PredictionConfidence float32   `json:"prediction_confidence,omitempty"`
    
    // Classification tags
    AttackVectorTags   []string  `json:"attack_vector_tags,omitempty"`
    WeaknessTypes      []string  `json:"weakness_types,omitempty"` // CWE categories
    
    // LLM-generated content
    RemediationAdvice  string    `json:"remediation_advice,omitempty"`
    TechnicalSummary   string    `json:"technical_summary,omitempty"`
    
    // Metadata
    ProcessedAt        time.Time `json:"processed_at,omitempty"`
    ModelVersion       string    `json:"model_version,omitempty"`
}
```

---

## 7. Event-Driven Architecture

### 7.1 Domain Events

```
Topic: osv.vuln.imported
Payload: { vuln_id, source, ecosystem, imported_at, schema_version }
Consumers: Impact Analysis, AI Enrichment, Search Indexer, Notification

Topic: osv.vuln.updated  
Payload: { vuln_id, source, changed_fields[], updated_at, diff_hash }
Consumers: Search Indexer, Notification, Alias Service, AI Enrichment

Topic: osv.vuln.withdrawn
Payload: { vuln_id, withdrawn_at, reason }
Consumers: Search Indexer, Notification, Query cache invalidation

Topic: osv.source.sync.started
Payload: { source_name, source_type, triggered_by, started_at }
Consumers: Notification, Monitoring

Topic: osv.source.sync.completed
Payload: { source_name, new_count, updated_count, deleted_count, duration_ms }
Consumers: Notification, Monitoring, SLA tracking

Topic: osv.impact.analysis.completed
Payload: { vuln_id, commit_count, version_count, has_changes }
Consumers: Ingestion (to update vuln), Notification

Topic: osv.ai.enrichment.completed
Payload: { vuln_id, enrichment_type, model_version, processed_at }
Consumers: Ingestion (to update vuln), Search Indexer

Topic: osv.alias.group.updated
Payload: { group_id, bug_ids[], last_modified }
Consumers: Query (cache invalidation), Notification
```

### 7.2 Saga Pattern for Vulnerability Import

```
VulnerabilityImportSaga:
  
  Step 1: SourceSyncService detects change
          → Emit: SourceChangeDetected{source, path, hash}
  
  Step 2: IngestionService validates & persists
          → Command: ImportVulnerability
          → Emit: VulnImported{vuln_id, source}
          ← On failure: CompensateImport (mark as failed)
  
  Step 3: ImpactAnalysisService enriches (async)
          → Command: AnalyzeImpact
          → Emit: ImpactAnalysisCompleted{vuln_id, has_changes}
          ← On failure: SkipImpactAnalysis (mark as skipped)
  
  Step 4: IngestionService applies impact analysis results
          → Command: ApplyImpactAnalysis
          → Emit: VulnUpdated{vuln_id, changed_fields}
  
  Step 5: AIEnrichmentService enriches (async, non-blocking)
          → Command: EnrichVulnerability
          → Emit: AIEnrichmentCompleted{vuln_id}
  
  Step 6: SearchService indexes
          → Command: IndexVulnerability
          → Emit: VulnIndexed{vuln_id}
  
  Step 7: NotificationService broadcasts
          → Emit downstream events (webhooks, Pub/Sub)
```

---

## 8. Service Communication

### 8.1 Synchronous (gRPC)

```
API Gateway → Query Service     (query vulns)
API Gateway → Version Index     (determine version)
API Gateway → Search Service    (full-text search)
API Gateway → Ingestion Service (admin: manual import)
API Gateway → Web BFF           (website pages)

Ingestion Service → Impact Analysis Service (on-demand)
Web BFF → Query Service
Web BFF → Search Service
```

### 8.2 Asynchronous (Kafka Events)

```
Source Sync → Ingestion (new vuln detected)
Ingestion → Impact Analysis (enrich after import)
Ingestion → AI Enrichment (AI enrichment after import)
Ingestion → Search (index after import)
Ingestion → Notification (broadcast after import)
Impact Analysis → Ingestion (apply results)
AI Enrichment → Ingestion (apply AI metadata)
Alias Service → Ingestion (update aliases)
```

---

## 9. Shared Libraries (`pkg/`)

```
pkg/
├── osvschema/          # OSV Schema types (Go structs)
│   ├── vulnerability.go
│   ├── affected.go
│   └── version.go
├── ecosystem/          # Ecosystem-specific version handling
│   ├── interface.go    # EcosystemHelper interface
│   ├── pypi/
│   ├── go/
│   ├── npm/
│   └── ... 30+ ecosystems
├── purl/               # Package URL parsing
├── semver/             # SemVer normalization
├── osv_proto/          # Shared protobuf definitions
│   └── v1/
├── middleware/         # Shared gRPC/HTTP middleware
│   ├── auth/
│   ├── ratelimit/
│   ├── logging/
│   └── tracing/
├── errors/             # Domain error types
├── pagination/         # Cursor-based pagination
├── testutil/           # Test helpers
└── config/             # Config loading utilities
```

---

## 10. Service Index

| Document | Service | Priority | Status |
|----------|---------|----------|--------|
| [01-api-gateway.md](./01-api-gateway.md) | API Gateway | P0 | ✅ Done |
| [02-vulnerability-query-service.md](./02-vulnerability-query-service.md) | Query Service | P0 | ✅ Done |
| [03-ingestion-service.md](./03-ingestion-service.md) | Ingestion Service | P0 | ✅ Done |
| [04-impact-analysis-service.md](./04-impact-analysis-service.md) | Impact Analysis | P1 | ✅ Done |
| [05-version-index-service.md](./05-version-index-service.md) | Version Index | P1 | ✅ Done |
| [06-source-sync-service.md](./06-source-sync-service.md) | Source Sync | P0 | ✅ Done |
| [07-notification-service.md](./07-notification-service.md) | Notification | P2 | ✅ Done |
| [08-search-service.md](./08-search-service.md) | Search | P1 | ✅ Done |
| [09-web-bff.md](./09-web-bff.md) | Web BFF | P1 | ✅ Done |
| [10-ai-enrichment-service.md](./10-ai-enrichment-service.md) | AI Enrichment | P2 | ✅ Done |
| [11-alias-relations-service.md](./11-alias-relations-service.md) | Alias & Relations | P1 | ✅ Done |
| [12-migration-strategy.md](./12-migration-strategy.md) | Migration Strategy | P0 | ✅ Done |
| [13-infrastructure.md](./13-infrastructure.md) | Infrastructure | P0 | ✅ Done |

---

## 11. Implementation Status

> **Last Updated:** 2026-06-01  
> **Overall Progress:** Core services implemented — production wiring pending

| Service | Domain | Application | Infra | Interface | Tests | Deploy-ready |
|---------|--------|-------------|-------|-----------|-------|-------------|
| T00 Shared Libs | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| T01 API Gateway | ✅ | ✅ | ✅ | ✅ | ⬜ | ⬜ |
| T02 Vuln Query | ✅ | ✅ | 🔶 | ⬜ | ⬜ | ⬜ |
| T03 Ingestion | ✅ | ✅ | ✅ | ⬜ | ✅ | ⬜ |
| T04 Source Sync | ✅ | ✅ | ✅ | ⬜ | ✅ | ⬜ |
| T05 Impact Analysis | ✅ | ✅ | ✅ | ⬜ | ⬜ | ⬜ |
| T06 Version Index | ✅ | ✅ | ⬜ | ⬜ | ✅ | ⬜ |
| T07 Search | ✅ | ✅ | 🔶 | ⬜ | ⬜ | ⬜ |
| T08 Web BFF | ⬜ | ✅ | ⬜ | ✅ | ⬜ | ⬜ |
| T09 Alias Relations | ✅ | ✅ | ✅ | ✅ | ✅ | ⬜ |
| T10 Notification | ✅ | ✅ | ✅ | ✅ | ✅ | ⬜ |
| T11 AI Enrichment | ✅ | ✅ | ✅ | ✅ | ⬜ | ⬜ |
| T12 Infrastructure | — | — | ✅ | — | ⬜ | ⬜ |
| T13 Migration | — | — | ✅ | — | ⬜ | ⬜ |

**Legend:** ✅ Done | 🔶 Partial | ⬜ Pending

### Key Deviations from Original Spec
- **Message Bus:** Using **NATS JetStream** (not Kafka as originally in overview diagrams). All consumers/producers are NATS-based.
- **Storage:** Firestore (not Spanner) is primary write + read store. GCS for JSON blobs.
- **AI Enrichment:** Ollama adapter available for local dev. VertexAI adapter is planned (not yet implemented).
- **gRPC Handlers:** Proto files defined; code-gen handlers pending `protoc` toolchain setup.
