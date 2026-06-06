# OSV.dev — Architecture Document

> **Version:** 1.0  
> **Date:** 2026-05-31  
> **Status:** Draft  
> **Project:** Open Source Vulnerabilities (OSV.dev)  
> **Repository:** https://github.com/google/osv.dev

---

## 1. Tổng Quan Hệ Thống

### 1.1 Giới Thiệu

**OSV.dev** (Open Source Vulnerabilities) là một nền tảng cơ sở dữ liệu lỗ hổng bảo mật mã nguồn mở do Google phát triển và vận hành. Hệ thống cung cấp một schema chuẩn hóa (OSV Schema) và API để tra cứu, tổng hợp và phân phối thông tin về các lỗ hổng bảo mật (CVE, GHSA, OSV, ...) cho hàng chục hệ sinh thái phần mềm khác nhau.

### 1.2 Mục Tiêu Kiến Trúc

| Mục tiêu | Mô tả |
|-----------|-------|
| **Scalability** | Xử lý hàng triệu lỗ hổng và hàng nghìn request/giây |
| **Data Freshness** | Đồng bộ dữ liệu từ nhiều nguồn gần như real-time |
| **Openness** | API công khai, dữ liệu mở, schema chuẩn hóa |
| **Reliability** | Tận dụng GCP managed services để đảm bảo uptime cao |
| **Extensibility** | Dễ dàng thêm nguồn dữ liệu mới qua cấu hình YAML |

### 1.3 Hệ Sinh Thái Được Hỗ Trợ

OSV.dev tổng hợp lỗ hổng từ hơn **30+ nguồn** trải dài trên các hệ sinh thái:

- **Linux distributions**: Debian, Ubuntu, Red Hat, AlmaLinux, Rocky Linux, SUSE, Alpine, openEuler, Azure Linux...
- **Language ecosystems**: PyPI (Python), Go, Rust (crates.io), npm (JavaScript), Maven (Java), crates...
- **Security programs**: OSS-Fuzz, GHSA, CVE NVD, Malicious Packages...
- **Other**: Android, V8/Chromium, curl, Drupal, Haskell, R, Julia, OCaml...

---

## 2. Kiến Trúc Tổng Thể

### 2.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        EXTERNAL DATA SOURCES                        │
│  Git Repos │ GCS Buckets │ REST APIs │ OSS-Fuzz  │ NVD CVE          │
└────┬────────────┬─────────────┬──────────┬──────────┬───────────────┘
     │            │             │          │          │
     └────────────┴─────────────┴──────────┴──────────┘
                              │
                    ┌─────────▼──────────┐
                    │  IMPORTER SERVICE  │
                    │  (gcp/workers/     │
                    │   importer)        │
                    └─────────┬──────────┘
                              │ Pub/Sub: tasks
                    ┌─────────▼──────────┐
                    │   WORKER SERVICE   │
                    │  (gcp/workers/     │
                    │   worker)          │
                    └─────┬──────┬───────┘
                          │      │
              ┌───────────┘      └────────────┐
              ▼                               ▼
   ┌──────────────────┐           ┌───────────────────┐
   │  Cloud Datastore │           │   Cloud Storage   │
   │  (NDB/Firestore) │           │   (GCS Buckets)   │
   │  - Vulnerability │           │  - osv-vulns/     │
   │  - Bug           │           │  - osv-vulner-    │
   │  - SourceRepo    │           │    abilities/     │
   │  - AliasGroup    │           │  (JSON/YAML)      │
   └────────┬─────────┘           └────────┬──────────┘
            │                              │
            └──────────────┬───────────────┘
                           │
                ┌──────────▼──────────┐
                │    API SERVER       │
                │  (gcp/api)          │
                │  gRPC + REST (ESP)  │
                └──────────┬──────────┘
                           │
                ┌──────────▼──────────┐
                │   WEBSITE BACKEND   │
                │  (gcp/website)      │
                │  Flask + Hugo       │
                └──────────┬──────────┘
                           │
                ┌──────────▼──────────┐
                │    END USERS        │
                │ Browsers │ CLI Tools│
                │ Integrations        │
                └─────────────────────┘
```

### 2.2 Kiến Trúc Xử Lý Dữ Liệu (Data Pipeline)

```
External Source
      │
      ▼
[Importer] ──── detects changes ────────────────────────────────┐
      │                                                          │
      │ publishes Pub/Sub message                                │
      ▼                                                          │
