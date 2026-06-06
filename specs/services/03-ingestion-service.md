# Service 03 — Vulnerability Ingestion Service

> **Version:** 1.0 | **Status:** Proposed | **Priority:** P0  
> **Language:** Go  
> **Pattern:** CQRS (Command side) + Event Sourcing + Saga

---

## 1. Trách Nhiệm

Write-optimized service xử lý toàn bộ **vulnerability write operations**. Là trung tâm của data pipeline — nhận raw vulnerability data, validate, enrich, persist, và publish events.

**Responsibilities:**
- Receive import commands từ Source Sync Service (via Kafka)
- Validate OSV Schema (JSON Schema validation)
- Deduplicate: detect no-change updates
- Persist vulnerability to Firestore + GCS
- Publish domain events (VulnImported, VulnUpdated, VulnWithdrawn)
- Coordinate import saga (trigger impact analysis, AI enrichment)
- Track import quality (ImportFindings)
- Handle deletions/withdrawals
- Expose API: `GET /importfindings/{source}`

**NOT Responsible for:**
- Impact analysis (delegated to Impact Analysis Service)
- AI enrichment (delegated to AI Enrichment Service)
- Querying vulnerabilities (delegated to Query Service)

---

## 2. Clean Architecture Layers

```
Domain:
  ├── Vulnerability aggregate (write model - full)
  ├── SourceRepository entity
  ├── ImportFinding entity
  ├── Domain events: VulnImported, VulnUpdated, VulnWithdrawn
  ├── Domain services: VulnerabilityValidator, DeduplicationService
  └── Repository interfaces

Application (Command side):
  ├── ImportVulnerabilityCommand + Handler
  ├── WithdrawVulnerabilityCommand + Handler
  ├── ApplyImpactAnalysisCommand + Handler (triggered by event)
  ├── ApplyAIMetadataCommand + Handler (triggered by event)
  └── RecordImportFindingCommand + Handler

Infrastructure:
  ├── FirestoreVulnerabilityRepo (write)
  ├── GCSVulnerabilityStore (blob write)
  ├── KafkaEventPublisher (emit domain events)
  ├── KafkaEventConsumer (listen: impact results, AI results)
  ├── ImpactAnalysisGrpcClient (sync call for immediate needs)
  ├── JSONSchemaValidator
  └── RedisIdempotencyStore (dedup)

Interface:
  ├── gRPC handler (admin import, import findings)
  ├── Kafka consumer (source change events)
  └── HTTP health handler
```

---

## 3. Directory Structure

