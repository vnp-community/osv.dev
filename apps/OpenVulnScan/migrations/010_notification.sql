-- 010_notification.sql
-- notification-service schema: notification_rules, alerts, delivery_records
-- Source: services/notification-service/migrations/002_dd_tables.sql

BEGIN;

CREATE TABLE IF NOT EXISTS notification_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID,
    product_id UUID,
    scan_added TEXT[] DEFAULT '{}',
    finding_added TEXT[] DEFAULT '{}',
    finding_status_changed TEXT[] DEFAULT '{}',
    sla_breach TEXT[] DEFAULT '{}',
    sla_expiring_soon TEXT[] DEFAULT '{}',
    engagement_added TEXT[] DEFAULT '{}',
    engagement_closed TEXT[] DEFAULT '{}',
    jira_update TEXT[] DEFAULT '{}',
    risk_acceptance_expiration TEXT[] DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, product_id)
);
CREATE INDEX IF NOT EXISTS idx_notification_rules_user ON notification_rules(user_id);
CREATE INDEX IF NOT EXISTS idx_notification_rules_product ON notification_rules(product_id);

-- System-wide default rule
INSERT INTO notification_rules (id, user_id, product_id, sla_breach, sla_expiring_soon)
VALUES (gen_random_uuid(), NULL, NULL, '{"email","inapp"}', '{"email","inapp"}')
ON CONFLICT (user_id, product_id) DO NOTHING;

CREATE TABLE IF NOT EXISTS alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    url TEXT,
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_alerts_user_unread ON alerts(user_id) WHERE is_read = FALSE;
CREATE INDEX IF NOT EXISTS idx_alerts_user_created ON alerts(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS delivery_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id UUID,
    event_type VARCHAR(100) NOT NULL,
    channel VARCHAR(50) NOT NULL,
    recipient TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    last_attempt_at TIMESTAMPTZ,
    next_retry_at TIMESTAMPTZ,
    error_message TEXT,
    payload JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
) PARTITION BY RANGE (created_at);

-- Default partition
CREATE TABLE IF NOT EXISTS delivery_records_default PARTITION OF delivery_records DEFAULT;

COMMIT;
