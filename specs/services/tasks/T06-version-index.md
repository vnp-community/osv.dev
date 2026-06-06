# Task T06 — Version Index Service

> **Priority:** P1 | **Phase:** 3 | **Spec:** `specs/services/05-version-index-service.md`  
> **Depends on:** T00-shared-libs, T12-infrastructure (NATS, Firestore, GCS)

## Mục Tiêu
Thay thế Go Indexer cũ. Index file hashes của git repos theo 512-bucket scheme, phục vụ `DetermineVersion` API.

## Trách Nhiệm
- Controller mode: phát hiện repos cần re-index, publish tasks
- Worker mode: clone repo, hash files, lưu 512 buckets vào Firestore
- Serve `DetermineVersion` queries (file hashes → version matches)
- Re-index khi có new tags/versions

## Cấu Trúc File

```
services/version-index/
├── cmd/
│   ├── controller/main.go   # Schedule indexing tasks
│   └── worker/main.go       # Execute indexing tasks
├── internal/
│   ├── domain/
│   │   ├── aggregate/repo_index/
│   │   │   ├── repo_index.go           # {RepoURL, LatestTag, IndexedAt, FileCount}
│   │   │   └── repo_index_test.go
│   │   ├── entity/
│   │   │   ├── repo_index_bucket.go    # {RepoURL, Version, BucketIdx, BucketHash}
│   │   │   └── file_hash.go
│   │   ├── valueobject/
│   │   │   ├── bucket_hash.go          # MD5 of sorted hashes in a bucket
│   │   │   ├── bucket_index.go         # 0-511
│   │   │   ├── version_match.go        # {RepoURL, Version, Score float64}
│   │   │   └── repo_config.go          # Config loaded from textproto
│   │   ├── service/
│   │   │   ├── bucket_hasher.go        # Core 512-bucket algorithm
│   │   │   ├── version_scorer.go       # Log-formula scoring
│   │   │   └── repo_scanner.go         # Walk repo files + compute MD5
│   │   └── repository/
│   │       ├── repo_index_repo.go
│   │       └── repo_index_bucket_repo.go
│   ├── application/
│   │   ├── command/index_repository/{command,handler,handler_test}.go
│   │   └── query/
│   │       ├── determine_version/{query,handler,handler_test}.go
│   │       └── list_indexed_repos/{query,handler}.go
│   └── infra/
│       ├── persistence/firestore/
│       │   ├── repo_index_repo.go
│       │   └── repo_index_bucket_repo.go
│       ├── git/repo_cloner.go
│       ├── storage/gcs/
│       │   ├── repo_cache_store.go
│       │   └── config_loader.go        # Load textproto configs
│       └── messaging/nats/
│           ├── task_publisher.go
│           └── task_consumer.go
├── interface/
│   ├── grpc/
│   │   ├── handler/version_index_handler.go
│   │   └── proto/version_index_service.proto
│   └── http/handler/health_handler.go
└── config/config.go
```

## Core Algorithm — Bucket Hashing (QUAN TRỌNG)

```go
// domain/service/bucket_hasher.go
const NumBuckets = 512

type BucketSet struct {
    Buckets [NumBuckets][]string  // sorted file hashes per bucket
    Bitmap  [NumBuckets]bool      // true = non-empty
}

func (h *BucketHasher) Hash(fileHashes map[string]string) *BucketSet:
  // For each file hash:
  //   hashBytes = hex.DecodeString(hash)
  //   bucketIdx = int(binary.BigEndian.Uint16(hashBytes[:2])) % NumBuckets
  //   Append hash to bs.Buckets[bucketIdx]
  // Sort each bucket's hashes (deterministic)

func (h *BucketHasher) ComputeBucketHash(hashes []string) string:
  // MD5 of concatenated sorted hashes
  // hasher := md5.New()
  // for _, h := range hashes { hasher.Write([]byte(h)) }
  // return hex.EncodeToString(hasher.Sum(nil))
```

## Core Algorithm — Version Scorer (QUAN TRỌNG)

```go
// domain/service/version_scorer.go
const MinScoreThreshold = 0.05
const MaxResults = 10

type ScoringInput struct {
    QueryFileCount int
    IndexFileCount int
    BucketMatches  int  // buckets with identical hashes
    EmptyBuckets   int  // buckets empty in both
    MissedEmpty    int  // empty in query, not in index
    SkippedBuckets int  // >100 matches, too noisy
}

func (s *VersionScorer) Score(input ScoringInput) float64:
  numBucketChange := NumBuckets - BucketMatches - EmptyBuckets + MissedEmpty - SkippedBuckets
  fileDiff := abs(QueryFileCount - IndexFileCount)
  estimatedDiff = estimateDiff(numBucketChange, fileDiff)
  maxFiles := max(QueryFileCount, IndexFileCount)
  score := float64(maxFiles - estimatedDiff) / float64(maxFiles)
  return max(0, score)

func estimateDiff(numBucketChange, fileDiff int) int:
  // Log formula: estimate files changed from bucket changes
  estimate := NumBuckets * log((NumBuckets+1) / (NumBuckets-numBucketChange+1))
  additional := max(estimate - fileDiff, 0) / 2
  return fileDiff + round(additional)
```