[Pub/Sub: tasks topic]                                           │
      │                                                          │
      ▼                                                          │
[Worker] ──── parses OSV YAML/JSON ──► impact analysis          │
      │              │                 (git bisection,          │
      │              │                  version enumeration)     │
      │              ▼                                           │
      │     [Cloud Datastore] ─── stores enriched vuln ─────────┘
      │              │
      │              ▼
      │     [Cloud Storage] ─── stores vuln JSON/YAML for API
      │
      ▼
[Alias Worker] ──── groups related vulns (CVE + GHSA + OSV)
      │
      ▼
[Exporter / Cron] ──── exports data dumps to gs://osv-vulnerabilities
```

---

## 3. Các Thành Phần Kiến Trúc Chi Tiết

### 3.1 Thư Viện Lõi OSV (`osv/`)

**Ngôn ngữ:** Python 3.13  
**Vai trò:** Thư viện dùng chung cho tất cả các Python services

| Module | Chức năng |
|--------|-----------|
| `models.py` | Định nghĩa Datastore entities (Bug, Vulnerability, SourceRepository, AliasGroup, UpstreamGroup...) |
| `sources.py` | Parse OSV YAML/JSON, validate schema, git operations |
| `impact.py` | Phân tích impact - tính toán affected versions, git bisection |
| `ecosystems/` | Version parsing cho từng hệ sinh thái (PyPI, Go, npm, Maven...) |
| `gcs.py` | GCS utilities: upload/download vulnerability JSON |
| `purl_helpers.py` | Package URL (PURL) parsing và generation |
| `semver_index.py` | SemVer normalization cho querying |
| `repos.py` | Git repository management |
| `cache.py` | In-memory caching layer |
| `logs.py` | GCP structured logging setup |
| `pubsub.py` | Pub/Sub publish utilities |

**Data Models quan trọng:**

```
Bug (Datastore entity)
├── db_id: str              # Vulnerability ID (CVE-xxx, GHSA-xxx, OSV-xxx)
├── source_id: str          # source_name:path/to/file
├── status: int             # UNPROCESSED/PROCESSED/INVALID
├── affected_packages[]     # AffectedPackage[]
│   ├── package             # Package (ecosystem, name, purl)
│   ├── ranges[]            # AffectedRange2[] (GIT/SEMVER/ECOSYSTEM)
│   │   └── events[]        # AffectedEvent[] (introduced/fixed/limit)
│   └── versions[]          # list of affected version strings
├── search_indices[]        # tokenized search index
├── affected_fuzzy[]        # normalized versions for fuzzy matching
└── semver_fixed_indexes[]  # SEMVER fixed indexes for query

