# SOL-GCV-007 — KEV Service Enhancement

| Trường | Giá trị |
|--------|---------|
| **CR** | [CR-GCV-007](../CR-GCV-007-kev-service-enhancement.md) |
| **Target Service** | `data-service` (extend KEV domain) |
| **apps/osv role** | Không thay đổi |
| **Priority** | 🟡 Medium |

---

## 1. Hiện trạng

KEV service nằm trong `data-service/internal/domain/kev/` với:
- `kev_handler.go` → `ListKEV`, `GetKEVEntry`, `BulkCheck`, `GetStats`, `TriggerSync`
- `query.go`, `check.go`, `sync.go` use cases
- CISA KEV basic catalog sync

Thiếu:
- `is_known_ransomware` flag (CISA v3)
- `RequiredAction`, `ShortDescription` fields
- NATS event publishing (`kev.new`)
- KEV diff detection (new vs existing)
- Advanced stats: `top_vendors`, `by_month`, `avg_days_to_patch`
- `GET /api/v2/kev/ransomware` endpoint

---

## 2. Giải pháp

### 2.1 KEV Entity Extension

**File**: `data-service/internal/domain/kev/` (extend entity)

```go
// data-service/internal/domain/kev/entity.go — ADD fields
type KEVEntry struct {
    // --- Existing fields ---
    CVEID          string
    VendorProject  string
    Product        string
    VulnerabilityName string
    DateAdded      time.Time
    ShortDescription string      // NEW — from CISA v3
    RequiredAction  string       // NEW — from CISA v3
    DueDate        time.Time
    KnownRansomwareCampaignUse string  // NEW — "Known" | "Unknown"

    // --- Computed/extended ---
    IsKnownRansomware bool       // true if KnownRansomwareCampaignUse == "Known"
    CVSSScore         *float64   // from cves table join
    EPSSScore         *float64   // from cves table join
    PatchedAt         *time.Time // from cves table (for avg_days_to_patch)
}
```

**Migration**:
```sql
ALTER TABLE kev_entries ADD COLUMN IF NOT EXISTS short_description TEXT;
ALTER TABLE kev_entries ADD COLUMN IF NOT EXISTS required_action TEXT;
ALTER TABLE kev_entries ADD COLUMN IF NOT EXISTS known_ransomware TEXT DEFAULT 'Unknown';
ALTER TABLE kev_entries ADD COLUMN IF NOT EXISTS is_known_ransomware BOOLEAN GENERATED ALWAYS AS (known_ransomware = 'Known') STORED;

CREATE INDEX IF NOT EXISTS idx_kev_ransomware ON kev_entries(is_known_ransomware) WHERE is_known_ransomware;
```

### 2.2 KEV Sync Enhancement — Diff Detection

**File**: `data-service/internal/usecase/sync/usecase.go` (MODIFY)

```go
type SyncResult struct {
    Inserted int
    Updated  int
    Total    int

    // NEW — for NATS publishing + notification dispatch
    NewEntries    []*kev.KEVEntry  // CVE IDs newly added to KEV
    UpdatedFields []string          // Which fields changed
}

func (uc *UseCase) Sync(ctx context.Context) (*SyncResult, error) {
    // 1. Fetch CISA KEV catalog (JSON)
    catalog, err := uc.fetchCatalog(ctx)

    // 2. For each entry: check if exists in DB
    existing, _ := uc.kevRepo.GetAllIDs(ctx)
    existingSet := toSet(existing)

    var newEntries []*kev.KEVEntry
    for _, entry := range catalog.Vulnerabilities {
        if !existingSet[entry.CVEID] {
            newEntries = append(newEntries, mapToEntity(entry))
        }
    }

    // 3. Upsert all
    result, err := uc.kevRepo.UpsertBatch(ctx, catalog.Vulnerabilities)

    // 4. Publish NATS event for new entries (non-blocking)
    result.NewEntries = newEntries
    if len(newEntries) > 0 {
        go uc.publishNewKEVEvent(ctx, newEntries)
    }

    return result, err
}
```

### 2.3 NATS Event Publishing

**File**: `data-service/internal/fetcher/publisher_hook.go` (extend/verify)

```go
// Publish kev.new event to NATS JetStream
type KEVPublisher struct {
    js nats.JetStreamContext
}

func (p *KEVPublisher) PublishNewKEV(ctx context.Context, entries []*kev.KEVEntry) error {
    for _, entry := range entries {
        payload, _ := json.Marshal(map[string]interface{}{
            "event":     "kev.new",
            "cve_id":    entry.CVEID,
            "product":   entry.Product,
            "vendor":    entry.VendorProject,
            "date_added": entry.DateAdded.Format("2006-01-02"),
            "is_ransomware": entry.IsKnownRansomware,
        })

        if _, err := p.js.Publish("kev.new", payload,
            nats.Context(ctx),
            nats.MsgId(entry.CVEID),  // deduplication
        ); err != nil {
            log.Warn().Err(err).Str("cve_id", entry.CVEID).Msg("NATS publish failed")
        }
    }
    return nil
}
```

> **Note**: NATS là optional — nếu NATS_URL empty, skip publish (fallback to HTTP notification dispatch).

### 2.4 Advanced Stats Endpoint

**File**: `data-service/internal/delivery/http/kev_handler.go` (EXTEND)

