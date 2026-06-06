# Service 06 — Source Sync Service

> **Version:** 1.0 | **Status:** Proposed | **Priority:** P0  
> **Language:** Go  
> **Pattern:** Clean Architecture + Event-Driven  
> **Communication:** NATS (publish source changes) + gRPC admin API

---

## 1. Trách Nhiệm

Thay thế **Importer Service** Python cũ (`gcp/workers/importer/`). Phát hiện thay đổi từ 30+ nguồn dữ liệu bên ngoài và phát sự kiện để trigger ingestion pipeline.

**Responsibilities:**
- Polling & watching external sources: Git repos, GCS buckets, REST APIs
- Change detection: so sánh với last synced state
- Publish `SourceChangeDetected` events tới NATS/Kafka
- Deletion safety check (refuse > 10% mass deletion)
- Manage `SourceRepository` sync state
- Admin API: add/update/trigger sources
- Import log publishing (quality monitoring)

**Source Types Supported:**
- `GIT (type=0)`: Clone repo, walk commits since last hash
- `BUCKET (type=1)`: List GCS blobs, compare timestamps/hashes
- `REST_ENDPOINT (type=2)`: HEAD check → GET all.json → diff

**NOT Responsible for:**
- Parsing/validating OSV content (Ingestion Service)
- Persisting vulnerabilities (Ingestion Service)
- Impact analysis (Impact Analysis Service)

---

## 2. Clean Architecture Layers

```
Domain:
  ├── SourceRepository aggregate (sync state + config)
  ├── SourceChange entity (changed/deleted file record)
  ├── DeletionSafetyPolicy (10% threshold rule)
  ├── SourceType enum (GIT | BUCKET | REST_ENDPOINT)
  └── Repository: SourceRepositoryRepo

Application (Command):
  ├── SyncSourceCommand + Handler
  ├── AddSourceCommand + Handler
  ├── UpdateSourceCommand + Handler
  └── TriggerFullResyncCommand + Handler

Application (Query):
  └── GetSourceStatusQuery + Handler

Infrastructure:
  ├── GitSyncAdapter (go-git)
  ├── GCSBucketAdapter (Google Cloud Storage)
  ├── RESTAPIAdapter (HTTP client)
  ├── FirestoreSourceRepo
  ├── NATSEventPublisher
  ├── GCSImportLogWriter (public import logs)
  └── SchedulerAdapter (cron-like scheduling)

Interface:
  ├── gRPC handler (SourceSyncService - admin)
  └── HTTP handler (health)
```

---

## 3. Directory Structure

