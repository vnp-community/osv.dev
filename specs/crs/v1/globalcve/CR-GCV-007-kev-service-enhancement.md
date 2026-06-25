# CR-GCV-007 — KEV Service Enhancement (KnownRansomware, Stats, NATS)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-GCV-007 |
| **Tiêu đề** | KEV Service Enhancement — KnownRansomware Tracking, Advanced Stats, NATS Event Publishing |
| **Nguồn tham chiếu** | `globalcve/specs/services/04-kev-service.md`, `globalcve/specs/services/00-overview.md §5.3` |
| **Target Service** | `kev-service` (extend) |
| **Ưu tiên** | 🟡 Medium |
| **Loại** | Feature Enhancement |
| **Ngày tạo** | 2026-06-14 |
| **Trạng thái** | ✅ IMPLEMENTED — 2026-06-17 |

---

## 1. Tổng quan

OSV hiện tại có `kev-service` cơ bản. GlobalCVE v3.0 định nghĩa các tính năng nâng cao:

1. **KnownRansomware** — CISA v3 catalog thêm field `knownRansomwareCampaignUse` để đánh dấu CVEs được dùng trong ransomware campaigns
2. **Advanced Stats** — detailed statistics với breakdown theo vendor, thống kê 7d/30d/1y
3. **NATS Event Publishing** — khi có KEV mới, publish event `kev.new` để notification-service bắt
4. **KEV Enrichment API** — internal API cho cve-search-service query KEV status batch

---

## 2. Gap Analysis

| Feature | OSV KEV Service | GlobalCVE |
|---------|----------------|-----------|
| Basic KEV sync | ✅ | ✅ |
| KnownRansomware flag | ❌ | ✅ v3 catalog |
| Ransomware campaign name | ❌ | ✅ |
| Stats by vendor/product | ❌ | ✅ |
| Stats last 1 year | ❌ | ✅ |
| NATS event publishing | ❌ | ✅ `kev.new` |
| Bulk check Redis cache | ❌ | ✅ |
| KEV trend analysis | ❌ | ✅ |
| RequiredAction field | ⚠️ Partial | ✅ |
| ShortDescription field | ❌ | ✅ |

---

## 3. Domain Model Extensions

### 3.1 KEVEntry Entity Enhancement

```go
// kev-service/internal/domain/entity/kev.go
// Extended từ GlobalCVE v3.0 spec

// KEVEntry — extended to CISA KEV catalog v3.0
type KEVEntry struct {
    CVEID             string    `json:"cveID"`
    VendorProject     string    `json:"vendorProject"`
    Product           string    `json:"product"`
    VulnerabilityName string    `json:"vulnerabilityName"`
    ShortDescription  string    `json:"shortDescription"`   // NEW — short desc
    RequiredAction    string    `json:"requiredAction"`     // Action CISA requires
    DateAdded         time.Time `json:"dateAdded"`
    DueDate           time.Time `json:"dueDate"`
    Notes             string    `json:"notes"`

    // NEW — CISA KEV catalog v3 fields
    KnownRansomwareCampaignUse bool   `json:"knownRansomwareCampaignUse"` // In ransomware campaign
    RansomwareCampaignName     string `json:"ransomwareCampaignName"`     // e.g., "LockBit", "Conti"

    CreatedAt time.Time
    UpdatedAt time.Time
}

// KEVStats — extended stats
type KEVStats struct {
    Total           int64              `json:"total"`
    AddedLast7d     int64              `json:"added_last_7_days"`
    AddedLast30d    int64              `json:"added_last_30_days"`
    AddedLast365d   int64              `json:"added_last_365_days"`  // NEW — 1 year
    LastSyncAt      time.Time          `json:"last_sync_at"`

    // NEW
    RansomwareCount  int64             `json:"ransomware_count"`     // Known ransomware CVEs
    TopVendors       []VendorStat      `json:"top_vendors"`          // Top 10 vendors
    ByMonth          []MonthStat       `json:"by_month"`             // Last 12 months

    // Durations
    AvgDaysToPatch   float64           `json:"avg_days_to_patch"`    // Avg due_date - date_added
}

type VendorStat struct {
    Vendor string `json:"vendor"`
    Count  int64  `json:"count"`
}

type MonthStat struct {
    Month string `json:"month"`   // "2026-06"
    Count int64  `json:"count"`
}

// KEVFilter — extended
type KEVFilter struct {
    Query              string
    VendorProject      string
    KnownRansomware    *bool  // NEW — filter ransomware-linked CVEs
    Since              *time.Time
    Page               int
    Limit              int
}
```

---

## 4. CISA KEV Client Update

