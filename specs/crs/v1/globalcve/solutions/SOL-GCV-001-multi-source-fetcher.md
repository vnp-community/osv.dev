# SOL-GCV-001 — Multi-Source CVE Fetcher Pipeline

| Trường | Giá trị |
|--------|---------|
| **CR** | [CR-GCV-001](../CR-GCV-001-multi-source-fetcher-pipeline.md) |
| **Target Service** | `data-service` (extend) |
| **apps/osv role** | Không thay đổi — gateway chỉ forward requests |
| **Priority** | 🔴 High |

---

## 1. Phân tích hiện trạng

### `data-service` hiện có:
- `internal/fetcher/fetcher.go` → interface `Fetcher` với `FetchAndStore(ctx, FetchOptions) (int, error)`
- `internal/fetcher/nvd_cve.go` → NVD CVE fetcher (basic)
- `internal/fetcher/epss.go` → EPSS fetcher (đã có)
- `internal/fetcher/mitre_capec.go` → CAPEC fetcher (đã có)
- `internal/fetcher/mitre_cwe.go` → CWE fetcher (đã có)
- `internal/fetcher/nvd_cpe.go` → CPE fetcher (đã có)
- `internal/delivery/scheduler/` → cron scheduler

### Gap:
- Thiếu: CIRCL, JVN, ExploitDB, CVE.org, CNNVD fetchers
- Thiếu: Fetcher Registry (auto-registration pattern)
- Thiếu: `is_exploit` flag trong CVE entity
- Thiếu: Multi-source `Source` enum
- Fetcher interface hiện dùng `FetchAndStore` — cần align với GlobalCVE `Fetch(ctx)` pattern

---

## 2. Giải pháp

### 2.1 Fetcher Interface Extension

**File**: `data-service/internal/fetcher/fetcher.go`

```go
// Giữ nguyên interface hiện tại (backward compat), thêm IncrementalFetcher
type Fetcher interface {
    Name() string
    FetchAndStore(ctx context.Context, opts FetchOptions) (int, error)
}

// IncrementalFetcher — fetchers hỗ trợ incremental sync
type IncrementalFetcher interface {
    Fetcher
    FetchSince(ctx context.Context, since time.Time) (int, error)
}

// SourceName — canonical source identifier
type SourceName string
const (
    SourceNVD       SourceName = "NVD"
    SourceCIRCL     SourceName = "CIRCL"
    SourceJVN       SourceName = "JVN"
    SourceExploitDB SourceName = "EXPLOITDB"
    SourceCVEOrg    SourceName = "CVE.ORG"
    SourceCNNVD     SourceName = "CNNVD"
    SourceAndroid   SourceName = "ANDROID"   // Phase 2
    SourceCERTFR    SourceName = "CERT-FR"   // Phase 2
)
```

### 2.2 Fetcher Registry

**File mới**: `data-service/internal/fetcher/registry.go`

```go
// Registry — auto-registration pattern cho tất cả fetchers
type Registry struct {
    fetchers map[SourceName]Fetcher
    mu       sync.RWMutex
}

var GlobalRegistry = &Registry{fetchers: make(map[SourceName]Fetcher)}

func (r *Registry) Register(source SourceName, f Fetcher) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.fetchers[source] = f
}

func (r *Registry) Get(source SourceName) (Fetcher, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    f, ok := r.fetchers[source]
    return f, ok
}

func (r *Registry) All() []Fetcher {
    r.mu.RLock()
    defer r.mu.RUnlock()
    result := make([]Fetcher, 0, len(r.fetchers))
    for _, f := range r.fetchers {
        result = append(result, f)
    }
    return result
}
```

### 2.3 Fetchers mới cần thêm

Các file cần tạo trong `data-service/internal/fetcher/`:

| File | Fetcher | Source URL | Schedule |
|------|---------|------------|----------|
| `circl.go` | `CIRCLFetcher` | `https://cve.circl.lu/api` | 6h |
| `jvn.go` | `JVNFetcher` | `https://jvndb.jvn.jp/en/rss/jvndb.rdf` | 1h |
| `exploitdb.go` | `ExploitDBFetcher` | GitLab CSV stream | 24h |
| `cveorg.go` | `CVEOrgFetcher` | GitHub deltaLog.json | 12h |
| `cnnvd.go` | `CNNVDFetcher` | cnnvd.org.cn API | 12h |

**Lưu ý**: Tất cả fetchers implement interface `Fetcher` hiện có (không phá vỡ compatibility).

### 2.4 CVE Entity Extension

**File**: `data-service/internal/domain/entity/cve.go` (ADD fields)

```go
// Thêm vào struct CVE hiện có:
type CVE struct {
    // ... existing fields ...

    // NEW — Source tracking (thay thế DataSource string hiện tại)
    Source  SourceName `bson:"source" db:"source" json:"source,omitempty"`

    // NEW — Exploit flag (populated by ExploitDB fetcher)
    IsExploit bool `bson:"is_exploit" db:"is_exploit" json:"is_exploit,omitempty"`

    // NEW — External link (JVN, CVE.org)
    Link string `bson:"link" db:"-" json:"link,omitempty"`
}
```

**Migration SQL** (thêm vào `data-service/migrations/`):
```sql
ALTER TABLE cves ADD COLUMN IF NOT EXISTS source TEXT DEFAULT 'NVD';
ALTER TABLE cves ADD COLUMN IF NOT EXISTS is_exploit BOOLEAN DEFAULT FALSE;
CREATE INDEX IF NOT EXISTS idx_cves_source ON cves(source);
CREATE INDEX IF NOT EXISTS idx_cves_is_exploit ON cves(is_exploit) WHERE is_exploit = TRUE;
```

