# Task T05 — Impact Analysis Service

> **Priority:** P1 | **Phase:** 3 | **Spec:** `specs/services/04-impact-analysis-service.md`  
> **Depends on:** T00-shared-libs, T12-infrastructure (NATS, Redis, GCS)

## Mục Tiêu
Phân tích ảnh hưởng của vulnerability: git bisection để tìm affected commits, version enumeration từ SEMVER/ECOSYSTEM ranges.

## Trách Nhiệm
- Git bisection: affected_commits[] từ GIT ranges (introduced/fixed)
- Version enumeration: versions[] từ SEMVER/ECOSYSTEM ranges
- Cherry-pick detection
- Publish `ImpactAnalysisCompleted` event
- gRPC API cho Ingestion Service (sync call)
- Consume `VulnImported` events từ NATS (async trigger)

## Không Làm
- Persist vulnerability data, query vulns, file hash indexing

## Cấu Trúc File

```
services/impact-analysis/
├── cmd/server/main.go
├── internal/
│   ├── domain/
│   │   ├── aggregate/
│   │   │   ├── git_range/git_range.go
│   │   │   └── version_range/version_range.go
│   │   ├── entity/
│   │   │   ├── affected_result.go    # {AffectedCommits, AffectedVersions, HasChanges}
│   │   │   └── commit.go
│   │   ├── valueobject/
│   │   │   ├── commit_hash.go
│   │   │   ├── version_range_event.go  # introduced/fixed/limit/last_affected
│   │   │   ├── ecosystem.go
│   │   │   └── content_hash.go
│   │   ├── service/
│   │   │   ├── git_bisector.go         # Core git bisection
│   │   │   ├── version_enumerator.go   # Enumerate affected versions
│   │   │   ├── cherrypick_detector.go
│   │   │   └── range_evaluator.go      # Is version in range?
│   │   └── repository/
│   │       ├── git_repo_cache.go       # Interface
│   │       └── ecosystem_fetcher.go    # Interface
│   ├── application/
│   │   ├── command/
│   │   │   ├── analyze_git_impact/{command,handler,handler_test}.go
│   │   │   ├── analyze_version_impact/{command,handler}.go
│   │   │   └── detect_cherrypicks/{command,handler}.go
│   │   └── port/
│   │       ├── event_publisher.go
│   │       └── git_provider.go
│   └── infra/
│       ├── git/
│       │   ├── gogit_adapter.go        # go-git implementation
│       │   └── repo_cache.go           # Local + GCS cache
│       ├── ecosystem/
│       │   ├── registry.go
│       │   ├── pypi/pypi_helper.go
│       │   ├── golang/go_helper.go
│       │   ├── npm/npm_helper.go
│       │   └── maven/maven_helper.go
│       ├── storage/gcs/repo_cache_store.go
│       ├── cache/redis/metadata_cache.go
│       └── messaging/nats/
│           ├── event_publisher.go
│           └── consumer.go             # Consume VulnImported
├── interface/
│   ├── grpc/
│   │   ├── handler/impact_handler.go
│   │   └── proto/impact_service.proto
│   └── http/handler/health_handler.go
└── config/config.go
```

## Proto Definition

```protobuf
service ImpactAnalysisService {
  rpc AnalyzeGitImpact(AnalyzeGitImpactRequest) returns (AnalyzeGitImpactResponse);
  rpc AnalyzeVersionImpact(AnalyzeVersionImpactRequest) returns (AnalyzeVersionImpactResponse);
  rpc DetectCherryPicks(DetectCherryPicksRequest) returns (DetectCherryPicksResponse);
  rpc AnalyzeVulnerability(AnalyzeVulnerabilityRequest) returns (AnalyzeVulnerabilityResponse);
}

message AnalyzeVulnerabilityRequest {
  string vuln_id      = 1;
  string content_hash = 2;  // staleness detection
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
  string vuln_id; string content_hash; bool has_changes;
  repeated AffectedResult results;
  AnalysisStats stats;
}
message AffectedResult {
  string package_ecosystem; string package_name;
  repeated string commits;   // affected git hashes
  repeated string versions;  // affected version strings
}
```

## Git Bisector (Core Logic)

```go
// domain/service/git_bisector.go
type GitBisector struct {
    repoCache repository.GitRepoCache
    tracer    trace.Tracer
}
type BisectionResult struct {
    AffectedCommits []string
    HasChanges      bool
}

func (b *GitBisector) Bisect(ctx, repoURL, events []RangeEvent, opts BisectionOptions) (*BisectionResult, error):
  // 1. Get repo from cache (clone nếu cần, fetch để update)
  // 2. Resolve introduced/fixed commit hashes
  // 3. Walk git history giữa introduced và fixed (git log)
  // 4. Cherry-pick detection nếu opts.DetectCherryPicks
  // 5. Return deduped commit list
```

