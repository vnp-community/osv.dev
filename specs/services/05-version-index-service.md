# Service 05 — Version Index Service

> **Version:** 1.0 | **Status:** Proposed | **Priority:** P1  
> **Language:** Go  
> **Pattern:** CQRS + Clean Architecture  
> **Communication:** gRPC (query) + NATS (indexing tasks)

---

## 1. Trách Nhiệm

Service thay thế **Indexer Go service** (`gcp/indexer/`) trong kiến trúc cũ. Xử lý **file hash indexing** cho git repositories và phục vụ `DetermineVersion` API.

**Responsibilities:**
- Controller mode: phát hiện repositories cần index mới
- Worker mode: clone repo, hash files, lưu bucket indexes vào Firestore
- Serve `DetermineVersion` queries (file hash → version matching)
- Manage `RepoIndex` và `RepoIndexBucket` entities
- Re-index khi repos có new tags/versions

**NOT Responsible for:**
- Git bisection (Impact Analysis Service)
- Vulnerability querying (Query Service)
- Source sync (Source Sync Service)

---

## 2. Clean Architecture Layers

```
Domain:
  ├── RepoIndex aggregate (metadata about indexed repo)
  ├── RepoIndexBucket entity (512 buckets per repo version)
  ├── FileBucket value object (hash of sorted file hashes per bucket)
  ├── VersionMatch value object (match score + repo info)
  └── Repository: RepoIndexRepository, RepoIndexBucketRepository

Application:
  ├── IndexRepositoryCommand + Handler    (write)
  ├── DetermineVersionQuery + Handler     (read)
  └── ListIndexedReposQuery + Handler     (read)

Infrastructure:
  ├── FirestoreRepoIndexRepo
  ├── GCSRepoCacheStore
  ├── NATSIndexTaskPublisher + Consumer
  └── TextProtoConfigLoader (repo configs from GCS)

Interface:
  ├── gRPC handler (VersionIndexService)
  └── NATS consumer (index tasks)
```

---

## 3. Directory Structure

```
services/version-index/
├── cmd/
│   ├── controller/
│   │   └── main.go              # Controller mode: schedule indexing
│   └── worker/
│       └── main.go              # Worker mode: execute indexing
├── internal/
│   ├── domain/
│   │   ├── aggregate/
│   │   │   └── repo_index/
│   │   │       ├── repo_index.go           # Aggregate root
│   │   │       └── repo_index_test.go
│   │   ├── entity/
│   │   │   ├── repo_index_bucket.go        # 512 buckets per version
│   │   │   └── file_hash.go
│   │   ├── valueobject/
│   │   │   ├── bucket_hash.go              # MD5 of sorted hashes in bucket
│   │   │   ├── bucket_index.go             # 0-511 bucket number
│   │   │   ├── version_match.go            # Match result with score
│   │   │   └── repo_config.go              # Repository config (from textproto)
│   │   ├── service/
│   │   │   ├── bucket_hasher.go            # Core bucket hashing algorithm
│   │   │   ├── version_scorer.go           # Score calculation (log formula)
│   │   │   └── repo_scanner.go             # Scan repo files + hash
│   │   └── repository/
│   │       ├── repo_index_repo.go          # Interface
│   │       └── repo_index_bucket_repo.go   # Interface
│   ├── application/
│   │   ├── command/
│   │   │   └── index_repository/
│   │   │       ├── command.go
│   │   │       ├── handler.go
│   │   │       └── handler_test.go
│   │   └── query/
│   │       ├── determine_version/
│   │       │   ├── query.go
│   │       │   ├── handler.go
│   │       │   └── handler_test.go
│   │       └── list_indexed_repos/
│   │           ├── query.go
│   │           └── handler.go
│   └── infra/
│       ├── persistence/
│       │   └── firestore/
│       │       ├── repo_index_repo.go
│       │       └── repo_index_bucket_repo.go
│       ├── git/
│       │   └── repo_cloner.go
│       ├── storage/
│       │   └── gcs/
│       │       ├── repo_cache_store.go
│       │       └── config_loader.go       # Load textproto configs
│       └── messaging/
│           └── nats/
│               ├── task_publisher.go
│               └── task_consumer.go
├── interface/
│   ├── grpc/
│   │   ├── handler/
│   │   │   └── version_index_handler.go
│   │   └── proto/
│   │       └── version_index_service.proto
│   └── http/
│       └── handler/
│           └── health_handler.go
├── config/config.go
├── Dockerfile
└── go.mod
```

