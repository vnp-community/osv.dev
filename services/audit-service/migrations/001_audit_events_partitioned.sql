-- Migration 001: Audit events — partitioned table by month

BEGIN;

-- ── Audit Events (Partitioned) ────────────────────────────────────────────────
-- APPEND-ONLY: enforced by application layer + RLS policies (see migration 002).
-- Partitioned by occurred_at for scalability and efficient purging.

CREATE TABLE IF NOT EXISTS audit_events (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    event_id VARCHAR(255),            -- NATS message ID for dedup
    event_type VARCHAR(200) NOT NULL, -- NATS subject: "defectdojo.finding.status_changed"
    -- WHO
    actor_id UUID,
    actor_email VARCHAR(255),
    actor_type VARCHAR(50) NOT NULL DEFAULT 'user'
        CONSTRAINT audit_actor_type_check CHECK (actor_type IN ('user', 'system', 'service')),
    service_name VARCHAR(100),
    -- WHAT
    resource_type VARCHAR(100) NOT NULL,
    resource_id UUID NOT NULL,
    action VARCHAR(100) NOT NULL,
    -- PAYLOAD
    changes JSONB NOT NULL DEFAULT '{}',
    metadata JSONB NOT NULL DEFAULT '{}',
    -- WHEN
    occurred_at TIMESTAMPTZ NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- INTEGRITY
    signature VARCHAR(64)  -- HMAC-SHA256 hex
) PARTITION BY RANGE (occurred_at);

-- ── Partitions (monthly, 6 months pre-created) ────────────────────────────────
CREATE TABLE IF NOT EXISTS audit_events_2026_06
    PARTITION OF audit_events FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
CREATE TABLE IF NOT EXISTS audit_events_2026_07
    PARTITION OF audit_events FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');
CREATE TABLE IF NOT EXISTS audit_events_2026_08
    PARTITION OF audit_events FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');
CREATE TABLE IF NOT EXISTS audit_events_2026_09
    PARTITION OF audit_events FOR VALUES FROM ('2026-09-01') TO ('2026-10-01');
CREATE TABLE IF NOT EXISTS audit_events_2026_10
    PARTITION OF audit_events FOR VALUES FROM ('2026-10-01') TO ('2026-11-01');
CREATE TABLE IF NOT EXISTS audit_events_2026_11
    PARTITION OF audit_events FOR VALUES FROM ('2026-11-01') TO ('2026-12-01');

-- ── Performance indexes ────────────────────────────────────────────────────────
-- Created on each partition by PostgreSQL automatically
CREATE INDEX IF NOT EXISTS idx_audit_resource
    ON audit_events(resource_type, resource_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_actor
    ON audit_events(actor_id, occurred_at DESC)
    WHERE actor_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_audit_event_type
    ON audit_events(event_type, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_occurred
    ON audit_events(occurred_at DESC);

-- ── Unique index for NATS message deduplication ───────────────────────────────
CREATE UNIQUE INDEX IF NOT EXISTS idx_audit_event_id
    ON audit_events(event_id)
    WHERE event_id IS NOT NULL;

COMMIT;
