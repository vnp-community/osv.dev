# Task T03 — Vulnerability Ingestion Service

> **Priority:** P0 | **Phase:** 1 | **Spec:** `specs/services/03-ingestion-service.md`  
> **Depends on:** T00-shared-libs, T12-infrastructure (NATS, Firestore, GCS, Redis)

## Mục Tiêu
Write-optimized service — trung tâm data pipeline. Nhận raw vuln data từ Source Sync, validate, persist, publish domain events.

## Trách Nhiệm
- Consume `SourceChangeDetected` events từ NATS
- Validate OSV schema (JSON Schema)
- Idempotency: hash-based deduplication (Redis)
- Persist: Firestore (read model) + GCS (full JSON blob)
- Publish domain events: `VulnImported`, `VulnUpdated`, `VulnWithdrawn`
- Apply results từ ImpactAnalysis + AIEnrichment (consume events)
- Track import quality: `ImportFinding` records
- `GET /v1experimental/importfindings/{source}` API
- Deletion safety: từ chối nếu >10% records bị xóa

## Không Làm
- Impact analysis, AI enrichment, querying (delegate)

## Cấu Trúc File

```
services/ingestion/
├── cmd/server/main.go
├── internal/
│   ├── domain/
│   │   ├── aggregate/vulnerability/
│   │   │   ├── vulnerability.go      # Write-side aggregate root
│   │   │   ├── events.go
│   │   │   └── vulnerability_test.go
│   │   ├── entity/
│   │   │   ├── source_repository.go
│   │   │   └── import_finding.go
│   │   ├── valueobject/
│   │   │   ├── vuln_id.go
│   │   │   ├── source_ref.go         # SourceName + path
│   │   │   ├── import_finding_type.go
│   │   │   ├── osv_schema_version.go
│   │   │   └── content_hash.go       # SHA256
│   │   ├── event/
│   │   │   ├── vuln_imported.go
│   │   │   ├── vuln_updated.go
│   │   │   ├── vuln_withdrawn.go
│   │   │   └── import_finding_recorded.go
│   │   ├── service/
│   │   │   ├── osv_validator.go      # JSON Schema validation
│   │   │   └── deduplication_service.go
│   │   └── repository/
│   │       ├── vulnerability_writer.go
│   │       ├── source_repository_repo.go
│   │       └── import_finding_repo.go
│   ├── application/
│   │   ├── command/
│   │   │   ├── import_vulnerability/{command,handler,handler_test}.go
│   │   │   ├── withdraw_vulnerability/{command,handler}.go
│   │   │   ├── apply_impact_analysis/{command,handler}.go
│   │   │   ├── apply_ai_metadata/{command,handler}.go
│   │   │   └── record_import_finding/{command,handler}.go
│   │   ├── query/get_import_findings/{query,handler}.go
│   │   └── port/
│   │       ├── event_publisher.go
│   │       ├── impact_analysis_port.go
│   │       └── blob_store_port.go
│   └── infra/
│       ├── persistence/firestore/
│       │   ├── vulnerability_writer.go
│       │   ├── source_repository_repo.go
│       │   └── import_finding_repo.go
│       ├── storage/gcs/vulnerability_blob_store.go
│       ├── messaging/kafka/
│       │   ├── event_publisher.go
│       │   └── consumer/
│       │       ├── source_change_consumer.go
│       │       ├── impact_result_consumer.go
│       │       └── ai_result_consumer.go
│       ├── validation/jsonschema/osv_validator.go
│       ├── idempotency/redis/idempotency_store.go
│       └── client/impact_analysis_client.go
├── interface/
│   ├── grpc/handler/ingestion_handler.go
│   └── http/handler/health_handler.go
└── config/config.go
```

## Vulnerability Aggregate (Core Domain)

```go
// domain/aggregate/vulnerability/vulnerability.go
type VulnerabilityAggregate struct {
    id          valueobject.VulnID
    version     int64          // optimistic concurrency
    schemaVersion string
    summary     string
    details     string
    published   time.Time
    modified    time.Time
    withdrawn   *time.Time
    aliases     []string
    related     []string
    upstream    []string
    affected    []AffectedPackage
    references  []Reference
    severity    []Severity
    
    source      valueobject.SourceRef
    contentHash valueobject.ContentHash
    importedAt  time.Time
    isWithdrawn bool
    aiMetadata  *AIMetadata
    
    events []domain.Event    // pending domain events
}

// Business rules:
func NewFromOSV(raw OSVRecord, source SourceRef) (*VulnerabilityAggregate, error)
// Validates: ID not empty, modified date exists, at least 1 affected (unless withdrawn)
// Appends: VulnImported event

func (v *VulnerabilityAggregate) Update(raw OSVRecord) (bool, error)
// Returns changed=false nếu không có thay đổi có ý nghĩa (bỏ qua timestamp-only)
// Appends: VulnUpdated event nếu changed=true

func (v *VulnerabilityAggregate) Withdraw(reason string) error
// Idempotent (return nil nếu đã withdrawn)
// Appends: VulnWithdrawn event

func (v *VulnerabilityAggregate) ApplyImpactAnalysis(result ImpactResult) error
// Check: result.ContentHash == v.contentHash (staleness guard)
// Update: affected[].Versions, affected[].Ranges
// Appends: VulnUpdated event

func (v *VulnerabilityAggregate) PullEvents() []domain.Event
// Return pending events và clear chúng (transactional outbox pattern)
```

## Domain Events