SourceRepository (Datastore entity)
├── name: str               # e.g. "ghsa", "python", "oss-fuzz"
├── type: int               # GIT(0) | BUCKET(1) | REST_ENDPOINT(2)
├── repo_url: str           # Git repo URL (for GIT type)
├── bucket: str             # GCS bucket (for BUCKET type)
├── rest_api_url: str       # REST endpoint (for REST_ENDPOINT type)
├── last_synced_hash: str   # Last processed git commit hash
├── last_update_date: ts    # Last bucket sync timestamp
└── extension: str          # ".json" or ".yaml"
```

### 3.2 API Server (`gcp/api/`)

**Ngôn ngữ:** Python  
**Framework:** gRPC + Google Cloud Endpoints (ESP)  
**Protocol:** gRPC over HTTP/2, REST via transcoding  

**gRPC Service Definition** (`v1/osv_service_v1.proto`):

```protobuf
service OSV {
  // GET /v1/vulns/{id}
  rpc GetVulnById(GetVulnByIdParameters) returns (Vulnerability)

  // POST /v1/query
  rpc QueryAffected(QueryAffectedParameters) returns (VulnerabilityList)

  // POST /v1/querybatch  (max 1000 queries)
  rpc QueryAffectedBatch(QueryAffectedBatchParameters) returns (BatchVulnerabilityList)

  // POST /v1experimental/determineversion
  rpc DetermineVersion(DetermineVersionParameters) returns (VersionMatchList)

  // GET /v1experimental/importfindings/{source}
  rpc ImportFindings(ImportFindingsParameters) returns (ImportFindingList)
}
```

**Query Strategies:**

| Query Type | Mechanism |
|------------|-----------|
| By ID | GCS direct lookup → Datastore alias fallback |
| By package+version | Datastore index on `ecosystem`, `affected_fuzzy` |
| By commit hash | Datastore index on `affected_commits` |
| By PURL | PURL parsing → ecosystem/name/version extraction |
| DetermineVersion | File hash bucketing + Datastore `RepoIndexBucket` |

**Performance Limits:**
- Single query timeout: 20 seconds
- Batch query timeout: 35 seconds
- Max batch queries: 1,000
- Max vulnerabilities per response (pre-threshold): 1,000
- Max vulnerabilities per response (post-threshold): 5
- Thread pool for GCS operations: 32 workers

### 3.3 Importer Service (`gcp/workers/importer/`)

**Ngôn ngữ:** Python  
**Vai trò:** Phát hiện thay đổi từ nguồn ngoài và phát Pub/Sub tasks

**Các luồng xử lý:**

#### 3.3.1 Git Source (`type: 0`)
```
1. Clone/update git repository via pygit2
2. Walk commits since last_synced_hash
3. Detect added/modified/deleted OSV files
4. Parse vulnerability for validation
5. batch put_if_newer to Datastore
6. Publish Pub/Sub "update" task for each changed file
7. Update source_repo.last_synced_hash
```

#### 3.3.2 GCS Bucket Source (`type: 1`)
```
1. List all blobs in source GCS bucket
2. Parallel parse blobs (20 threads)
3. Compare blob.updated vs source_repo.last_update_date
4. Compare blob hash vs stored hash in Datastore
5. Publish Pub/Sub "update" task for changed blobs
6. Update source_repo.last_update_date
```

#### 3.3.3 REST API Source (`type: 2`)
```
1. HEAD request to check Last-Modified header
2. If modified, GET all.json from REST endpoint
3. Compare individual records with Datastore
4. Publish Pub/Sub "update" task for changed records
```

**Deletion Safety:**
- Compares Datastore records vs GCS blobs
- Safety threshold: refuse to delete if > 10% of records removed at once
- Prevents accidental mass deletion

### 3.4 Worker Service (`gcp/workers/worker/`)

**Ngôn ngữ:** Python  
**Vai trò:** Xử lý Pub/Sub tasks, enrich vulnerabilities, lưu vào Datastore + GCS

**Task Types:**

| Task Type | Description |
|-----------|-------------|
| `update` | Standard source update (Git/Bucket/REST) |
| `update-oss-fuzz` | OSS-Fuzz specific update |
| `impact` | Internal impact analysis for OSS-Fuzz |
| `regressed` / `fixed` | OSS-Fuzz bisection results |
| `gcs_retry` | Retry failed GCS writes |

**Vulnerability Processing Pipeline:**

```
Receive Pub/Sub message
        │
        ▼
Parse vulnerability (YAML/JSON)
        │
        ▼
Normalize package names (ecosystem-specific)
        │
        ▼
Filter unknown ecosystems
        │
        ▼
Impact analysis (if not kernel vulnerability):
  ├── Git bisection (introduced/fixed commits)
  ├── Version enumeration from git tags
  └── Cherry-pick detection
        │
        ▼
NDB Transaction:
  ├── Fetch current state (Datastore + GCS)
  ├── Compare old vs new vulnerability
  ├── Update schema_version, PURLs, source links
  ├── Overwrite aliases/upstream from AliasGroup/UpstreamGroup
  ├── Set modified/published timestamps
  └── Write to Datastore (Bug + Vulnerability entities)
        │
        ▼
Upload to GCS (JSON format)
        │
        ▼
Notify ecosystem bridges (e.g., PyPI bridge)
        │
        ▼
Clear stale ImportFindings
```

**Concurrency & Reliability:**
- PubSub lease renewal via background thread (max 6 hours)
- Redis cache for git repository metadata
- Exponential backoff on GCS write failures
- NDB transactions for atomic Datastore + GCS updates

### 3.5 Indexer Service (`gcp/indexer/`)

**Ngôn ngữ:** Go  
**Vai trò:** Đánh index file hashes của các git repositories để phục vụ `DetermineVersion` API

**Architecture Pattern:** Controller-Worker via Pub/Sub

```
Controller Mode:
  1. Load repo configs from GCS textproto files
  2. For each repo: clone/update, check if already indexed
  3. Publish indexing tasks to Pub/Sub