```
services/source-sync/
├── cmd/server/main.go
├── internal/
│   ├── domain/
│   │   ├── aggregate/
│   │   │   └── source_repository/
│   │   │       ├── source_repository.go      # Aggregate root
│   │   │       ├── sync_state.go             # Sync state value object
│   │   │       └── source_repository_test.go
│   │   ├── entity/
│   │   │   ├── source_change.go              # Changed/deleted file record
│   │   │   └── import_log.go                 # Log entry for quality tracking
│   │   ├── valueobject/
│   │   │   ├── source_type.go                # GIT | BUCKET | REST_ENDPOINT
│   │   │   ├── source_ref.go                 # source_name + path
│   │   │   ├── content_hash.go               # SHA256 of raw content
│   │   │   └── deletion_threshold.go         # 10% safety threshold
│   │   ├── policy/
│   │   │   └── deletion_safety_policy.go     # Business rule: refuse mass deletion
│   │   ├── service/
│   │   │   ├── git_change_detector.go        # Detect changes in git repos
│   │   │   ├── bucket_change_detector.go     # Detect changes in GCS buckets
│   │   │   └── rest_change_detector.go       # Detect changes via REST API
│   │   └── repository/
│   │       └── source_repository_repo.go     # Interface
│   ├── application/
│   │   ├── command/
│   │   │   ├── sync_source/
│   │   │   │   ├── command.go
│   │   │   │   ├── handler.go
│   │   │   │   └── handler_test.go
│   │   │   ├── add_source/
│   │   │   │   ├── command.go
│   │   │   │   └── handler.go
│   │   │   └── trigger_resync/
│   │   │       ├── command.go
│   │   │       └── handler.go
│   │   ├── query/
│   │   │   └── get_source_status/
│   │   │       ├── query.go
│   │   │       └── handler.go
│   │   └── port/
│   │       ├── event_publisher.go            # Outbound: publish source changes
│   │       ├── source_fetcher.go             # Outbound: fetch from source
│   │       └── log_writer.go                 # Outbound: write import logs
│   └── infra/
│       ├── persistence/
│       │   └── firestore/
│       │       └── source_repository_repo.go
│       ├── git/
│       │   └── git_sync_adapter.go           # go-git based sync
│       ├── bucket/
│       │   └── gcs_bucket_adapter.go         # GCS bucket listing + download
│       ├── rest/
│       │   └── rest_api_adapter.go           # HTTP client for REST sources
│       ├── messaging/
│       │   └── nats/
│       │       └── event_publisher.go
│       ├── storage/
│       │   └── gcs/
│       │       └── import_log_writer.go      # Write to gs://osv-public-import-logs/
│       └── scheduler/
│           └── cron_scheduler.go             # Schedule periodic syncs
├── interface/
│   ├── grpc/
│   │   ├── handler/
│   │   │   └── source_sync_handler.go
│   │   └── proto/
│   │       └── source_sync_service.proto
│   └── http/
│       └── handler/
│           └── health_handler.go
├── config/
│   ├── config.go
│   └── sources.yaml                          # Source configurations
├── Dockerfile
└── go.mod
```

---

## 4. Proto Definition

```protobuf
// proto/source_sync_service.proto
syntax = "proto3";
package osv.sourcesync.v1;

service SourceSyncService {
  // Admin: manually trigger sync for a source
  rpc TriggerSync(TriggerSyncRequest) returns (TriggerSyncResponse);
  
  // Admin: add a new source
  rpc AddSource(AddSourceRequest) returns (AddSourceResponse);
  
  // Admin: update source configuration
  rpc UpdateSource(UpdateSourceRequest) returns (UpdateSourceResponse);
  
  // Admin: get sync status for a source
  rpc GetSourceStatus(GetSourceStatusRequest) returns (SourceStatus);
  
  // Admin: list all sources
  rpc ListSources(ListSourcesRequest) returns (ListSourcesResponse);
}

message TriggerSyncRequest {
  string source_name   = 1;
  bool   force_resync  = 2;  // Ignore last_synced_hash, process all
}

message SourceStatus {
  string source_name     = 1;
  string source_type     = 2;  // GIT | BUCKET | REST_ENDPOINT
  string last_synced_at  = 3;
  string last_synced_hash = 4;
  int64  vuln_count      = 5;
  SyncHealth health      = 6;
  repeated string recent_errors = 7;
}

enum SyncHealth {
  HEALTH_UNKNOWN = 0;
  HEALTH_OK      = 1;
  HEALTH_WARNING = 2;
  HEALTH_ERROR   = 3;
}
```

---

## 5. Domain — Source Repository Aggregate