---

## 4. Proto Definition

```protobuf
// proto/version_index_service.proto
syntax = "proto3";
package osv.versionindex.v1;

service VersionIndexService {
  // Determine which version(s) a set of file hashes match
  rpc DetermineVersion(DetermineVersionRequest) returns (DetermineVersionResponse);
  
  // List all indexed repositories
  rpc ListIndexedRepos(ListIndexedReposRequest) returns (ListIndexedReposResponse);
  
  // Admin: trigger re-indexing of a specific repo
  rpc TriggerReindex(TriggerReindexRequest) returns (TriggerReindexResponse);
}

message DetermineVersionRequest {
  repeated FileHash file_hashes = 1;  // From client's local project
}

message FileHash {
  string file_path = 1;  // Relative path in project
  string hash      = 2;  // MD5 hex string
}

message DetermineVersionResponse {
  repeated VersionMatch matches = 1;
}

message VersionMatch {
  string repo_address = 1;   // e.g., "https://github.com/org/repo"
  string version      = 2;   // Matched version tag
  double score        = 3;   // 0.0-1.0 match quality
  int32  minimum_file_count_seen = 4;
}

message ListIndexedReposRequest {
  int32 page_size  = 1;
  string page_token = 2;
}

message ListIndexedReposResponse {
  repeated RepoIndexInfo repos = 1;
  string next_page_token = 2;
}

message RepoIndexInfo {
  string repo_url     = 1;
  string latest_tag   = 2;
  int64  file_count   = 3;
  string indexed_at   = 4;
}
```

---

## 5. Core Algorithm — Bucket Hashing

```go
// domain/service/bucket_hasher.go
package service

const NumBuckets = 512

// BucketHasher computes the 512-bucket hash representation
// of a set of file hashes. This is the core of DetermineVersion.
type BucketHasher struct{}

type BucketSet struct {
    Buckets  [NumBuckets][]string  // Sorted file hashes per bucket
    Bitmap   [NumBuckets]bool      // true = non-empty bucket
}

// Hash assigns each file hash to a bucket and computes aggregate hashes.
func (h *BucketHasher) Hash(fileHashes map[string]string) *BucketSet {
    bs := &BucketSet{}
    
    for _, hash := range fileHashes {
        // Assign to bucket using first 2 bytes of MD5 hash
        hashBytes, _ := hex.DecodeString(hash)
        bucketIdx := int(binary.BigEndian.Uint16(hashBytes[:2])) % NumBuckets
        bs.Buckets[bucketIdx] = append(bs.Buckets[bucketIdx], hash)
    }
    
    // Sort hashes within each bucket (deterministic)
    for i := range bs.Buckets {
        sort.Strings(bs.Buckets[i])
        bs.Bitmap[i] = len(bs.Buckets[i]) > 0
    }
    
    return bs
}

// ComputeBucketHash returns MD5 of sorted hashes in a bucket.
func (h *BucketHasher) ComputeBucketHash(hashes []string) string {
    hasher := md5.New()
    for _, hash := range hashes { // Already sorted
        hasher.Write([]byte(hash))
    }
    return hex.EncodeToString(hasher.Sum(nil))
}
```

---

## 6. Core Algorithm — Version Scorer

