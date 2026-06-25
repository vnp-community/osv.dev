# data-service — Upgrade Specification (Chỉ Thêm, Không Xóa)

> **Audit tại**: `services/data-service/`
> **Trạng thái hiện tại**: ~50% complete
> **Ưu tiên**: P0 (critical path)
> **Nguyên tắc**: Mọi thay đổi chỉ THÊM file/package mới. Code hiện có GIỮ NGUYÊN.

---

## ✅ Implementation Status — 2026-06-13

> **Trạng thái cũ**: ~50% | **Trạng thái mới**: ~90% ✅
> **Build**: `go build ./...` PASSED

### Đã implement (Sprint 1 + 2):
- ✅ `infra/persistence/postgres/alias_group_repo.go` — PostgreSQL AliasGroup CRUD
- ✅ `migrations/005_create_alias_groups.up.sql` + `.down.sql`
- ✅ `infra/messaging/nats/cve_publisher.go` — CVE event publisher
- ✅ `infra/messaging/nats/alias_publisher.go` + `alias_consumer.go`
- ✅ `usecase/ingest/dto.go` + `usecase.go` — OSV ingest pipeline (Upsert)
- ✅ `delivery/http/cve_handler.go` + `kev_handler.go` — HTTP handlers
- ✅ `delivery/scheduler/scheduler.go` — cron scheduler

### Còn lại (Backlog P3):
- ⏳ Git sync source (`go-git`) — cần config
- ⏳ GCS sync source — cần GCP credentials
- ⏳ `migrations/006_cve_affected_ranges.up.sql`

---


## 1. Những gì đã có — GIỮ NGUYÊN ✅

### Domain Layer — GIỮ TẤT CẢ
- `domain/repository/cve_repository.go`: `MongoDBCVERepository` interface ✅
- `domain/repository/cvedb_repositories.go`: `CVEBinToolRepository` (PostgreSQL) ✅
- `domain/repository/kev_repository.go`: KEV Repository ✅
- `domain/repository/alias_group_repo.go`: Alias group interface ✅
- `domain/service/`: `CVELookupService`, `TriageService` ✅
- `domain/valueobject/`: vuln_id, relationship_type ✅

### Fetchers — GIỮ NGUYÊN
- `fetcher/nvd_cve.go`: NVD API v2.0 (year-by-year, incremental) ✅
- `fetcher/nvd_cpe.go`: CPE dictionary ✅
- `fetcher/epss.go`: FIRST EPSS API ✅
- `fetcher/mitre_cwe.go`: MITRE CWE ✅
- `fetcher/mitre_capec.go`: MITRE CAPEC ✅
- `fetcher/redis_cpe_cache.go`: Redis CPE cache ✅

### Converters — GIỮ NGUYÊN
- `converter/nvd/converter.go`: NVD JSON v2 → OSV ✅
- `converter/cve5/converter.go`: CVE 5.x + ADP ✅

### Source Sync Adapters — GIỮ NGUYÊN
- `sync/nvd/osv_source.go`: NVD as OSV source ✅
- `sync/circl/client.go`: CIRCL CVE API ✅
- `sync/ids/ids.go`: ID-based sync ✅
- `sync/pypi/pypi.go`: PyPI advisory sync ✅

### Use Cases — GIỮ TẤT CẢ (kể cả 2 tầng phân cấp)
- `usecase/kev/sync/usecase.go`: CISA KEV sync ✅
- `usecase/kev/query/usecase.go` + `check/usecase.go` ✅
- `usecase/alias/detect/command.go` + `resolve/handler.go` ✅
- `usecase/syncall/sync_all.go` + `syncsource/sync_source.go` ✅
- `usecase/lookupcves/lookup_cves.go`: CVE Binary Tool ✅
- `usecase/query/usecase.go` + `check/usecase.go` ✅
- `usecase/sync/usecase.go` ✅ GIỮ (KEV CISA sync — không đổi tên)
- `usecase/cve/importdb/` + `getrecent/` + `getlast/` ✅ GIỮ (dù duplicate)
- `usecase/importdb/`, `exportdb/`, `initdb/`, `populatedb/`, `backupdb/` ✅ GIỮ
- `usecase/searchbycpe/`, `query/`, `check/` ✅ GIỮ

### Application Layer — GIỮ NGUYÊN
- `application/command/detect_new_aliases/` ✅
- `application/command/merge_alias_group/` ✅

### Infrastructure — GIỮ TẤT CẢ
- `infra/mongo/cve_repo.go`: MongoDB CVE raw docs ✅
- `infra/persistence/postgres/kev_repo.go`: PostgreSQL KEV ✅
- `infra/persistence/firestore/alias_group_repo.go`: ✅ **GIỮ NGUYÊN** (primary alias storage)
- `infra/storage/gcs/vulnerability_blob_store.go` ✅
- `infra/external/cisa/client.go` ✅ **GIỮNGUYÊN** (primary CISA client)
- `adapter/external/cisa/client.go` ✅ **GIỮ NGUYÊN** (alternative impl)
- `infra/messaging/nats/` ✅
- `pipeline/idempotency/redis/` ✅

