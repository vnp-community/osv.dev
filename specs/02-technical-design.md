# OSV.dev — Technical Design Document (TDD)

> **Version:** 1.0  
> **Date:** 2026-05-31  
> **Status:** Draft  
> **Project:** Open Source Vulnerabilities (OSV.dev)  
> **Repository:** https://github.com/google/osv.dev

---

## 1. Giới Thiệu

### 1.1 Mục Đích Tài Liệu

Tài liệu này mô tả chi tiết kỹ thuật của từng component trong hệ thống OSV.dev, bao gồm:
- Design decisions và rationale
- Interface contracts giữa các services
- Data schemas và storage design
- Algorithm design cho các tính năng quan trọng
- Error handling và recovery strategies
- Testing approach

### 1.2 Phạm Vi

Tài liệu bao phủ toàn bộ hệ thống backend của OSV.dev:
- Core library (`osv/`)
- API Server (`gcp/api/`)
- Importer Worker (`gcp/workers/importer/`)
- Bisection/Impact Worker (`gcp/workers/worker/`)
- Indexer Service (`gcp/indexer/`)
- Website Backend (`gcp/website/`)
- Vuln Feeds Converters (`vulnfeeds/`)
- Go Shared Library (`go/`)

---

## 2. OSV Schema & Data Model

### 2.1 OSV Schema (v1.7.5)

OSV sử dụng schema chuẩn được định nghĩa tại https://ossf.github.io/osv-schema/. Đây là giao thức dữ liệu trung tâm của toàn bộ hệ thống.

**Vulnerability Record Structure:**

```json
{
  "schema_version": "1.7.5",
  "id": "GHSA-xxxx-xxxx-xxxx",
  "modified": "2024-01-15T00:00:00Z",
  "published": "2023-06-01T00:00:00Z",
  "withdrawn": null,
  "aliases": ["CVE-2023-12345"],
  "related": ["GHSA-yyyy-yyyy-yyyy"],
  "upstream": [],
  "summary": "Remote code execution in foo",
  "details": "A vulnerability in foo package...",
  "severity": [
    {
      "type": "CVSS_V3",
      "score": "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H"
    }
  ],
  "affected": [
    {
      "package": {
        "ecosystem": "PyPI",
        "name": "requests",
        "purl": "pkg:pypi/requests"
      },
      "ranges": [
        {
          "type": "ECOSYSTEM",
          "events": [
            {"introduced": "0"},
            {"fixed": "2.32.0"}
          ]
        },
        {
          "type": "GIT",
          "repo": "https://github.com/psf/requests",
          "events": [
            {"introduced": "0"},
            {"fixed": "abc123def456"}
          ]
        }
      ],
      "versions": ["2.25.0", "2.26.0", "2.27.0", "2.28.0"],
      "database_specific": {"source": "https://..."},
      "ecosystem_specific": {}
    }
  ],
  "references": [
    {"type": "ADVISORY", "url": "https://..."},
    {"type": "FIX", "url": "https://github.com/.../commit/abc123"}
  ],
  "credits": [
    {
      "name": "Security Researcher",
      "contact": ["https://twitter.com/researcher"],
      "type": "FINDER"
    }
  ],
  "database_specific": {}
}
```

**Range Types:**

| Type | Description | Events |
|------|-------------|--------|
| `GIT` | Git commit range | `introduced`, `fixed`, `last_affected`, `limit` |
| `SEMVER` | Semantic version range | same |
| `ECOSYSTEM` | Ecosystem-specific versions | same |

**Special Values:**
- `introduced: "0"` → vulnerability exists from the beginning
- `limit: "N"` → exclusive upper bound (not a fix, just a limit)
- `last_affected: "N"` → inclusive last affected version

### 2.2 Protobuf Definition

OSV sử dụng Protocol Buffers (`vulnerability.proto`) như representation nội bộ:

```protobuf
message Vulnerability {
  string schema_version = 10;
  string id = 1;
  google.protobuf.Timestamp modified = 2;
  google.protobuf.Timestamp published = 3;
  google.protobuf.Timestamp withdrawn = 12;
  repeated string aliases = 4;
  repeated string related = 5;
  repeated string upstream = 19;
  string summary = 6;
  string details = 7;
  repeated Affected affected = 8;
  repeated Reference references = 9;
  repeated Severity severity = 11;
  repeated Credit credits = 14;
  google.protobuf.Struct database_specific = 13;
}
```

**Design Decision: Protobuf vs JSON**
- Protobuf dùng cho internal representation (type-safe, efficient comparison)
- JSON/YAML dùng cho storage và external API (human-readable, interoperable)
- Conversion qua `json_format.ParseDict()` và `json_format.MessageToDict()`

### 2.3 Datastore Entity Design

#### 2.3.1 Bug Entity