```go
// domain/service/version_scorer.go
package service

// VersionScorer calculates match quality score using log-based formula
// from the original Python implementation.
type VersionScorer struct{}

type ScoringInput struct {
    QueryFileCount   int
    IndexFileCount   int
    BucketMatches    int  // Buckets with identical hashes
    EmptyBuckets     int  // Buckets empty in both query and index
    MissedEmpty      int  // Buckets empty in query but not in index
    SkippedBuckets   int  // Buckets with > 100 matches (noisy, skipped)
}

func (s *VersionScorer) Score(input ScoringInput) float64 {
    numBucketChange := NumBuckets -
        input.BucketMatches -
        input.EmptyBuckets +
        input.MissedEmpty -
        input.SkippedBuckets
    
    fileDiff := abs(input.QueryFileCount - input.IndexFileCount)
    
    estimatedDiff := s.estimateDiff(numBucketChange, fileDiff)
    
    maxFiles := max(input.QueryFileCount, input.IndexFileCount)
    if maxFiles == 0 {
        return 0
    }
    
    score := float64(maxFiles-estimatedDiff) / float64(maxFiles)
    return math.Max(0, score)
}

func (s *VersionScorer) estimateDiff(numBucketChange, fileDiff int) int {
    // Log formula: estimate how many files changed based on bucket changes
    estimate := NumBuckets * math.Log(
        float64(NumBuckets+1)/float64(NumBuckets-numBucketChange+1),
    )
    additional := math.Max(estimate-float64(fileDiff), 0) / 2
    return fileDiff + int(math.Round(additional))
}

const MinScoreThreshold = 0.05
const MaxResults = 10
```

---

## 7. Application — DetermineVersion Query

```go
// application/query/determine_version/handler.go
package determine_version

type Query struct {
    FileHashes map[string]string  // path → MD5 hash
}

type Handler struct {
    bucketRepo  repository.RepoIndexBucketRepository
    indexRepo   repository.RepoIndexRepository
    hasher      *service.BucketHasher
    scorer      *service.VersionScorer
    tracer      trace.Tracer
}

func (h *Handler) Handle(ctx context.Context, q Query) (*Result, error) {
    ctx, span := h.tracer.Start(ctx, "DetermineVersion")
    defer span.End()
    
    // 1. Compute bucket hash set from query's file hashes
    bs := h.hasher.Hash(q.FileHashes)
    
    // 2. Query Firestore for each non-empty bucket in parallel
    type bucketResult struct {
        bucketIdx int
        matches   []*entity.RepoIndexBucket
    }
    
    resultCh := make(chan bucketResult, NumBuckets)
    sem := semaphore.NewWeighted(50) // Max 50 concurrent Firestore queries
    
    var wg sync.WaitGroup
    for i, hashes := range bs.Buckets {
        if len(hashes) == 0 {
            continue
        }
        
        wg.Add(1)
        go func(idx int, hashes []string) {
            defer wg.Done()
            sem.Acquire(ctx, 1)
            defer sem.Release(1)
            
            bucketHash := h.hasher.ComputeBucketHash(hashes)
            matches, err := h.bucketRepo.QueryByBucketHash(ctx, idx, bucketHash, 100)
            if err != nil {
                return
            }
            
            resultCh <- bucketResult{idx, matches}
        }(i, hashes)
    }
    
    go func() {
        wg.Wait()
        close(resultCh)
    }()
    
    // 3. Aggregate matches by repo+version
    matchCounts := map[string]*matchAccumulator{}
    for result := range resultCh {
        for _, bucket := range result.matches {
            key := fmt.Sprintf("%s@%s", bucket.RepoURL, bucket.Version)
            if _, ok := matchCounts[key]; !ok {
                matchCounts[key] = &matchAccumulator{RepoURL: bucket.RepoURL, Version: bucket.Version}
            }
            matchCounts[key].BucketMatches++
        }
    }
    
    // 4. Score each candidate
    var scored []valueobject.VersionMatch
    for _, acc := range matchCounts {
        idx, _ := h.indexRepo.GetByRepoVersion(ctx, acc.RepoURL, acc.Version)
        score := h.scorer.Score(service.ScoringInput{
            QueryFileCount: len(q.FileHashes),
            IndexFileCount: idx.FileCount,
            BucketMatches:  acc.BucketMatches,
        })
        
        if score >= MinScoreThreshold {
            scored = append(scored, valueobject.VersionMatch{
                RepoURL: acc.RepoURL,
                Version: acc.Version,
                Score:   score,
            })
        }
    }
    
    // 5. Sort by score desc, take top 10
    sort.Slice(scored, func(i, j int) bool {
        return scored[i].Score > scored[j].Score
    })
    if len(scored) > MaxResults {
        scored = scored[:MaxResults]
    }
    
    return &Result{Matches: scored}, nil
}
```

