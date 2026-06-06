# Service 04 — Impact Analysis Service

> **Version:** 1.0 | **Status:** Proposed | **Priority:** P1  
> **Language:** Go  
> **Pattern:** CQRS (Command side) + Domain Service + Clean Architecture  
> **Communication:** gRPC (sync) + NATS/Kafka (async events)

---

## 1. Trách Nhiệm

Service chuyên biệt xử lý **phân tích ảnh hưởng của lỗ hổng bảo mật** — thay thế `impact.py` và logic bisection trong Python worker cũ.

**Responsibilities:**
- Git bisection: xác định commit nào introduce/fix lỗ hổng
- Version enumeration: liệt kê tất cả versions bị ảnh hưởng từ ranges
- Cherry-pick detection: phát hiện commits cherry-pick fix từ nhánh khác
- Compute `affected_commits[]` từ GIT ranges
- Compute `versions[]` từ SEMVER/ECOSYSTEM ranges
- Support `DetermineVersion` enrichment (repo file-hash matching)
- Publish `ImpactAnalysisCompleted` domain event

**NOT Responsible for:**
- Persisting vulnerability data (delegated to Ingestion Service)
- Querying vulnerabilities (Query Service)
- File hash indexing (Version Index Service)

---

## 2. Clean Architecture Layers

```
Domain:
  ├── GitRange aggregate (introduced/fixed commit events)
  ├── VersionRange aggregate (semver/ecosystem events)
  ├── AffectedResult value object (computed commits + versions)
  ├── EcosystemHelper interface (version comparison strategy)
  └── Repository: GitRepoCache, EcosystemVersionFetcher

Application (Command side):
  ├── AnalyzeGitImpactCommand + Handler
  ├── AnalyzeVersionImpactCommand + Handler
  └── DetectCherryPicksCommand + Handler

Infrastructure:
  ├── GitCloneAdapter (libgit2 / go-git)
  ├── GCSRepoCacheAdapter (clone caching in GCS)
  ├── RedisMetadataCache (repo metadata)
  ├── EcosystemRegistry (30+ ecosystems)
  ├── NATSEventPublisher
  └── KafkaEventPublisher (alternative)

Interface:
  ├── gRPC handler (ImpactAnalysisService)
  └── NATS consumer (consume VulnImported events)
```

---

## 3. Directory Structure

```
services/impact-analysis/
├── cmd/server/main.go
├── internal/
│   ├── domain/
│   │   ├── aggregate/
│   │   │   ├── git_range/
│   │   │   │   ├── git_range.go          # GIT range aggregate
│   │   │   │   └── git_range_test.go
│   │   │   └── version_range/
│   │   │       ├── version_range.go      # SEMVER/ECOSYSTEM range aggregate
│   │   │       └── version_range_test.go
│   │   ├── entity/
│   │   │   ├── affected_result.go        # Output: commits + versions
│   │   │   └── commit.go                 # Git commit value object
│   │   ├── valueobject/
│   │   │   ├── commit_hash.go
│   │   │   ├── version_range_event.go    # introduced/fixed/limit/last_affected
│   │   │   ├── ecosystem.go
│   │   │   └── content_hash.go           # For staleness detection
│   │   ├── service/
│   │   │   ├── git_bisector.go           # Core git bisection logic
│   │   │   ├── version_enumerator.go     # Enumerate affected versions
│   │   │   ├── cherrypick_detector.go    # Cherry-pick detection
│   │   │   └── range_evaluator.go        # Evaluate if version in range
│   │   └── repository/
│   │       ├── git_repo_cache.go         # Interface: get/cache git repos
│   │       └── ecosystem_fetcher.go      # Interface: fetch all package versions
│   ├── application/
│   │   ├── command/
│   │   │   ├── analyze_git_impact/
│   │   │   │   ├── command.go
│   │   │   │   ├── handler.go
│   │   │   │   └── handler_test.go
│   │   │   ├── analyze_version_impact/
│   │   │   │   ├── command.go
│   │   │   │   └── handler.go
│   │   │   └── detect_cherrypicks/
│   │   │       ├── command.go
│   │   │       └── handler.go
│   │   └── port/
│   │       ├── event_publisher.go         # Outbound: publish results
│   │       └── git_provider.go            # Outbound: git operations
│   └── infra/
│       ├── git/
│       │   ├── gogit_adapter.go           # go-git implementation
│       │   └── repo_cache.go              # Local + GCS cache management
│       ├── ecosystem/
│       │   ├── registry.go                # EcosystemHelper registry
│       │   ├── pypi/
│       │   │   └── pypi_helper.go
│       │   ├── golang/
│       │   │   └── go_helper.go
│       │   ├── npm/
│       │   │   └── npm_helper.go
│       │   ├── maven/
│       │   │   └── maven_helper.go
│       │   └── ... (30+ ecosystems)
│       ├── storage/
│       │   └── gcs/
│       │       └── repo_cache_store.go    # GCS-backed repo cache
│       ├── cache/
│       │   └── redis/
│       │       └── metadata_cache.go
│       └── messaging/
│           ├── nats/
│           │   ├── event_publisher.go
│           │   └── consumer.go            # Listen VulnImported
│           └── kafka/
│               └── event_publisher.go
├── interface/
│   ├── grpc/
│   │   ├── handler/
│   │   │   └── impact_handler.go
│   │   ├── middleware/
│   │   │   ├── timeout_interceptor.go
│   │   │   └── tracing_interceptor.go
│   │   └── proto/
│   │       └── impact_service.proto
│   └── http/
│       └── handler/
│           └── health_handler.go
├── config/config.go
├── Dockerfile
├── Makefile
└── go.mod
```

