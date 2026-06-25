# TASK-GCV-014 — KEV Entity Extension + Migration

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-GCV-014 |
| **Service** | `data-service` |
| **CR** | CR-GCV-007 |
| **Phase** | 2 — Enrichment |
| **Priority** | 🟡 Medium |
| **Prerequisites** | — |

## Context

Mở rộng KEV entity trong `data-service` để thêm `ShortDescription`, `RequiredAction`, `KnownRansomwareCampaignUse` từ CISA KEV v3 API. Tạo migration thêm columns tương ứng và thêm computed column `is_known_ransomware`.

## Reference

- Solution: [SOL-GCV-007](../solutions/SOL-GCV-007-kev-enhancement.md) §2.1
- CR: [CR-GCV-007](../CR-GCV-007-kev-service-enhancement.md)

## Files to Create/Modify

```
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/data-service/internal/domain/kev/
        (đọc cấu trúc, tìm entity file)
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/data-service/migrations/XXXX_kev_ransomware.sql
```

**Đọc trước**: `data-service/internal/domain/kev/` để xác định tên entity struct và file.

## Implementation Spec

### entity extension — ADD fields

Tìm struct KEVEntry (hoặc tương đương) trong `data-service/internal/domain/kev/`, thêm:

```go
type KEVEntry struct {
    // ─── EXISTING fields (giữ nguyên) ─────────────────────────────────────
    CVEID              string    // e.g. "CVE-2021-44228"
    VendorProject      string    // e.g. "Apache"
    Product            string    // e.g. "Log4j"
    VulnerabilityName  string
    DateAdded          time.Time
    DueDate            time.Time
    // (other existing fields)

    // ─── NEW fields from CISA KEV v3 ──────────────────────────────────────
    ShortDescription   string    `db:"short_description"   json:"short_description,omitempty"`
    RequiredAction     string    `db:"required_action"     json:"required_action,omitempty"`
    // CISA uses: "Known" | "Unknown"
    KnownRansomwareCampaignUse string `db:"known_ransomware" json:"known_ransomware_campaign_use,omitempty"`

    // ─── Computed (from DB generated column) ──────────────────────────────
    IsKnownRansomware  bool      `db:"is_known_ransomware" json:"is_known_ransomware"`
}

// IsRansomware returns true if CISA classifies this CVE as known ransomware.
func (e *KEVEntry) IsRansomware() bool {
    return strings.EqualFold(e.KnownRansomwareCampaignUse, "Known")
}
```

### Migration SQL

```sql
-- Migration: add CISA KEV v3 fields (short_description, required_action, known_ransomware)
-- Up

ALTER TABLE kev_entries ADD COLUMN IF NOT EXISTS short_description TEXT DEFAULT NULL;
ALTER TABLE kev_entries ADD COLUMN IF NOT EXISTS required_action TEXT DEFAULT NULL;
ALTER TABLE kev_entries ADD COLUMN IF NOT EXISTS known_ransomware TEXT NOT NULL DEFAULT 'Unknown'
    CHECK (known_ransomware IN ('Known', 'Unknown'));

-- Generated computed column (PostgreSQL 12+)
ALTER TABLE kev_entries ADD COLUMN IF NOT EXISTS is_known_ransomware BOOLEAN
    GENERATED ALWAYS AS (known_ransomware = 'Known') STORED;

-- Index for ransomware filter
CREATE INDEX IF NOT EXISTS idx_kev_ransomware
    ON kev_entries(is_known_ransomware)
    WHERE is_known_ransomware = TRUE;

-- Down (rollback)
-- DROP INDEX IF EXISTS idx_kev_ransomware;
-- ALTER TABLE kev_entries DROP COLUMN IF EXISTS is_known_ransomware;
-- ALTER TABLE kev_entries DROP COLUMN IF EXISTS known_ransomware;
-- ALTER TABLE kev_entries DROP COLUMN IF EXISTS required_action;
-- ALTER TABLE kev_entries DROP COLUMN IF EXISTS short_description;
```

### CISA KEV API Field Mapping

CISA KEV JSON field → KEVEntry field:

```
"cveID"                      → CVEID
"vendorProject"              → VendorProject
"product"                    → Product
"vulnerabilityName"          → VulnerabilityName
"dateAdded"                  → DateAdded
"shortDescription"           → ShortDescription (NEW)
"requiredAction"             → RequiredAction (NEW)
"dueDate"                    → DueDate
"knownRansomwareCampaignUse" → KnownRansomwareCampaignUse (NEW)
```

## Acceptance Criteria

- [x] `KEVEntry` struct có fields: `ShortDescription`, `RequiredAction`, `KnownRansomwareCampaignUse`, `IsKnownRansomware`
- [x] `db:"is_known_ransomware"` tag đúng → DB computed column được đọc
- [x] `IsRansomware()` method trả `true` khi `KnownRansomwareCampaignUse == "Known"` (case-insensitive)
- [x] Migration thêm columns với DEFAULT an toàn (existing rows không bị NULL)
- [x] `CHECK (known_ransomware IN ('Known', 'Unknown'))` constraint tồn tại
- [x] Migration idempotent (`IF NOT EXISTS`)
- [x] Existing KEV data không bị mất sau migration
- [x] `go build ./...` pass không lỗi