### 4.1 Extended CISA JSON Parsing

```go
// kev-service/internal/adapter/external/cisa/client.go

// CISA KEV catalog v3 JSON response
type cisaResponse struct {
    Title          string `json:"title"`
    CatalogVersion string `json:"catalogVersion"`
    DateReleased   string `json:"dateReleased"`
    Count          int    `json:"count"`
    Vulnerabilities []struct {
        CveID             string `json:"cveID"`
        VendorProject     string `json:"vendorProject"`
        Product           string `json:"product"`
        VulnerabilityName string `json:"vulnerabilityName"`
        DateAdded         string `json:"dateAdded"`    // "2021-11-03"
        ShortDescription  string `json:"shortDescription"`
        RequiredAction    string `json:"requiredAction"`
        DueDate           string `json:"dueDate"`      // "2021-11-17"
        Notes             string `json:"notes"`

        // CISA v3 — new fields
        KnownRansomwareCampaignUse string `json:"knownRansomwareCampaignUse"` // "Known" | "Unknown"
    } `json:"vulnerabilities"`
}

func (c *Client) FetchKEVCatalog(ctx context.Context) ([]*entity.KEVEntry, error) {
    req, _ := http.NewRequestWithContext(ctx, "GET", cisaKEVURL, nil)
    resp, err := c.httpClient.Do(req)
    if err != nil { return nil, err }
    defer resp.Body.Close()

    var cisaData cisaResponse
    json.NewDecoder(resp.Body).Decode(&cisaData)

    entries := make([]*entity.KEVEntry, 0, len(cisaData.Vulnerabilities))
    for _, v := range cisaData.Vulnerabilities {
        dateAdded, _ := time.Parse("2006-01-02", v.DateAdded)
        dueDate, _ := time.Parse("2006-01-02", v.DueDate)

        entry := &entity.KEVEntry{
            CVEID:             v.CveID,
            VendorProject:     v.VendorProject,
            Product:           v.Product,
            VulnerabilityName: v.VulnerabilityName,
            ShortDescription:  v.ShortDescription,
            RequiredAction:    v.RequiredAction,
            DateAdded:         dateAdded,
            DueDate:           dueDate,
            Notes:             v.Notes,

            // Map "Known" → true
            KnownRansomwareCampaignUse: v.KnownRansomwareCampaignUse == "Known",
        }

        entries = append(entries, entry)
    }

    return entries, nil
}
```

---

## 5. NATS Event Publishing

### 5.1 Event Publisher

```go
// kev-service/internal/adapter/event/nats_publisher.go

type NATSPublisher struct {
    js   nats.JetStreamContext
    logger zerolog.Logger
}

// Published events:
// kev.new     — When new CVEs are added to KEV catalog
// kev.updated — When existing KEV entries are updated

type KEVNewEvent struct {
    CVEID        string    `json:"cve_id"`
    VendorProject string   `json:"vendor_project"`
    Product      string    `json:"product"`
    DateAdded    time.Time `json:"date_added"`
    DueDate      time.Time `json:"due_date"`
    IsRansomware bool      `json:"is_ransomware"`
    SyncedAt     time.Time `json:"synced_at"`
}

// PublishNewKEVs — publish event for each newly added KEV entry
func (p *NATSPublisher) PublishNewKEVs(ctx context.Context, newEntries []*entity.KEVEntry) error {
    for _, entry := range newEntries {
        event := &KEVNewEvent{
            CVEID:         entry.CVEID,
            VendorProject: entry.VendorProject,
            Product:       entry.Product,
            DateAdded:     entry.DateAdded,
            DueDate:       entry.DueDate,
            IsRansomware:  entry.KnownRansomwareCampaignUse,
            SyncedAt:      time.Now(),
        }

        payload, _ := json.Marshal(event)

        _, err := p.js.Publish("kev.new", payload,
            nats.Context(ctx),
            nats.MsgId(entry.CVEID),          // Deduplication
            nats.ExpectStream("KEV_EVENTS"),   // JetStream stream
        )
        if err != nil {
            p.logger.Error().Err(err).Str("cve", entry.CVEID).Msg("NATS publish kev.new failed")
        }
    }
    return nil
}
```

### 5.2 KEV Sync with Diff Detection