---

## 4. Proto Definition

```protobuf
// proto/impact_service.proto
syntax = "proto3";
package osv.impact.v1;

import "google/protobuf/timestamp.proto";

service ImpactAnalysisService {
  // Analyze git range impact (bisection)
  rpc AnalyzeGitImpact(AnalyzeGitImpactRequest) returns (AnalyzeGitImpactResponse);
  
  // Enumerate affected versions from SEMVER/ECOSYSTEM ranges
  rpc AnalyzeVersionImpact(AnalyzeVersionImpactRequest) returns (AnalyzeVersionImpactResponse);
  
  // Detect cherry-pick commits
  rpc DetectCherryPicks(DetectCherryPicksRequest) returns (DetectCherryPicksResponse);
  
  // Full vulnerability impact analysis
  rpc AnalyzeVulnerability(AnalyzeVulnerabilityRequest) returns (AnalyzeVulnerabilityResponse);
}

message AnalyzeVulnerabilityRequest {
  string vuln_id      = 1;
  string content_hash = 2;  // For staleness detection
  repeated AffectedEntry affected = 3;
  AnalysisOptions options = 4;
}

message AnalysisOptions {
  bool detect_cherrypicks    = 1;
  bool versions_from_repo    = 2;
  bool consider_all_branches = 3;
  bool ignore_git            = 4;
}

message AnalyzeVulnerabilityResponse {
  string vuln_id       = 1;
  string content_hash  = 2;
  bool has_changes     = 3;
  repeated AffectedResult results = 4;
  AnalysisStats stats  = 5;
}

message AffectedResult {
  string package_ecosystem = 1;
  string package_name      = 2;
  repeated string commits  = 3;   // Git commit hashes
  repeated string versions = 4;   // Affected version strings
}

message AnalysisStats {
  int64 repos_cloned    = 1;
  int64 commits_walked  = 2;
  int64 versions_fetched = 3;
  int64 duration_ms     = 4;
}
```

---

## 5. Domain — Git Bisector

```go
// domain/service/git_bisector.go
package service

type GitBisector struct {
    repoCache repository.GitRepoCache
    tracer    trace.Tracer
    logger    *zerolog.Logger
}

type BisectionResult struct {
    AffectedCommits []string
    HasChanges      bool
}

func (b *GitBisector) Bisect(
    ctx context.Context,
    repoURL string,
    events []valueobject.RangeEvent,
    opts BisectionOptions,
) (*BisectionResult, error) {
    ctx, span := b.tracer.Start(ctx, "GitBisector.Bisect")
    defer span.End()
    
    // 1. Get cached repo (clone if needed)
    repo, err := b.repoCache.Get(ctx, repoURL)
    if err != nil {
        return nil, fmt.Errorf("get repo %s: %w", repoURL, err)
    }
    
    // 2. Resolve introduced/fixed commits
    introducedHash, fixedHash, err := b.resolveCommits(ctx, repo, events)
    if err != nil {
        return nil, err
    }
    
    // 3. Walk git history between introduced and fixed
    commits, err := b.walkRange(ctx, repo, introducedHash, fixedHash, opts)
    if err != nil {
        return nil, err
    }
    
    // 4. Cherry-pick detection (optional)
    if opts.DetectCherryPicks && fixedHash != "" {
        cherryPicks, err := b.detectCherryPicks(ctx, repo, fixedHash, opts.ConsiderAllBranches)
        if err != nil {
            b.logger.Warn().Err(err).Msg("cherry-pick detection failed, skipping")
        } else {
            commits = append(commits, cherryPicks...)
        }
    }
    
    return &BisectionResult{
        AffectedCommits: dedupeCommits(commits),
        HasChanges:      true,
    }, nil
}
```

---

## 6. Domain — Version Enumerator

