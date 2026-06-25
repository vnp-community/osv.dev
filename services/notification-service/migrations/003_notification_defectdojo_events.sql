-- Migration 003: Extend notification_rules for DefectDojo events
-- + delivery_records (partitioned) + alerts table
-- Idempotent: uses DO $$ IF NOT EXISTS $$ pattern

BEGIN;

-- ─── Add DefectDojo event columns to notification_rules ──────────────────────
DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='notification_rules' AND column_name='scan_added') THEN
    ALTER TABLE notification_rules ADD COLUMN scan_added TEXT[] NOT NULL DEFAULT '{}'; END IF; END $$;

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='notification_rules' AND column_name='test_added') THEN
    ALTER TABLE notification_rules ADD COLUMN test_added TEXT[] NOT NULL DEFAULT '{}'; END IF; END $$;

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='notification_rules' AND column_name='finding_added') THEN
    ALTER TABLE notification_rules ADD COLUMN finding_added TEXT[] NOT NULL DEFAULT '{}'; END IF; END $$;

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='notification_rules' AND column_name='finding_status_changed') THEN
    ALTER TABLE notification_rules ADD COLUMN finding_status_changed TEXT[] NOT NULL DEFAULT '{}'; END IF; END $$;

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='notification_rules' AND column_name='jira_update') THEN
    ALTER TABLE notification_rules ADD COLUMN jira_update TEXT[] NOT NULL DEFAULT '{}'; END IF; END $$;

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='notification_rules' AND column_name='engagement_added') THEN
    ALTER TABLE notification_rules ADD COLUMN engagement_added TEXT[] NOT NULL DEFAULT '{}'; END IF; END $$;

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='notification_rules' AND column_name='engagement_closed') THEN
    ALTER TABLE notification_rules ADD COLUMN engagement_closed TEXT[] NOT NULL DEFAULT '{}'; END IF; END $$;

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='notification_rules' AND column_name='risk_acceptance_expiration') THEN
    ALTER TABLE notification_rules ADD COLUMN risk_acceptance_expiration TEXT[] NOT NULL DEFAULT '{}'; END IF; END $$;

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='notification_rules' AND column_name='sla_breach') THEN
    ALTER TABLE notification_rules ADD COLUMN sla_breach TEXT[] NOT NULL DEFAULT '{}'; END IF; END $$;

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='notification_rules' AND column_name='sla_expiring_soon') THEN
    ALTER TABLE notification_rules ADD COLUMN sla_expiring_soon TEXT[] NOT NULL DEFAULT '{}'; END IF; END $$;

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='notification_rules' AND column_name='product_added') THEN
    ALTER TABLE notification_rules ADD COLUMN product_added TEXT[] NOT NULL DEFAULT '{}'; END IF; END $$;

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='notification_rules' AND column_name='user_mentioned') THEN
    ALTER TABLE notification_rules ADD COLUMN user_mentioned TEXT[] NOT NULL DEFAULT '{}'; END IF; END $$;

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='notification_rules' AND column_name='closed_finding_removed') THEN
    ALTER TABLE notification_rules ADD COLUMN closed_finding_removed TEXT[] NOT NULL DEFAULT '{}'; END IF; END $$;

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='notification_rules' AND column_name='review_requested') THEN
    ALTER TABLE notification_rules ADD COLUMN review_requested TEXT[] NOT NULL DEFAULT '{}'; END IF; END $$;

-- ─── delivery_records: retry tracking (partitioned by month) ─────────────────
CREATE TABLE IF NOT EXISTS delivery_records (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    event_type VARCHAR(100) NOT NULL,
    channel VARCHAR(50) NOT NULL,
    recipient TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'retrying', 'sent', 'failed')),
    attempts INTEGER NOT NULL DEFAULT 0,
    last_attempt_at TIMESTAMPTZ,
    error_message TEXT,
    payload JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

CREATE TABLE IF NOT EXISTS delivery_records_2026
    PARTITION OF delivery_records
    FOR VALUES FROM ('2026-01-01') TO ('2027-01-01');

CREATE TABLE IF NOT EXISTS delivery_records_2027
    PARTITION OF delivery_records
    FOR VALUES FROM ('2027-01-01') TO ('2028-01-01');

CREATE INDEX IF NOT EXISTS idx_delivery_status_created
    ON delivery_records(status, created_at DESC)
    WHERE status IN ('pending', 'retrying');

-- ─── alerts: in-app notifications ────────────────────────────────────────────
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

CREATE INDEX IF NOT EXISTS idx_alerts_user_unread
    ON alerts(user_id, is_read) WHERE NOT is_read;

CREATE INDEX IF NOT EXISTS idx_alerts_user_created
    ON alerts(user_id, created_at DESC);

COMMIT;
