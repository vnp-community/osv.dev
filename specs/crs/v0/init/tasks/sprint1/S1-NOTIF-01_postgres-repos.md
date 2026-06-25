# S1-NOTIF-01 — Thêm PostgreSQL Rule/Alert Repos (notification-service)

## ✅ Execution Status: COMPLETED
- **Executed**: 2026-06-13
- **Result**: `go build` + `go vet` PASSED
- **Files Created**:
  - `migrations/005_notification_rules.up.sql` ← notification_rules, inapp_alerts, delivery_records tables
  - `migrations/005_notification_rules.down.sql` ← rollback
  - `internal/infra/persistence/postgres/rule_repo.go` ← PostgresRuleRepo implements rule.Repository
  - `internal/infra/persistence/postgres/alert_repo.go` ← PostgresAlertRepo implements alert.Repository
  - `internal/config/storage_config.go` ← RULE_BACKEND env var selector
- **Key Adjustments vs Spec**:
  - Domain `NotificationRule.UserID` / `ProductID` are `*uuid.UUID` (not `uuid.UUID`) — scanRule correctly handles nil
  - `IsActive` không có trong domain struct — scanned into local var, ignored
  - alert table named `inapp_alerts` (to avoid collision with keyword `alerts` in some DBs)
  - Alert.Repository interface: `ListForUser(ctx, userID, unreadOnly, limit, offset) ([]*Alert, int, error)` — repo returns (list, total, err)

## Metadata
- **Task ID**: S1-NOTIF-01
- **Service**: notification-service
- **Sprint**: 1 (P0)
- **Ước tính**: 3 giờ
- **Dependencies**: Không có
- **Spec nguồn**: `specs/develop/07_notification-service-upgrade.md` § "P0 — Thêm: PostgreSQL Rule Repo"

## Context

```bash
# Đọc Firestore implementation để biết interface:
cat services/notification-service/internal/infra/persistence/firestore/repos.go

# Đọc domain repository interface:
cat services/notification-service/internal/domain/repository/repository.go

# Đọc domain entities:
cat services/notification-service/internal/domain/rule/entity.go
cat services/notification-service/internal/domain/alert/entity.go

# Pattern từ service khác:
cat services/notification-service/internal/infra/persistence/postgres/webhook_repo.go
```

## Files to Create

### File 1: `services/notification-service/migrations/005_notification_rules.up.sql`

```sql
-- 005_notification_rules.up.sql
-- PostgreSQL alternative to Firestore for notification rules + alert history

-- Notification rules table
CREATE TABLE IF NOT EXISTS notification_rules (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID,                    -- NULL = system-wide rule
    product_id  UUID,                    -- NULL = applies to all products

    -- Per-event-type channel lists (stored as text arrays)
    scan_added                TEXT[] NOT NULL DEFAULT '{}',
    finding_added             TEXT[] NOT NULL DEFAULT '{}',
    finding_status_changed    TEXT[] NOT NULL DEFAULT '{}',
    jira_update               TEXT[] NOT NULL DEFAULT '{}',
    engagement_added          TEXT[] NOT NULL DEFAULT '{}',
    engagement_closed         TEXT[] NOT NULL DEFAULT '{}',
    risk_acceptance_expiration TEXT[] NOT NULL DEFAULT '{}',
    sla_breach                TEXT[] NOT NULL DEFAULT '{}',
    sla_expiring_soon         TEXT[] NOT NULL DEFAULT '{}',
    product_added             TEXT[] NOT NULL DEFAULT '{}',
    user_mentioned            TEXT[] NOT NULL DEFAULT '{}',

    is_active   BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_notif_rules_user_active
    ON notification_rules(user_id, is_active)
    WHERE is_active = TRUE;

CREATE INDEX IF NOT EXISTS idx_notif_rules_product_active
    ON notification_rules(product_id, is_active)
    WHERE is_active = TRUE;

-- Alert history table
CREATE TABLE IF NOT EXISTS alerts (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type  VARCHAR(100) NOT NULL,
    payload     JSONB       NOT NULL DEFAULT '{}',
    rule_id     UUID        REFERENCES notification_rules(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_alerts_event_type ON alerts(event_type);
CREATE INDEX IF NOT EXISTS idx_alerts_created ON alerts(created_at DESC);

-- Delivery records
CREATE TABLE IF NOT EXISTS delivery_records (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    alert_id    UUID        REFERENCES alerts(id) ON DELETE CASCADE,
    channel     VARCHAR(50) NOT NULL,
    status      VARCHAR(20) NOT NULL DEFAULT 'pending',  -- pending | sent | failed
    attempts    INT         NOT NULL DEFAULT 0,
    last_error  TEXT,
    sent_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_delivery_alert ON delivery_records(alert_id);
CREATE INDEX IF NOT EXISTS idx_delivery_status ON delivery_records(status) WHERE status != 'sent';

-- Auto-update updated_at for rules
CREATE OR REPLACE FUNCTION update_notification_rules_updated_at()
RETURNS TRIGGER AS $$
BEGIN NEW.updated_at = NOW(); RETURN NEW; END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER notification_rules_updated_at
    BEFORE UPDATE ON notification_rules
    FOR EACH ROW EXECUTE FUNCTION update_notification_rules_updated_at();
```