```python
class Bug(ndb.Model):
    # Primary key: db_id (e.g., "GHSA-xxxx-xxxx-xxxx")
    db_id: str = ndb.StringProperty()
    
    # Source tracking
    source_id: str = ndb.StringProperty()  # "ghsa:advisories/.../GHSA-xxx.json"
    source: str = ndb.StringProperty()      # "ghsa"
    source_of_truth: int = ndb.IntegerProperty()  # INTERNAL=1 | SOURCE_REPO=2
    
    # Core vulnerability data (denormalized for query performance)
    project: list[str] = ndb.StringProperty(repeated=True)
    ecosystem: list[str] = ndb.StringProperty(repeated=True)
    purl: list[str] = ndb.StringProperty(repeated=True)
    
    # Timestamps
    timestamp: datetime          # published
    last_modified: datetime      # modified
    modified_raw: datetime       # source file's modified date
    withdrawn: datetime
    
    # Search indexes (auto-populated in _pre_put_hook)
    search_indices: list[str]    # tokenized project, ecosystem, ID
    search_tags: list[str]       # lowercase project names + ID
    affected_fuzzy: list[str]    # normalized version strings
    semver_fixed_indexes: list[str]  # normalized semver fixed versions
    
    # Relationship tracking
    aliases: list[str]
    related: list[str]
    upstream_raw: list[str]
    alias_raw: list[str]
    related_raw: list[str]
    
    # Computed flags
    has_affected: bool
    is_fixed: bool
    is_withdrawn: bool
    
    # Full structured data (stored for completeness, not queryable)
    affected_packages: list[AffectedPackage]  # LocalStructuredProperty
```

**Indexing Strategy:**

```
# Queries supported:
1. Bug WHERE db_id == "CVE-2023-12345"
   → Indexed: db_id

2. Bug WHERE ecosystem == "PyPI" AND project == "requests"
     AND affected_fuzzy == "2.25.0"
   → Indexed: ecosystem, project, affected_fuzzy

3. Bug WHERE ecosystem == "PyPI" 
     AND semver_fixed_indexes >= normalize("2.32.0")
   → Indexed: ecosystem, semver_fixed_indexes

4. Bug WHERE ecosystem == "GIT" AND search_indices == "requests"
   → Indexed: ecosystem, search_indices

# Composite indexes (index.yaml):
- Bug: (ecosystem, affected_fuzzy, -timestamp)
- Bug: (ecosystem, semver_fixed_indexes)
- Bug: (search_indices, ecosystem, -timestamp)
```

#### 2.3.2 Vulnerability Entity (Lightweight)

```python
class Vulnerability(ndb.Model):
    """Thin entity for listing queries (avoids loading full Bug)."""
    # Mirrors key fields from Bug for efficient list queries
    source_id: str
    modified: datetime
    alias_raw: list[str]
    upstream_raw: list[str]
    is_withdrawn: bool
    modified_raw: datetime
```

#### 2.3.3 ListedVulnerability Entity

```python
class ListedVulnerability(ndb.Model):
    """Optimized for website /list page queries."""
    # Projection queries use these fields
    ecosystems: list[str]
    search_indices: list[str]
    published: datetime
    # Summary fields for display without full fetch
    summary: str
    severities: list[Severity]
```

#### 2.3.4 SourceRepository Entity

```python
class SourceRepository(ndb.Model):
    name: str           # primary key
    type: int           # GIT=0 | BUCKET=1 | REST_ENDPOINT=2
    
    # GIT source config
    repo_url: str
    repo_branch: str    # default: main/master
    repo_username: str  # for SSH auth
    
    # BUCKET source config  
    bucket: str
    directory_path: str
    
    # REST source config
    rest_api_url: str
    
    # Common config
    extension: str           # ".json" or ".yaml"
    db_prefix: list[str]     # ID prefixes this source owns
    accepted_ecosystems: list[str]  # ["*"] = all
    key_path: str            # nested JSON path to OSV object
    ignore_patterns: list[str]  # regex patterns to skip files
    
    # Sync state
    last_synced_hash: str    # GIT: last processed commit
    last_update_date: datetime  # BUCKET: last processed time
    ignore_last_import_time: bool  # force full reimport
    
    # Behavior flags
    editable: bool           # can Worker push changes back?
    strict_validation: bool  # fail on invalid records?
    ignore_git: bool         # skip git range processing?
    versions_from_repo: bool # enumerate versions from git tags?
    detect_cherrypicks: bool # detect cherry-picked fixes?
    consider_all_branches: bool
    
    # Display
    link: str                # base URL to source record
    human_link: str          # Jinja2 template for human-readable URL
```

---

## 3. API Server Technical Design

### 3.1 Architecture Pattern

```
Client Request (HTTP/gRPC)
        │
        ▼
ESP (Endpoints Service Proxy)
├── Authentication (API key / OAuth2)
├── Rate limiting
├── HTTP → gRPC transcoding (JSON body ↔ protobuf)
└── Routing to API server
        │
        ▼
OSVServicer (Python gRPC server)
├── NDB context management (decorator @ndb_context)
├── Cloud Trace integration (decorator @trace_filter.log_trace)
├── NDB sync tasklet (decorator @ndb.synctasklet)
└── Business logic
        │
        ├── Datastore queries (via NDB async API)
        └── GCS reads (via ThreadPoolExecutor)
```

### 3.2 Pagination Design

API sử dụng cursor-based pagination để handle large result sets:

```python
@dataclass
class QueryCursor:
    query_number: int = 0        # Batch query index
    ndb_cursor: ndb.Cursor = None  # Datastore cursor
    metadata: QueryCursorMetadata = None

@dataclass  
class QueryCursorMetadata:
    # Extra state per query type
    # (e.g., which of the 3 query strategies we're in)
    pass

# Encoding: cursor serialized to base64 URL-safe string
# Client sends back as page_token in next request
```

**Multi-Query Cursor Strategy:**

Một query (e.g., package + version) thực ra chạy **nhiều sub-queries** (by commit, by ecosystem, by PURL), mỗi query có cursor riêng. QueryContext tracks:

```python
@dataclass
class QueryContext:
    service_context: grpc.ServicerContext
    input_cursor: QueryCursor      # cursor from client
    output_cursor: QueryCursor     # cursor to return to client
    request_cutoff_time: datetime  # time limit for this request
    total_responses: ResponsesCount  # shared across batch
    query_counter: int = 0         # which sub-query we're on
    single_page_limit_override: int | None = None
    
    def should_break_page(self, response_count) -> bool:
        """Break when hitting page limit OR timeout."""
        return (response_count >= page_limit or
                datetime.now() > self.request_cutoff_time)
    
    def should_skip_query(self) -> bool:
        """Skip if cursor says we haven't reached this query yet."""
        return (self.query_counter < self.input_cursor.query_number or
                not self.output_cursor.ended)
```

### 3.3 DetermineVersion Algorithm

This is a probabilistic version identification algorithm using file hash buckets:

**Indexing Phase (done by Indexer service):**
```
For each git repository at each tagged version:
  1. Hash all files using MD5
  2. Assign each file to bucket: bucket_index = first_2_bytes(hash) % 512
  3. Sort hashes within each bucket
  4. Compute bucket hash: MD5(concat(sorted hashes))
  5. Store as RepoIndexBucket entity
  6. Store empty bucket bitmap (1=non-empty, 0=empty)
```

**Query Phase (done by API):**
```
Given: set of file hashes from client

1. Compute same bucket structure as indexer
2. Query Datastore for each non-empty bucket:
   RepoIndexBucket WHERE node_hash == bucket_hash
   (limit 100 results per bucket to avoid noise)

3. For each bucket with ≤ 100 matches:
   - bucket_match_count[project] += 1
   - file_match_count[project] += files_in_bucket

4. For each candidate project:
   score = estimate_match_quality(
     num_matched_buckets,
     empty_bucket_comparison,
     file_count_difference
   )
```

**Score Calculation:**

```python
def estimate_diff(num_bucket_change: int, file_count_diff: int) -> int:
    """Estimate number of changed files from bucket changes."""
    # Log formula: as more buckets change, more files changed
    estimate = 512 * math.log(
        (512 + 1) / (512 - num_bucket_change + 1)
    )
    # Blend with actual file count diff
    return file_count_diff + round(max(estimate - file_count_diff, 0) / 2)

def score_match(idx: RepoIndex, ...) -> float:
    # missed_empty_buckets: buckets empty in query but not in index
    missed_empty_buckets = (~user_bitmap & repo_bitmap).bit_count()
    
    estimated_diff = estimate_diff(
        512 
        - bucket_matches       # matched buckets don't change
        - empty_bucket_count   # empty buckets don't change
        + missed_empty_buckets  # missed empty = changed
        - skipped_buckets,     # skipped assumed unchanged
        abs(idx.file_count - query_file_count)
    )
    
    max_files = max(idx.file_count, query_file_count)
    score = (max_files - estimated_diff) / max_files
    return score  # score ∈ (0, 1]
```

**Cutoff:**
- `score < 0.05` → filtered out
- Returns top 10 results sorted by score descending

### 3.4 Response Throttling

```python
_MAX_VULN_RESP_THRESH = 3000      # Total responses before throttling
_MAX_VULN_LISTED_PRE_EXCEEDED = 1000   # Per-page before threshold
_MAX_VULN_LISTED_POST_EXCEEDED = 5     # Per-page after threshold

class ResponsesCount:
    count: int
    
    def exceeded(self) -> bool:
        return self.count > 3000
    
    def page_limit(self) -> int:
        if self.exceeded():
            return 5   # Ultra-conservative after threshold
        return 1000
```

**Rationale:** Batch queries có thể có 1000 sub-queries. Nếu mỗi query trả 1000 results → 1M records. Response throttling đảm bảo bounded memory và latency.

---

## 4. Importer Technical Design

### 4.1 Change Detection Algorithms

#### 4.1.1 Git Source Change Detection

```python
def _sync_from_previous_commit(source_repo, repo):
    """
    Walk git history from HEAD to last_synced_hash.
    Collect changed/deleted files.
    """
    walker = repo.walk(repo.head.target, SortMode.TOPOLOGICAL)
    walker.hide(source_repo.last_synced_hash)
    
    changed = {}   # {path: commit_timestamp}
    deleted = {}   # {path: commit_timestamp}
    
    for commit in walker:
        # Skip commits by OSV itself (avoid loops)
        if commit.author.email == AUTHOR_EMAIL:
            continue
        # Skip commits with no-update marker
        if _NO_UPDATE_MARKER in commit.message:
            continue
            
        for parent in commit.parents:
            diff = repo.diff(parent, commit)
            for delta in diff.deltas:
                if delta.status == DeltaStatus.DELETED:
                    deleted[delta.old_file.path] = commit_time
                else:
                    changed[delta.new_file.path] = commit_time
    
    return changed, deleted
```