Worker Mode:
  1. Subscribe to Pub/Sub topic
  2. Clone target git repository to GCS bucket (cached)
  3. Hash all source files (MD5)
  4. Divide hashes into 512 buckets (using first 2 bytes of hash)
  5. For each bucket: compute aggregate MD5 of sorted hashes
  6. Store RepoIndexBucket entities in Datastore
  7. Store RepoIndex entity with metadata
```

**Bucket Hashing Algorithm:**
```
bucket_index = int.from_bytes(file_hash[:2], byteorder='big') % 512
bucket_hash = MD5(sorted(hashes_in_bucket))
```

This enables fuzzy version matching by comparing bucket hash distributions.

### 3.6 Website Backend (`gcp/website/`)

**Ngôn ngữ:** Python + Hugo (frontend)  
**Framework:** Flask  

**URL Routes:**

| Route | Handler |
|-------|---------|
| `/` | Homepage with ecosystem counts |
| `/list?q=&ecosystem=` | Search vulnerabilities |
| `/vulnerability/<id>` | Vulnerability detail page |
| `/<id>` | Redirect to vulnerability page |
| `/<id>.json` | Redirect to API JSON |
| `/blog/` | Blog index |
| `/linter` | OSV JSON linter tool |

**Caching Strategy:**
- Ecosystem counts: hard timeout 24h, soft timeout 30min
- Rate limiting: 30 requests/minute per IP (via Redis)
- CORS support for local development

**Frontend:**
- Hugo static site generator for documentation
- Modern frontend in `frontend3/` directory (likely Svelte/TypeScript based on structure)
- Blog posts in `blog/` as HTML/Markdown

### 3.7 Vuln Feeds (`vulnfeeds/`)

**Ngôn ngữ:** Go + Python  
**Vai trò:** Convert external vulnerability formats → OSV Schema

| Sub-component | Source | Target |
|--------------|--------|--------|
| `cmd/` | NVD CVE JSON feeds | OSV JSON (cve-osv-conversion bucket) |
| `cmd/alpine` | Alpine Linux secdb | OSV JSON |
| `tools/debian` | Debian security tracker | OSV JSON |

**NVD CVE Conversion Pipeline:**
```
NVD CVE API → Fetch CVE records → Convert to OSV Schema
           → Resolve affected packages via CPE/PURL
           → Upload to gs://cve-osv-conversion/osv-output/
           → Importer picks up from bucket
```

### 3.8 Go Shared Library (`go/`)

**Ngôn ngữ:** Go  
**Vai trò:** Shared utilities cho Go services

| Package | Purpose |
|---------|---------|
| `cmd/exporter` | Export data dumps to GCS |
| `cmd/recordchecker` | Validate OSV records |
| `osv/ecosystem` | Ecosystem-specific version comparison |
| `internal/` | Internal utilities |
| `purl/` | PURL parsing |
| `logger/` | GCP structured logging |

### 3.9 Alias & Upstream Workers

**`gcp/workers/importer/` (Alias Worker):**
- Groups vulnerability IDs from different sources that refer to the same vulnerability
- Creates `AliasGroup` entities in Datastore
- Example: CVE-2021-12345 ↔ GHSA-xxxx-xxxx-xxxx ↔ PYSEC-2021-xxx

**`go/cmd/relations` (Upstream/Related):**
- Manages `UpstreamGroup` and `RelatedGroup` entities
- Tracks which vulnerabilities are upstream/downstream of others
- Used for vulnerability inheritance tracking

---

## 4. Hạ Tầng GCP

### 4.1 GCP Services Sử Dụng

| Service | Mục đích |
|---------|----------|
| **Cloud Datastore (Firestore NDB)** | Primary database cho vulnerability metadata |
| **Cloud Storage (GCS)** | Lưu trữ OSV JSON/YAML files, repo snapshots, data dumps |
| **Cloud Pub/Sub** | Message queue giữa Importer → Worker |
| **Cloud Run** | Host API Server và Website (serverless) |
| **Cloud Build** | CI/CD pipeline |
| **Cloud Deploy** | Deployment management |
| **Endpoints (ESP)** | API Gateway, gRPC-HTTP transcoding, rate limiting |
| **Redis (Memorystore)** | Caching cho website (ecosystem counts) |
| **Cloud Functions** | PyPI vulnerability publishing |
| **Terraform** | Infrastructure as Code |

### 4.2 GCS Bucket Structure

| Bucket | Content |
|--------|---------|
| `osv-vulnerabilities/` | Curated data dumps per ecosystem |
| `osv-public-import-logs/` | Import failure logs per source |
| `oss-fuzz-osv-vulns/` | OSS-Fuzz exported vulnerabilities |
| `cve-osv-conversion/` | NVD CVE → OSV conversions |
| `debian-osv/` | Debian security data |
| `android-osv/` | Android security bulletins |
| `resf-osv-data/` | Rocky Linux data |
| `go-vulndb/` | Go vulnerability database |
| `<source-specific>/` | Other per-source buckets |

### 4.3 Cloud Datastore Schema

```
Kind: Bug (primary vulnerability entity)
Kind: Vulnerability (lightweight listing entity)
Kind: ListedVulnerability (search-optimized)
Kind: SourceRepository (source configuration)
Kind: AliasGroup (vulnerability alias groups)
Kind: UpstreamGroup (upstream vulnerability tracking)
Kind: RelatedGroup (related vulnerability tracking)
Kind: ImportFinding (import quality issues)
Kind: RepoIndex (git repo indexing metadata)
Kind: RepoIndexBucket (file hash buckets)
Kind: IDCounter (OSV ID allocation)
```

### 4.4 Deployment Architecture

```
Cloud Run Services:
├── osv-api          (gcp/api)          gRPC + ESP sidecar
├── osv-website      (gcp/website)      Flask + Hugo static
└── osv-website-test (staging)