```
services/ingestion/
├── cmd/server/main.go
├── internal/
│   ├── domain/
│   │   ├── aggregate/
│   │   │   └── vulnerability/
│   │   │       ├── vulnerability.go      # Write model aggregate root
│   │   │       ├── vulnerability_test.go
│   │   │       └── events.go             # Domain events for this aggregate
│   │   ├── entity/
│   │   │   ├── source_repository.go
│   │   │   └── import_finding.go
│   │   ├── valueobject/
│   │   │   ├── vuln_id.go
│   │   │   ├── source_ref.go            # SourceName + path
│   │   │   ├── import_finding_type.go   # INVALID_JSON | SCHEMA_VIOLATION | ...
│   │   │   ├── osv_schema_version.go
│   │   │   └── content_hash.go          # SHA256 of raw content
│   │   ├── event/
│   │   │   ├── vuln_imported.go
│   │   │   ├── vuln_updated.go
│   │   │   ├── vuln_withdrawn.go
│   │   │   └── import_finding_recorded.go
│   │   ├── service/
│   │   │   ├── osv_validator.go         # OSV schema validation
│   │   │   └── deduplication_service.go # Changed-field detection
│   │   └── repository/
│   │       ├── vulnerability_writer.go
│   │       ├── source_repository_repo.go
│   │       └── import_finding_repo.go
│   ├── application/
│   │   ├── command/
│   │   │   ├── import_vulnerability/
│   │   │   │   ├── command.go
│   │   │   │   ├── handler.go
│   │   │   │   └── handler_test.go
│   │   │   ├── withdraw_vulnerability/
│   │   │   │   ├── command.go
│   │   │   │   └── handler.go
│   │   │   ├── apply_impact_analysis/
│   │   │   │   ├── command.go
│   │   │   │   └── handler.go
│   │   │   ├── apply_ai_metadata/
│   │   │   │   ├── command.go
│   │   │   │   └── handler.go
│   │   │   └── record_import_finding/
│   │   │       ├── command.go
│   │   │       └── handler.go
│   │   ├── query/
│   │   │   └── get_import_findings/
│   │   │       ├── query.go
│   │   │       └── handler.go
│   │   └── port/
│   │       ├── event_publisher.go       # outbound port
│   │       ├── impact_analysis_port.go  # outbound port
│   │       └── blob_store_port.go       # outbound port
│   └── infra/
│       ├── persistence/
│       │   └── firestore/
│       │       ├── vulnerability_writer.go
│       │       ├── source_repository_repo.go
│       │       └── import_finding_repo.go
│       ├── storage/
│       │   └── gcs/
│       │       └── vulnerability_blob_store.go
│       ├── messaging/
│       │   ├── kafka/
│       │   │   ├── event_publisher.go
│       │   │   └── consumer/
│       │   │       ├── source_change_consumer.go  # listen source changes
│       │   │       ├── impact_result_consumer.go  # listen impact results
│       │   │       └── ai_result_consumer.go      # listen AI results
│       ├── validation/
│       │   └── jsonschema/
│       │       └── osv_validator.go
│       ├── idempotency/
│       │   └── redis/
│       │       └── idempotency_store.go
│       └── client/
│           └── impact_analysis_client.go  # gRPC client
├── interface/
│   ├── grpc/
│   │   └── handler/
│   │       └── ingestion_handler.go
│   └── http/
│       └── handler/
│           └── health_handler.go
└── config/config.go
```

---

## 4. Domain — Vulnerability Aggregate

```go
// domain/aggregate/vulnerability/vulnerability.go
package vulnerability

// VulnerabilityAggregate is the write-side aggregate root.
// All mutations go through this aggregate to ensure business rules.
type VulnerabilityAggregate struct {
    // Identity
    id       valueobject.VulnID
    version  int64 // optimistic concurrency

    // Core data (from OSV schema)
    schemaVersion string
    summary       string
    details       string
    published     time.Time
    modified      time.Time
    withdrawn     *time.Time
    aliases       []string
    related       []string
    upstream      []string
    affected      []AffectedPackage
    references    []Reference
    severity      []Severity
    credits       []Credit
    dbSpecific    map[string]interface{}

    // Tracking
    source       valueobject.SourceRef
    contentHash  valueobject.ContentHash  // SHA256 of raw content
    importedAt   time.Time
    lastModified time.Time
    isWithdrawn  bool

    // AI metadata (applied after import)
    aiMetadata *AIMetadata

    // Pending domain events
    events []domain.Event
}

// Business rules enforced by aggregate:

func NewFromOSV(raw OSVRecord, source valueobject.SourceRef) (*VulnerabilityAggregate, error) {
    // 1. Validate ID not empty
    // 2. Validate modified date exists
    // 3. Validate at least one affected package (unless withdrawn)
    // 4. Check source prefix matches expected prefixes
    // ... business rules ...
    
    agg := &VulnerabilityAggregate{...}
    agg.events = append(agg.events, event.NewVulnImported(agg.id, source))
    return agg, nil
}

func (v *VulnerabilityAggregate) Update(raw OSVRecord) (bool, error) {
    // Detect meaningful changes (ignore timestamp-only changes)
    changed := v.hasSignificantChanges(raw)
    if !changed {
        return false, nil
    }
    
    // Apply update
    v.summary = raw.Summary
    // ...
    v.events = append(v.events, event.NewVulnUpdated(v.id, v.source))
    return true, nil
}

func (v *VulnerabilityAggregate) Withdraw(reason string) error {
    if v.isWithdrawn {
        return nil // idempotent
    }
    now := time.Now().UTC()
    v.withdrawn = &now
    v.isWithdrawn = true
    v.events = append(v.events, event.NewVulnWithdrawn(v.id, reason))
    return nil
}

func (v *VulnerabilityAggregate) ApplyImpactAnalysis(result ImpactResult) error {
    // Only apply if analysis is for current content hash
    if result.ContentHash != v.contentHash {
        return domain.ErrStaleResult
    }
    for i, affected := range v.affected {
        if updatedRange, ok := result.Ranges[affected.Package.Name]; ok {
            v.affected[i].Versions = updatedRange.Versions
            v.affected[i].Ranges = updatedRange.Ranges
        }
    }
    v.events = append(v.events, event.NewVulnUpdated(v.id, v.source))
    return nil
}

func (v *VulnerabilityAggregate) PullEvents() []domain.Event {
    events := v.events
    v.events = nil
    return events
}
```