---

## 8. Indexing Pipeline

```
Controller Mode (scheduled, periodic):
  1. Load repo configs from GCS (textproto files)
  2. For each repo:
     a. Clone/fetch latest
     b. Walk tags/branches
     c. Check if tag already indexed (RepoIndex entity)
     d. If not indexed: publish IndexingTask to NATS

Worker Mode (event-driven):
  1. Subscribe to NATS topic: osv.version.index.tasks
  2. For each task:
     a. Clone target repo at specific tag (from GCS cache)
     b. Walk all files, compute MD5 hash
     c. Compute 512 bucket hashes via BucketHasher
     d. Write RepoIndexBucket entities to Firestore
     e. Write RepoIndex entity (metadata)
     f. Update GCS repo cache
  3. Parallel: up to 10 concurrent indexing workers
```

---

## 9. SLO Targets

| Metric | Target |
|--------|--------|
| Availability | 99.9% |
| DetermineVersion P50 | < 200ms |
| DetermineVersion P99 | < 2s |
| Indexing throughput | > 50 repos/hour |
| Index freshness | < 24h lag for new releases |
| Firestore write success | > 99.5% |

---

## 10. Implementation Status

> **Status:** ✅ Core Implemented | **Updated:** 2026-06-01

### Implemented
- [x] `domain/service/bucket_hasher.go` — BucketHasher (512-bucket algorithm, deterministic ComputeBucketHash)
- [x] `domain/service/version_scorer.go` — VersionScorer (log-formula estimateDiff, ScoringInput)
- [x] `domain/repository/repository.go` — RepoIndexRepo, RepoIndexBucketRepo, BucketMatch interfaces
- [x] `application/query/determine_version/handler.go` — DetermineVersionHandler (parallel fan-out sem=50 → aggregate → score → top-10)
- [x] Unit tests: BucketHasher determinism + bucket assignment, VersionScorer
- [x] `Dockerfile` (2 binaries: controller + worker), `go.mod`

### Pending
- [ ] `domain/service/repo_scanner.go` — Walk repo files, compute MD5 per file
- [ ] `domain/aggregate/repo_index/repo_index.go` — RepoIndex aggregate + RepoIndexBucket entity
- [ ] `infra/persistence/firestore/repo_index_repo.go` — Firestore RepoIndex + Bucket repos
- [ ] `infra/storage/gcs/repo_cache_store.go` — GCS-backed repo clone cache
- [ ] `infra/storage/gcs/config_loader.go` — Load textproto repo configs
- [ ] `application/command/index_repository/handler.go` — IndexRepository handler (full indexing pipeline)
- [ ] `infra/messaging/nats/task_publisher.go` + `task_consumer.go`
- [ ] `cmd/controller/main.go` + `cmd/worker/main.go`
- [ ] `interface/grpc/handler/version_index_handler.go` — gRPC handler
- [ ] Integration tests, Makefile

### Deviations from Spec
- Controller/Worker split is planned but not yet implemented; currently single binary
- RepoScanner uses go-git (not libgit2) to match Impact Analysis Service dependency
