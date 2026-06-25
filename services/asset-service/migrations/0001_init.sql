-- Migration: SEED-005-A — Asset Service initial schema
-- Creates osv_asset schema, assets table, and asset_vulnerabilities table.
-- Compatible with PostgreSQL 16.

CREATE SCHEMA IF NOT EXISTS osv_asset;

-- ── Assets table ──────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS osv_asset.assets (
    id            UUID         DEFAULT gen_random_uuid() PRIMARY KEY,
    ip_address    INET         UNIQUE,
    hostname      VARCHAR(255),
    os            VARCHAR(100),
    mac_address   VARCHAR(17),
    services      JSONB        DEFAULT '[]'::JSONB,
    tags          TEXT[]       DEFAULT '{}',
    labels        JSONB        DEFAULT '{}'::JSONB,
    risk_score    NUMERIC(4,2) DEFAULT 0,
    finding_count INT          DEFAULT 0,
    status        VARCHAR(20)  DEFAULT 'active'
                  CHECK (status IN ('active','inactive','decommissioned')),
    last_seen_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ  DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  DEFAULT NOW()
);

-- GIN index for fast tag filtering (array containment @>, &&)
CREATE INDEX IF NOT EXISTS idx_assets_tags
    ON osv_asset.assets USING GIN(tags);

CREATE INDEX IF NOT EXISTS idx_assets_status
    ON osv_asset.assets(status);

CREATE INDEX IF NOT EXISTS idx_assets_ip_address
    ON osv_asset.assets(ip_address);

-- ── Asset Vulnerabilities table ───────────────────────────────────────────────
-- Vulnerabilities injected manually via API (SEED-005) or detected by scan agents (SEED-005-C)
CREATE TABLE IF NOT EXISTS osv_asset.asset_vulnerabilities (
    id          UUID         DEFAULT gen_random_uuid() PRIMARY KEY,
    asset_id    UUID         NOT NULL REFERENCES osv_asset.assets(id) ON DELETE CASCADE,
    cve_id      VARCHAR(50)  NOT NULL,
    severity    VARCHAR(10)  NOT NULL
                CHECK (severity IN ('critical','high','medium','low','none')),
    cvss        NUMERIC(4,2),
    detected_at TIMESTAMPTZ  DEFAULT NOW(),
    UNIQUE(asset_id, cve_id)
);

CREATE INDEX IF NOT EXISTS idx_asset_vulns_asset_id
    ON osv_asset.asset_vulnerabilities(asset_id);

CREATE INDEX IF NOT EXISTS idx_asset_vulns_cve_id
    ON osv_asset.asset_vulnerabilities(cve_id);

-- ── Scheduled scan jobs table (SEED-005-C) ─────────────────────────────────
CREATE TABLE IF NOT EXISTS osv_asset.scan_schedules (
    id             UUID         DEFAULT gen_random_uuid() PRIMARY KEY,
    asset_id       UUID         NOT NULL REFERENCES osv_asset.assets(id) ON DELETE CASCADE,
    scan_type      VARCHAR(30)  NOT NULL DEFAULT 'nmap',  -- nmap|zap|agent|manual
    schedule_cron  VARCHAR(50)  NOT NULL,
    enabled        BOOLEAN      DEFAULT TRUE,
    last_run_at    TIMESTAMPTZ,
    next_run_at    TIMESTAMPTZ,
    created_by     UUID,
    created_at     TIMESTAMPTZ  DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  DEFAULT NOW(),
    UNIQUE(asset_id, scan_type)
);

CREATE INDEX IF NOT EXISTS idx_scan_schedules_enabled
    ON osv_asset.scan_schedules(enabled, next_run_at)
    WHERE enabled = TRUE;

COMMENT ON TABLE osv_asset.assets IS
    'Network assets registry — created by scans (NATS) or manually via API (SEED-005)';
COMMENT ON TABLE osv_asset.asset_vulnerabilities IS
    'Vulnerabilities manually injected into assets for seeding (SEED-005) or detected by scan agent (SEED-005-C)';
COMMENT ON TABLE osv_asset.scan_schedules IS
    'Scheduled scan jobs for assets (SEED-005-C)';
