# data-service

**Bounded Context**: Vulnerability Data Management
**Go Module**: `github.com/osv/data-service`

---

## Merge từ

| Source | Trạng thái |
|--------|-----------|
| `services/vulnerability-service` | ✅ Active — base chính |
| `services/ingestion-service` | ✅ Active — merged |
| `archive/cve-service` | 📦 Archive — merged |
| `archive/ingestion` | 📦 Archive — merged |
| `archive/source-sync` | 📦 Archive — merged |
| `archive/kev-service` | 📦 Archive — merged |
| `archive/taxonomy-service` | 📦 Archive — merged |
| `archive/cve-sync-service` | 📦 Archive — merged |
| `archive/converter` | 📦 Archive — merged |
| `archive/alias-relations` | 📦 Archive — merged |
| `archive/version-index` | 📦 Archive → moved to impact-service |

---

## Chức năng

| # | Chức năng | Mô tả |
|---|-----------|-------|
| 1 | **CVE Store** | Lưu trữ, CRUD toàn bộ CVE records từ mọi nguồn |
| 2 | **Multi-source Ingest** | Thu thập từ NVD, OSV, GHSA, GitHub Advisory, MITRE |
| 3 | **Incremental Sync** | Sync delta updates theo timestamp từ upstream |
| 4 | **Full Sync** | Rebuild toàn bộ database từ upstream sources |
| 5 | **KEV Management** | Quản lý CISA KEV list (Known Exploited Vulnerabilities) |
| 6 | **Taxonomy** | CWE weakness taxonomy, CPE platform enumeration |
| 7 | **Alias Resolution** | Cross-reference CVE ↔ GHSA ↔ CWE ↔ npm advisories |
| 8 | **Event Publishing** | Phát sự kiện khi CVE được tạo/cập nhật |
| 9 | **Format Conversion** | Convert CVE 4.x / 5.x / OSV JSON về chuẩn nội bộ |
| 10 | **OSV Schema** | Xuất dữ liệu theo OSV open standard |

---

## Clean Architecture Layout

```
data-service/
├── cmd/
│   └── server/
│       └── main.go
│
├── internal/
│   ├── domain/                         # ← Business rules (no external deps)
│   │   ├── cve/
│   │   │   ├── entity.go               # CVE aggregate root
│   │   │   ├── repository.go           # CVERepository interface
│   │   │   ├── events.go               # CVECreated, CVEUpdated, CVEWithdrawn
│   │   │   └── service.go              # Domain service (merge, validate)
│   │   ├── kev/
│   │   │   ├── entity.go               # KEVEntry entity
│   │   │   └── repository.go
│   │   ├── taxonomy/
│   │   │   ├── cwe.go                  # CWE entity
│   │   │   ├── cpe.go                  # CPE entity
│   │   │   └── repository.go
│   │   ├── alias/
│   │   │   ├── entity.go               # AliasMap entity
│   │   │   └── repository.go
│   │   ├── source/
│   │   │   ├── entity.go               # DataSource entity (NVD, OSV, etc.)
│   │   │   └── sync_job.go             # SyncJob entity
│   │   └── errors/
│   │       └── errors.go
│   │
│   ├── usecase/                        # ← Application use cases
│   │   ├── ingest/
│   │   │   ├── usecase.go              # Run ingestion from a source
│   │   │   └── dto.go
│   │   ├── sync/
│   │   │   ├── incremental.go          # Incremental sync logic
│   │   │   └── full.go                 # Full rebuild sync
│   │   ├── get_cve/
│   │   │   └── usecase.go
│   │   ├── batch_query/
│   │   │   └── usecase.go
│   │   ├── update_cve/
│   │   │   └── usecase.go
│   │   ├── resolve_alias/
│   │   │   └── usecase.go
│   │   ├── manage_kev/
│   │   │   └── usecase.go              # CRUD KEV entries
│   │   └── manage_taxonomy/
│   │       └── usecase.go              # Sync CWE/CPE taxonomy
│   │
│   ├── delivery/                       # ← Transport layer
│   │   ├── grpc/
│   │   │   ├── server.go
│   │   │   ├── cve_handler.go          # CVEService RPC impl
│   │   │   └── sync_handler.go         # DataSyncService RPC impl
│   │   └── http/
│   │       ├── router.go
│   │       ├── cve_handler.go
│   │       ├── kev_handler.go
│   │       └── admin_handler.go        # Trigger sync, admin ops
│   │
│   ├── infra/                          # ← External systems
│   │   ├── postgres/
│   │   │   ├── cve_repo.go
│   │   │   ├── kev_repo.go
│   │   │   └── taxonomy_repo.go
│   │   ├── mongo/
│   │   │   ├── cve_raw_repo.go         # Raw CVE documents
│   │   │   └── alias_repo.go
│   │   ├── firestore/
│   │   │   └── cve_cache.go            # Hot cache for frequently accessed CVEs
│   │   ├── gcs/
│   │   │   └── dataset_store.go        # Large dataset downloads (NVD feeds)
│   │   └── nats/
│   │       └── publisher.go            # Publish CVE events
│   │
│   └── fetcher/                        # ← Data source adapters
│       ├── nvd/
│       │   ├── client.go               # NVD API 2.0 client
│       │   └── converter.go            # NVD JSON → domain CVE
│       ├── osv/
│       │   ├── client.go               # OSV.dev API client
│       │   └── converter.go
│       ├── ghsa/
│       │   ├── client.go               # GitHub Advisory DB
│       │   └── converter.go
│       └── mitre/
│           ├── client.go               # CVE List from MITRE
│           └── converter.go
│
├── migrations/
│   ├── 001_create_cves.sql
│   ├── 002_create_kev.sql
│   ├── 003_create_taxonomy.sql
│   └── 004_create_sync_jobs.sql
│
├── go.mod
└── Dockerfile
```

