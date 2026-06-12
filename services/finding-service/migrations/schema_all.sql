-- Consolidated schema for finding-service
-- Generated: 2026-06-12T15:25:30Z
-- DO NOT EDIT — regenerate by running: cat migrations/*.sql > migrations/schema_all.sql


-- ============================================
-- Migration: 001_initial.sql
-- ============================================
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

-- ============================================
-- Migration: 002_initial.sql
-- ============================================
-- Migration 001: product-management initial schema

BEGIN;

CREATE TABLE product_types (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    critical_product BOOLEAN NOT NULL DEFAULT FALSE,
    key_product BOOLEAN NOT NULL DEFAULT FALSE,
    enable_full_risk_acceptance BOOLEAN NOT NULL DEFAULT FALSE,
    enable_simple_risk_acceptance BOOLEAN NOT NULL DEFAULT FALSE,
    tags TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_type_id UUID NOT NULL REFERENCES product_types(id),
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    prod_numeric_grade SMALLINT NOT NULL DEFAULT 0,
    business_criticality VARCHAR(20),
    platform VARCHAR(100),
    lifecycle VARCHAR(20),
    origin VARCHAR(100),
    external_audience BOOLEAN NOT NULL DEFAULT FALSE,
    internet_accessible BOOLEAN NOT NULL DEFAULT FALSE,
    enable_full_risk_acceptance BOOLEAN NOT NULL DEFAULT FALSE,
    enable_simple_risk_acceptance BOOLEAN NOT NULL DEFAULT FALSE,
    tags TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_products_type ON products(product_type_id);
CREATE UNIQUE INDEX idx_products_name_type ON products(name, product_type_id);

CREATE TABLE engagements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    name VARCHAR(300) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    lead_id UUID,
    engagement_type VARCHAR(20) NOT NULL DEFAULT 'Interactive',
    status VARCHAR(20) NOT NULL DEFAULT 'In Progress',
    start_date DATE NOT NULL DEFAULT CURRENT_DATE,
    end_date DATE,
    version VARCHAR(100),
    build_id VARCHAR(150),
    commit_hash VARCHAR(150),
    branch_tag VARCHAR(150),
    source_code_management_uri TEXT,
    deduplication_on_engagement BOOLEAN NOT NULL DEFAULT FALSE,
    tags TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (product_id, name)
);
CREATE INDEX idx_engagements_product ON engagements(product_id);
CREATE INDEX idx_engagements_status ON engagements(status);

CREATE TABLE tests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    engagement_id UUID NOT NULL REFERENCES engagements(id) ON DELETE CASCADE,
    scan_type VARCHAR(200) NOT NULL,
    title VARCHAR(500),
    description TEXT,
    target_start TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    target_end TIMESTAMPTZ,
    lead_id UUID,
    version VARCHAR(100),
    build_id VARCHAR(150),
    commit_hash VARCHAR(150),
    branch_tag VARCHAR(150),
    tags TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_tests_engagement ON tests(engagement_id);
CREATE UNIQUE INDEX idx_tests_eng_scantype ON tests(engagement_id, scan_type);

COMMIT;

-- ============================================
-- Migration: 003_orchestrator_001_initial.sql
-- ============================================
-- Migration 001: scan-orchestrator initial schema

BEGIN;

CREATE TABLE scan_imports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    test_id UUID NOT NULL,
    engagement_id UUID NOT NULL,
    product_id UUID NOT NULL,
    scan_type VARCHAR(200) NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'pending',
    file_key TEXT,
    options JSONB NOT NULL DEFAULT '{}',
    result JSONB,
    error_msg TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_scan_imports_test ON scan_imports(test_id);
CREATE INDEX idx_scan_imports_product ON scan_imports(product_id);
CREATE INDEX idx_scan_imports_status ON scan_imports(status, created_at DESC);

COMMIT;

-- ============================================
-- Migration: 004_initial_schema.sql
-- ============================================
-- Report service schema
SET search_path TO report;

CREATE TABLE IF NOT EXISTS reports (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scan_id      UUID NOT NULL,
    user_id      UUID NOT NULL,
    status       VARCHAR(20) NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending','generating','ready','failed')),
    storage_key  VARCHAR(500),
    download_url TEXT,
    error_msg    TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_reports_scan_id  ON report.reports(scan_id);
CREATE INDEX IF NOT EXISTS idx_reports_user_id  ON report.reports(user_id);
CREATE INDEX IF NOT EXISTS idx_reports_status   ON report.reports(status);
CREATE INDEX IF NOT EXISTS idx_reports_created  ON report.reports(created_at DESC);

-- ============================================
-- Migration: 005_sla_001_initial.sql
-- ============================================
-- SLA service initial schema
BEGIN;

CREATE TABLE sla_configurations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID UNIQUE,  -- NULL = global default
    critical_days INTEGER NOT NULL DEFAULT 7,
    high_days INTEGER NOT NULL DEFAULT 30,
    medium_days INTEGER NOT NULL DEFAULT 90,
    low_days INTEGER NOT NULL DEFAULT 180,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Insert global default
INSERT INTO sla_configurations (id, product_id, critical_days, high_days, medium_days, low_days)
VALUES (gen_random_uuid(), NULL, 7, 30, 90, 180);

COMMIT;

-- ============================================
-- Migration: 006_audit_001_initial.sql
-- ============================================
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
