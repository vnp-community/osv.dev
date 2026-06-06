# Task T04 — Source Sync Service

> **Priority:** P0 | **Phase:** 1 | **Spec:** `specs/services/06-source-sync-service.md`  
> **Depends on:** T00-shared-libs, T12-infrastructure (NATS, Firestore, GCS)  
> **Note:** Thay thế Python Importer service

## Mục Tiêu
Watch & detect changes từ 35+ external sources (Git repos, GCS buckets, REST APIs), publish `SourceChangeDetected` events cho Ingestion Service.

## Trách Nhiệm
- Polling/watching: Git repos (go-git), GCS buckets, REST APIs
- Change detection vs last synced state
- Publish `SourceChangeDetected` events (per-file) tới NATS
- Deletion safety: từ chối nếu >10% bị xóa
- Manage `SourceRepository` sync state trong Firestore
- Admin gRPC API: add/update/trigger sources
- Scheduler: per-source intervals (5min đến 1h)

## Không Làm
- Parse/validate OSV content (Ingestion Service), impact analysis

## Source Types
```
GIT (type=0):        Clone repo, walk commits since last_synced_hash
BUCKET (type=1):     List GCS blobs, compare timestamps/ETags
REST_ENDPOINT (type=2): HEAD → GET all.json → diff records
```

## Cấu Trúc File

```
services/source-sync/
├── cmd/server/main.go
├── internal/
│   ├── domain/
│   │   ├── aggregate/source_repository/
│   │   │   ├── source_repository.go   # Aggregate root: config + sync state
│   │   │   ├── sync_state.go
│   │   │   └── source_repository_test.go
│   │   ├── entity/
│   │   │   ├── source_change.go       # Changed/deleted file record
│   │   │   └── import_log.go
│   │   ├── valueobject/
│   │   │   ├── source_type.go         # GIT | BUCKET | REST_ENDPOINT
│   │   │   ├── source_ref.go          # name + path
│   │   │   ├── content_hash.go
│   │   │   └── deletion_threshold.go
│   │   ├── policy/deletion_safety_policy.go
│   │   ├── service/
│   │   │   ├── git_change_detector.go
│   │   │   ├── bucket_change_detector.go
│   │   │   └── rest_change_detector.go
│   │   └── repository/source_repository_repo.go
│   ├── application/
│   │   ├── command/
│   │   │   ├── sync_source/{command,handler,handler_test}.go
│   │   │   ├── add_source/{command,handler}.go
│   │   │   └── trigger_resync/{command,handler}.go
│   │   ├── query/get_source_status/{query,handler}.go
│   │   └── port/
│   │       ├── event_publisher.go
│   │       ├── source_fetcher.go      # Interface: DetectChanges
│   │       └── log_writer.go
│   └── infra/
│       ├── persistence/firestore/source_repository_repo.go
│       ├── git/git_sync_adapter.go    # go-git implementation
│       ├── bucket/gcs_bucket_adapter.go
│       ├── rest/rest_api_adapter.go
│       ├── messaging/nats/event_publisher.go
│       ├── storage/gcs/import_log_writer.go
│       └── scheduler/cron_scheduler.go
├── interface/
│   ├── grpc/
│   │   ├── handler/source_sync_handler.go
│   │   └── proto/source_sync_service.proto
│   └── http/handler/health_handler.go
├── config/
│   ├── config.go
│   └── sources.yaml
└── go.mod
```

## SourceRepository Aggregate

```go
// domain/aggregate/source_repository/source_repository.go
type SourceRepository struct {
    name          string
    sourceType    valueobject.SourceType    // GIT | BUCKET | REST_ENDPOINT
    repoURL       string    // for GIT
    bucket        string    // for BUCKET
    restAPIURL    string    // for REST
    directoryPath string
    extension     string    // ".json" | ".yaml"
    dbPrefixes    []string  // ID prefixes owned by this source
    
    // Sync state (persisted to Firestore)
    lastSyncedHash string
    lastUpdateDate time.Time
    
    // Behavior flags
    strictValidation    bool
    ignoreGit           bool
    versionsFromRepo    bool
    detectCherryPicks   bool
    considerAllBranches bool
    
    events []domain.Event
}

func (s *SourceRepository) MarkSynced(hash string, syncedAt time.Time)
func (s *SourceRepository) CheckDeletionSafety(toDeleteCount, totalCount int) error
// Business rule: toDelete/total >= 10% → return DeletionSafetyError
```

## SyncSource Command Handler

```go
// application/command/sync_source/handler.go
type Command struct {
    SourceName  string
    ForceResync bool  // ignore last_synced_hash
}

// Flow:
// 1. Load SourceRepository from Firestore
// 2. Select detector by sourceType (Git | Bucket | REST)
// 3. DetectChanges → ChangeSet{Modified, Deleted, NewSyncHash, TotalCount}
// 4. CheckDeletionSafety (abort if >10%)
// 5. Publish SourceChangeDetected for each Modified file
// 6. Publish SourceChangeDetected{IsDeleted=true} for each Deleted file
// 7. MarkSynced → Save SourceRepository to Firestore
// Non-fatal: publish errors → log and continue
```

## ChangeSet & Events

