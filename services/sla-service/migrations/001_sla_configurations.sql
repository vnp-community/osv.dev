-- Migration 001: SLA service — configuration tables

BEGIN;

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ── SLA Configurations ────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS sla_configurations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(200) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    -- Days to remediate per severity (0 = no SLA enforced)
    critical_days SMALLINT NOT NULL DEFAULT 7,
    high_days     SMALLINT NOT NULL DEFAULT 30,
    medium_days   SMALLINT NOT NULL DEFAULT 90,
    low_days      SMALLINT NOT NULL DEFAULT 365,
    -- Only one default may exist at a time
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Partial unique index: only one row may have is_default=true
CREATE UNIQUE INDEX IF NOT EXISTS idx_sla_configs_single_default
    ON sla_configurations(is_default)
    WHERE is_default = TRUE;

-- ── SLA Product Assignments ───────────────────────────────────────────────────
-- Maps products to specific SLA configurations.
-- If no assignment exists, the service falls back to the default config.

CREATE TABLE IF NOT EXISTS sla_product_assignments (
    product_id UUID PRIMARY KEY,
    sla_configuration_id UUID NOT NULL REFERENCES sla_configurations(id) ON DELETE CASCADE,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    assigned_by UUID  -- user who made the assignment
);

CREATE INDEX IF NOT EXISTS idx_sla_assignments_config ON sla_product_assignments(sla_configuration_id);

-- ── Seed: Default SLA Configuration ──────────────────────────────────────────

INSERT INTO sla_configurations (name, description, critical_days, high_days, medium_days, low_days, is_default)
VALUES (
    'Default CVSS-based SLA',
    'Industry standard remediation timeframes based on CVSS severity ratings',
    7, 30, 90, 365, TRUE
) ON CONFLICT DO NOTHING;

COMMIT;