## DetermineVersion Query Handler

```go
// application/query/determine_version/handler.go
// Input: FileHashes map[path]md5Hash (from client project)
// Output: []VersionMatch sorted by score desc

func Handle(ctx, q Query) (*Result, error):
  // 1. Compute BucketSet from q.FileHashes
  // 2. For each non-empty bucket (parallel, sem=50):
  //    bucketHash = ComputeBucketHash(bucket.hashes)
  //    matches = repo_index_bucket_repo.QueryByBucketHash(ctx, bucketIdx, bucketHash, 100)
  // 3. Aggregate: group by "repoURL@version", count BucketMatches
  // 4. For each candidate: get RepoIndex (file count), compute Score
  // 5. Filter score >= 0.05, sort desc, take top 10
```

## Indexing Pipeline

```
Controller Mode (scheduled):
  1. Load repo configs from GCS (textproto files listing repos to index)
  2. For each repo: clone/fetch → walk tags/branches
  3. Check if tag already indexed (RepoIndex entity)
  4. If not indexed: publish IndexingTask to NATS topic "osv.version.index.tasks"

Worker Mode (event-driven):
  1. Subscribe NATS "osv.version.index.tasks"
  2. For each task:
     a. Clone target repo at specific tag (from GCS cache)
     b. Walk all files: compute MD5 hash per file
     c. Compute 512 bucket hashes via BucketHasher
     d. Write RepoIndexBucket entities to Firestore
     e. Write RepoIndex entity (metadata)
  3. Max concurrent workers: 10
```

## Proto

```protobuf
service VersionIndexService {
  rpc DetermineVersion(DetermineVersionRequest) returns (DetermineVersionResponse);
  rpc ListIndexedRepos(ListIndexedReposRequest) returns (ListIndexedReposResponse);
  rpc TriggerReindex(TriggerReindexRequest) returns (TriggerReindexResponse);
}
message DetermineVersionRequest {
  repeated FileHash file_hashes = 1;
}
message FileHash { string file_path = 1; string hash = 2; }  // MD5 hex
message DetermineVersionResponse {
  repeated VersionMatch matches = 1;
}
message VersionMatch {
  string repo_address = 1; string version = 2;
  double score = 3; int32 minimum_file_count_seen = 4;
}
```

## Firestore Schema
```
repo-indexes/{sha256(repoURL@version)}:
  repo_url: string
  version: string
  file_count: int
  indexed_at: timestamp

repo-index-buckets/{sha256(repoURL@version@bucketIdx)}:
  repo_url: string
  version: string
  bucket_idx: int     # 0-511
  bucket_hash: string # MD5
```

## SLO Targets
- DetermineVersion P50: <200ms, P99: <2s
- Indexing throughput: >50 repos/hour
- Index freshness: <24h lag for new releases

## Checklist Thực Thi

> **Status: ✅ COMPLETED (Core)** — 2026-06-01

- [x] Implement `BucketHasher` với `Hash()` và `ComputeBucketHash()` (critical algorithm)
- [x] Implement `VersionScorer` với log-formula scoring (`estimateDiff`, ScoringInput)
- [x] `domain/repository`: RepoIndexRepo, RepoIndexBucketRepo, BucketMatch interfaces
- [x] Implement `DetermineVersionHandler` (parallel bucket fan-out sem=50 → aggregate → score → top-10)
- [x] Unit tests: `BucketHasher` deterministic + bucket assignment, `VersionScorer` perfect/no-match
- [x] `Dockerfile` (2 binaries: controller + worker, multi-stage)
- [x] `go.mod` + workspace entry
- [ ] Implement `RepoScanner` (walk repo files, compute MD5)
- [ ] Implement `RepoIndex` aggregate (indexed_at, file_count, latest_tag)
- [ ] Implement Firestore repos: `RepoIndexRepo`, `RepoIndexBucketRepo`
- [ ] Implement `IndexRepositoryHandler` (full indexing pipeline)
- [ ] Implement NATS publisher (controller) + consumer (worker)
- [ ] Implement `GCSRepoCacheStore` (reuse from T05 pattern)
- [ ] Implement GCS `ConfigLoader` (load repo configs từ textproto)
- [ ] Controller `cmd/controller/main.go`
- [ ] Worker `cmd/worker/main.go`
- [ ] gRPC handler + proto (`version_index_service.proto`)
- [ ] Integration test: index mock repo → DetermineVersion → verify match
- [ ] Makefile