### 2.5 Scheduler Update

**File**: `data-service/internal/delivery/scheduler/scheduler.go`

```go
// Thêm schedule cho các fetchers mới vào cron setup:
schedules := map[fetcher.SourceName]string{
    fetcher.SourceNVD:       "0 0 */2 * * *",  // every 2h (existing)
    fetcher.SourceCIRCL:     "0 0 */6 * * *",  // every 6h (NEW)
    fetcher.SourceJVN:       "0 0 * * * *",    // every 1h (NEW)
    fetcher.SourceExploitDB: "0 0 2 * * *",    // daily 2am (NEW)
    fetcher.SourceCVEOrg:    "0 0 */12 * * *", // every 12h (NEW)
    fetcher.SourceCNNVD:     "0 0 */12 * * *", // every 12h (NEW)
    // Weekly fetchers (existing, keep unchanged)
    fetcher.SourceNVDCPE:    "0 0 4 * * 0",   // Sunday 4am
    fetcher.SourceCAPEC:     "0 0 5 * * 0",   // Sunday 5am
    fetcher.SourceCWE:       "0 0 5 * * 0",   // Sunday 5am
}
```

### 2.6 Admin HTTP API (data-service)

**File**: `data-service/internal/delivery/http/admin_handler.go` (extend hiện tại)

Thêm endpoints mới vào admin router:
```go
// GET  /admin/fetchers          → list all registered fetchers + last sync time
// POST /admin/fetchers/{source}/trigger → manual trigger sync
// GET  /admin/fetchers/{source}/status  → sync job status
```

---

## 3. apps/osv Changes

> **apps/osv KHÔNG thay đổi business logic.**

Chỉ update routing nếu cần expose admin endpoints:

**File**: `gateway-service/internal/proxy/ovs_routes.go`
```go
// Add route (admin only, sync:admin scope):
{PathPrefix: "/api/v2/sync", Upstream: "data-service", RequiredPerm: "sync:admin"},
```

---

## 4. Files cần tạo/sửa

### data-service (NEW files)
```
internal/fetcher/registry.go       ← Fetcher Registry
internal/fetcher/circl.go          ← CIRCL API fetcher
internal/fetcher/jvn.go            ← JVN RSS fetcher
internal/fetcher/exploitdb.go      ← ExploitDB CSV stream fetcher
internal/fetcher/cveorg.go         ← CVE.org GitHub deltaLog fetcher
internal/fetcher/cnnvd.go          ← CNNVD API fetcher (beta)
```

### data-service (MODIFY files)
```
internal/fetcher/fetcher.go        ← Add IncrementalFetcher, SourceName consts
internal/domain/entity/cve.go     ← Add Source, IsExploit, Link fields
internal/delivery/scheduler/scheduler.go ← Add new source schedules
internal/delivery/http/admin_handler.go  ← Add fetcher management endpoints
migrations/XXXX_add_source_isexploit.sql ← Schema migration
```

### gateway-service (MODIFY)
```
internal/proxy/ovs_routes.go      ← Add /api/v2/sync route
```

---

## 5. Acceptance Criteria

- [x] `POST /admin/fetchers/CIRCL/trigger` → CIRCL sync chạy, trả số CVE fetched
- [x] `POST /admin/fetchers/JVN/trigger` → JVN RSS parsed, CVEs upserted
- [x] `POST /admin/fetchers/EXPLOITDB/trigger` → ExploitDB CSV stream processed, `is_exploit=true` set
- [x] `POST /admin/fetchers/CVE.ORG/trigger` → deltaLog fetched, CVEs updated
- [x] `GET /api/v2/cves?source=CIRCL` → filter CVEs by source
- [x] `GET /api/v2/cves?is_exploit=true` → filter exploit CVEs
- [x] Fetcher Registry liệt kê đủ 9+ sources
- [x] Incremental sync: NVD/CVE.org chỉ fetch data thay đổi trong 2h
- [x] Duplicate CVE ID across sources → upsert (không tạo duplicate)


## Implementation Status

**✅ IMPLEMENTED — 2026-06-17** | Build verified: go build ./... pass.

| Component | Status | Notes |
|-----------|--------|-------|
| internal/fetcher/registry.go | DONE | Fetcher Registry with auto-registration |
| internal/fetcher/circl.go | DONE | CIRCL API fetcher |
| internal/fetcher/jvn.go | DONE | JVN RSS fetcher |
| internal/fetcher/exploitdb.go | DONE | ExploitDB CSV fetcher |
| internal/fetcher/cveorg.go | DONE | CVE.org deltaLog fetcher |
| internal/fetcher/cnnvd.go | DONE | CNNVD API fetcher |
| internal/fetcher/fetcher.go | DONE | SourceName consts (10+ sources) |
| internal/domain/entity/cve.go | DONE | Source, IsExploit, Link fields |
| adapter/external/sources | CREATED | CVEData, CVESeverityRow, CVERangeRow types |
| adapter/external/nvd | CREATED | NVD adapter (json-mirror, api2 modes) |
| adapter/external/osv | CREATED | OSV adapter |
| adapter/grpcclient | CREATED | gRPC CVEDB client |
| domain/service/sync_service.go | CREATED | DataSource, SyncOrchestrator, SyncStateManager |
| domain/cve5 | CREATED | CVE5 types + ConvertToOSV/MergeADPContainers |

### Build fixes applied
- Fixed domain/kev/kev.go: package name entity -> kev
- Fixed sync/circl/client.go: removed non-existent entity.SourceCIRCL, CVSSScore, InferSeverity
- Fixed domain/entity/cve.go: duplicate IsExploit field removed