GKE / Cloud Run Jobs:
├── osv-worker       (gcp/workers/worker)
├── osv-importer     (gcp/workers/importer)
├── osv-alias-worker (gcp/workers/alias)
├── osv-indexer      (gcp/indexer)
└── cron jobs        (gcp/workers/cron)
```

---

## 5. Data Flow Chi Tiết

### 5.1 Luồng Import Dữ Liệu Mới

```
Source Security Advisory Published
           │
           │ (Git push / GCS upload / REST update)
           ▼
    Importer Service
    (runs periodically)
           │
           │ detects change
           ▼
    Validate OSV JSON/YAML
    (jsonschema validation)
           │
           │ if valid
           ▼
    put_if_newer() → Datastore (Bug entity pre-created)
           │
           │ publish Pub/Sub
           ▼
    tasks topic: {type="update", source=..., path=...}
           │
           ▼
    Worker Service (auto-scaled)
           │
           ├── Parse full vulnerability
           ├── Filter unknown ecosystems
           ├── Impact analysis:
           │   ├── Clone affected git repos
           │   ├── Walk git history for commits
           │   ├── Detect cherry-picks
           │   └── Enumerate affected versions
           │
           ├── NDB Transaction:
           │   ├── Compare with stored vulnerability
           │   ├── Update if changed
           │   ├── Set modified timestamp
           │   └── Write Datastore entities
           │
           └── Upload JSON to GCS
                   │
                   ▼
            Available via API
```

### 5.2 Luồng API Query

```
Client Request: POST /v1/query
{
  "package": {"name": "requests", "ecosystem": "PyPI"},
  "version": "2.25.0"
}
           │
           ▼
    ESP (Endpoints Service Proxy)
    - Authentication check
    - Rate limiting
    - HTTP → gRPC transcoding
           │
           ▼
    OSVServicer.QueryAffected()
           │
           ├── Parse PURL (if provided)
           ├── Validate input
           │
           ├── Query 1: By SEMVER index
           │   Datastore: Bug WHERE semver_fixed_indexes >= normalize(version)
           │              AND ecosystem == "PyPI"
           │              AND project == "requests"
           │
           ├── Query 2: By ECOSYSTEM (fuzzy version)
           │   Datastore: Bug WHERE affected_fuzzy == version
           │              AND ecosystem == "PyPI"
           │
           └── For each matching Bug:
               └── Fetch full Vulnerability JSON from GCS
                   (parallel via ThreadPoolExecutor)
           │
           ▼
    Paginated Response (cursor-based)
    max 1000 results, up to 20s timeout