```go
// domain/aggregate/source_repository/source_repository.go
package source_repository

// SourceRepository is the aggregate root for a data source configuration.
type SourceRepository struct {
    // Config (from sources.yaml / Firestore)
    name           string
    sourceType     valueobject.SourceType
    repoURL        string           // for GIT type
    bucket         string           // for BUCKET type
    restAPIURL     string           // for REST type
    directoryPath  string
    extension      string           // ".json" | ".yaml"
    dbPrefixes     []string         // ID prefixes this source owns
    
    // Sync state
    lastSyncedHash  string          // Git: last commit hash
    lastUpdateDate  time.Time       // Bucket/REST: last sync time
    
    // Behavior flags
    strictValidation      bool
    ignoreGit             bool
    versionsFromRepo      bool
    detectCherryPicks     bool
    considerAllBranches   bool
    
    // Pending events
    events []domain.Event
}

// MarkSynced updates sync state after successful sync.
func (s *SourceRepository) MarkSynced(hash string, syncedAt time.Time) {
    s.lastSyncedHash = hash
    s.lastUpdateDate = syncedAt
    s.events = append(s.events, event.NewSourceSyncCompleted(s.name, syncedAt))
}

// CheckDeletionSafety enforces the 10% deletion threshold rule.
func (s *SourceRepository) CheckDeletionSafety(toDeleteCount, totalCount int) error {
    if totalCount == 0 {
        return nil
    }
    pct := float64(toDeleteCount) / float64(totalCount) * 100
    if pct >= 10.0 {
        return domain.NewDeletionSafetyError(
            fmt.Sprintf("refusing to delete %.1f%% of %s records (%d/%d)",
                pct, s.name, toDeleteCount, totalCount),
        )
    }
    return nil
}
```

---

## 6. Application — Sync Source Command

```go
// application/command/sync_source/handler.go
package sync_source

type Handler struct {
    sourceRepo      repository.SourceRepositoryRepo
    gitDetector     port.SourceFetcher  // Git implementation
    bucketDetector  port.SourceFetcher  // GCS implementation
    restDetector    port.SourceFetcher  // REST implementation
    eventPublisher  port.EventPublisher
    logWriter       port.LogWriter
    tracer          trace.Tracer
    logger          *zerolog.Logger
}

func (h *Handler) Handle(ctx context.Context, cmd Command) error {
    ctx, span := h.tracer.Start(ctx, "SyncSource")
    defer span.End()
    
    source, err := h.sourceRepo.GetByName(ctx, cmd.SourceName)
    if err != nil {
        return err
    }
    
    span.SetAttributes(
        attribute.String("source.name", source.Name()),
        attribute.String("source.type", source.Type().String()),
    )
    
    // Select appropriate detector
    detector := h.selectDetector(source)
    
    // Detect changes
    changes, err := detector.DetectChanges(ctx, source, cmd.ForceResync)
    if err != nil {
        h.logWriter.WriteError(ctx, source.Name(), err)
        return fmt.Errorf("detect changes for %s: %w", source.Name(), err)
    }
    
    h.logger.Info().
        Str("source", source.Name()).
        Int("changes", len(changes.Modified)).
        Int("deleted", len(changes.Deleted)).
        Msg("changes detected")
    
    // Apply deletion safety
    if len(changes.Deleted) > 0 {
        if err := source.CheckDeletionSafety(len(changes.Deleted), changes.TotalCount); err != nil {
            h.logWriter.WriteError(ctx, source.Name(), err)
            return err // Abort — refuse mass deletion
        }
    }
    
    // Publish SourceChangeDetected events for each changed file
    for _, change := range changes.Modified {
        evt := &event.SourceChangeDetected{
            SourceName:  source.Name(),
            FilePath:    change.Path,
            ContentHash: change.Hash,
            IsDeleted:   false,
            DetectedAt:  time.Now().UTC(),
        }
        if err := h.eventPublisher.Publish(ctx, evt); err != nil {
            h.logger.Error().Err(err).Str("path", change.Path).Msg("failed to publish change event")
        }
    }
    
    for _, deleted := range changes.Deleted {
        evt := &event.SourceChangeDetected{
            SourceName: source.Name(),
            FilePath:   deleted.Path,
            IsDeleted:  true,
            DetectedAt: time.Now().UTC(),
        }
        h.eventPublisher.Publish(ctx, evt)
    }
    
    // Update sync state
    source.MarkSynced(changes.NewSyncHash, time.Now().UTC())
    if err := h.sourceRepo.Save(ctx, source); err != nil {
        return fmt.Errorf("save source state: %w", err)
    }
    
    return nil
}
```

---

## 7. Git Change Detector