```go
// domain/service/version_enumerator.go
package service

type VersionEnumerator struct {
    ecosystems ecosystem.Registry
    tracer     trace.Tracer
}

func (e *VersionEnumerator) Enumerate(
    ctx context.Context,
    ecosystemName string,
    packageName string,
    events []valueobject.RangeEvent,
) ([]string, error) {
    ctx, span := e.tracer.Start(ctx, "VersionEnumerator.Enumerate")
    defer span.End()
    
    helper := e.ecosystems.Get(ecosystemName)
    if helper == nil {
        return nil, domain.ErrUnknownEcosystem
    }
    
    // Fetch all known versions from ecosystem registry
    allVersions, err := helper.EnumerateVersions(ctx, packageName)
    if err != nil {
        return nil, fmt.Errorf("enumerate versions for %s/%s: %w", ecosystemName, packageName, err)
    }
    
    // Filter versions that fall within affected ranges
    var affected []string
    for _, v := range allVersions {
        if e.isVersionAffected(v, events, helper) {
            affected = append(affected, v)
        }
    }
    
    return affected, nil
}

func (e *VersionEnumerator) isVersionAffected(
    version string,
    events []valueobject.RangeEvent,
    helper ecosystem.Helper,
) bool {
    inRange := false
    
    for _, event := range sortEventsByVersion(events, helper) {
        versionKey := helper.SortKey(version)
        eventKey := helper.SortKey(event.Value)
        
        switch event.Type {
        case "introduced":
            if versionKey >= eventKey {
                inRange = true
            }
        case "fixed":
            if versionKey >= eventKey {
                inRange = false
            }
        case "last_affected":
            if versionKey > eventKey {
                inRange = false
            }
        case "limit":
            if versionKey >= eventKey {
                inRange = false
            }
        }
    }
    
    return inRange
}
```

---

## 7. Application — Analyze Vulnerability Command

```go
// application/command/analyze_git_impact/handler.go
package analyze_git_impact

type Command struct {
    VulnID      string
    ContentHash string   // Staleness guard
    RepoURL     string
    Events      []RangeEvent
    Options     AnalysisOptions
}

type Handler struct {
    bisector       *service.GitBisector
    enumerator     *service.VersionEnumerator
    eventPublisher port.EventPublisher
    tracer         trace.Tracer
    logger         *zerolog.Logger
}

func (h *Handler) Handle(ctx context.Context, cmd Command) (*Result, error) {
    ctx, span := h.tracer.Start(ctx, "AnalyzeGitImpact")
    defer span.End()
    
    span.SetAttributes(
        attribute.String("vuln_id", cmd.VulnID),
        attribute.String("repo_url", cmd.RepoURL),
    )
    
    // 1. Git bisection
    bisectResult, err := h.bisector.Bisect(ctx, cmd.RepoURL, cmd.Events, service.BisectionOptions{
        DetectCherryPicks:   cmd.Options.DetectCherryPicks,
        ConsiderAllBranches: cmd.Options.ConsiderAllBranches,
    })
    if err != nil {
        return nil, fmt.Errorf("bisection failed: %w", err)
    }
    
    // 2. Publish result event
    evt := &event.ImpactAnalysisCompleted{
        VulnID:          cmd.VulnID,
        ContentHash:     cmd.ContentHash,
        AffectedCommits: bisectResult.AffectedCommits,
        HasChanges:      bisectResult.HasChanges,
        OccurredAt:      time.Now().UTC(),
    }
    
    if err := h.eventPublisher.Publish(ctx, evt); err != nil {
        h.logger.Error().Err(err).Str("vuln_id", cmd.VulnID).Msg("failed to publish event")
    }
    
    return &Result{
        VulnID:          cmd.VulnID,
        AffectedCommits: bisectResult.AffectedCommits,
        HasChanges:      bisectResult.HasChanges,
    }, nil
}
```

---

## 8. Ecosystem Helper Interface

```go
// domain/repository/ecosystem_fetcher.go
package repository

// EcosystemHelper provides version comparison and enumeration per ecosystem.
// Implemented for 30+ ecosystems.
type EcosystemHelper interface {
    // Compare two version strings. Returns -1, 0, or 1.
    Compare(v1, v2 string) int
    
    // SortKey returns a comparable key for ordering versions.
    SortKey(version string) string
    
    // IsValid returns true if version string is valid for this ecosystem.
    IsValid(version string) bool
    
    // EnumerateVersions fetches all known versions from the package registry.
    EnumerateVersions(ctx context.Context, packageName string) ([]string, error)
    
    // NextVersion returns the next version after the given version.
    // Used for range enumeration.
    NextVersion(ctx context.Context, packageName, version string) (string, error)
}

// Registry maps ecosystem name → EcosystemHelper
type EcosystemRegistry interface {
    Get(name string) EcosystemHelper
    List() []string
}
```

---

## 9. Event Schema