### File 2: `services/notification-service/migrations/005_notification_rules.down.sql`

```sql
DROP TRIGGER IF EXISTS notification_rules_updated_at ON notification_rules;
DROP FUNCTION IF EXISTS update_notification_rules_updated_at();
DROP TABLE IF EXISTS delivery_records;
DROP TABLE IF EXISTS alerts;
DROP TABLE IF EXISTS notification_rules;
```

### File 3: `services/notification-service/internal/infra/persistence/postgres/rule_repo.go`

```go
package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	// Adjust import paths based on actual domain package:
	"github.com/osv/notification-service/internal/domain/rule"
)

// PostgresRuleRepo implements the rule repository using PostgreSQL.
type PostgresRuleRepo struct {
	db  *pgxpool.Pool
	log zerolog.Logger
}

// NewRuleRepo creates a new PostgresRuleRepo.
func NewRuleRepo(db *pgxpool.Pool, log zerolog.Logger) *PostgresRuleRepo {
	return &PostgresRuleRepo{db: db, log: log}
}

// FindByUserID returns all active rules for a user.
func (r *PostgresRuleRepo) FindByUserID(ctx context.Context, userID uuid.UUID) ([]*rule.NotificationRule, error) {
	query := `
		SELECT id, user_id, product_id,
			scan_added, finding_added, finding_status_changed,
			jira_update, engagement_added, engagement_closed,
			risk_acceptance_expiration, sla_breach, sla_expiring_soon,
			product_added, user_mentioned, is_active, created_at, updated_at
		FROM notification_rules
		WHERE user_id = $1 AND is_active = TRUE
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []*rule.NotificationRule
	for rows.Next() {
		nr, err := scanRule(rows)
		if err != nil {
			r.log.Warn().Err(err).Msg("rule_repo: scan error")
			continue
		}
		rules = append(rules, nr)
	}
	return rules, rows.Err()
}

// FindByID returns a rule by its ID.
func (r *PostgresRuleRepo) FindByID(ctx context.Context, id uuid.UUID) (*rule.NotificationRule, error) {
	query := `
		SELECT id, user_id, product_id,
			scan_added, finding_added, finding_status_changed,
			jira_update, engagement_added, engagement_closed,
			risk_acceptance_expiration, sla_breach, sla_expiring_soon,
			product_added, user_mentioned, is_active, created_at, updated_at
		FROM notification_rules WHERE id = $1
	`
	row := r.db.QueryRow(ctx, query, id)
	return scanRule(row)
}

// Create creates a new notification rule.
func (r *PostgresRuleRepo) Create(ctx context.Context, nr *rule.NotificationRule) error {
	query := `
		INSERT INTO notification_rules (
			user_id, product_id,
			scan_added, finding_added, finding_status_changed,
			sla_breach, sla_expiring_soon, is_active
		) VALUES ($1, $2, $3, $4, $5, $6, $7, TRUE)
		RETURNING id, created_at, updated_at
	`
	return r.db.QueryRow(ctx, query,
		nr.UserID, nr.ProductID,
		nr.ScanAdded, nr.FindingAdded, nr.FindingStatusChanged,
		nr.SLABreach, nr.SLAExpiringSoon,
	).Scan(&nr.ID, &nr.CreatedAt, &nr.UpdatedAt)
}

// Update updates an existing rule.
func (r *PostgresRuleRepo) Update(ctx context.Context, nr *rule.NotificationRule) error {
	query := `
		UPDATE notification_rules SET
			scan_added = $2, finding_added = $3, finding_status_changed = $4,
			sla_breach = $5, sla_expiring_soon = $6, updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query,
		nr.ID, nr.ScanAdded, nr.FindingAdded, nr.FindingStatusChanged,
		nr.SLABreach, nr.SLAExpiringSoon,
	)
	return err
}

// Delete soft-deletes a rule (sets is_active=false).
func (r *PostgresRuleRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE notification_rules SET is_active = FALSE, updated_at = NOW() WHERE id = $1`,
		id,
	)
	return err
}