---

## 5. Application — Import Vulnerability Command

```go
// application/command/import_vulnerability/handler.go
package import_vulnerability

type Command struct {
    RawContent  []byte          // Raw JSON/YAML content
    ContentHash string          // SHA256 of content
    Source      SourceRef       // Source name + path
    Extension   string          // ".json" | ".yaml"
    SourceTimestamp *time.Time  // When source was last modified
    SkipHashCheck   bool        // Force reimport
}

type Handler struct {
    vulnRepo       repository.VulnerabilityWriter
    sourceRepo     repository.SourceRepositoryRepo
    findingRepo    repository.ImportFindingRepo
    validator      service.OSVValidator
    dedupService   service.DeduplicationService
    eventPublisher port.EventPublisher
    blobStore      port.BlobStore
    idempotency    IdempotencyStore
    tracer         trace.Tracer
    logger         *zerolog.Logger
}

func (h *Handler) Handle(ctx context.Context, cmd Command) error {
    ctx, span := h.tracer.Start(ctx, "ImportVulnerability")
    defer span.End()
    
    // 1. Idempotency check
    if processed := h.idempotency.IsProcessed(ctx, cmd.ContentHash); processed {
        h.logger.Debug().Str("hash", cmd.ContentHash).Msg("already processed, skipping")
        return nil
    }
    
    // 2. Validate OSV schema
    records, err := h.validator.Parse(cmd.RawContent, cmd.Extension)
    if err != nil {
        h.recordFinding(ctx, cmd.Source, "", finding.INVALID_JSON)
        return nil // Non-fatal: log and continue
    }
    
    for _, record := range records {
        if err := h.processRecord(ctx, record, cmd); err != nil {
            h.logger.Error().Err(err).Str("id", record.ID).Msg("failed to process record")
            h.recordFinding(ctx, cmd.Source, record.ID, finding.PROCESSING_ERROR)
        }
    }
    
    // 3. Mark as processed
    h.idempotency.MarkProcessed(ctx, cmd.ContentHash)
    return nil
}

func (h *Handler) processRecord(ctx context.Context, record OSVRecord, cmd Command) error {
    // 4. Load existing aggregate (if any)
    existing, err := h.vulnRepo.GetByID(ctx, record.ID)
    
    var agg *vulnerability.VulnerabilityAggregate
    var isNew bool
    
    if err == domain.ErrNotFound {
        // New vulnerability
        agg, err = vulnerability.NewFromOSV(record, cmd.Source)
        if err != nil {
            return err
        }
        isNew = true
    } else if err != nil {
        return err
    } else {
        // Existing vulnerability - check for changes
        changed, err := existing.Update(record)
        if err != nil {
            return err
        }
        if !changed && !cmd.SkipHashCheck {
            return nil // No changes detected
        }
        agg = existing
    }
    
    // 5. Persist (transaction: Firestore + GCS)
    if err := h.persistAggregate(ctx, agg); err != nil {
        return fmt.Errorf("persist: %w", err)
    }
    
    // 6. Publish domain events
    for _, evt := range agg.PullEvents() {
        if err := h.eventPublisher.Publish(ctx, evt); err != nil {
            h.logger.Error().Err(err).Msg("failed to publish event")
            // Non-fatal: events are best-effort, can be replayed
        }
    }
    
    return nil
}

func (h *Handler) persistAggregate(ctx context.Context, agg *vulnerability.VulnerabilityAggregate) error {
    // Transactional:
    // 1. Write to Firestore (read model projection)
    // 2. Upload full JSON to GCS
    // Both wrapped in a two-phase approach (Firestore first, GCS second)
    // On GCS failure: publish retry event
    
    readModel := projectToReadModel(agg)
    if err := h.vulnRepo.Upsert(ctx, readModel); err != nil {
        return fmt.Errorf("firestore upsert: %w", err)
    }
    
    fullJSON, err := agg.MarshalJSON()
    if err != nil {
        return err
    }
    
    if err := h.blobStore.Upload(ctx, agg.ID(), fullJSON, agg.ContentHash()); err != nil {
        // Publish GCS retry event instead of failing
        h.eventPublisher.Publish(ctx, event.NewGCSRetry(agg.ID(), fullJSON))
        h.logger.Warn().Err(err).Msg("GCS upload failed, queued for retry")
    }
    
    return nil
}
```

