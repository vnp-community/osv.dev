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
