-- 005_notification_rules.up.sql
-- PostgreSQL alternative to Firestore for notification rules + alert history.
-- Existing Firestore repos are PRESERVED — this is purely additive.

-- Notification rules table
-- Each row maps a user/product scope to channel lists per event type.
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

-- In-app alert table
-- Stores notification records visible in the DefectDojo-style UI.
CREATE TABLE IF NOT EXISTS inapp_alerts (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL,
    event_type  VARCHAR(100) NOT NULL,
    title       VARCHAR(500) NOT NULL DEFAULT '',
    description TEXT        NOT NULL DEFAULT '',
    url         TEXT        NOT NULL DEFAULT '',
    is_read     BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_inapp_alerts_user_unread
    ON inapp_alerts(user_id, is_read, created_at DESC)
    WHERE is_read = FALSE;

CREATE INDEX IF NOT EXISTS idx_inapp_alerts_created
    ON inapp_alerts(created_at DESC);

-- Delivery records table
-- Tracks one attempt per channel delivery.
CREATE TABLE IF NOT EXISTS delivery_records (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id         UUID        REFERENCES notification_rules(id) ON DELETE SET NULL,
    event_type      VARCHAR(100) NOT NULL,
    channel         VARCHAR(50) NOT NULL,
    recipient       TEXT        NOT NULL DEFAULT '',
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',  -- pending | sent | failed | retrying
    attempts        INT         NOT NULL DEFAULT 0,
    last_attempt_at TIMESTAMPTZ,
    next_retry_at   TIMESTAMPTZ,
    error_message   TEXT        NOT NULL DEFAULT '',
    payload         JSONB       NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_delivery_records_status
    ON delivery_records(status, next_retry_at)
    WHERE status != 'sent';

CREATE INDEX IF NOT EXISTS idx_delivery_records_event ON delivery_records(event_type);

-- Auto-update updated_at for rules
CREATE OR REPLACE FUNCTION update_notification_rules_updated_at()
RETURNS TRIGGER AS $$
BEGIN NEW.updated_at = NOW(); RETURN NEW; END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS notification_rules_updated_at ON notification_rules;
CREATE TRIGGER notification_rules_updated_at
    BEFORE UPDATE ON notification_rules
    FOR EACH ROW EXECUTE FUNCTION update_notification_rules_updated_at();
