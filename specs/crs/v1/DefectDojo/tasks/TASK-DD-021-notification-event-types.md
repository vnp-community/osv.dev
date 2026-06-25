# ✅ COMPLETED — TASK-DD-021 — Notification Event Types Extension

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-DD-021 |
| **Service** | `notification-service` |
| **CR** | CR-DD-007 |
| **Phase** | 2 — Security Management |
| **Priority** | 🔴 High |
| **Prerequisites** | — (độc lập) |
| **Estimated effort** | 0.5 ngày |

## Context

Extend `notification-service` với 15+ event types mới từ DefectDojo. Thêm DB migration để mở rộng `notification_rules` table. Đây là precondition cho task Dispatch và Channels.

## Reference

- Solution: [`sol-notification-service.md § Event Types`](../solutions/sol-notification-service.md)

## Working Directory

```
/Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service/
```

## Files to Create/Modify

```
internal/domain/rule/event_types.go    # ADD DefectDojo event types
internal/domain/rule/entity.go         # ADD new channel fields
migrations/
└── 003_notification_defectdojo_events.sql
```

## Implementation Spec

### `internal/domain/rule/event_types.go` — ADD new constants

```go
// ADD to existing EventType constants:
const (
    // Scans
    EventScanAdded    EventType = "scan_added"

    // Findings
    EventFindingAdded            EventType = "finding_added"
    EventFindingStatusChanged    EventType = "finding_status_changed"

    // JIRA
    EventJIRAUpdate    EventType = "jira_update"

    // Engagements
    EventEngagementAdded   EventType = "engagement_added"
    EventEngagementClosed  EventType = "engagement_closed"

    // Risk Acceptance
    EventRiskAcceptanceExpiration EventType = "risk_acceptance_expiration"

    // SLA
    EventSLABreach       EventType = "sla_breach"
    EventSLAExpiringSoon EventType = "sla_expiring_soon"

    // Product
    EventProductAdded          EventType = "product_added"
    EventUserMentioned         EventType = "user_mentioned"
    EventClosedFindingRemoved  EventType = "closed_finding_removed"
    EventReviewRequested       EventType = "review_requested"
)
```

### `internal/domain/rule/entity.go` — ADD channel fields to NotificationRule

```go
// ADD these fields to existing NotificationRule struct:
ScanAdded                []Channel
TestAdded                []Channel
FindingAdded             []Channel
FindingStatusChanged     []Channel
JIRAUpdate               []Channel
EngagementAdded          []Channel
EngagementClosed         []Channel
RiskAcceptanceExpiration []Channel
SLABreach                []Channel
SLAExpiringSoon          []Channel
UserMentioned            []Channel
ProductAdded             []Channel
ClosedFindingRemoved     []Channel
ReviewRequested          []Channel
```

### `migrations/003_notification_defectdojo_events.sql`

```sql
-- Add DefectDojo event columns to notification_rules table
DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns
    WHERE table_name='notification_rules' AND column_name='scan_added') THEN
    ALTER TABLE notification_rules ADD COLUMN scan_added TEXT[] DEFAULT '{}'; END IF; END $$;

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns
    WHERE table_name='notification_rules' AND column_name='finding_added') THEN
    ALTER TABLE notification_rules ADD COLUMN finding_added TEXT[] DEFAULT '{}'; END IF; END $$;

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns
    WHERE table_name='notification_rules' AND column_name='finding_status_changed') THEN
    ALTER TABLE notification_rules ADD COLUMN finding_status_changed TEXT[] DEFAULT '{}'; END IF; END $$;

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns
    WHERE table_name='notification_rules' AND column_name='jira_update') THEN
    ALTER TABLE notification_rules ADD COLUMN jira_update TEXT[] DEFAULT '{}'; END IF; END $$;

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns
    WHERE table_name='notification_rules' AND column_name='engagement_added') THEN
    ALTER TABLE notification_rules ADD COLUMN engagement_added TEXT[] DEFAULT '{}'; END IF; END $$;

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns
    WHERE table_name='notification_rules' AND column_name='engagement_closed') THEN
    ALTER TABLE notification_rules ADD COLUMN engagement_closed TEXT[] DEFAULT '{}'; END IF; END $$;

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns
    WHERE table_name='notification_rules' AND column_name='risk_acceptance_expiration') THEN
    ALTER TABLE notification_rules ADD COLUMN risk_acceptance_expiration TEXT[] DEFAULT '{}'; END IF; END $$;

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns
    WHERE table_name='notification_rules' AND column_name='sla_breach') THEN
    ALTER TABLE notification_rules ADD COLUMN sla_breach TEXT[] DEFAULT '{}'; END IF; END $$;

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns
    WHERE table_name='notification_rules' AND column_name='sla_expiring_soon') THEN
    ALTER TABLE notification_rules ADD COLUMN sla_expiring_soon TEXT[] DEFAULT '{}'; END IF; END $$;

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns
    WHERE table_name='notification_rules' AND column_name='product_added') THEN
    ALTER TABLE notification_rules ADD COLUMN product_added TEXT[] DEFAULT '{}'; END IF; END $$;

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns
    WHERE table_name='notification_rules' AND column_name='user_mentioned') THEN
    ALTER TABLE notification_rules ADD COLUMN user_mentioned TEXT[] DEFAULT '{}'; END IF; END $$;

-- delivery_records table for retry tracking
CREATE TABLE IF NOT EXISTS delivery_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type VARCHAR(100) NOT NULL,
    channel VARCHAR(50) NOT NULL,
    recipient TEXT NOT NULL,
    status VARCHAR(20) DEFAULT 'pending',
    attempts INTEGER DEFAULT 0,
    last_attempt_at TIMESTAMPTZ,
    error_message TEXT,
    payload JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
) PARTITION BY RANGE (created_at);

CREATE TABLE IF NOT EXISTS delivery_records_2026
    PARTITION OF delivery_records
    FOR VALUES FROM ('2026-01-01') TO ('2027-01-01');

-- alerts table for in-app notifications
CREATE TABLE IF NOT EXISTS alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    url TEXT,
    is_read BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_alerts_user_unread ON alerts(user_id, is_read) WHERE NOT is_read;
CREATE INDEX IF NOT EXISTS idx_alerts_user_created ON alerts(user_id, created_at DESC);
```

## Acceptance Criteria

- [x] `notification_rules` table có 11 new columns (scan_added, finding_added, etc.)
- [x] Migration idempotent
- [x] `delivery_records` partitioned table created
- [x] `alerts` table created với proper indexes
- [x] `EventSLABreach`, `EventFindingAdded`, etc. constants available in codebase
- [x] `NotificationRule` struct có đủ channel fields
- [x] `go build ./...` thành công

## Implementation Status: ✅ DONE

> `notification-service/internal/domain/rule/entity.go` — 14 EventType constants + 14 Channel fields trong NotificationRule struct + ChannelsForEvent()
> `notification-service/migrations/003_notification_defectdojo_events.sql` — idempotent ADD COLUMN (DO $$ IF NOT EXISTS), delivery_records partitioned, alerts table + 2 indexes
> `notification-service/migrations/005_notification_rules.up.sql` — extended notification_rules schema
