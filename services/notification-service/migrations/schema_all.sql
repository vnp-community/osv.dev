-- Consolidated schema for notification-service
-- Generated: 2026-06-12T15:25:30Z
-- DO NOT EDIT — regenerate by running: cat migrations/*.sql > migrations/schema_all.sql


-- ============================================
-- Migration: 001_dd_tables.sql
-- ============================================
-- Notification service new tables (002 — OSV webhook tables are 001)
BEGIN;

CREATE TABLE notification_rules (
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
CREATE INDEX idx_notification_rules_user ON notification_rules(user_id);
CREATE INDEX idx_notification_rules_product ON notification_rules(product_id);

-- Insert system-wide default rule (user_id=NULL, product_id=NULL)
INSERT INTO notification_rules (id, user_id, product_id, sla_breach, sla_expiring_soon)
VALUES (gen_random_uuid(), NULL, NULL, '{"email","inapp"}', '{"email","inapp"}');

CREATE TABLE alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    url TEXT,
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_alerts_user_unread ON alerts(user_id) WHERE is_read = FALSE;
CREATE INDEX idx_alerts_user_created ON alerts(user_id, created_at DESC);

CREATE TABLE delivery_records (
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

-- Default partition (catches everything before monthly partitions are created)
CREATE TABLE delivery_records_default PARTITION OF delivery_records DEFAULT;

COMMIT;

-- ============================================
-- Migration: 002_create_jira_integrations.up.sql
-- ============================================
CREATE TABLE jira_integrations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id  UUID,
    server_url  TEXT NOT NULL,
    project_key VARCHAR(50) NOT NULL,
    issue_type  VARCHAR(50) DEFAULT 'Bug',
    api_token   TEXT,
    auto_create BOOLEAN DEFAULT FALSE,
    auto_sync   BOOLEAN DEFAULT FALSE,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE jira_issues (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    finding_id       UUID NOT NULL,
    integration_id   UUID REFERENCES jira_integrations(id) ON DELETE CASCADE,
    issue_key        VARCHAR(100),
    issue_url        TEXT,
    status           VARCHAR(50),
    synced_at        TIMESTAMPTZ,
    created_at       TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_jira_issues_finding_id ON jira_issues(finding_id);
CREATE INDEX idx_jira_issues_integration_id ON jira_issues(integration_id);

-- ============================================
-- Migration: 003_globalcve_001_create_webhooks.down.sql
-- ============================================
DROP INDEX IF EXISTS idx_webhooks_events;
DROP INDEX IF EXISTS idx_webhooks_active;
DROP INDEX IF EXISTS idx_webhooks_owner;
DROP TABLE IF EXISTS webhooks CASCADE;

-- ============================================
-- Migration: 004_globalcve_001_create_webhooks.up.sql
-- ============================================
-- Notification service: webhooks table
CREATE TABLE IF NOT EXISTS webhooks (
    id          TEXT        PRIMARY KEY DEFAULT gen_random_uuid()::text,
    owner_id    TEXT        NOT NULL,
    url         TEXT        NOT NULL,
    events      TEXT[]      NOT NULL DEFAULT '{}',
    secret      TEXT        NOT NULL DEFAULT '',
    active      BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_webhooks_owner  ON webhooks(owner_id);
CREATE INDEX IF NOT EXISTS idx_webhooks_active ON webhooks(active) WHERE active = TRUE;
CREATE INDEX IF NOT EXISTS idx_webhooks_events ON webhooks USING GIN(events) WHERE active = TRUE;
