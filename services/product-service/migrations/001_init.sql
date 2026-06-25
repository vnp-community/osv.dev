-- Product service schema
-- Products, Engagements, Tests

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS products (
    id                          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name                        TEXT NOT NULL,
    description                 TEXT NOT NULL DEFAULT '',
    business_criticality        TEXT NOT NULL DEFAULT 'medium',
    platform                    TEXT NOT NULL DEFAULT 'web',
    lifecycle                   TEXT NOT NULL DEFAULT 'production',
    origin                      TEXT NOT NULL DEFAULT 'internal',
    external_audience           BOOLEAN NOT NULL DEFAULT FALSE,
    internet_accessible         BOOLEAN NOT NULL DEFAULT FALSE,
    enable_full_risk_acceptance BOOLEAN NOT NULL DEFAULT FALSE,
    enable_simple_risk_acceptance BOOLEAN NOT NULL DEFAULT TRUE,
    tags                        TEXT[] NOT NULL DEFAULT '{}',
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS engagements (
    id                          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    product_id                  UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    name                        TEXT NOT NULL,
    description                 TEXT NOT NULL DEFAULT '',
    engagement_type             TEXT NOT NULL DEFAULT 'Interactive',
    status                      TEXT NOT NULL DEFAULT 'In Progress',
    start_date                  DATE,
    end_date                    DATE,
    version                     TEXT NOT NULL DEFAULT '1.0.0',
    tags                        TEXT[] NOT NULL DEFAULT '{}',
    deduplication_on_engagement BOOLEAN NOT NULL DEFAULT TRUE,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tests (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    engagement_id   UUID NOT NULL REFERENCES engagements(id) ON DELETE CASCADE,
    scan_type       TEXT NOT NULL DEFAULT 'Manual Pentest',
    title           TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    target_start    DATE,
    target_end      DATE,
    tags            TEXT[] NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_engagements_product_id ON engagements(product_id);
CREATE INDEX IF NOT EXISTS idx_tests_engagement_id ON tests(engagement_id);
