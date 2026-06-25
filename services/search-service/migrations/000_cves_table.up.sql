-- Migration: 000_cves_table.up.sql
-- Creates the main 'cves' table required by search-service Postgres backend.
-- This table is the read-side store for CVE full-text search queries.
-- Data is synced from data-service (MongoDB) by the ingest pipeline.

CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS cves (
    id               TEXT PRIMARY KEY,               -- CVE-YYYY-NNNNN
    description      TEXT NOT NULL DEFAULT '',
    severity         TEXT NOT NULL DEFAULT 'UNKNOWN', -- CRITICAL|HIGH|MEDIUM|LOW|UNKNOWN
    published        TIMESTAMPTZ,
    source           TEXT NOT NULL DEFAULT 'NVD',
    is_kev           BOOLEAN NOT NULL DEFAULT FALSE,
    is_exploit       BOOLEAN NOT NULL DEFAULT FALSE,
    link             TEXT,
    cvss_score       NUMERIC(4,1),
    cvss3_score      NUMERIC(4,1),
    epss             NUMERIC(8,7),
    epss_percentile  NUMERIC(8,7),
    cwe              TEXT[] NOT NULL DEFAULT '{}',
    vendors          TEXT[] NOT NULL DEFAULT '{}',
    products         TEXT[] NOT NULL DEFAULT '{}',
    embedding        vector(1536),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Full-text search index
CREATE INDEX IF NOT EXISTS idx_cves_fts
    ON cves USING GIN(to_tsvector('english', id || ' ' || description));

-- Severity/EPSS filters
CREATE INDEX IF NOT EXISTS idx_cves_severity  ON cves(severity);
CREATE INDEX IF NOT EXISTS idx_cves_epss      ON cves(epss DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS idx_cves_published ON cves(published DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS idx_cves_is_kev    ON cves(is_kev) WHERE is_kev = TRUE;
CREATE INDEX IF NOT EXISTS idx_cves_is_exploit ON cves(is_exploit) WHERE is_exploit = TRUE;
CREATE INDEX IF NOT EXISTS idx_cves_cwe       ON cves USING GIN(cwe);
CREATE INDEX IF NOT EXISTS idx_cves_vendors   ON cves USING GIN(vendors);
CREATE INDEX IF NOT EXISTS idx_cves_products  ON cves USING GIN(products);

-- Seed from kev_entries: populate is_kev=true for any existing KEV records
-- (this syncs the 1623 KEV entries already in the DB)
INSERT INTO cves (id, description, severity, source, is_kev, created_at, updated_at)
SELECT
    k.cve_id,
    COALESCE(k.vulnerability_name, ''),
    'UNKNOWN',
    'CISA-KEV',
    TRUE,
    NOW(),
    NOW()
FROM kev_entries k
ON CONFLICT (id) DO UPDATE
    SET is_kev = TRUE, updated_at = NOW();