---

## 6. Domain Events

```go
// domain/event/vuln_imported.go
package event

const TopicVulnImported = "osv.vuln.imported"

type VulnImported struct {
    EventID     string    `json:"event_id"`    // UUID
    EventType   string    `json:"event_type"`  // "osv.vuln.imported"
    OccurredAt  time.Time `json:"occurred_at"`
    
    // Payload
    VulnID      string    `json:"vuln_id"`
    Source      string    `json:"source"`
    Ecosystems  []string  `json:"ecosystems"`
    IsNew       bool      `json:"is_new"`
    ContentHash string    `json:"content_hash"`
    SchemaVersion string  `json:"schema_version"`
}

// Consumers:
// - Impact Analysis Service (trigger analysis)
// - AI Enrichment Service (trigger enrichment)  
// - Search Service (trigger indexing)
// - Notification Service (broadcast)
// - Alias Service (check for new aliases)
```

---

## 7. Import Quality Tracking

```go
// domain/entity/import_finding.go

type ImportFindingType string
const (
    FindingInvalidJSON      ImportFindingType = "INVALID_JSON"
    FindingSchemaViolation  ImportFindingType = "SCHEMA_VIOLATION"
    FindingMissingID        ImportFindingType = "MISSING_ID"
    FindingUnknownEcosystem ImportFindingType = "UNKNOWN_ECOSYSTEM"
    FindingProcessingError  ImportFindingType = "PROCESSING_ERROR"
)

type ImportFinding struct {
    BugID       string              `firestore:"bug_id"`
    Source      string              `firestore:"source"`
    Findings    []ImportFindingType `firestore:"findings"`
    FirstSeen   time.Time          `firestore:"first_seen"`
    LastAttempt time.Time          `firestore:"last_attempt"`
}

// Exposed via API:
// GET /v1experimental/importfindings/{source}
// → Returns all findings for a source, enabling data quality monitoring
```

---

## 8. Saga: Vulnerability Import Lifecycle

```
VulnerabilityImportSaga (Choreography-based):

1. [SourceSync] → publish SourceChangeDetected{source, path, hash}

2. [Ingestion] consume SourceChangeDetected
   → validate, persist
   → publish VulnImported{vuln_id, source, content_hash}
   → publish VulnUpdated{vuln_id, changed_fields} (if update)

3. [ImpactAnalysis] consume VulnImported
   → run git bisection + version enumeration
   → publish ImpactAnalysisCompleted{vuln_id, content_hash, result}

4. [Ingestion] consume ImpactAnalysisCompleted
   → command: ApplyImpactAnalysis
   → publish VulnUpdated{vuln_id, ["affected"]}

5. [AIEnrichment] consume VulnImported (parallel with step 3)
   → enrich with AI
   → publish AIEnrichmentCompleted{vuln_id, metadata}

6. [Ingestion] consume AIEnrichmentCompleted
   → command: ApplyAIMetadata

7. [Search] consume VulnUpdated
   → reindex vulnerability

8. [Notification] consume VulnImported/VulnUpdated
   → broadcast to subscribers
```