// scanRule scans a pgx Row/Rows into a NotificationRule.
func scanRule(scanner interface{ Scan(...any) error }) (*rule.NotificationRule, error) {
	nr := &rule.NotificationRule{}
	var userID, productID *uuid.UUID
	err := scanner.Scan(
		&nr.ID, &userID, &productID,
		&nr.ScanAdded, &nr.FindingAdded, &nr.FindingStatusChanged,
		&nr.JiraUpdate, &nr.EngagementAdded, &nr.EngagementClosed,
		&nr.RiskAcceptanceExpiration, &nr.SLABreach, &nr.SLAExpiringSoon,
		&nr.ProductAdded, &nr.UserMentioned,
		&nr.IsActive, &nr.CreatedAt, &nr.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if userID != nil {
		nr.UserID = *userID
	}
	if productID != nil {
		nr.ProductID = *productID
	}
	return nr, nil
}
```

### File 4: `services/notification-service/internal/infra/persistence/postgres/alert_repo.go`

```go
package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/osv/notification-service/internal/domain/alert"
)

// PostgresAlertRepo implements the alert repository using PostgreSQL.
type PostgresAlertRepo struct {
	db  *pgxpool.Pool
	log zerolog.Logger
}

// NewAlertRepo creates a new PostgresAlertRepo.
func NewAlertRepo(db *pgxpool.Pool, log zerolog.Logger) *PostgresAlertRepo {
	return &PostgresAlertRepo{db: db, log: log}
}

// Save persists a new alert and returns its generated ID.
func (r *PostgresAlertRepo) Save(ctx context.Context, a *alert.Alert) error {
	payload, err := json.Marshal(a.Payload)
	if err != nil {
		return err
	}

	return r.db.QueryRow(ctx,
		`INSERT INTO alerts (event_type, payload, rule_id)
		 VALUES ($1, $2, $3)
		 RETURNING id, created_at`,
		a.EventType, payload, a.RuleID,
	).Scan(&a.ID, &a.CreatedAt)
}

// FindByID returns an alert by ID.
func (r *PostgresAlertRepo) FindByID(ctx context.Context, id uuid.UUID) (*alert.Alert, error) {
	var a alert.Alert
	var payloadJSON []byte
	err := r.db.QueryRow(ctx,
		`SELECT id, event_type, payload, rule_id, created_at FROM alerts WHERE id = $1`,
		id,
	).Scan(&a.ID, &a.EventType, &payloadJSON, &a.RuleID, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(payloadJSON, &a.Payload)
	return &a, nil
}

// ListRecent returns recent alerts with pagination.
func (r *PostgresAlertRepo) ListRecent(ctx context.Context, limit, offset int) ([]*alert.Alert, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, event_type, payload, rule_id, created_at
		 FROM alerts ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []*alert.Alert
	for rows.Next() {
		var a alert.Alert
		var payloadJSON []byte
		if err := rows.Scan(&a.ID, &a.EventType, &payloadJSON, &a.RuleID, &a.CreatedAt); err != nil {
			continue
		}
		json.Unmarshal(payloadJSON, &a.Payload)
		alerts = append(alerts, &a)
	}
	return alerts, rows.Err()
}
```

### File 5: `services/notification-service/internal/config/storage_config.go`

```go
package config

// RuleBackend defines the storage backend for notification rules.
type RuleBackend string

const (
	RuleBackendFirestore RuleBackend = "firestore"  // default (current)
	RuleBackendPostgres  RuleBackend = "postgres"    // new addition
)

// StorageConfig holds backend selection for notification-service.
type StorageConfig struct {
	RuleBackend RuleBackend `env:"RULE_BACKEND" default:"firestore"`
}
```

## Files to Extend

### Extend: `services/notification-service/cmd/server/main.go`

```go
// Thêm switch:
var ruleRepo domain.RuleRepository
switch cfg.Storage.RuleBackend {
case config.RuleBackendPostgres:
    ruleRepo = postgres_infra.NewRuleRepo(pgPool, logger)
default:
    ruleRepo = firestore_infra.NewRuleRepo(firestoreClient)  // existing
}

alertRepo := postgres_infra.NewAlertRepo(pgPool, logger)  // always use postgres for alerts
```

## Verification

```bash
cd services/notification-service && go build ./...

# Run migrations:
psql $DATABASE_URL -f migrations/005_notification_rules.up.sql

# Test với RULE_BACKEND=postgres
```

## Notes

- Đọc actual `rule.NotificationRule` struct để điều chỉnh exact field names trong `scanRule()`
- Nếu domain struct có khác biệt, update field mapping trong Create/Update/scanRule
- Firestore repos.go GIỮ NGUYÊN, không sửa
