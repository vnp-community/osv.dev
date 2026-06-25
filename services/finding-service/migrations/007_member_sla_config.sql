-- Migration 007: Product member RBAC + SLA configuration
-- Adds: product_members, product_type_members, sla_configurations
-- Adds columns: products.sla_configuration_id, products.enable_product_tag_inheritance
-- Adds columns: engagements.build_server_id, engagements.orchestration_engine_id
-- Adds columns: tests.percent_complete

BEGIN;

-- ── SLA Configurations ─────────────────────────────────────────────────────────
-- Defines deadline policies per severity. Products reference these.

CREATE TABLE sla_configurations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    -- Days to resolve by severity
    critical_sla_days SMALLINT NOT NULL DEFAULT 7,
    high_sla_days     SMALLINT NOT NULL DEFAULT 30,
    medium_sla_days   SMALLINT NOT NULL DEFAULT 90,
    low_sla_days      SMALLINT NOT NULL DEFAULT 180,
    -- Optionally enforce SLA on risk-accepted or false-positive findings
    enforce_on_active_findings BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO sla_configurations (name, description, critical_sla_days, high_sla_days, medium_sla_days, low_sla_days)
VALUES ('Default', 'Default SLA policy', 7, 30, 90, 180);

-- ── Product extensions ─────────────────────────────────────────────────────────

ALTER TABLE products
    ADD COLUMN IF NOT EXISTS sla_configuration_id UUID REFERENCES sla_configurations(id),
    ADD COLUMN IF NOT EXISTS enable_product_tag_inheritance BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX idx_products_sla_config ON products(sla_configuration_id) WHERE sla_configuration_id IS NOT NULL;

-- ── Engagement extensions ──────────────────────────────────────────────────────

ALTER TABLE engagements
    ADD COLUMN IF NOT EXISTS build_server_id UUID,
    ADD COLUMN IF NOT EXISTS orchestration_engine_id UUID;

-- FKs to tool_configurations added after that table is created (see migration 008)

-- ── Test extensions ────────────────────────────────────────────────────────────

ALTER TABLE tests
    ADD COLUMN IF NOT EXISTS percent_complete SMALLINT NOT NULL DEFAULT 0
        CONSTRAINT tests_percent_complete_check CHECK (percent_complete BETWEEN 0 AND 100);

-- ── Product Members (RBAC) ─────────────────────────────────────────────────────

CREATE TABLE product_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    role VARCHAR(50) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT product_members_role_check CHECK (role IN ('Owner', 'Maintainer', 'Writer', 'API Importer', 'Reader')),
    UNIQUE (product_id, user_id)
);
CREATE INDEX idx_product_members_product ON product_members(product_id);
CREATE INDEX idx_product_members_user ON product_members(user_id);

-- ── ProductType Members (RBAC) ─────────────────────────────────────────────────

CREATE TABLE product_type_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_type_id UUID NOT NULL REFERENCES product_types(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    role VARCHAR(50) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT product_type_members_role_check CHECK (role IN ('Owner', 'Maintainer', 'Writer', 'API Importer', 'Reader')),
    UNIQUE (product_type_id, user_id)
);
CREATE INDEX idx_product_type_members_type ON product_type_members(product_type_id);
CREATE INDEX idx_product_type_members_user ON product_type_members(user_id);

COMMIT;