---

## Domain Model

### CVE Aggregate Root
```go
type CVE struct {
    ID            string          // CVE-2024-XXXXX
    State         CVEState        // PUBLISHED | REJECTED | RESERVED
    Title         string
    Description   string
    Published     time.Time
    LastModified  time.Time
    CVSS          []CVSSScore     // v2, v3.0, v3.1, v4.0
    Severity      SeverityLevel   // CRITICAL | HIGH | MEDIUM | LOW | NONE
    CWEIDs        []string        // CWE-89, CWE-79, etc.
    CPEs          []CPE           // Affected platforms
    References    []Reference     // URLs
    Aliases       []string        // GHSA-xxx, PYSEC-xxx
    KEV           *KEVInfo        // nil if not in KEV list
    AffectedPkgs  []AffectedPkg  // Package-level affected versions
    OSVSchema     *OSVRecord      // OSV format representation
    Source        DataSource      // NVD | OSV | GHSA | MITRE
    CreatedAt     time.Time
    UpdatedAt     time.Time
}

type CVSSScore struct {
    Version    string      // 2.0 | 3.0 | 3.1 | 4.0
    VectorStr  string
    BaseScore  float64
    Severity   string
    Source     string      // NVD | CNA | ADP
}

type AffectedPkg struct {
    Ecosystem   string      // npm | pypi | go | maven | etc.
    PackageName string
    PURL        string
    Versions    VersionRange
}
```

### KEV Entry
```go
type KEVEntry struct {
    CVEID           string
    VendorProject   string
    Product         string
    VulnerabilityName string
    DateAdded       time.Time
    ShortDescription string
    RequiredAction  string
    DueDate         time.Time
}
```

### SyncJob
```go
type SyncJob struct {
    ID        uuid.UUID
    Source    DataSource
    Type      SyncType     // FULL | INCREMENTAL
    Status    SyncStatus   // PENDING | RUNNING | SUCCESS | FAILED
    StartedAt *time.Time
    EndedAt   *time.Time
    Stats     SyncStats    // total, created, updated, skipped
}
```

---

## API Specification

### HTTP REST Endpoints

| Method | Path | Auth | Mô tả |
|--------|------|------|-------|
| `GET`  | `/cve/{id}` | JWT | Lấy chi tiết CVE |
| `POST` | `/cve/query` | JWT | Batch query nhiều CVE |
| `GET`  | `/cve/{id}/kev` | JWT | KEV status của CVE |
| `GET`  | `/cve/{id}/aliases` | JWT | Alias list |
| `GET`  | `/cve/{id}/affected` | JWT | Danh sách affected packages |
| `GET`  | `/kev` | JWT | Danh sách toàn bộ KEV |
| `GET`  | `/kev/{cve_id}` | JWT | Chi tiết KEV entry |
| `GET`  | `/taxonomy/cwe/{id}` | JWT | CWE detail |
| `GET`  | `/taxonomy/cpe` | JWT | CPE lookup |
| `POST` | `/admin/sync` | Admin | Trigger sync job |
| `GET`  | `/admin/sync/jobs` | Admin | Danh sách sync jobs |
| `GET`  | `/admin/sync/jobs/{id}` | Admin | Chi tiết sync job |