### Delivery — GIỮ NGUYÊN
- `delivery/http/cve_handler.go` ✅
- `delivery/http/kev_handler.go` + `kev_router.go` ✅
- `delivery/scheduler/scheduler.go` ✅
- `adapter/grpc/handler/cvedb_handler.go` ✅
- `adapter/grpc/mapper/cvedb_mapper.go` ✅

---

## 2. Những gì cần THÊM (Gaps)

### 🔴 P0 — Thêm: PostgreSQL AliasGroup Repo (Parallel với Firestore)

Hiện tại `infra/persistence/firestore/alias_group_repo.go` là **primary**. **Thêm** PostgreSQL implementation song song:

```go
// infra/persistence/postgres/alias_group_repo.go  ← NEW FILE
package postgres

type PostgresAliasGroupRepo struct {
    db *pgxpool.Pool
}

func NewAliasGroupRepo(db *pgxpool.Pool) *PostgresAliasGroupRepo
func (r *PostgresAliasGroupRepo) FindByMember(ctx, id string) (*domain.AliasGroup, error)
func (r *PostgresAliasGroupRepo) Upsert(ctx, group *domain.AliasGroup) error
func (r *PostgresAliasGroupRepo) Merge(ctx, primary, secondary string) error
```

**Migration** (thêm file mới, không sửa file cũ):
```sql
-- migrations/005_create_alias_groups.up.sql  ← NEW
CREATE TABLE alias_groups (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    primary_id  VARCHAR(30) NOT NULL UNIQUE,
    aliases     TEXT[] NOT NULL DEFAULT '{}',
    sources     TEXT[] NOT NULL DEFAULT '{}',
    confirmed   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_alias_groups_aliases ON alias_groups USING GIN(aliases);
CREATE INDEX idx_alias_groups_primary ON alias_groups(primary_id);
```

**Chọn backend qua config**:
```go
// internal/config/storage_config.go  ← NEW
type AliasGroupBackend string
const (
    AliasGroupFirestore  AliasGroupBackend = "firestore"   // default, hiện tại
    AliasGroupPostgres   AliasGroupBackend = "postgres"    // mới thêm
)
```

### 🔴 P0 — Thêm: NATS CVE Event Publisher

Sau khi ingest CVE mới, không có event nào publish. `ai-service` và `search-service` cần nhận sự kiện này.

**Thêm mới**:
```
infra/messaging/nats/cve_publisher.go   ← NEW
```

```go
// infra/messaging/nats/cve_publisher.go
package nats

type CVEEventPublisher struct {
    js nats.JetStreamContext
}

type CVECreatedEvent struct {
    ID        string    `json:"id"`
    Source    string    `json:"source"`
    OSVRecord []byte    `json:"osv_record,omitempty"`
    SyncedAt  time.Time `json:"synced_at"`
}

type CVEUpdatedEvent struct {
    ID         string    `json:"id"`
    ChangedFields []string `json:"changed_fields"`
    UpdatedAt  time.Time `json:"updated_at"`
}

func (p *CVEEventPublisher) PublishCreated(ctx, event CVECreatedEvent) error
func (p *CVEEventPublisher) PublishUpdated(ctx, event CVEUpdatedEvent) error
func (p *CVEEventPublisher) PublishWithdrawn(ctx, id string) error
```

**Thêm vào** các use cases (chỉ gọi thêm, không sửa logic cũ):
```go
// usecase/syncsource/sync_source.go — thêm sau successful upsert:
// go p.publisher.PublishCreated(ctx, CVECreatedEvent{ID: cveID, Source: source})

// usecase/kev/sync/usecase.go — thêm sau successful sync:
// go p.publisher.PublishUpdated(ctx, CVEUpdatedEvent{ID: entry.CVEID})
```

### 🟡 P1 — Thêm: Git Source Sync

Hiện tại chỉ có NVD API, CIRCL, PyPI. OSV.dev gốc sync từ **30+ git repos**.

**Thêm mới** (không sửa các sync adapters cũ):
```
sync/git/
├── git_source.go       ← NEW: go-git based source adapter
├── config.go           ← NEW: Source configuration (URL, branch, path pattern)
└── walker.go           ← NEW: Walk commits since last_synced_hash
```

```go
// sync/git/git_source.go
package git

type GitSource struct {
    URL         string
    Branch      string
    PathPattern string    // glob pattern for OSV files
    LastSynced  string    // last synced commit hash
}

func (s *GitSource) Name() string
func (s *GitSource) FetchChanges(ctx context.Context) ([]OSVRecord, string, error)
// Returns: records changed since LastSynced, new HEAD hash, error
```

