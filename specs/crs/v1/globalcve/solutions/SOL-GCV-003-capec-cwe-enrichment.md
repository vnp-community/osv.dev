# SOL-GCV-003 — MITRE CAPEC + CWE Enrichment

| Trường | Giá trị |
|--------|---------|
| **CR** | [CR-GCV-003](../CR-GCV-003-mitre-capec-cwe-enrichment.md) |
| **Target Service** | `data-service` (sync) + `search-service` (endpoints + filter) |
| **apps/osv role** | Không thay đổi |
| **Priority** | 🟡 Medium |

---

## 1. Hiện trạng

- `data-service/internal/fetcher/mitre_capec.go` → **đã có** CAPEC fetcher
- `data-service/internal/fetcher/mitre_cwe.go` → **đã có** CWE fetcher
- `data-service/internal/domain/taxonomy/` → có thể đã có CWE/CAPEC domain
- `data-service/internal/domain/entity/cve.go` → `CWE []string` **đã có**
- `search-service` → **chưa có** `/api/v2/cwe` và `/api/v2/capec` endpoints

---

## 2. Giải pháp

### 2.1 data-service — Verify fetchers hiện có

**Verify `mitre_cwe.go`** phải:
1. Download từ `https://cwe.mitre.org/data/xml/cwec_latest.xml.zip`
2. Parse XML → extract `CWE-ID`, `Name`, `Description`, `Related_Attack_Patterns` (links CAPEC)
3. Upsert vào bảng `cwe_weaknesses`

**Verify `mitre_capec.go`** phải:
1. Download từ `https://capec.mitre.org/data/xml/capec_latest.xml`
2. Parse XML → extract `CAPEC-ID`, `Name`, `Description`, `Related_Weaknesses` (links CWE)
3. Upsert vào bảng `capec_patterns`

**Migration** (nếu chưa có):
```sql
CREATE TABLE IF NOT EXISTS cwe_weaknesses (
    id              TEXT    PRIMARY KEY,   -- "CWE-89"
    name            TEXT    NOT NULL,
    description     TEXT,
    abstraction     TEXT,                  -- "Base" | "Class" | "Variant"
    status          TEXT,
    capec_ids       TEXT[],               -- linked CAPEC IDs
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS capec_patterns (
    id              TEXT    PRIMARY KEY,   -- "CAPEC-66"
    name            TEXT    NOT NULL,
    description     TEXT,
    likelihood      TEXT,                  -- "High" | "Medium" | "Low"
    severity        TEXT,
    cwe_ids         TEXT[],               -- linked CWE IDs
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Link CVE ↔ CWE (many-to-many, denormalized on cves table)
-- cve_id đã có column CWE TEXT[] trong bảng cves
-- Thêm index:
CREATE INDEX IF NOT EXISTS idx_cves_cwe ON cves USING GIN(cwe);
```

### 2.2 search-service — CWE/CAPEC Endpoints

**File mới**: `search-service/internal/delivery/http/taxonomy_handler.go`

```go
// GET /api/v2/cwe
// Params: q (search name), page, limit
func (h *Handler) ListCWE(w http.ResponseWriter, r *http.Request) { ... }

// GET /api/v2/cwe/{id}
// e.g. GET /api/v2/cwe/CWE-89
func (h *Handler) GetCWE(w http.ResponseWriter, r *http.Request) { ... }

// GET /api/v2/capec
// Params: q, cwe_id (filter by linked CWE), page, limit
func (h *Handler) ListCAPEC(w http.ResponseWriter, r *http.Request) { ... }

// GET /api/v2/capec/{id}
// e.g. GET /api/v2/capec/CAPEC-66
func (h *Handler) GetCAPEC(w http.ResponseWriter, r *http.Request) { ... }
```

**CWE Filter trong CVE Search**:

```go
// search-service/internal/usecase/cvesearch/request.go
type Request struct {
    // ... existing fields ...
    CWE string  // NEW: filter by CWE ID, e.g. "CWE-89"
}

// Trong usecase: WHERE $CWE = ANY(cwe)
if req.CWE != "" {
    query = query.Where("? = ANY(cwe)", req.CWE)
}
```