```go
// Outbound event: ImpactAnalysisCompleted
// Topic: osv.impact.analysis.completed

type ImpactAnalysisCompleted struct {
    EventID     string    `json:"event_id"`
    EventType   string    `json:"event_type"`  // "osv.impact.analysis.completed"
    OccurredAt  time.Time `json:"occurred_at"`
    
    // Correlation
    VulnID      string    `json:"vuln_id"`
    ContentHash string    `json:"content_hash"`  // For staleness detection in consumer
    
    // Results
    HasChanges      bool     `json:"has_changes"`
    AffectedCommits []string `json:"affected_commits"`
    AffectedVersions map[string][]string `json:"affected_versions"` // ecosystem:package → versions
    
    // Metadata
    DurationMs  int64  `json:"duration_ms"`
    ReposCloned int    `json:"repos_cloned"`
}

// Inbound events consumed:
// - osv.vuln.imported (trigger: new vuln needs analysis)
// - osv.source.sync.requested (trigger: batch analysis)
```

---

## 10. Resilience Patterns

```go
// Git clone operations can be slow (large repos).
// Apply timeout + circuit breaker per repo.

type RepoFetchConfig struct {
    CloneTimeout    time.Duration // Default: 30min for large repos
    FetchTimeout    time.Duration // Default: 5min for updates
    MaxRepoSizeMB   int          // Default: 10GB
    MaxConcurrency  int          // Default: 5 concurrent clones
}

// Circuit breaker per repo URL
// After 3 consecutive failures: open circuit for 10 minutes
// Prevents hammering unavailable repos

// Retry config:
// - Network errors: 3 retries with exponential backoff
// - Rate limiting: respect Retry-After header
// - Non-retryable: repo not found, permission denied
```

---

## 11. GCS Repo Cache Design

```
Cache Structure in GCS:
  gs://osv-repo-cache/
  └── {sha256(repo_url)}/
      ├── repo.git/            # Bare git clone (or bundle)
      ├── metadata.json        # {cloned_at, head_commit, size_bytes}
      └── refs.json            # {branches, tags} snapshot

Cache Strategy:
  1. Check local ephemeral disk first (fast)
  2. If miss, download from GCS (slower but persistent)
  3. Always git fetch to get latest commits
  4. After use, upload updated clone to GCS

Eviction:
  - LRU based on last_accessed_at in metadata
  - Max GCS usage: 500GB total
  - Per-repo TTL: 7 days if unused
```

---

## 12. SLO Targets

| Metric | Target |
|--------|--------|
| Availability | 99.9% |
| Git analysis P50 | < 30s per vulnerability |
| Git analysis P99 | < 10min per vulnerability |
| Version enumeration P50 | < 5s per package |
| Event publication latency | < 500ms |
| Repo cache hit rate | > 70% |
| Concurrent analysis | 10 simultaneous jobs |

---

## 13. Implementation Status

> **Status:** ✅ Core Implemented | **Updated:** 2026-06-01

### Implemented
- [x] `domain/service/bisector.go` — GitBisector (LogBetween, resolveHash, cherry-pick detection)
- [x] `domain/entity/entity.go` — AffectedResult, Commit entity
- [x] `domain/valueobject/valueobject.go` — CommitHash, RangeEvent (introduced/fixed/limit/last_affected), Ecosystem
- [x] `domain/repository/repository.go` — GitRepoCache, EcosystemVersionFetcher interfaces
- [x] `application/command/analyze_vulnerability/handler.go` — Full handler (git + version fan-out sem=10)
- [x] `infra/git/gogit_adapter.go` — go-git clone, fetch, walk commits, diff
- [x] `infra/messaging/nats/messaging.go` — NATS publisher + VulnImportedConsumer (MaxDeliver=5, AckWait=30min)
- [x] `cmd/server/main.go` — Service entry point + graceful shutdown
- [x] `Dockerfile`, `go.mod`

### Pending
- [ ] `domain/service/version_enumerator.go` — Ecosystem version enumeration (PyPI, Go, npm, Maven)
- [ ] `domain/service/cherrypick_detector.go` — Parse git trailers for cherry-pick markers
- [ ] `infra/ecosystem/` — EcosystemHelper implementations for 30+ ecosystems
- [ ] `infra/storage/gcs/repo_cache_store.go` — GCS-backed repo clone cache
- [ ] `infra/cache/redis/metadata_cache.go` — Repo metadata caching
- [ ] `interface/grpc/handler/impact_handler.go` — gRPC handler (proto-gen)
- [ ] Unit tests for bisector, version enumerator
- [ ] Integration tests, Makefile

### Deviations from Spec
- AnalyzeVulnerabilityHandler combines git + version analysis in single command (not split into 3 separate commands per spec)
- Kafka publisher listed as alternative; only NATS implemented