### gRPC Services (internal)

```protobuf
service CVEService {
    rpc GetCVE(GetCVERequest) returns (CVEResponse);
    rpc BatchGetCVE(BatchGetCVERequest) returns (BatchGetCVEResponse);
    rpc ListCVE(ListCVERequest) returns (stream CVEResponse);
    rpc GetCVEsByPackage(GetCVEsByPackageRequest) returns (GetCVEsByPackageResponse);
}

service DataSyncService {
    rpc TriggerSync(TriggerSyncRequest) returns (SyncJobResponse);
    rpc GetSyncStatus(GetSyncStatusRequest) returns (SyncStatusResponse);
}
```

---

## Event Publishing (NATS)

| Event | Subject | Payload |
|-------|---------|---------|
| `CVECreated` | `data.cve.created` | CVE ID, source, severity |
| `CVEUpdated` | `data.cve.updated` | CVE ID, changed fields |
| `CVEWithdrawn` | `data.cve.withdrawn` | CVE ID |
| `KEVUpdated` | `data.kev.updated` | Count added/removed |
| `SyncCompleted` | `data.sync.completed` | Source, stats |

**Consumers**: search-service, ai-service, finding-service

---

## Dependencies

### External Libraries
```
github.com/jackc/pgx/v5                  # PostgreSQL
go.mongodb.org/mongo-driver              # MongoDB
cloud.google.com/go/firestore            # Firestore
cloud.google.com/go/storage              # GCS
github.com/nats-io/nats.go               # NATS messaging
github.com/go-chi/chi/v5                 # HTTP router
google.golang.org/grpc                   # gRPC
github.com/robfig/cron/v3                # Scheduled sync
github.com/ossf/osv-schema/bindings/go  # OSV schema validation
github.com/osv/shared/pkg                # Shared utilities
github.com/osv/shared/proto              # gRPC contracts
```

---

## Database Schema (PostgreSQL)

```sql
-- CVE table
CREATE TABLE cves (
    id            VARCHAR(30) PRIMARY KEY,  -- CVE-2024-XXXXX
    state         VARCHAR(20) NOT NULL,
    title         TEXT,
    description   TEXT,
    published_at  TIMESTAMPTZ,
    modified_at   TIMESTAMPTZ,
    severity      VARCHAR(20),
    cvss_v31_score NUMERIC(3,1),
    cvss_v31_vector TEXT,
    source        VARCHAR(20),
    created_at    TIMESTAMPTZ DEFAULT NOW(),
    updated_at    TIMESTAMPTZ DEFAULT NOW()
);

-- KEV
CREATE TABLE kev_entries (
    cve_id         VARCHAR(30) PRIMARY KEY REFERENCES cves(id),
    vendor_project VARCHAR(255),
    product        TEXT,
    date_added     DATE NOT NULL,
    due_date       DATE,
    required_action TEXT
);

-- Aliases
CREATE TABLE cve_aliases (
    cve_id     VARCHAR(30) REFERENCES cves(id),
    alias      VARCHAR(100),
    alias_type VARCHAR(20),  -- GHSA | PYSEC | RUSTSEC | etc.
    PRIMARY KEY (cve_id, alias)
);

-- Sync jobs
CREATE TABLE sync_jobs (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source     VARCHAR(20),
    type       VARCHAR(20),
    status     VARCHAR(20),
    started_at TIMESTAMPTZ,
    ended_at   TIMESTAMPTZ,
    stats      JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

---

## Configuration

```yaml
server:
  http_port: 8082
  grpc_port: 50052

postgres:
  dsn: "${POSTGRES_DSN}"

mongo:
  uri: "${MONGO_URI}"
  database: "cve_raw"

firestore:
  project_id: "${GCP_PROJECT_ID}"

gcs:
  bucket: "${GCS_BUCKET}"

nats:
  url: "${NATS_URL}"
  stream: "DATA_EVENTS"

sync:
  nvd:
    api_key: "${NVD_API_KEY}"
    schedule: "0 2 * * *"       # Daily at 02:00
  osv:
    schedule: "0 */4 * * *"     # Every 4 hours
  ghsa:
    token: "${GITHUB_TOKEN}"
    schedule: "0 3 * * *"
```