**Performance:** O(commits × files_per_commit) – linear in history size since last sync.

#### 4.1.2 GCS Bucket Change Detection

```python
def _convert_blob_to_vuln(storage_client, ndb_client, source_repo, blob,
                           ignore_last_import_time):
    """
    Two-stage filtering:
    1. blob.updated > source_repo.last_update_date  (quick check, no download)
    2. blob hash != stored hash in Datastore         (slow check, with download)
    """
    # Stage 1: Time-based filter (no download needed)
    if not ignore_last_import_time and blob.updated <= utc_last_update_date:
        return None
    
    # Stage 2: Download and hash
    blob_bytes = blob.download_as_bytes()
    blob_hash = sha256_bytes(blob_bytes)
    
    # Parse vulnerability
    vulns = parse_vulnerabilities_from_data(blob_bytes, ...)
    
    # Stage 3: Compare with Datastore (check if actually changed)
    for vuln in vulns:
        v = Vulnerability.get_by_id(vuln.id)
        if v is None or v.modified_raw != vuln.modified:
            return (blob_hash, blob.name, blob.updated, vulns)
    
    return None  # No change
```

**Parallelism:** 20 concurrent threads via `ThreadPoolExecutor`.

#### 4.1.3 REST API Change Detection

```python
def _process_updates_rest(source_repo):
    """
    1. HEAD request to check Last-Modified header
    2. If newer than last_update_date: fetch all records
    3. For each record: compare individually with Datastore
    """
    head_resp = requests.head(rest_api_url, timeout=60)
    last_modified = parse_http_date(head_resp.headers.get('Last-Modified'))
    
    if last_modified <= source_repo.last_update_date:
        return  # No changes
    
    # Fetch all records
    resp = requests.get(rest_api_url, timeout=60)
    all_vulns = resp.json()
    
    for vuln_data in all_vulns:
        vuln = parse_vulnerability_from_dict(vuln_data)
        existing = Vulnerability.get_by_id(vuln.id)
        if existing is None or existing.modified_raw != vuln.modified:
            publish_update_task(source_repo, vuln.id)
```

### 4.2 Deletion Safety System

```python
def _process_deletions_bucket(source_repo, threshold=10.0):
    """Safe deletion with percentage check."""
    
    # Get all non-withdrawn vulns for this source from Datastore
    ds_vulns = query_vulns_by_source(source_repo.name)
    
    # Get all vulnerability IDs from GCS bucket
    gcs_vuln_ids = parallel_parse_all_blobs(source_repo)
    
    # Find vulns in DS but not in GCS
    to_delete = [v for v in ds_vulns if v.id not in gcs_vuln_ids]
    
    # Safety check: refuse if too many deletions
    pct = len(to_delete) / len(ds_vulns) * 100
    if pct >= threshold:  # Default: 10%
        logging.error('Cowardly refusing to delete %d records (%.1f%%)',
                      len(to_delete), pct)
        return
    
    # Queue deletion tasks
    for v in to_delete:
        publish_update_task(source_repo, v.path, deleted=True)
```

### 4.3 Parallel Processing

```python
# Bucket import: 20 parallel threads
with ThreadPoolExecutor(max_workers=20) as executor:
    futures = {
        executor.submit(_convert_blob_to_vuln, client, ndb_client, 
                       source_repo, blob, ignore_time): blob
        for blob in listed_blobs
    }
    
    for future in as_completed(futures):
        result = future.result()
        if result:
            converted_vulns.append(result)

# Export: 32 parallel threads  
with ThreadPoolExecutor(max_workers=32) as executor:
    ...
```

---

## 5. Worker Technical Design

### 5.1 NDB Transaction Pattern

The Worker uses a transaction pattern để đảm bảo atomic Datastore + GCS consistency:

```python
def _do_update(source_repo, repo, vulnerability, path, original_sha256):
    """
    Pattern: Read from GCS + Datastore, compare, write atomically.
    """
    # Pre-fetch GCS data (outside transaction for efficiency)
    vuln_and_gen = osv.gcs.get_by_id_with_generation(vulnerability.id)
    
    def xact():
        nonlocal gcs_gen
        
        # Fetch current Datastore state
        ds_vuln = osv.Vulnerability.get_by_id(vulnerability.id)
        is_new = ds_vuln is None
        
        # Compare with existing (normalized to avoid false diffs)
        if not is_new:
            old_vuln, gcs_gen = vuln_and_gen
            # Clear fields that are computed separately
            old_vuln.aliases.clear(); old_vuln.upstream.clear()
            new_vuln.aliases.clear(); new_vuln.upstream.clear()
            has_changed = (old_vuln != new_vuln)
        else:
            has_changed = True
        
        # Overwrite aliases/upstream from computed groups
        alias_group = AliasGroup.query(AliasGroup.bug_ids == vuln.id).get()
        if alias_group:
            vulnerability.aliases[:] = sorted(alias_group.bug_ids - {vuln.id})
            ds_vuln.modified = max(alias_group.last_modified, ds_vuln.modified)
        
        # Write to Datastore
        osv.models.put_entities(ds_vuln, vulnerability)
    
    ndb.transaction(xact)
    
    # Write to GCS (outside transaction, with generation check)
    osv.gcs.upload_vulnerability(vulnerability, gcs_gen)
```