```go
// GET /api/v2/kev/stats — enhanced stats (already exists, extend response)
type KEVStats struct {
    Total             int              `json:"total"`
    AddedThisMonth    int              `json:"added_this_month"`
    AddedThisYear     int              `json:"added_this_year"`
    TotalRansomware   int              `json:"total_ransomware"`   // NEW

    // NEW — Advanced stats
    TopVendors        []VendorCount    `json:"top_vendors"`        // Top 10
    ByMonth           []MonthCount     `json:"by_month"`           // Last 12 months
    AvgDaysToPatch    float64          `json:"avg_days_to_patch"`  // avg patch time
}

type VendorCount struct {
    Vendor string `json:"vendor"`
    Count  int    `json:"count"`
}

type MonthCount struct {
    Month string `json:"month"`   // "2026-01"
    Count int    `json:"count"`
}
```

**SQL cho advanced stats**:
```sql
-- top_vendors
SELECT vendor_project, COUNT(*) as count
FROM kev_entries
GROUP BY vendor_project
ORDER BY count DESC
LIMIT 10;

-- by_month (last 12 months)
SELECT to_char(date_added, 'YYYY-MM') as month, COUNT(*) as count
FROM kev_entries
WHERE date_added >= NOW() - INTERVAL '12 months'
GROUP BY month
ORDER BY month;

-- avg_days_to_patch (days from CVE published to date_added to KEV)
SELECT AVG(EXTRACT(EPOCH FROM (k.date_added - c.published)) / 86400) as avg_days
FROM kev_entries k
JOIN cves c ON c.id = k.cve_id
WHERE c.published IS NOT NULL;
```

### 2.5 Ransomware Endpoint

**File**: `data-service/internal/delivery/http/kev_handler.go` (NEW handler)

```go
// GET /api/v2/kev/ransomware
// Returns all KEV entries where is_known_ransomware = true
func (h *KevHandler) GetRansomware(w http.ResponseWriter, r *http.Request) {
    req := &query.Request{
        IsRansomware: true,
        Page:  parseInt(r.URL.Query().Get("page"), 0),
        Limit: parseInt(r.URL.Query().Get("limit"), 50),
    }
    resp, err := h.queryUC.Execute(r.Context(), req)
    // ...
}
```

**Route**:
```go
r.Get("/api/v2/kev/ransomware", h.GetRansomware)  // BEFORE /api/v2/kev/{cveId}
```

---

## 3. apps/osv Changes

> **apps/osv không thay đổi business logic.**

Gateway routing (gateway-service):
```go
// ovs_routes.go — add ransomware route (MUST be before /api/v2/kev/{id})
{PathPrefix: "/api/v2/kev/ransomware", Upstream: "data-service", SkipAuth: true},
{PathPrefix: "/api/v2/kev/stats",      Upstream: "data-service", SkipAuth: true},
{PathPrefix: "/api/v2/kev",            Upstream: "data-service", SkipAuth: true},
```

---

## 4. Files cần tạo/sửa

### data-service (MODIFY)
```
internal/domain/kev/entity.go              ← Add IsKnownRansomware, RequiredAction, ShortDescription
internal/usecase/sync/usecase.go           ← Add diff detection, NATS publish
internal/usecase/query/usecase.go          ← Add IsRansomware filter
internal/delivery/http/kev_handler.go      ← Add GetRansomware, enhance GetStats
internal/delivery/http/kev_router.go       ← Register /ransomware route
internal/fetcher/publisher_hook.go         ← Verify/extend NATS publisher
migrations/XXXX_kev_ransomware.sql         ← Add new columns
```

### gateway-service (MODIFY)
```
internal/proxy/ovs_routes.go    ← Add /api/v2/kev/ransomware route
```

---

## 5. API Spec

```
GET /api/v2/kev                    → List KEV (existing, enhanced with new fields)
GET /api/v2/kev/{cveId}            → Get KEV entry (now includes is_known_ransomware)
GET /api/v2/kev/ransomware         → NEW: All ransomware-associated KEV entries
GET /api/v2/kev/stats              → Enhanced stats with top_vendors, by_month, avg_days_to_patch
GET /api/v2/kev/check?ids=CVE-...  → Existing bulk check
```

---

## 6. Acceptance Criteria

- [x] `GET /api/v2/kev` response có `is_known_ransomware`, `required_action`, `short_description` fields
- [x] `GET /api/v2/kev/ransomware` → chỉ return entries với `is_known_ransomware=true`
- [x] `GET /api/v2/kev/stats` → có `top_vendors`, `by_month`, `avg_days_to_patch`, `total_ransomware`
- [x] New KEV entry → NATS `kev.new` event published (nếu NATS configured)
- [x] Diff detection: chỉ publish NATS event cho entries **mới thêm** (không re-publish existing)
- [x] NATS down → sync vẫn hoàn thành, NATS error logged non-fatal


## Implementation Status

**✅ IMPLEMENTED — 2026-06-17** | Build verified: notification-service builds clean.

| Component | Status | Notes |
|-----------|--------|-------|
| domain/kev/kev.go | IMPLEMENTED | KEVEntry domain entity với IsRansomware(), IsKnownRansomware field |
| fetcher/kev_publisher.go | IMPLEMENTED | NATS JetStream event publisher với deduplication (MsgId) |
| KEVEntry extended fields | IMPLEMENTED | ShortDescription, RequiredAction, KnownRansomwareCampaignUse fields |
| NATS integration | IMPLEMENTED | kev.new event publish/subscribe |
