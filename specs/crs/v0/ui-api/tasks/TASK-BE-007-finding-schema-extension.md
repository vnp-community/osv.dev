# TASK-BE-007 — finding-service: Extended Finding Response DTO + Aggregations

| Field | Value |
|-------|-------|
| **Task ID** | TASK-BE-007 |
| **Service** | `services/finding-service` |
| **Solution Ref** | [SOL-UI-004 §1.1–1.2](../solutions/SOL-UI-004-finding-product-reports-admin.md) |
| **Priority** | 🔴 P0 |
| **Depends On** | — |
| **Estimated** | 4h |

---

## Context

Finding list response hiện thiếu nhiều fields mà UI cần:
- `epss_score`, `is_kev` — risk prioritization
- `sla_status`, `sla_days_left` — SLA badge color
- `product_name` — display context
- `jira_issue_key`, `jira_url` — integration status
- `by_severity{}`, `by_status{}`, `sla_stats{}` — aggregate counts trong response

---

## Goal

1. Thêm columns DB cho `epss_score`, `is_kev`, `assigned_to`, `asset_ip`, `asset_hostname` vào `findings` table
2. Mở rộng `FindingListItem` DTO
3. Thêm aggregation counts vào `FindingListResponse`

---

## Target Files

| Action | File Path |
|--------|-----------|
| CREATE | `services/finding-service/db/migrations/004_finding_extensions.sql` |
| MODIFY | `services/finding-service/internal/adapter/http/dto.go` |
| MODIFY | `services/finding-service/internal/adapter/http/finding_handler.go` |
| MODIFY | `services/finding-service/internal/infra/postgres/finding_repo.go` |

---

## Implementation

### File 1: `services/finding-service/db/migrations/004_finding_extensions.sql`

```sql
-- +migrate Up
ALTER TABLE findings ADD COLUMN IF NOT EXISTS epss_score    NUMERIC(6,5);
ALTER TABLE findings ADD COLUMN IF NOT EXISTS is_kev        BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE findings ADD COLUMN IF NOT EXISTS assigned_to   VARCHAR(255);
ALTER TABLE findings ADD COLUMN IF NOT EXISTS asset_ip      INET;
ALTER TABLE findings ADD COLUMN IF NOT EXISTS asset_hostname VARCHAR(255);

CREATE INDEX IF NOT EXISTS idx_findings_epss      ON findings(epss_score DESC) WHERE epss_score IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_findings_is_kev    ON findings(is_kev) WHERE is_kev = true;
CREATE INDEX IF NOT EXISTS idx_findings_assigned  ON findings(assigned_to) WHERE assigned_to IS NOT NULL;

-- +migrate Down
ALTER TABLE findings
    DROP COLUMN IF EXISTS epss_score,
    DROP COLUMN IF EXISTS is_kev,
    DROP COLUMN IF EXISTS assigned_to,
    DROP COLUMN IF EXISTS asset_ip,
    DROP COLUMN IF EXISTS asset_hostname;
```

### File 2: Extended `FindingListItem` DTO (update existing dto.go)

```go
// services/finding-service/internal/adapter/http/dto.go

type FindingListItem struct {
    ID          string   `json:"id"`
    Title       string   `json:"title"`
    Description string   `json:"description"`
    CveID       *string  `json:"cve_id"`
    Severity    string   `json:"severity"`
    CVSSv3      *float64 `json:"cvss_v3"`
    EPSSScore   *float64 `json:"epss_score"`     // NEW
    IsKEV       bool     `json:"is_kev"`          // NEW

    // Status (derived from bool flags)
    Status      string   `json:"status"` // "Active"|"Mitigated"|"FalsePositive"|"RiskAccepted"|"OutOfScope"

    // Flags
    IsDuplicate  bool    `json:"is_duplicate"`
    DupOfID      *string `json:"duplicate_finding_id"`

    // Hierarchy
    ProductID   string   `json:"product_id"`
    ProductName string   `json:"product_name"`   // NEW via JOIN
    EngagementID string  `json:"engagement_id"`
    TestID      string   `json:"test_id"`

    // Asset
    AssetIP       *string `json:"asset_ip"`
    AssetHostname *string `json:"asset_hostname"`
    ComponentName *string `json:"component_name"`
    ComponentVersion *string `json:"component_version"`

    // SLA
    SLAExpiry   *string `json:"sla_expiration_date"`
    SLAStatus   string  `json:"sla_status"`    // NEW: "ok"|"at_risk"|"breached"
    SLADaysLeft *int    `json:"sla_days_left"` // NEW: computed

    // Meta
    CreatedAt   string  `json:"created_at"`
    UpdatedAt   string  `json:"updated_at"`
    MitigatedAt *string `json:"mitigated_at"`
    AssignedTo  *string `json:"assigned_to"`   // NEW

    // JIRA integration
    JiraIssueKey *string `json:"jira_issue_key"` // NEW via LEFT JOIN
    JiraURL      *string `json:"jira_url"`        // NEW constructed
}

// FindingListResponse is the paginated findings response
type FindingListResponse struct {
    Findings   []FindingListItem  `json:"findings"`
    Total      int                `json:"total"`
    Page       int                `json:"page"`
    PageSize   int                `json:"page_size"`
    BySeverity map[string]int     `json:"by_severity"`  // NEW
    ByStatus   map[string]int     `json:"by_status"`    // NEW
    SLAStats   *SLASummary        `json:"sla_stats"`    // NEW
}

type SLASummary struct {
    Breached int `json:"breached"`
    AtRisk   int `json:"at_risk"`
    OK       int `json:"ok"`
}

// computeSLAStatus derives SLA status from expiration date
func computeSLAStatus(slaExpiry *time.Time) (status string, daysLeft *int) {
    if slaExpiry == nil {
        return "ok", nil
    }
    d := int(time.Until(*slaExpiry).Hours() / 24)
    switch {
    case d < 0:
        abs := -d
        return "breached", &abs
    case d <= 7:
        return "at_risk", &d
    default:
        return "ok", &d
    }
}

// deriveStatus maps boolean flags to status string
func deriveStatus(mitigated, fp, riskAccepted, outOfScope, duplicate bool) string {
    switch {
    case mitigated:    return "Mitigated"
    case fp:           return "FalsePositive"
    case riskAccepted: return "RiskAccepted"
    case outOfScope:   return "OutOfScope"
    default:           return "Active"
    }
}
```