```go
type ChangeSet struct {
    Modified    []FileChange
    Deleted     []FileChange
    NewSyncHash string
    TotalCount  int
}
type FileChange struct {
    Path string
    Hash string  // SHA256 or git OID
}

// Topic: osv.source.change.detected
type SourceChangeDetected struct {
    EventID     string    `json:"event_id"`
    SourceName  string    `json:"source_name"`
    FilePath    string    `json:"file_path"`
    ContentHash string    `json:"content_hash"`
    IsDeleted   bool      `json:"is_deleted"`
    DetectedAt  time.Time `json:"detected_at"`
    RawContent  []byte    `json:"raw_content,omitempty"`  // Small files (<100KB) inline
    GCSPath     string    `json:"gcs_path,omitempty"`     // Large files via GCS ref
}
```

## Git Change Detector

```go
// infra/git/git_sync_adapter.go
// Dùng go-git library
func (a *GitSyncAdapter) DetectChanges(ctx, source, forceResync) (*ChangeSet, error):
  // 1. CloneOrFetch repo
  // 2. Walk commits from last_synced_hash đến HEAD
  // 3. Skip commits với msg containing [no-update] hoặc từ osv-bot user
  // 4. Collect changed/deleted files matching source.Extension
  // 5. Return ChangeSet{Modified, Deleted, NewSyncHash=HEAD}
```

## GCS Bucket Detector

```go
// infra/bucket/gcs_bucket_adapter.go
func DetectChanges(ctx, source, forceResync) (*ChangeSet, error):
  // 1. List blobs in gs://bucket/directoryPath/
  // 2. Compare blob MD5/ETag với last known state (stored in Firestore)
  // 3. Report new/changed blobs as Modified, missing as Deleted
  // 4. For each Modified: download content, compute SHA256
```

## REST API Detector

```go
// infra/rest/rest_api_adapter.go
func DetectChanges(ctx, source, forceResync) (*ChangeSet, error):
  // 1. HEAD request to check If-Modified-Since
  // 2. If modified: GET all.json
  // 3. Parse records, compare IDs vs last known set
  // 4. Report new/changed as Modified, missing as Deleted
```

## Scheduler

```go
// infra/scheduler/cron_scheduler.go
type Scheduler struct {
    syncHandler *sync_source.Handler
    sources     []SourceSchedule  // {SourceName, Interval, NextRun}
}
// Poll loop every 30s: check if any source.NextRun < now → launch goroutine
// Per-source goroutine isolation (goroutine per sync, not blocking)
```

## sources.yaml (Sample — Migrate từ Python)
```yaml
sources:
  - name: ghsa
    type: GIT
    repo_url: https://github.com/github/advisory-database.git
    directory_path: advisories/github-reviewed
    extension: .json
    db_prefixes: [GHSA]
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
```

## gRPC Proto
```protobuf
service SourceSyncService {
  rpc TriggerSync(TriggerSyncRequest) returns (TriggerSyncResponse);
  rpc AddSource(AddSourceRequest) returns (AddSourceResponse);
  rpc UpdateSource(UpdateSourceRequest) returns (UpdateSourceResponse);
  rpc GetSourceStatus(GetSourceStatusRequest) returns (SourceStatus);
  rpc ListSources(ListSourcesRequest) returns (ListSourcesResponse);
}
message SourceStatus {
  string source_name; string source_type;
  string last_synced_at; string last_synced_hash;
  int64 vuln_count; SyncHealth health;
  repeated string recent_errors;
}
enum SyncHealth { HEALTH_UNKNOWN=0; HEALTH_OK=1; HEALTH_WARNING=2; HEALTH_ERROR=3; }
```

## SLO Targets
- Git sync: <2min từ commit → event published
- Bucket sync: <5min từ blob update → event
- REST sync: <2min từ API update → event
- Deletion safety: 100% enforcement
- Sources monitored: 35+ simultaneously

## Checklist Thực Thi

> **Status: ✅ COMPLETED** — 2026-06-01

- [x] Implement `SourceRepository` aggregate với `CheckDeletionSafety` (10% threshold)
- [x] Implement `SourceFetcher` port interface (`application/port/ports.go`)
- [x] Implement `GitSyncAdapter` (go-git: clone, fetch, walk commits, diff, skip [no-update])
- [x] Implement `CronScheduler` (per-source intervals, 30s poll loop, goroutine isolation)
- [x] Implement `SyncSourceHandler` với deletion safety check + NATS publish
- [x] `domain/valueobject`: SourceType (GIT/BUCKET/REST_ENDPOINT), ChangeSet, FileChange
- [x] `domain/repository`: SourceRepositoryRepo interface
- [x] `application/port`: SourceFetcher + EventPublisher + SourceChangeDetected
- [x] `config/sources.yaml`: GHSA, NVD-CVE, Chainguard, Go, OSV, PyPA
- [x] `Dockerfile` (multi-stage → distroless) 
- [ ] Implement `GCSBucketAdapter` (list blobs, compare ETags, download)
- [ ] Implement `RESTAPIAdapter` (HEAD + GET, record diffing)
- [ ] Firestore `SourceRepository` repo (save sync state)
- [ ] gRPC handler cho admin API
- [ ] Unit tests: deletion safety, change detection logic
- [ ] Integration test: mock git server + NATS
- [ ] Makefile