**GCS Generation Check:**
```python
# Upload only if GCS hasn't changed since we read it
# This prevents overwriting a newer version
def upload_vulnerability(vuln: Vulnerability, expected_generation: int | None):
    blob.upload_from_string(
        data, 
        if_generation_match=expected_generation  # Atomic CAS operation
    )
```

### 5.2 Impact Analysis Design

```python
def analyze(vulnerability, checkout_path, analyze_git, 
            detect_cherrypicks, versions_from_repo, consider_all_branches):
    """
    Enrich vulnerability with:
    - Affected commit list (from git ranges)
    - Affected version list (from ecosystem or git tags)
    """
    result = AnalyzeResult(has_changes=False, commits=[])
    
    for affected in vulnerability.affected:
        for affected_range in affected.ranges:
            if affected_range.type == GIT:
                # Git range analysis
                commits, tags = analyze_git_range(
                    affected_range.repo,
                    affected_range.events,
                    checkout_path,
                    detect_cherrypicks,
                    consider_all_branches
                )
                result.commits.extend(commits)
                
                if versions_from_repo:
                    # Map commits → version tags
                    versions = commits_to_versions(tags, commits)
                    if versions != set(affected.versions):
                        affected.versions[:] = sorted(versions)
                        result.has_changes = True
                        
            elif affected_range.type in (SEMVER, ECOSYSTEM):
                # Version enumeration
                versions = enumerate_versions(
                    affected.package.ecosystem,
                    affected.package.name,
                    affected_range.events
                )
                if versions != set(affected.versions):
                    affected.versions[:] = sorted(versions)
                    result.has_changes = True
    
    return result
```

**Cherry-pick Detection:**
```python
def detect_cherrypicks(repo, introduced_commit, fixed_commit):
    """
    Find all commits that cherry-pick the fix.
    These commits also "fix" the vulnerability, even if not in main branch.
    """
    fix_patch = get_patch(repo, fixed_commit)
    
    # Walk all branches and tags
    for ref in repo.references:
        for commit in walk_from(ref):
            if patches_match(get_patch(repo, commit), fix_patch):
                yield commit  # This commit is a cherry-pick of the fix
```

### 5.3 Version Enumeration Design

```python
def enumerate_versions(ecosystem: str, package: str, events: list) -> set[str]:
    """
    Enumerate all versions of package affected by the given range events.
    
    Algorithm:
    1. Fetch all versions of package from registry
    2. For each version, check if it's in any "introduced" range
       but not past any "fixed"/"limit" range
    """
    eco_helper = ecosystems.get(ecosystem)
    if eco_helper is None:
        return set()
    
    all_versions = eco_helper.enumerate_versions(package)
    
    affected = set()
    for version in all_versions:
        if is_version_affected(version, events, eco_helper):
            affected.add(version)
    
    return affected

def is_version_affected(version: str, events: list, helper) -> bool:
    """
    Determine if a version is in any [introduced, fixed) range.
    """
    in_range = False
    
    for event in sorted_events(events):
        if event.type == 'introduced':
            if helper.sort_key(version) >= helper.sort_key(event.value):
                in_range = True
        elif event.type == 'fixed':
            if helper.sort_key(version) >= helper.sort_key(event.value):
                in_range = False
        elif event.type == 'last_affected':
            if helper.sort_key(version) > helper.sort_key(event.value):
                in_range = False
        elif event.type == 'limit':
            if helper.sort_key(version) >= helper.sort_key(event.value):
                in_range = False
    
    return in_range
```

### 5.4 PubSub Lease Management

Long-running tasks (git clone, impact analysis) can exceed default PubSub ack deadline:

```python
class _PubSubLeaserThread(threading.Thread):
    """Background thread that continuously renews message lease."""
    
    EXTENSION_TIME_SECONDS = 10 * 60  # 10 minutes
    
    def run(self):
        latest_end = time.time() + MAX_LEASE_SECONDS  # 6 hours max
        
        while True:
            time_left = latest_end - time.time()
            if time_left <= 0:
                logging.warning('Lease reached maximum, stopping renewal.')
                break
            
            extension = min(self.EXTENSION_TIME_SECONDS, time_left)
            subscriber.modify_ack_deadline(
                ack_ids=[self.ack_id],
                ack_deadline_seconds=int(extension)
            )
            
            # Wait until next renewal OR task completes
            wait = min(time_left, self.EXTENSION_TIME_SECONDS // 2)
            if self.done_event.wait(wait):
                break  # Task completed
```

---

## 6. Indexer Technical Design (Go)

### 6.1 Controller-Worker Pattern

```
Controller:
  1. Load repo configs from GCS (textproto format)
  2. For each repo:
     a. Check if already indexed (query RepoIndex in Datastore)
     b. If not indexed or tag has new commits:
        → Publish preparation task to Pub/Sub
  3. The preparation stage clones/updates repos and publishes processing tasks

Worker:
  1. Subscribe to Pub/Sub
  2. For each message: run processing stage
```