```go
// infra/git/git_sync_adapter.go

type GitSyncAdapter struct {
    cloneDir string    // Local temp dir for git clones
    tracer   trace.Tracer
}

type ChangeSet struct {
    Modified    []FileChange
    Deleted     []FileChange
    NewSyncHash string
    TotalCount  int
}

func (a *GitSyncAdapter) DetectChanges(
    ctx context.Context,
    source *source_repository.SourceRepository,
    forceResync bool,
) (*ChangeSet, error) {
    // 1. Clone or fetch repository
    repo, err := a.cloneOrFetch(ctx, source.RepoURL())
    if err != nil {
        return nil, err
    }
    
    // 2. Walk commits since last_synced_hash
    walker, err := repo.Walk()
    if err != nil {
        return nil, err
    }
    walker.Sorting(git.SortTopological)
    
    if !forceResync && source.LastSyncedHash() != "" {
        startOID, _ := git.NewOid(source.LastSyncedHash())
        walker.Hide(startOID)
    }
    
    headRef, _ := repo.Head()
    walker.Push(headRef.Target())
    
    // 3. Collect changed files
    changed := map[string]FileChange{}
    deleted := map[string]FileChange{}
    
    walker.Iterate(func(commit *git.Commit) bool {
        // Skip OSV bot commits (avoid import loops)
        if isOSVBotCommit(commit) || hasNoUpdateMarker(commit) {
            return true
        }
        
        if commit.ParentCount() > 0 {
            parent := commit.Parent(0)
            diff, _ := repo.DiffTreeToTree(parent.Tree(), commit.Tree(), nil)
            diff.ForEach(func(delta git.DiffDelta, progress float64) error {
                switch delta.Status {
                case git.DeltaDeleted:
                    deleted[delta.OldFile.Path] = FileChange{Path: delta.OldFile.Path}
                default:
                    if matchesExtension(delta.NewFile.Path, source.Extension()) {
                        changed[delta.NewFile.Path] = FileChange{
                            Path: delta.NewFile.Path,
                            Hash: delta.NewFile.Oid.String(),
                        }
                    }
                }
                return nil
            }, nil)
        }
        return true
    })
    
    return &ChangeSet{
        Modified:    mapToSlice(changed),
        Deleted:     mapToSlice(deleted),
        NewSyncHash: headRef.Target().String(),
    }, nil
}
```

---

## 8. Source Configuration (sources.yaml)

```yaml
# Migrated from Python source.yaml
# Supports all 30+ active sources

sources:
  - name: ghsa
    type: GIT
    repo_url: https://github.com/github/advisory-database.git
    directory_path: advisories/github-reviewed
    extension: .json
    db_prefixes: [GHSA]
    strict_validation: false
    detect_cherrypicks: false
    sync_interval: 5m

  - name: nvd-cve
    type: BUCKET
    bucket: cve-osv-conversion
    directory_path: osv-output/
    extension: .json
    db_prefixes: [CVE]
    sync_interval: 1h

  - name: chainguard
    type: REST_ENDPOINT
    rest_api_url: https://packages.cgr.dev/chainguard/osv/all.json
    extension: .json
    db_prefixes: [CGA]
    sync_interval: 30m

  - name: go
    type: BUCKET
    bucket: go-vulndb
    directory_path: ID/
    extension: .json
    db_prefixes: [GO]
    sync_interval: 15m

  # ... 30+ more sources
```

---

## 9. Scheduler Design

```go
// infra/scheduler/cron_scheduler.go

// Each source has its own sync interval.
// Scheduler manages per-source goroutines with configurable intervals.

type Scheduler struct {
    syncHandler *sync_source.Handler
    sources     []SourceSchedule
    logger      *zerolog.Logger
}

type SourceSchedule struct {
    SourceName   string
    Interval     time.Duration
    NextRun      time.Time
}

func (s *Scheduler) Run(ctx context.Context) error {
    for {
        now := time.Now()
        
        for i, schedule := range s.sources {
            if now.After(schedule.NextRun) {
                go func(name string) {
                    if err := s.syncHandler.Handle(ctx, sync_source.Command{
                        SourceName: name,
                    }); err != nil {
                        s.logger.Error().Err(err).Str("source", name).Msg("sync failed")
                    }
                }(schedule.SourceName)
                
                s.sources[i].NextRun = now.Add(schedule.Interval)
            }
        }
        
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(30 * time.Second): // Check every 30s
        }
    }
}
```