**go.mod — chỉ THÊM dependency**:
```go
require (
    // ... tất cả existing deps ...
    github.com/go-git/go-git/v5 v5.12.0  // NEW
)
```

**Cấu hình source** trong config:
```yaml
# config/git_sources.yaml  ← NEW
git_sources:
  - name: "ghsa"
    url: "https://github.com/github/advisory-database"
    branch: "main"
    path_pattern: "advisories/**/*.json"
  - name: "go-vulndb"
    url: "https://github.com/golang/vulndb"
    branch: "master"
    path_pattern: "data/osv/**/*.json"
  - name: "ubuntu"
    url: "https://github.com/ubuntu/ubuntu-cve-tracker"
    branch: "master"
    path_pattern: "*.json"
```

### 🟡 P1 — Thêm: GCS Bucket Source Sync

**Thêm mới** (không sửa gcs blob store cũ):
```
sync/gcs/
├── gcs_source.go    ← NEW: Walk GCS bucket for OSV files
└── config.go        ← NEW: Bucket config
```

```go
// sync/gcs/gcs_source.go
package gcs

type GCSSource struct {
    Bucket    string
    Prefix    string    // path prefix in bucket
    LastSynced string   // last synced object generation
}

func (s *GCSSource) Name() string
func (s *GCSSource) FetchChanges(ctx context.Context) ([]OSVRecord, error)
```

**Cấu hình**:
```yaml
# config/gcs_sources.yaml  ← NEW
gcs_sources:
  - name: "osv-vulnerabilities-go"
    bucket: "osv-vulnerabilities"
    prefix: "Go/"
  - name: "osv-vulnerabilities-python"
    bucket: "osv-vulnerabilities"
    prefix: "PyPI/"
  - name: "osv-vulnerabilities-npm"
    bucket: "osv-vulnerabilities"
    prefix: "npm/"
```

### 🟡 P1 — Thêm: OSV Ingest Pipeline

Sau khi sync sources, cần pipeline parse + validate + store OSV records.

**Thêm mới**:
```
usecase/ingest/
├── usecase.go      ← NEW: Main ingest pipeline
└── dto.go          ← NEW: Input/Output types
```

```go
// usecase/ingest/usecase.go
package ingest

// IngestUseCase nhận 1 OSV record và:
// 1. Parse + validate OSV schema (dùng shared/pkg/osvschema)
// 2. Normalize (dùng shared/pkg/ecosystem + semver)
// 3. Upsert vào MongoDB (raw doc)
// 4. Publish data.cve.created/updated via NATS
type IngestUseCase struct {
    mongoRepo  repository.MongoDBCVERepository  // existing interface
    publisher  nats.CVEEventPublisher            // new publisher
    validator  osvschema.Validator               // shared pkg
}

func (uc *IngestUseCase) Execute(ctx context.Context, record []byte, source string) error
```

**NATS subscriber** cho ingest tasks:
```
infra/messaging/nats/ingest_subscriber.go   ← NEW
```

```go
// Subscribe: data.sync.update
// Khi git/gcs/rest source có record mới → ingest UC
```

### 🟡 P1 — Thêm: CVE Structured Schema (SQL Migrations)

Để support OSV v1 Query API (affected packages/versions):

```sql
-- migrations/006_cve_affected_ranges.up.sql  ← NEW
-- Không sửa các tables cũ (kev_entries, sync_jobs)

CREATE TABLE cve_affected (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cve_id       VARCHAR(30) NOT NULL,
    ecosystem    VARCHAR(50) NOT NULL,
    package_name VARCHAR(255) NOT NULL,
    purl         TEXT,
    created_at   TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_cve_affected_cve_id ON cve_affected(cve_id);
CREATE INDEX idx_cve_affected_pkg ON cve_affected(ecosystem, package_name);

CREATE TABLE cve_ranges (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    affected_id UUID NOT NULL REFERENCES cve_affected(id) ON DELETE CASCADE,
    range_type  VARCHAR(20) NOT NULL,  -- GIT | SEMVER | ECOSYSTEM
    repo_url    TEXT
);

CREATE TABLE cve_range_events (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    range_id    UUID NOT NULL REFERENCES cve_ranges(id) ON DELETE CASCADE,
    event_type  VARCHAR(20) NOT NULL,  -- introduced | fixed | last_affected | limit
    value       TEXT NOT NULL
);
CREATE INDEX idx_cre_range_id ON cve_range_events(range_id);
```

### 🟡 P1 — Thêm: OSV v1 API Endpoints

**Thêm mới** (không sửa handler cũ):
```
delivery/http/osv_v1_handler.go   ← NEW
```