### 6.2 File Hash Bucketing

```go
const BucketSize = 512

type FileResult struct {
    Path string
    Hash []byte  // MD5 of file content
}

func processIntoBuckets(files []FileResult) []RepoIndexBucket {
    buckets := make([][][]byte, BucketSize)
    
    for _, f := range files {
        // Skip vendored directories
        if shouldSkipBucket(f.Path) {
            continue
        }
        
        // First 2 bytes of hash determine bucket
        bucketIdx := binary.BigEndian.Uint16(f.Hash[:2]) % BucketSize
        buckets[bucketIdx] = append(buckets[bucketIdx], f.Hash)
    }
    
    result := make([]RepoIndexBucket, BucketSize)
    for i, bucket := range buckets {
        sort.Slice(bucket, func(a, b int) bool {
            return bytes.Compare(bucket[a], bucket[b]) < 0
        })
        
        h := md5.New()
        for _, hash := range bucket {
            h.Write(hash)
        }
        result[i] = RepoIndexBucket{
            NodeHash:        h.Sum(nil),
            FilesContained: len(bucket),
        }
    }
    return result
}
```

**Vendored directory filtering:**
```go
var vendoredLibNames = map[string]bool{
    "3rdparty": true, "dep": true, "deps": true,
    "thirdparty": true, "third-party": true, "third_party": true,
    "libs": true, "external": true, "externals": true,
    "vendor": true, "vendored": true,
}

func shouldSkipBucket(path string) bool {
    for _, component := range strings.Split(path, "/") {
        if vendoredLibNames[component] {
            return true
        }
    }
    return false
}
```

**Rationale:** Vendored libraries would create false positives when matching versions, since many projects vendor the same libraries.

### 6.3 Empty Bucket Bitmap

```go
// Bitmap: bit[i] = 1 if bucket[i] has files
// Stored as little-endian bytes in Datastore

// During indexing:
var emptyBucketBitmap big.Int
for i, bucket := range buckets {
    if len(bucket) > 0 {
        emptyBucketBitmap.SetBit(&emptyBucketBitmap, i, 1)
    }
}
```

During query scoring, the bitmap helps calculate "missed empty buckets":
- User's query has bucket[i] = empty (bit = 0)
- Repository has bucket[i] = non-empty (bit = 1)
- → User is missing files that the repo has → lower score

---

## 7. Website Backend Technical Design

### 7.1 Flask Blueprint Architecture

```python
# main.py
app = Flask(__name__)
app.register_blueprint(frontend_handlers.blueprint)

# frontend_handlers.py - all routes in one Blueprint
blueprint = Blueprint('frontend_handlers', __name__)

@blueprint.route('/')          # Homepage
@blueprint.route('/list')      # Search page
@blueprint.route('/vulnerability/<id>')  # Detail page
@blueprint.route('/<id>')      # Redirect helper
@blueprint.route('/blog/')     # Blog
@blueprint.route('/linter')    # JSON linter
```

### 7.2 Search Implementation

```python
def osv_query(search_string: str, page: int, ecosystem: str) -> dict:
    query = ListedVulnerability.query()
    
    if search_string and len(search_string) <= 300:
        # Token-based search via search_indices
        query = query.filter(
            ListedVulnerability.search_indices == search_string.lower()
        )
    
    if ecosystem:
        query = query.filter(
            ListedVulnerability.ecosystems == ecosystem
        )
    
    query = query.order(-ListedVulnerability.published)
    
    # Fetch page
    results, _, _ = query.fetch_page(
        page_size=16,
        offset=(page - 1) * 16
    )
    
    # Special handling: put exact ID match at top
    if _VALID_VULN_ID.match(search_string):
        results = sorted(results, key=lambda v: (
            0 if v.key.id() == search_string else -v.published.timestamp()
        ))
    
    return {'items': results, 'total': total}
```

**Search Index Tokenization:**
```python
def _tokenize(value: str) -> set[str]:
    """Break string into tokens for search indexing."""
    tokens = set()
    # Split on word boundaries
    for word in re.split(r'\W+', value.lower()):
        if word:
            tokens.add(word)
    return tokens
```

### 7.3 Vulnerability Detail Enrichment

When serving vulnerability detail page:

```python
def vuln_to_response(vuln):
    """Enrich vulnerability for display."""
    response = vulnerability_to_dict(vuln)
    
    # 1. Add CVSS score and rating
    add_cvss_score(response)
    
    # 2. Add commit links for GIT ranges
    add_links(response)
    
    # 3. Add source provenance link
    add_source_info(response)  # → links to GitHub/bucket source
    
    # 4. Add upstream/downstream hierarchy
    add_stream_info(response)  # → AliasGroup/UpstreamGroup lookup
    
    # 5. Add known OSV IDs for hyperlinks
    add_known_osv_bugs(response)
    
    # 6. Generate hierarchy HTML strings
    add_stream_strings(response)
    
    return response
```

