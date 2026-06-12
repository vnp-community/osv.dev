-- Audit service initial schema — partitioned by month for efficient querying
BEGIN;

CREATE TABLE audit_events (
    id UUID NOT NULL,
    event_type VARCHAR(200) NOT NULL,
    entity_type VARCHAR(100) NOT NULL,
    entity_id VARCHAR(100),
    actor_id VARCHAR(200),
    old_state JSONB,
    new_state JSONB NOT NULL DEFAULT '{}',
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, occurred_at)  -- include occurred_at for partitioning
) PARTITION BY RANGE (occurred_at);

-- Create partitions for current and next 3 months
CREATE TABLE audit_events_2026_06 PARTITION OF audit_events
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');

CREATE TABLE audit_events_2026_07 PARTITION OF audit_events
    FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');

CREATE TABLE audit_events_2026_08 PARTITION OF audit_events
    FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');

-- Default partition for overflow
CREATE TABLE audit_events_default PARTITION OF audit_events DEFAULT;

-- Indexes on the parent table (auto-propagate to partitions)
CREATE INDEX idx_audit_entity ON audit_events(entity_type, entity_id, occurred_at DESC);
CREATE INDEX idx_audit_actor ON audit_events(actor_id, occurred_at DESC);
CREATE INDEX idx_audit_event_type ON audit_events(event_type, occurred_at DESC);

COMMIT;