```

### 5.3 Luồng DetermineVersion

```
Client: POST /v1experimental/determineversion
{
  "file_hashes": [
    {"file_path": "src/main.c", "hash": "...md5..."},
    ...
  ]
}
           │
           ▼
    Compute bucket hashes (same algo as indexer):
    - For each file hash → bucket_index = first_2_bytes % 512
    - Sort hashes per bucket → compute MD5 of sorted list
           │
           ▼
    Query Datastore:
    RepoIndexBucket WHERE node_hash == bucket_hash
    (parallel, limit 100 per bucket)
           │
           ▼
    Aggregate matches by project (RepoIndex parent key)
           │
           ▼
    Estimate diff using log-based formula:
    estimate = 512 * log((513) / (513 - num_changed_buckets))
    score = (max_files - estimated_diff) / max_files
           │
           ▼
    Return top 10 matches sorted by score desc
```

---

## 6. Ecosystem Version Handling

### 6.1 Version Comparison Architecture

The `osv/ecosystems/` module provides ecosystem-specific version parsing:

```
Ecosystem Helper Interface:
  ├── next_version(package, version) → str
  ├── sort_key(version) → comparable
  └── is_valid(version) → bool

Supported Ecosystems (partial):
  ├── PyPI → packaging.version
  ├── Go → semver
  ├── npm → semver
  ├── Maven → Maven version spec
  ├── Cargo/crates.io → semver
  ├── RubyGems → gem versioning
  ├── NuGet → NuGet versioning
  ├── Packagist → Composer versioning
  └── ... 30+ ecosystems total
```

### 6.2 Version Index Strategy

```
SEMVER ranges:
  → semver_index.normalize(fixed_version)
  → stored in Bug.semver_fixed_indexes[]
  → queried via range filter (>= normalized_version)

ECOSYSTEM ranges:
  → Bug.affected_fuzzy[] contains normalized version strings
  → exact match query

GIT ranges:
  → commit hashes stored in affected_commits
  → queried via exact match on hex commit hash
```

---

## 7. Security Considerations

### 7.1 Data Integrity

- **SHA256 checksums** trên mọi vulnerability file khi import
- **NDB Transactions** đảm bảo atomic update giữa Datastore và GCS
- **GCS generation matching** tránh race conditions khi concurrent writes
- **Deletion safety threshold** (10%) ngăn xóa hàng loạt do lỗi

### 7.2 Input Validation

- OSV JSON Schema validation (jsonschema) trên mọi record import
- Strict validation mode có thể bật per-source
- ID length limit (100 chars) trong API
- Query string length limit (300 chars) trong website search
- PURL validation với purl_helpers module

### 7.3 API Security

- gRPC + ESP: Google Cloud Endpoints authentication
- Rate limiting: 30 req/min per IP (website), via Redis
- Cloud Trace integration cho observability
- CORS configurable cho development

### 7.4 Operational Security

- SSH key management cho git repo access
- Service accounts với least-privilege permissions
- VPC-native networking trên GKE
- Cloud Armor (WAF) trên Load Balancer (inferred from GCP deployment)

---

## 8. Scalability & Performance

### 8.1 Horizontal Scaling

| Component | Scaling Method |
|-----------|---------------|
| API Server | Cloud Run auto-scaling (request-based) |
| Website | Cloud Run auto-scaling |
| Worker | Cloud Run Jobs / GKE với multiple replicas |
| Importer | Scheduled job, single instance |
| Indexer | Controller + Worker pool (Pub/Sub fan-out) |

### 8.2 Performance Optimizations

- **Bucket thread pool (32 threads)** trong API để parallel GCS reads
- **ThreadPoolExecutor (20 threads)** trong Importer để parallel blob parsing
- **Datastore projection queries** để tránh đọc full entities
- **Smart caching** (hard/soft timeout) cho ecosystem counts
- **Cursor-based pagination** để handle large result sets
- **Query cutoff time** để bound latency (20s single, 35s batch)

### 8.3 Data Volume

Estimated scale:
- **~9 million+** vulnerability records (growing)
- **30+** active data sources
- **Billions** of package versions indexed
- **Terabytes** of data in GCS

---

## 9. Observability

### 9.1 Logging

- **GCP Cloud Logging** với structured JSON logs
- **Cloud Trace** integration (trace IDs propagated từ gRPC headers)
- **Log-based metrics** cho monitoring (importer run duration, task latency)
- Per-task latency tracking (req_timestamp → processing time)

### 9.2 Monitoring

- **OpenSSF Scorecard** badge cho supply chain security
- Cloud Monitoring dashboards (inferred)
- Import failure logs published to public GCS bucket per source

### 9.3 Health Checks

```python
# gRPC Health Check Protocol
def Check(request, context):
    osv.Vulnerability.query().fetch(1)  # Datastore connectivity check
    return HealthCheckResponse(status=SERVING)