**CVSS Score Calculation:**
```python
cvss_calculators = {
    'CVSS_V2': CVSS2,   # cvss package
    'CVSS_V3': CVSS3,
    'CVSS_V4': CVSS4,
}

def calculate_severity(severity: dict) -> tuple[float, str]:
    type_ = severity.get('type')
    score = severity.get('score')
    c = cvss_calculators[type_](score)
    return c.base_score, c.severities()[0]  # (9.8, "Critical")
```

### 7.4 Rate Limiting

```python
# Redis-backed rate limiter
class RateLimiter:
    def __init__(self, redis_host, redis_port, requests_per_min=30):
        self.redis = redis.Redis(redis_host, redis_port)
        self.limit = requests_per_min
    
    def check_request(self, ip_addr: str) -> bool:
        key = f"rate_limit:{ip_addr}:{current_minute()}"
        count = self.redis.incr(key)
        if count == 1:
            self.redis.expire(key, 60)
        return count <= self.limit

@blueprint.before_request
def check_rate_limit():
    ip = request.headers.get('X-Forwarded-For', 'unknown').split(',')[0]
    if not limiter.check_request(ip):
        abort(429)
```

---

## 8. Caching Design

### 8.1 Cache Hierarchy

```
L1: In-Process Cache (InMemoryCache)
    ├── OSV Schema (jsonschema validation schema)
    ├── Source repository configs
    └── Ecosystem helper instances

L2: Redis Cache (website)
    ├── Ecosystem counts (hard: 24h, soft: 30min)
    └── Rate limiting counters

L3: GCS (API)
    ├── Vulnerability JSON files
    └── Data dumps (gs://osv-vulnerabilities/)

L4: Cloud Datastore
    └── All metadata (authoritative)
```

### 8.2 Smart Cache Pattern

```python
@cache.smart_cache(
    "osv_get_ecosystem_counts",
    hard_timeout=24 * 60 * 60,   # 24 hours: always use cache
    soft_timeout=30 * 60          # 30 minutes: refresh in background
)
def osv_get_ecosystem_counts_cached():
    return osv_get_ecosystem_counts()
```

**Smart cache behavior:**
- Before soft_timeout: return cached value immediately
- Between soft_timeout and hard_timeout: return stale value BUT trigger background refresh
- After hard_timeout: block until fresh value fetched

---

## 9. Error Handling Design

### 9.1 Error Categories

| Category | Examples | Strategy |
|----------|---------|----------|
| **Transient** | Network timeout, rate limit | Retry with backoff |
| **Data Quality** | Invalid OSV JSON | Log + record ImportFinding |
| **Conflict** | Concurrent git push | Rebase + retry |
| **Fatal** | Datastore unavailable | Fail fast, alert |

### 9.2 Import Error Recording

```python
class ImportFindings(enum.Enum):
    INVALID_JSON = "INVALID_JSON"
    INVALID_OSVSSCHEMA = "INVALID_OSV_SCHEMA"
    # ... other quality issues

def _record_quality_finding(source, bug_id, finding):
    """Persist import quality issues for API reporting."""
    existing = ImportFinding.get_by_id(bug_id)
    if existing:
        if finding not in existing.findings:
            existing.findings.append(finding)
        existing.last_attempt = utcnow()
        existing.put()
    else:
        ImportFinding(
            bug_id=bug_id,
            source=source,
            findings=[finding],
            first_seen=utcnow(),
            last_attempt=utcnow()
        ).put()
```

Findings are accessible via `GET /v1experimental/importfindings/{source}`.

### 9.3 GCS Write Failure Retry

```python
try:
    osv.gcs.upload_vulnerability(vulnerability, gcs_gen)
except Exception:
    logging.error('GCS write failed for %s', vuln_id)
    # Publish to retry queue
    data = vulnerability.SerializeToString(deterministic=True)
    osv.pubsub.publish_failure(data, type='gcs_retry')
```

A separate consumer handles `gcs_retry` messages.

### 9.4 Git Push Conflicts

```python
def push_source_changes(repo, commit_message, git_callbacks, expected_hashes):
    """Push with conflict retry logic."""
    for retry in range(1 + PUSH_RETRIES):  # 3 attempts
        try:
            repo.remotes['origin'].push([repo.head.name], git_callbacks)
            return True
        except pygit2.GitError:
            if retry == PUSH_RETRIES:
                repos.reset_repo(repo, git_callbacks, True)
                return False
            
            time.sleep(10)  # Wait before rebase
            
            # Verify expected file hashes haven't changed
            for path, expected_hash in expected_hashes.items():
                if sha256(path) != expected_hash:
                    continue  # Upstream changed, skip
            
            # Cherry-pick our commit on top of upstream
            repos.reset_repo(repo, git_callbacks, True)
            repo.cherrypick(our_commit.id)
            if repo.index.conflicts:
                repo.state_cleanup()
                return False
```

---

## 10. Testing Design

### 10.1 Test Architecture

```
tests/
├── osv/                    # Unit tests for core library
│   ├── models_test.py      # Data model tests
│   ├── impact_test.py      # Impact analysis tests
│   ├── sources_test.py     # Source parsing tests
│   └── ...
├── gcp/api/
│   ├── server_test.py      # API server unit tests
│   ├── integration_tests.py  # API integration tests (with Datastore emulator)
│   └── snapshot_tests.yaml   # API response snapshot tests
├── gcp/workers/importer/
│   └── importer_test.py    # Importer tests (heavy, 50KB)
└── gcp/workers/worker/
    └── worker_test.py      # Worker tests (heavy, 36KB)
```