```go
// kev-service/internal/usecase/sync/usecase.go

func (uc *SyncUseCase) Sync(ctx context.Context) (*SyncResult, error) {
    start := time.Now()

    // 1. Fetch current KEV catalog from CISA
    newEntries, err := uc.cisaClient.FetchKEVCatalog(ctx)
    if err != nil { return nil, err }

    // 2. Get existing KEV IDs from DB
    existingIDs, _ := uc.kevRepo.GetAllIDs(ctx)
    existingSet := make(map[string]bool)
    for _, id := range existingIDs { existingSet[id] = true }

    // 3. Detect new entries
    var justAdded []*entity.KEVEntry
    for _, entry := range newEntries {
        if !existingSet[entry.CVEID] {
            justAdded = append(justAdded, entry)
        }
    }

    // 4. Upsert all entries
    inserted, updated, err := uc.kevRepo.UpsertBatch(ctx, newEntries)
    if err != nil { return nil, err }

    // 5. Publish NATS events for new KEVs
    if len(justAdded) > 0 && uc.eventPublisher != nil {
        uc.eventPublisher.PublishNewKEVs(ctx, justAdded)
    }

    // 6. Update cve-search-service is_kev flags
    if len(justAdded) > 0 {
        ids := make([]string, len(justAdded))
        for i, e := range justAdded { ids[i] = e.CVEID }
        uc.cveWriteRepo.MarkKEV(ctx, ids, true)
    }

    return &SyncResult{
        Total:    len(newEntries),
        Inserted: inserted,
        Updated:  updated,
        NewKEVs:  len(justAdded),
        Duration: time.Since(start),
        SyncedAt: time.Now(),
    }, nil
}
```

---

## 6. Advanced Stats Use Case

```go
// kev-service/internal/usecase/stats/usecase.go

func (uc *StatsUseCase) GetStats(ctx context.Context) (*entity.KEVStats, error) {
    stats, err := uc.kevRepo.Stats(ctx)
    if err != nil { return nil, err }

    // Enrich with additional stats
    topVendors, _ := uc.kevRepo.TopVendors(ctx, 10)
    stats.TopVendors = topVendors

    byMonth, _ := uc.kevRepo.ByMonth(ctx, 12)  // Last 12 months
    stats.ByMonth = byMonth

    // Calculate average days to patch
    avgDays, _ := uc.kevRepo.AvgDaysToPatch(ctx)
    stats.AvgDaysToPatch = avgDays

    return stats, nil
}
```

### PostgreSQL Stats Queries

```sql
-- Top 10 vendors in KEV
SELECT vendor_project AS vendor, COUNT(*) AS count
FROM kev_entries
GROUP BY vendor_project
ORDER BY count DESC
LIMIT 10;

-- Monthly additions (last 12 months)
SELECT
    TO_CHAR(date_added, 'YYYY-MM') AS month,
    COUNT(*) AS count
FROM kev_entries
WHERE date_added >= NOW() - INTERVAL '12 months'
GROUP BY month
ORDER BY month ASC;

-- Average days to patch
SELECT AVG(due_date - date_added) AS avg_days
FROM kev_entries
WHERE due_date IS NOT NULL AND date_added IS NOT NULL;

-- Ransomware count
SELECT COUNT(*) FROM kev_entries WHERE known_ransomware_campaign_use = TRUE;
```

---

## 7. API Enhancements

```
# Enhanced endpoints
GET /api/v2/kev                        → List (add: kev=ransomware filter)
GET /api/v2/kev/:cveId                 → Get (add: ransomware info)
GET /api/v2/kev/check?ids=...          → Bulk check (unchanged)
GET /api/v2/kev/stats                  → Extended stats (add: vendors, months)
GET /api/v2/kev/ransomware             → NEW: Only ransomware-linked KEV entries
GET /api/v2/kev/ransomware/stats       → NEW: Ransomware stats

# Internal
POST /internal/kev/sync                → Trigger sync
GET  /internal/kev/ids                 → All KEV IDs (for sync service)
```

### Enhanced Response Examples

```json
// GET /api/v2/kev?vendor=apache&ransomware=true
{
  "entries": [
    {
      "cveID": "CVE-2021-44228",
      "vendorProject": "Apache",
      "product": "Log4j",
      "vulnerabilityName": "Apache Log4j2 Remote Code Execution",
      "shortDescription": "Apache Log4j2 contains a remote code execution vulnerability...",
      "requiredAction": "Apply updates per vendor instructions...",
      "dateAdded": "2021-12-10",
      "dueDate": "2021-12-24",
      "knownRansomwareCampaignUse": true
    }
  ]
}

// GET /api/v2/kev/stats
{
  "total": 1245,
  "added_last_7_days": 3,
  "added_last_30_days": 18,
  "added_last_365_days": 234,
  "ransomware_count": 156,
  "last_sync_at": "2026-06-14T02:00:00Z",
  "avg_days_to_patch": 14.2,
  "top_vendors": [
    {"vendor": "Microsoft", "count": 245},
    {"vendor": "Apache", "count": 89},
    {"vendor": "Cisco", "count": 67}
  ],
  "by_month": [
    {"month": "2026-01", "count": 12},
    {"month": "2026-02", "count": 23}
  ]
}
```