### SQL update for FindingList query (update existing query in repo):

```sql
-- Updated finding list query with JOINs and aggregations
-- services/finding-service/internal/infra/postgres/finding_repo.go

WITH filtered AS (
    SELECT
        f.id, f.title, f.description, f.cve_id, f.severity,
        f.cvss_v3_score, f.epss_score, f.is_kev,
        f.is_mitigated, f.false_positive, f.risk_accepted,
        f.out_of_scope, f.is_duplicate, f.duplicate_finding_id,
        f.sla_expiration_date, f.created_at, f.updated_at,
        f.mitigated_at, f.assigned_to,
        f.component_name, f.component_version,
        f.asset_ip::text, f.asset_hostname,
        f.product_id, f.engagement_id, f.test_id,
        p.name AS product_name,
        ji.jira_key AS jira_issue_key,
        CASE WHEN ji.jira_key IS NOT NULL
             THEN jc.server_url || '/browse/' || ji.jira_key
        END AS jira_url
    FROM findings f
    JOIN products p ON p.id = f.product_id
    LEFT JOIN jira_issues ji ON ji.finding_id = f.id
    LEFT JOIN jira_configs jc ON jc.product_id = f.product_id
    WHERE
        ($1::uuid IS NULL OR f.product_id = $1)
        AND ($2::text IS NULL OR f.severity = $2)
        AND ($3::bool IS NULL OR f.active = $3)
),
agg AS (
    SELECT
        COUNT(*) AS total,
        COUNT(*) FILTER (WHERE severity = 'Critical') AS sev_critical,
        COUNT(*) FILTER (WHERE severity = 'High')     AS sev_high,
        COUNT(*) FILTER (WHERE severity = 'Medium')   AS sev_medium,
        COUNT(*) FILTER (WHERE severity = 'Low')      AS sev_low,
        COUNT(*) FILTER (WHERE NOT is_mitigated AND NOT false_positive AND NOT risk_accepted AND NOT out_of_scope) AS status_active,
        COUNT(*) FILTER (WHERE is_mitigated)           AS status_mitigated,
        COUNT(*) FILTER (WHERE false_positive)         AS status_fp,
        COUNT(*) FILTER (WHERE risk_accepted)          AS status_risk,
        COUNT(*) FILTER (WHERE sla_expiration_date < NOW() AND NOT is_mitigated) AS sla_breached,
        COUNT(*) FILTER (WHERE sla_expiration_date BETWEEN NOW() AND NOW() + INTERVAL '7 days' AND NOT is_mitigated) AS sla_at_risk
    FROM filtered
)
SELECT
    f.*,
    a.total, a.sev_critical, a.sev_high, a.sev_medium, a.sev_low,
    a.status_active, a.status_mitigated, a.status_fp, a.status_risk,
    a.sla_breached, a.sla_at_risk
FROM filtered f, agg a
ORDER BY
    CASE f.severity WHEN 'Critical' THEN 0 WHEN 'High' THEN 1 WHEN 'Medium' THEN 2 ELSE 3 END,
    f.sla_expiration_date ASC NULLS LAST
LIMIT $4 OFFSET ($5 - 1) * $4;
```

---

## Verification

```bash
cd services/finding-service

# Apply migration
go run cmd/migrate/main.go up

# Build
go build ./...

# Test response shape
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8085/v2/findings?product_id=xxx" | \
  jq '{has_epss: (.findings[0].epss_score != null), has_sla: (.findings[0].sla_status != null), has_agg: (.by_severity != null)}'
# Expected: {"has_epss":true,"has_sla":true,"has_agg":true}
```

---

## Checklist

- [x] Migration `004_finding_extensions.sql` thêm 5 columns + 3 indexes
- [x] `FindingListItem` DTO có đủ: `epss_score`, `is_kev`, `sla_status`, `sla_days_left`, `product_name`, `jira_issue_key`, `jira_url`, `assigned_to`
- [x] `FindingListResponse` có `by_severity`, `by_status`, `sla_stats`
- [x] `computeSLAStatus` trả về đúng "breached"/"at_risk"/"ok"
- [x] SQL dùng CTE để compute aggregations trong 1 query (không N+1)
- [x] `jira_url` được construct từ `server_url + '/browse/' + jira_key`
- [x] `go build ./...` thành công