---

## 9. Deletion Safety (Port to new service)

```go
// Deletion safety threshold preserved from Python implementation
type DeletionSafetyConfig struct {
    ThresholdPct float64 // Default: 10% - refuse if more than this % deleted
}

func (h *WithdrawHandler) CheckDeletionSafety(
    ctx context.Context,
    source string,
    toDeleteCount int,
    totalCount int,
) error {
    pct := float64(toDeleteCount) / float64(totalCount) * 100
    if pct >= h.config.ThresholdPct {
        return domain.NewDeletionSafetyError(
            fmt.Sprintf("refusing to delete %.1f%% of source %s records", pct, source),
        )
    }
    return nil
}
```

---

## 10. SLO Targets

| Metric | Target |
|--------|--------|
| Import latency P50 | < 500ms per vulnerability |
| Import latency P99 | < 5s per vulnerability |
| Import success rate | > 99.5% (non-schema errors) |
| Schema validation accuracy | > 99.9% |
| Event publication latency | < 100ms |
| Kafka consumer lag | < 1000 messages |
| Idempotency accuracy | 100% (no duplicates) |

---

## 11. Implementation Status

> **Status:** ✅ Core Implemented | **Updated:** 2026-06-01

### Implemented
- [x] `domain/aggregate/vulnerability/vulnerability.go` — VulnerabilityAggregate (NewFromOSV, Update, Withdraw, ApplyImpactAnalysis, PullEvents)
- [x] `domain/aggregate/vulnerability/vulnerability_test.go` — Unit tests for business rules
- [x] `domain/event/events.go` — VulnImported, VulnUpdated, VulnWithdrawn
- [x] `domain/entity/import_finding.go` — ImportFinding entity + FindingType enum
- [x] `domain/valueobject/valueobject.go` — VulnID, SourceRef, ContentHash, OSVSchemaVersion
- [x] `domain/repository/repository.go` — VulnerabilityWriter, ImportFindingRepo interfaces
- [x] `application/command/import_vulnerability/handler.go` — Full import flow (idempotency → parse → upsert → GCS → publish)
- [x] `application/port/ports.go` — EventPublisher, BlobStore, ImpactAnalysisPort interfaces
- [x] `infra/persistence/firestore/vulnerability_writer.go` — Firestore Upsert
- [x] `infra/storage/gcs/vulnerability_blob_store.go` — GCS JSON blob upload
- [x] `infra/messaging/nats/event_publisher.go` — NATS JetStream publish
- [x] `infra/messaging/nats/consumer/source_change_consumer.go` — SourceChangeDetected consumer
- [x] `infra/idempotency/redis/idempotency_store.go` — Redis SETNX idempotency
- [x] `cmd/server/main.go` — Full service wiring + graceful shutdown
- [x] `Dockerfile`, `config/config.yaml`

### Pending
- [ ] `application/command/apply_impact_analysis/handler.go` — ApplyImpactAnalysis command
- [ ] `application/command/apply_ai_metadata/handler.go` — ApplyAIMetadata command
- [ ] `application/query/get_import_findings/handler.go` — ImportFindings query + HTTP endpoint
- [ ] NATS consumer for ImpactAnalysisCompleted + AIEnrichmentCompleted events
- [ ] `interface/grpc/handler/ingestion_handler.go` — gRPC admin handler
- [ ] Integration tests (NATS + Firestore emulators)
- [ ] Makefile

### Deviations from Spec
- Spec uses Kafka; implementation uses NATS JetStream throughout
- GCS upload failure publishes retry event instead of failing transaction (matches spec intent)