## Version Enumerator (Core Logic)

```go
// domain/service/version_enumerator.go
func (e *VersionEnumerator) Enumerate(ctx, ecosystemName, packageName, events []RangeEvent) ([]string, error):
  // 1. Get EcosystemHelper cho ecosystemName
  // 2. helper.EnumerateVersions(ctx, packageName) → all known versions
  // 3. For each version: isVersionAffected(version, events, helper)
  // 4. Return affected versions

func isVersionAffected(version string, events []RangeEvent, helper) bool:
  // Sort events by version (using helper.SortKey)
  // Walk: introduced → inRange=true; fixed/limit/last_affected → inRange=false
  // Return final inRange state
```

## GCS Repo Cache Design

```
gs://osv-repo-cache/
└── {sha256(repo_url)}/
    ├── repo.git/       # Bare git clone
    ├── metadata.json   # {cloned_at, head_commit, size_bytes}
    └── refs.json       # {branches, tags}

Strategy:
  1. Check local ephemeral disk (fast)
  2. Miss → download from GCS
  3. git fetch to get latest
  4. After use → upload to GCS

Eviction:
  - LRU by last_accessed_at
  - Max total: 500GB
  - Per-repo TTL: 7 days if unused
```

## Resilience Config

```go
type RepoFetchConfig struct {
    CloneTimeout   time.Duration  // Default: 30min
    FetchTimeout   time.Duration  // Default: 5min
    MaxRepoSizeMB  int           // Default: 10000 (10GB)
    MaxConcurrency int           // Default: 5
}
// Circuit breaker per repo URL: 3 consecutive failures → open 10min
// Retry: 3 retries, exponential backoff (network errors only)
// Non-retryable: repo not found, permission denied
```

## Events

```go
// Outbound: osv.impact.analysis.completed
type ImpactAnalysisCompleted struct {
    EventID          string    `json:"event_id"`
    OccurredAt       time.Time `json:"occurred_at"`
    VulnID           string    `json:"vuln_id"`
    ContentHash      string    `json:"content_hash"`  // staleness guard
    HasChanges       bool      `json:"has_changes"`
    AffectedCommits  []string  `json:"affected_commits"`
    AffectedVersions map[string][]string `json:"affected_versions"` // "ecosystem:pkg" → versions
    DurationMs       int64     `json:"duration_ms"`
}
// Inbound consumed: osv.vuln.imported → trigger AnalyzeVulnerability
```

## SLO Targets
- Git analysis P50: <30s/vuln, P99: <10min
- Version enumeration P50: <5s/package
- Repo cache hit: >70%
- Max concurrent analyses: 10

## Checklist Thực Thi

> **Status: ✅ COMPLETED (Core)** — 2026-06-01

- [x] Implement `GitBisector` (go-git: resolve commits, walk history, `LogBetween`, cherry-pick detection)
- [x] Implement `VersionEnumerator` với `isVersionAffected` logic (introduced/fixed/limit/last_affected)
- [x] Implement `CherrypickDetector` (parse `(cherry picked from commit ...)` trailers)
- [x] Implement `parseWindows` — convert flat event list → (introduced, fixed) pairs
- [x] `domain/entity`: AffectedResult, Commit, AnalysisStats
- [x] `domain/valueobject`: CommitHash (validate SHA-1/SHA-256), VersionRangeEvent, EcosystemName, ContentHash
- [x] `domain/repository`: GitRepoCache, GitRepo, EcosystemVersionFetcher interfaces
- [x] `application/command/analyze_vulnerability/handler.go` — orchestrate git + version, fan-out sem(10)
- [x] `AnalysisOptions`: DetectCherryPicks, VersionsFromRepo, ConsiderAllBranches, IgnoreGit
- [x] Publish `ImpactAnalysisCompleted` event (goroutine, fire-and-forget)
- [x] `infra/git/gogit_adapter.go` — `LocalRepoCache` (clone, open, cache, LogBetween, resolveHash)
- [x] `infra/messaging/nats/messaging.go` — EventPublisher + VulnImportedConsumer (Pull, MaxDeliver=5, AckWait=30min)
- [x] `cmd/server/main.go` — graceful shutdown, wire all deps
- [x] `Dockerfile` (multi-stage → distroless)
- [x] `go.mod` + workspace entry
- [ ] `infra/ecosystem/` — PyPI, Go, npm, Maven EcosystemHelper implementations
- [ ] `infra/storage/gcs/repo_cache_store.go` — GCS-backed repo cache (L1 local + L2 GCS)
- [ ] `infra/cache/redis/metadata_cache.go`
- [ ] gRPC handler + proto generation (`impact_service.proto`)
- [ ] Unit tests: `isVersionAffected`, `parseWindows`, bisection logic
- [ ] Integration tests: mock git repos + NATS
- [ ] Makefile