### 10.2 Test Output Generation Pattern

```python
# Simple snapshot framework (osv/tests.py)
class TestCase:
    GENERATE = os.getenv('TESTS_GENERATE', '0') == '1'
    
    def check_output(self, actual: dict, expected_file: str):
        if self.GENERATE:
            # Regenerate expected output
            with open(expected_file, 'w') as f:
                json.dump(actual, f, indent=2)
        else:
            # Compare with stored expected output
            with open(expected_file) as f:
                expected = json.load(f)
            self.assertEqual(actual, expected)
```

**Usage:**
```bash
# Regenerate snapshots after intentional changes
TESTS_GENERATE=1 make all-tests

# Normal test run
make all-tests
```

### 10.3 Integration Test Dependencies

```bash
# API integration tests require:
gcloud components install cloud-firestore-emulator
gcloud auth application-default login

# Run with emulator
make api-server-tests

# Long tests (version enumeration, etc.)
LONG_TESTS=1 make api-server-tests
```

### 10.4 Coarse Monotonicity Tests

```bash
# Test that version ordering is monotonically increasing
# for all versions in the OSV database

# Step 1: Generate test data from all OSV records
./osv/ecosystem/testdata/regen_coarse_test_data.sh

# Step 2: Run large-scale test
RUN_COARSE_LARGE_TEST=1 go test ./osv/ecosystem -run TestCoarseMonotonicityLarge
```

This validates that ecosystem version parsers correctly order versions for all real-world packages in the database.

### 10.5 API E2E Snapshot Tests

```yaml
# snapshot_tests.yaml: defines API calls + expected response hashes
tests:
  - name: "query_by_package_version"
    method: POST
    path: /v1/query
    body:
      query:
        package:
          ecosystem: "PyPI"
          name: "requests"
        version: "2.25.0"
    expected_hash: "sha256:abc123..."
```

```bash
# Update snapshots after intentional API changes
gcloud auth application-default login
make update-api-snapshots
```

---

## 11. Deployment Design

### 11.1 Cloud Build CI/CD

Each service has a `cloudbuild.yaml`:

```yaml
# gcp/api/cloudbuild.yaml
steps:
  - name: 'gcr.io/cloud-builders/docker'
    args: ['build', '-t', 'gcr.io/$PROJECT_ID/osv-api', '.']
  - name: 'gcr.io/cloud-builders/docker'
    args: ['push', 'gcr.io/$PROJECT_ID/osv-api']
  - name: 'gcr.io/google.com/cloudsdktool/cloud-sdk'
    args: ['run', 'deploy', 'osv-api', ...]
```

### 11.2 Go Monorepo Docker Build

All Go services use a single multi-target Dockerfile:

```dockerfile
# go/Dockerfile
FROM golang:1.22 AS base
WORKDIR /app
COPY . .
# External bindings dependency
COPY --from=bindings / /bindings

FROM base AS importer
RUN go build -o /bin/importer ./cmd/importer

FROM base AS exporter  
RUN go build -o /bin/exporter ./cmd/exporter

# ... other targets
```

```bash
# Build specific service
cd go/
docker build \
  -t osv/importer \
  --target importer \
  --build-context bindings=../bindings \
  -f Dockerfile .
```

### 11.3 Terraform Infrastructure

```
deployment/
├── terraform/
│   ├── main.tf           # Cloud Run, IAM, networking
│   ├── storage.tf        # GCS buckets
│   ├── pubsub.tf         # Pub/Sub topics and subscriptions
│   ├── datastore.tf      # Datastore configuration
│   └── variables.tf
└── cloud-deploy/
    └── pipeline.yaml     # Progressive delivery pipeline
```

---

## 12. Operational Runbook Highlights

### 12.1 Force Re-import a Source

```bash
# Set ignore_last_import_time = True in Datastore for the source
# Next importer run will reimport all records regardless of timestamp
gcloud datastore export ... # backup first
# Update SourceRepository entity
```

### 12.2 Manual Vulnerability Update

```bash
# Publish update task directly to Pub/Sub
gcloud pubsub topics publish tasks \
  --message="" \
  --attribute=type=update,source=ghsa,path=advisories/.../GHSA-xxx.json,...
```

### 12.3 Database Backup

Handled by cron jobs in `gcp/workers/cron/`:
- Daily Cloud Datastore export to GCS
- Configurable retention policy

### 12.4 Adding a New Data Source

1. Add entry to `source.yaml` with appropriate `type`, `db_prefix`, `accepted_ecosystems`
2. Create `SourceRepository` entity in Datastore (or let config loader create it)
3. Test with `source_test.yaml` (staging config)
4. Deploy importer update
5. Monitor import logs in `gs://osv-public-import-logs/<source-name>`

---

*Tài liệu này được tạo từ phân tích mã nguồn repository. Để biết thêm chi tiết, tham khảo code tại các paths được đề cập.*