---

## 8. Database Schema Extensions

```sql
-- Add new CISA v3 columns to kev_entries
ALTER TABLE kev_entries
    ADD COLUMN IF NOT EXISTS short_description          TEXT DEFAULT '',
    ADD COLUMN IF NOT EXISTS required_action            TEXT DEFAULT '',
    ADD COLUMN IF NOT EXISTS known_ransomware_campaign_use BOOLEAN DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS ransomware_campaign_name   TEXT DEFAULT '';

-- Index for ransomware filter
CREATE INDEX IF NOT EXISTS idx_kev_ransomware ON kev_entries(known_ransomware_campaign_use)
    WHERE known_ransomware_campaign_use = TRUE;

-- Index for vendor stats
CREATE INDEX IF NOT EXISTS idx_kev_vendor_project ON kev_entries(vendor_project);

-- Index for monthly stats
CREATE INDEX IF NOT EXISTS idx_kev_date_year_month ON kev_entries(
    EXTRACT(YEAR FROM date_added), EXTRACT(MONTH FROM date_added)
);
```

---

## 9. NATS JetStream Setup

```go
// Required JetStream streams:
// Stream: KEV_EVENTS
//   Subjects: kev.*
//   Retention: 30 days
//   MaxMsgs: 100000
//   Storage: File

// Consumer: notification-service
//   FilterSubject: kev.new
//   DeliverPolicy: New (từ lúc consumer tạo)
//   AckPolicy: Explicit
```

---

## 10. Acceptance Criteria

- [x] `GET /api/v2/kev/stats` bao gồm `ransomware_count`, `top_vendors`, `by_month` (12 months)
- [x] KEV entry response có `knownRansomwareCampaignUse`, `shortDescription`, `requiredAction`
- [x] `GET /api/v2/kev?ransomware=true` → chỉ trả về ransomware-linked entries
- [x] `GET /api/v2/kev/ransomware` → dedicated ransomware endpoint
- [x] Khi KEV sync phát hiện entries mới → `kev.new` NATS event được publish
- [x] NATS event `kev.new` có đầy đủ: cve_id, vendor_project, product, is_ransomware
- [x] Notification-service nhận `kev.new` và trigger webhook alerts
- [x] `known_ransomware_campaign_use = TRUE` cho CVEs với "Known" trong CISA catalog
- [x] Stats cached trong Redis 5 phút
- [x] Top 10 vendors đúng theo count DESC
---

## Implementation Status

**✅ IMPLEMENTED — 2026-06-17** | Service: `data-service` | Build: `go build ./...` ✅

### Verified Components

| Component | File | Status |
|-----------|------|--------|
| KEV domain entity (KnownRansomwareCampaignUse, ShortDescription, RequiredAction) | `internal/domain/kev/kev.go` | ✅ DONE |
| KEV CISA sync (incremental + full) | `internal/infra/external/cisa/client.go` | ✅ DONE |
| KEV NATS JetStream publisher (`kev.new` events, deduplication via MsgId) | `internal/fetcher/kev_publisher.go` | ✅ DONE |
| KEV publisher: full event payload (cve_id, vendor_project, product, is_ransomware) | `internal/fetcher/kev_publisher.go` | ✅ DONE |
| NATS stream `KEV_EVENTS` (MaxMsgs=10000, subjects=kev.>) | `internal/fetcher/kev_publisher.go` | ✅ DONE |
| notification-service NATS subscriber handles `kev.new` | `notification-service/internal/nats/subscriber.go` | ✅ DONE |
| GET /api/v2/kev/stats (ransomware_count, top_vendors, by_month) | `internal/delivery/http/kev_handler.go` | ✅ DONE |
| GET /api/v2/kev/ransomware endpoint | `internal/delivery/http/kev_handler.go` | ✅ DONE |
| GET /api/v2/kev?ransomware=true filter | `internal/delivery/http/kev_handler.go` | ✅ DONE |
| Stats Redis cache 5 minutes | `internal/delivery/http/kev_handler.go` | ✅ DONE |
| known_ransomware_campaign_use = TRUE mapping | `internal/infra/external/cisa/client.go` | ✅ DONE |
| PostgreSQL kev_repo (Stats, FindRansomware, Top10Vendors) | `internal/infra/persistence/postgres/kev_repo.go` | ✅ DONE |
| Scheduler: KEV sync every 6h | `internal/delivery/scheduler/scheduler.go` | ✅ DONE |

### Acceptance Criteria: 10/10 ✅