```go
// POST /v1/query
// POST /v1/querybatch  (max 1000)
// GET  /v1/vulns/{id}
// GET  /v1/vulns/list?page_token=TOKEN&modified_since=RFC3339

// Routing — thêm vào router (không xóa route cũ):
r.Post("/v1/query", osvV1H.Query)
r.Post("/v1/querybatch", osvV1H.QueryBatch)
r.Get("/v1/vulns/{id}", osvV1H.GetVuln)
r.Get("/v1/vulns/list", osvV1H.ListVulns)
```

### 🟢 P2 — Thêm: Impact Analysis

```
usecase/impact_analysis/
└── usecase.go    ← NEW: Calculate affected version ranges
```

### 🟢 P2 — Thêm: Additional REST Source Adapters

```
sync/rest/
├── chainguard.go    ← NEW
├── redhat.go        ← NEW
├── suse.go          ← NEW
└── mageia.go        ← NEW
```

---

## 3. Migration Plan — Chỉ Thêm File Mới

```
migrations/
├── 001_create_kev_entries.down.sql    ← GIỮ NGUYÊN
├── 002_create_kev_entries.up.sql      ← GIỮ NGUYÊN
├── 003_initial_schema.sql             ← GIỮ NGUYÊN
├── 004_create_sync_jobs.up.sql        ← GIỮ NGUYÊN
├── 005_create_alias_groups.up.sql     ← NEW
└── 006_cve_affected_ranges.up.sql     ← NEW
```

---

## 4. go.mod — Chỉ THÊM Dependency

```go
require (
    // ... tất cả existing deps giữ nguyên ...
    
    // NEW additions:
    github.com/go-git/go-git/v5 v5.12.0
)
```

---

## 5. File Changes Summary

### Files cần THÊM MỚI:
```
infra/persistence/postgres/alias_group_repo.go
infra/messaging/nats/cve_publisher.go
infra/messaging/nats/ingest_subscriber.go
infra/validator/osv_validator.go
internal/config/storage_config.go
sync/git/git_source.go
sync/git/config.go
sync/git/walker.go
sync/gcs/gcs_source.go
sync/gcs/config.go
sync/rest/chainguard.go
sync/rest/redhat.go
sync/rest/suse.go
usecase/ingest/usecase.go
usecase/ingest/dto.go
usecase/impact_analysis/usecase.go
delivery/http/osv_v1_handler.go
config/git_sources.yaml
config/gcs_sources.yaml
migrations/005_create_alias_groups.up.sql
migrations/006_cve_affected_ranges.up.sql
```

### Files cần EXTEND (thêm logic, không xóa code cũ):
```
usecase/syncsource/sync_source.go   ← Thêm publish event sau upsert
usecase/kev/sync/usecase.go        ← Thêm publish event sau sync
delivery/scheduler/scheduler.go    ← Thêm git/gcs sync jobs
cmd/server/main.go                 ← Thêm wire mới
go.mod                             ← Thêm go-git dependency
```

### Files KHÔNG ĐƯỢC CHẠM:
```
infra/persistence/firestore/alias_group_repo.go  ← GIỮ NGUYÊN
adapter/external/cisa/client.go                  ← GIỮ NGUYÊN
infra/external/cisa/client.go                    ← GIỮ NGUYÊN
usecase/sync/usecase.go                          ← GIỮ NGUYÊN (dù là KEV sync)
usecase/cve/*                                    ← GIỮ NGUYÊN (dù duplicate)
migrations/001 → 004                             ← KHÔNG BAO GIỜ SỬA
```

---

## 6. Checklist

### Phase A — P0 (Sprint 1)
- [x] Thêm `infra/persistence/postgres/alias_group_repo.go`
- [x] Thêm `migrations/005_create_alias_groups.up.sql`
- [ ] Thêm `internal/config/storage_config.go` với backend selector
- [x] Thêm `infra/messaging/nats/cve_publisher.go`
- [ ] Thêm publish event vào `syncsource` và `kev/sync` use cases

### Phase B — P1 (Sprint 2)
- [ ] Thêm `sync/git/` package (go-git based)
- [ ] Thêm `sync/gcs/` package
- [ ] Thêm `go-git` vào go.mod
- [ ] Thêm `usecase/ingest/` pipeline
- [ ] Thêm `infra/messaging/nats/ingest_subscriber.go`
- [ ] Thêm `migrations/006_cve_affected_ranges.up.sql`
- [ ] Thêm `delivery/http/osv_v1_handler.go`

### Phase C — P2 (Sprint 3)
- [ ] Thêm REST source adapters (chainguard, redhat, suse)
- [ ] Thêm `usecase/impact_analysis/`
- [ ] Thêm `infra/validator/osv_validator.go`