---

## 10. Events Published

```go
// Topic: osv.source.sync.started
type SourceSyncStarted struct {
    EventID    string    `json:"event_id"`
    SourceName string    `json:"source_name"`
    SourceType string    `json:"source_type"`
    StartedAt  time.Time `json:"started_at"`
}

// Topic: osv.source.change.detected
type SourceChangeDetected struct {
    EventID     string    `json:"event_id"`
    SourceName  string    `json:"source_name"`
    FilePath    string    `json:"file_path"`
    ContentHash string    `json:"content_hash"`
    IsDeleted   bool      `json:"is_deleted"`
    DetectedAt  time.Time `json:"detected_at"`
    RawContent  []byte    `json:"raw_content,omitempty"` // Small files inline
    GCSPath     string    `json:"gcs_path,omitempty"`    // Large files via GCS ref
}

// Topic: osv.source.sync.completed
type SourceSyncCompleted struct {
    EventID      string    `json:"event_id"`
    SourceName   string    `json:"source_name"`
    NewCount     int       `json:"new_count"`
    UpdatedCount int       `json:"updated_count"`
    DeletedCount int       `json:"deleted_count"`
    DurationMs   int64     `json:"duration_ms"`
    CompletedAt  time.Time `json:"completed_at"`
}
```

---

## 11. SLO Targets

| Metric | Target |
|--------|--------|
| Availability | 99.9% |
| Git sync latency | < 2min from commit to event published |
| Bucket sync latency | < 5min from blob update to event |
| REST sync latency | < 2min from API update to event |
| Deletion safety enforcement | 100% (never bypass) |
| Change detection accuracy | > 99.9% (no missed changes) |
| Sources monitored simultaneously | 35+ |

---

## 12. Implementation Status

> **Status:** ✅ Core Implemented | **Updated:** 2026-06-01

### Implemented
- [x] `domain/aggregate/source_repository/source_repository.go` — SourceRepository aggregate (MarkSynced, CheckDeletionSafety 10% rule)
- [x] `domain/aggregate/source_repository/source_repository_test.go` — Unit tests
- [x] `domain/valueobject/valueobject.go` — SourceType (GIT/BUCKET/REST), SourceRef, ContentHash, DeletionThreshold
- [x] `domain/repository/repository.go` — SourceRepositoryRepo interface
- [x] `application/port/ports.go` — SourceFetcher, EventPublisher, LogWriter interfaces
- [x] `application/command/sync_source/handler.go` — Full sync flow (detect → deletion safety → publish events → save state)
- [x] `infra/git/git_sync_adapter.go` — go-git: clone, fetch, walk commits, diff per-file
- [x] `infra/scheduler/cron_scheduler.go` — Per-source interval scheduler (30s poll loop)
- [x] `config/sources.yaml` — GHSA, NVD-CVE, Chainguard, Go, OSV, PyPA sources
- [x] `Dockerfile`

### Pending
- [ ] `infra/bucket/gcs_bucket_adapter.go` — GCS bucket listing + download (SourceType=BUCKET)
- [ ] `infra/rest/rest_api_adapter.go` — HTTP client for REST_ENDPOINT sources
- [ ] `infra/persistence/firestore/source_repository_repo.go` — Firestore persistence
- [ ] `interface/grpc/handler/source_sync_handler.go` — gRPC admin handler
- [ ] `cmd/server/main.go` — Entry point wiring
- [ ] Unit + integration tests, Makefile

### Deviations from Spec
- GCSBucketAdapter and RESTAPIAdapter not yet implemented; GitSyncAdapter covers ~70% of sources
- Deletion safety threshold is a domain method on SourceRepository (not a standalone DeletionSafetyPolicy service)