```go
// domain/event/vuln_imported.go
const TopicVulnImported = "osv.vuln.imported"
type VulnImported struct {
    EventID     string    `json:"event_id"`    // UUID v4
    EventType   string    `json:"event_type"`
    OccurredAt  time.Time `json:"occurred_at"`
    VulnID      string    `json:"vuln_id"`
    Source      string    `json:"source"`
    Ecosystems  []string  `json:"ecosystems"`
    IsNew       bool      `json:"is_new"`
    ContentHash string    `json:"content_hash"`
    SchemaVersion string  `json:"schema_version"`
}
// Consumers: ImpactAnalysis, AIEnrichment, Search, Notification, Alias

const TopicVulnUpdated   = "osv.vuln.updated"
const TopicVulnWithdrawn = "osv.vuln.withdrawn"
```

## Import Command Handler (Main Flow)

```go
// application/command/import_vulnerability/handler.go
type Command struct {
    RawContent      []byte
    ContentHash     string      // SHA256(RawContent)
    Source          SourceRef
    Extension       string      // ".json" | ".yaml"
    SourceTimestamp *time.Time
    SkipHashCheck   bool        // force reimport
}

// Flow:
// 1. Idempotency check via Redis (key = content_hash, TTL 24h)
// 2. Validate OSV schema (JSON Schema validation)
// 3. For each record in file:
//    a. GetByID from Firestore
//    b. If not found → NewFromOSV (new vuln)
//    c. If found → Update (check changes)
//    d. If !changed && !SkipHashCheck → skip
//    e. persistAggregate: Firestore upsert + GCS upload
//    f. Publish PullEvents() to NATS
// 4. Mark ContentHash as processed in Redis
// Non-fatal: schema errors → record ImportFinding, continue
```

## Persist Aggregate

```go
// Two-phase persistence (Firestore first, GCS second)
func (h *Handler) persistAggregate(ctx context.Context, agg *vulnerability.VulnerabilityAggregate) error {
    // 1. Project agg → VulnerabilityReadModel (denormalized)
    // 2. Firestore upsert (main DB)
    // 3. Marshal full JSON
    // 4. GCS upload (if fail → publish GCSRetry event, non-fatal)
}
```

## ImportFinding Types
```go
const (
    FindingInvalidJSON      ImportFindingType = "INVALID_JSON"
    FindingSchemaViolation  ImportFindingType = "SCHEMA_VIOLATION"
    FindingMissingID        ImportFindingType = "MISSING_ID"
    FindingUnknownEcosystem ImportFindingType = "UNKNOWN_ECOSYSTEM"
    FindingProcessingError  ImportFindingType = "PROCESSING_ERROR"
)
// Stored in Firestore: import-findings/{source}/{bug_id}
// Exposed: GET /v1experimental/importfindings/{source}
```

## Deletion Safety
```go
func CheckDeletionSafety(toDeleteCount, totalCount int) error {
    pct := float64(toDeleteCount) / float64(totalCount) * 100
    if pct >= 10.0 { return domain.NewDeletionSafetyError(...) }
    return nil
}
```

## NATS Consumers
```go
// source_change_consumer.go: Subscribe "osv.source.change.>" → ImportVulnerabilityCommand
// impact_result_consumer.go: Subscribe "osv.impact.analysis.completed" → ApplyImpactAnalysisCommand
// ai_result_consumer.go: Subscribe "osv.ai.enrichment.completed" → ApplyAIMetadataCommand
// Durable consumers, explicit ack, MaxDeliver=5, AckWait=30min
```

## Config
```go
type Config struct {
    GRPC       struct { Port int }
    HTTP       struct { Port int }
    Firestore  struct { ProjectID string }
    GCS        struct { Bucket string }
    NATS       struct { URL string; StreamName string }
    Redis      struct { Addr string }
    ImpactSvc  struct { Addr string }
    Telemetry  struct { OTLPEndpoint string }
    Deletion   struct { SafetyThresholdPct float64 }  // Default: 10.0
}
```

## SLO Targets
- Import P50: <500ms/vuln, P99: <5s/vuln
- Import success rate: >99.5%
- Idempotency: 100% (no duplicates)
- Event publication: <100ms

## Checklist Thực Thi

> **Status: ✅ COMPLETED** — 2026-06-01

- [x] Implement `VulnerabilityAggregate` với đầy đủ business rules + unit tests (`vulnerability.go`, `vulnerability_test.go`)
- [x] Implement domain events: VulnImported, VulnUpdated, VulnWithdrawn (`event/events.go`)
- [x] `domain/valueobject`: VulnID, SourceRef, ContentHash, ImportFindingType
- [x] `domain/repository`: VulnerabilityWriter + ImportFindingRepo interfaces
- [x] `domain/entity`: ImportFinding entity
- [x] `application/port`: EventPublisher, BlobStore, IdempotencyStore interfaces
- [x] Implement `import_vulnerability/handler.go` với đầy đủ flow (idempotency → parse → upsert → GCS → publish)
- [x] Implement Firestore vulnerability writer (`infra/persistence/firestore/vulnerability_writer.go`)
- [x] Implement GCS blob store (`infra/storage/gcs/vulnerability_blob_store.go`)
- [x] Implement Redis idempotency store (`infra/idempotency/redis/idempotency_store.go`, SETNX + TTL)
- [x] Implement NATS JetStream event publisher (`infra/messaging/nats/event_publisher.go`)
- [x] Implement NATS consumer: SourceChangeDetected → ImportVulnerabilityCommand
- [x] `cmd/server/main.go`: graceful shutdown, wire all dependencies
- [x] `config/config.yaml` + `Dockerfile` (multi-stage → distroless) + `Makefile`
- [ ] `apply_impact_analysis/handler.go` (sử dụng khi ImpactAnalysis service ready)
- [ ] `apply_ai_metadata/handler.go` (sử dụng khi AI Enrichment service ready)
- [ ] `get_import_findings/handler.go` + HTTP handler
- [ ] Integration tests: full import flow với test containers