```

---

## 10. Development Architecture

### 10.1 Monorepo Structure

```
osv.dev/
├── osv/              # Core Python library (shared)
├── gcp/
│   ├── api/          # API Server (Python)
│   ├── website/      # Website Backend (Python + Hugo)
│   ├── workers/
│   │   ├── worker/   # Task Worker (Python)
│   │   ├── importer/ # Importer (Python)
│   │   ├── alias/    # Alias Worker
│   │   ├── cron/     # Cron jobs
│   │   └── ...
│   ├── indexer/      # Version Indexer (Go)
│   └── datastore/    # Datastore index.yaml
├── go/               # Go shared library + tools
├── vulnfeeds/        # Vulnerability feed converters (Go)
├── bindings/         # Language bindings (Go)
├── deployment/       # Terraform + Cloud Deploy
├── docker/           # Docker configs
└── docs/             # Jekyll documentation site
```

### 10.2 Language Strategy

| Layer | Language | Reason |
|-------|----------|--------|
| Data pipeline | Python 3.13 | Rich NLP/data processing ecosystem |
| High-performance services | Go | Better performance, type safety |
| Frontend | Hugo + modern JS | Static site generation |
| Infrastructure | Terraform | IaC best practice |
| API schema | Protocol Buffers | Language-agnostic, efficient |

### 10.3 Build System

- **Poetry** - Python dependency management
- **Make** - Build orchestration (`make all-tests`, `make run-api-server`)
- **Docker** - Containerization (Go monorepo: `go/Dockerfile` multi-target)
- **Cloud Build** - CI/CD (YAML configs per service)
- **Cloud Deploy** - Progressive delivery

---

## 11. Data Sources Configuration

### 11.1 Source Types (`source.yaml`)

```yaml
# Type 0: Git repository
- name: 'ghsa'
  type: 0
  repo_url: 'https://github.com/github/advisory-database.git'
  directory_path: 'advisories/github-reviewed'
  extension: '.json'

# Type 1: GCS Bucket
- name: 'go'
  type: 1
  bucket: 'go-vulndb'
  directory_path: 'ID'

# Type 2: REST API
- name: 'chainguard'
  type: 2
  rest_api_url: 'https://packages.cgr.dev/chainguard/osv/all.json'
```

### 11.2 Active Sources (from source.yaml)

| Prefix | Source | Type |
|--------|--------|------|
| ALBA/ALEA/ALSA | AlmaLinux | Git |
| ALPINE | Alpine Linux | Bucket |
| A-/ASB | Android | Bucket |
| AZL | Azure Linux | Git |
| BELL | BellSoft | Git |
| BIT | Bitnami | Git |
| ROOT | Root.io | REST |
| CGA | Chainguard | REST |
| CVE | NVD CVE | Bucket |
| DEBIAN/DLA/DSA/DTSA | Debian | Bucket |
| DRUPAL | Drupal | Git |
| EEF | Erlang ERLEF | REST |
| GHSA | GitHub | Git |
| GO | Go VulnDB | Bucket |
| HSEC | Haskell | Git |
| JLSEC | Julia | Git |
| MGASA | Mageia | REST |
| MAL | Malicious Packages | Git |
| MINI | MinimOS | REST |
| OSEC | OCaml | Git |
| OESA | openEuler | REST |
| OSV | OSS-Fuzz | Git |
| PSF | PSF Python | Git |
| PYSEC | PyPI | Git |
| RSEC | R Consortium | Git |
| RHBA/RHSA/RHEA | Red Hat | REST |
| RLSA/RXSA | Rocky Linux | Bucket |
| RUSTSEC | Rust | Git |
| openSUSE/SUSE | SUSE | REST |
| UBUNTU/LSN/USN | Ubuntu | Git |
| GSD | UVI/GSD | Git |
| V8 | V8/Chromium | Git |
| CURL | curl | REST |
| ECHO | Echo | REST |

---

*Tài liệu này được tạo tự động từ phân tích mã nguồn tại commit hiện tại của repository.*