**Route registration** trong `search-service/internal/delivery/http/search_handler.go`:
```go
r.Get("/api/v2/cwe", h.ListCWE)
r.Get("/api/v2/cwe/{id}", h.GetCWE)
r.Get("/api/v2/capec", h.ListCAPEC)
r.Get("/api/v2/capec/{id}", h.GetCAPEC)
r.Get("/api/v2/cves", h.SearchCVEs)  // now supports ?cwe=CWE-89
```

**Repository**:

```go
// search-service/internal/domain/repository/taxonomy_repo.go (NEW)
type CWERepository interface {
    List(ctx context.Context, q string, page, limit int) ([]*CWEEntry, int64, error)
    FindByID(ctx context.Context, id string) (*CWEEntry, error)
}

type CAPECRepository interface {
    List(ctx context.Context, q, cweID string, page, limit int) ([]*CAPECEntry, int64, error)
    FindByID(ctx context.Context, id string) (*CAPECEntry, error)
}
```

### 2.3 Entities trong search-service

**File mới**: `search-service/internal/domain/entity/taxonomy.go`

```go
type CWEEntry struct {
    ID          string   `json:"id"`           // "CWE-89"
    Name        string   `json:"name"`
    Description string   `json:"description"`
    Abstraction string   `json:"abstraction"`  // "Base" | "Class"
    CAPECIDs    []string `json:"capec_ids,omitempty"`
}

type CAPECEntry struct {
    ID          string   `json:"id"`           // "CAPEC-66"
    Name        string   `json:"name"`
    Description string   `json:"description"`
    Likelihood  string   `json:"likelihood"`
    Severity    string   `json:"severity"`
    CWEIDs      []string `json:"cwe_ids,omitempty"`
}
```

---

## 3. apps/osv Changes

> **Không thay đổi business logic.**

Gateway routing update:
```go
// gateway-service/internal/proxy/ovs_routes.go
// CWE/CAPEC routes cached 1h (public, rarely changing):
{PathPrefix: "/api/v2/cwe",   Upstream: "search-service", SkipAuth: true},
{PathPrefix: "/api/v2/capec", Upstream: "search-service", SkipAuth: true},
```

---

## 4. Files cần tạo/sửa

### data-service (VERIFY/FIX)
```
internal/fetcher/mitre_cwe.go      ← Verify complete
internal/fetcher/mitre_capec.go    ← Verify complete
migrations/XXXX_cwe_capec.sql      ← Schema if missing
```

### search-service (NEW/MODIFY)
```
internal/delivery/http/taxonomy_handler.go  ← NEW: CWE/CAPEC handlers
internal/domain/entity/taxonomy.go          ← NEW: CWE/CAPEC entities
internal/domain/repository/taxonomy_repo.go ← NEW: CWE/CAPEC repository interfaces
internal/infra/postgres/taxonomy_pg.go      ← NEW: PostgreSQL implementations
internal/usecase/cvesearch/request.go       ← Add CWE filter field
internal/usecase/cvesearch/usecase.go       ← Apply CWE filter
internal/delivery/http/search_handler.go    ← Register new routes
```

### gateway-service (MODIFY)
```
internal/proxy/ovs_routes.go    ← Add /api/v2/cwe, /api/v2/capec routes
```

---

## 5. API Spec

```
GET /api/v2/cwe                     → List CWE weaknesses (paginated)
GET /api/v2/cwe?q=injection         → Search by name
GET /api/v2/cwe/CWE-89              → Get SQL Injection CWE detail
GET /api/v2/capec                   → List CAPEC patterns
GET /api/v2/capec?cwe_id=CWE-89    → Filter CAPEC by linked CWE
GET /api/v2/capec/CAPEC-66          → Get SQL Injection CAPEC detail
GET /api/v2/cves?cwe=CWE-89         → CVEs with this CWE weakness
```

---

## 6. Acceptance Criteria

- [x] `GET /api/v2/cwe` → return ≥ 900 CWE entries
- [x] `GET /api/v2/cwe/CWE-89` → return SQL Injection detail with linked CAPEC IDs
- [x] `GET /api/v2/capec` → return ≥ 500 CAPEC patterns
- [x] `GET /api/v2/cves?cwe=CWE-89` → CVEs có `cwe` array chứa `CWE-89`
- [x] Weekly sync (Sunday 5am) update CWE/CAPEC data
- [x] CWE ↔ CAPEC cross-references đúng (bidirectional links)


## Implementation Status

**✅ IMPLEMENTED — 2026-06-17** | Toàn bộ giải pháp đã được triển khai đầy đủ và build verified.
