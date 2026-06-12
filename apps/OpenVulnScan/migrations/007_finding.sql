-- 007_finding.sql
-- finding-service schema: findings (DefectDojo model)
-- Source: services/finding-service/migrations/001_initial.sql (adjusted)
-- NOTE: finding-service dùng public schema (không có SET search_path)

BEGIN;

CREATE TABLE IF NOT EXISTS findings (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Reference đến scan-service
    scan_id               UUID,              -- reference scan.scans(id) — no FK vì cross-schema

    -- Reference đến product-service
    test_id               UUID,              -- reference tests(id)
    engagement_id         UUID,              -- reference engagements(id)
    product_id            UUID,              -- reference products(id)

    -- Finding identity
    title                 VARCHAR(1000) NOT NULL,
    description           TEXT NOT NULL DEFAULT '',
    mitigation            TEXT NOT NULL DEFAULT '',
    impact                TEXT NOT NULL DEFAULT '',
    references            TEXT NOT NULL DEFAULT '',
    cwe_id                INTEGER,
    cve_id                VARCHAR(30),

    -- Severity
    severity              VARCHAR(20) NOT NULL DEFAULT 'Info'
                          CHECK (severity IN ('Critical','High','Medium','Low','Info')),
    cvss_score            NUMERIC(4,1) DEFAULT 0,
    cvss_vector           TEXT,

    -- Status
    active                BOOLEAN NOT NULL DEFAULT TRUE,
    verified              BOOLEAN NOT NULL DEFAULT FALSE,
    false_positive        BOOLEAN NOT NULL DEFAULT FALSE,
    duplicate             BOOLEAN NOT NULL DEFAULT FALSE,
    risk_accepted         BOOLEAN NOT NULL DEFAULT FALSE,
    out_of_scope          BOOLEAN NOT NULL DEFAULT FALSE,

    -- Dedup
    unique_id_from_tool   VARCHAR(500),
    hash_code             VARCHAR(64),

    -- Metadata
    found_by              TEXT[] NOT NULL DEFAULT '{}',  -- scanner names
    tags                  TEXT[] NOT NULL DEFAULT '{}',
    numerical_severity    INTEGER NOT NULL DEFAULT 4,    -- 0=Crit, 1=High, 2=Med, 3=Low, 4=Info

    -- SLA
    sla_start_date        DATE,
    sla_expiration_date   DATE,

    -- Timestamps
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_findings_scan     ON findings(scan_id);
CREATE INDEX IF NOT EXISTS idx_findings_product  ON findings(product_id);
CREATE INDEX IF NOT EXISTS idx_findings_severity ON findings(severity) WHERE active = TRUE;
CREATE INDEX IF NOT EXISTS idx_findings_cve      ON findings(cve_id) WHERE cve_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_findings_active   ON findings(active, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_findings_hash     ON findings(hash_code) WHERE hash_code IS NOT NULL;

COMMIT;
