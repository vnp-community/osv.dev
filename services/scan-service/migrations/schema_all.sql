-- Consolidated schema for scan-service
-- Generated: 2026-06-12T15:25:30Z
-- DO NOT EDIT — regenerate by running: cat migrations/*.sql > migrations/schema_all.sql


-- ============================================
-- Migration: 001_initial_schema.sql
-- ============================================
-- Scan service schema
SET search_path TO scan;

CREATE TABLE IF NOT EXISTS scans (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL,
    targets         JSONB NOT NULL DEFAULT '[]',
    scan_type       VARCHAR(20) NOT NULL CHECK (scan_type IN ('full','discovery','web','agent')),
    status          VARCHAR(20) NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending','queued','running','completed','failed','cancelled')),
    priority        INT NOT NULL DEFAULT 5 CHECK (priority BETWEEN 1 AND 10),
    options         JSONB NOT NULL DEFAULT '{}',
    progress        INT NOT NULL DEFAULT 0 CHECK (progress BETWEEN 0 AND 100),
    finding_count   INT NOT NULL DEFAULT 0,
    error_msg       TEXT,
    scheduled_for   TIMESTAMPTZ,
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    failed_at       TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS findings (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scan_id     UUID NOT NULL REFERENCES scan.scans(id) ON DELETE CASCADE,
    ip_address  INET NOT NULL,
    hostname    VARCHAR(255),
    os          VARCHAR(255),
    open_ports  JSONB NOT NULL DEFAULT '[]',
    services    JSONB NOT NULL DEFAULT '[]',
    web_tech    JSONB NOT NULL DEFAULT '[]',
    cve_ids     JSONB NOT NULL DEFAULT '[]',
    severity    VARCHAR(20) DEFAULT 'none',
    raw_data    JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (scan_id, ip_address)
);

CREATE TABLE IF NOT EXISTS web_alerts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scan_id     UUID NOT NULL REFERENCES scan.scans(id) ON DELETE CASCADE,
    target_url  TEXT NOT NULL,
    alert_name  VARCHAR(255) NOT NULL,
    risk        VARCHAR(50),
    confidence  VARCHAR(50),
    description TEXT,
    solution    TEXT,
    reference   TEXT,
    evidence    TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS discovery_hosts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scan_id     UUID NOT NULL REFERENCES scan.scans(id) ON DELETE CASCADE,
    ip_address  INET NOT NULL,
    hostname    VARCHAR(255),
    status      VARCHAR(10) DEFAULT 'up',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (scan_id, ip_address)
);

CREATE INDEX IF NOT EXISTS idx_scans_user_id  ON scan.scans(user_id);
CREATE INDEX IF NOT EXISTS idx_scans_status   ON scan.scans(status);
CREATE INDEX IF NOT EXISTS idx_scans_created  ON scan.scans(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_findings_scan  ON scan.findings(scan_id);
CREATE INDEX IF NOT EXISTS idx_alerts_scan    ON scan.web_alerts(scan_id);

-- ============================================
-- Migration: 002_agent_001_initial_schema.sql
-- ============================================
-- Agent service schema
SET search_path TO agent;

CREATE TABLE IF NOT EXISTS agents (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          VARCHAR(255) NOT NULL DEFAULT '',
    hostname      VARCHAR(255) NOT NULL,
    ip_address    INET,
    os            VARCHAR(255),
    agent_version VARCHAR(50),
    api_key_id    UUID NOT NULL UNIQUE,
    status        VARCHAR(20) NOT NULL DEFAULT 'unknown'
                     CHECK (status IN ('active','inactive','unknown')),
    last_seen_at  TIMESTAMPTZ,
    tags          TEXT[] NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS agent_reports (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id       UUID NOT NULL REFERENCES agent.agents(id) ON DELETE CASCADE,
    hostname       VARCHAR(255),
    ip_address     INET,
    os_info        TEXT,
    kernel_version VARCHAR(100),
    package_count  INT NOT NULL DEFAULT 0,
    cve_count      INT NOT NULL DEFAULT 0,
    reported_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at   TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS packages (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    report_id    UUID NOT NULL REFERENCES agent.agent_reports(id) ON DELETE CASCADE,
    name         VARCHAR(255) NOT NULL,
    version      VARCHAR(100),
    ecosystem    VARCHAR(50) NOT NULL,
    architecture VARCHAR(20)
);

CREATE TABLE IF NOT EXISTS package_cves (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    package_id  UUID NOT NULL REFERENCES agent.packages(id) ON DELETE CASCADE,
    cve_id      VARCHAR(30) NOT NULL,
    severity    VARCHAR(20),
    cvss        NUMERIC(4,1)
);

CREATE INDEX IF NOT EXISTS idx_agents_api_key   ON agent.agents(api_key_id);
CREATE INDEX IF NOT EXISTS idx_agents_status    ON agent.agents(status);
CREATE INDEX IF NOT EXISTS idx_reports_agent    ON agent.agent_reports(agent_id);
CREATE INDEX IF NOT EXISTS idx_reports_date     ON agent.agent_reports(reported_at DESC);
CREATE INDEX IF NOT EXISTS idx_packages_report  ON agent.packages(report_id);

-- ============================================
-- Migration: 003_asset_001_initial_schema.sql
-- ============================================
-- Asset service schema
SET search_path TO asset;

CREATE TABLE IF NOT EXISTS assets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ip_address      INET NOT NULL UNIQUE,
    hostname        VARCHAR(255),
    os              VARCHAR(255),
    mac_address     VARCHAR(17),
    services        JSONB NOT NULL DEFAULT '[]',
    web_tech        JSONB NOT NULL DEFAULT '[]',
    labels          JSONB NOT NULL DEFAULT '{}',
    last_scanned_at TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tags (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(100) NOT NULL UNIQUE,
    color      VARCHAR(7) NOT NULL DEFAULT '#6366F1',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS asset_tags (
    asset_id UUID NOT NULL REFERENCES asset.assets(id) ON DELETE CASCADE,
    tag_id   UUID NOT NULL REFERENCES asset.tags(id) ON DELETE CASCADE,
    PRIMARY KEY (asset_id, tag_id)
);

CREATE TABLE IF NOT EXISTS vulnerabilities (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    asset_id       UUID NOT NULL REFERENCES asset.assets(id) ON DELETE CASCADE,
    cve_id         VARCHAR(30) NOT NULL,
    summary        TEXT,
    severity       VARCHAR(20) NOT NULL DEFAULT 'none',
    cvss           NUMERIC(4,1) DEFAULT 0,
    scan_id        UUID,
    detected_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    remediated_at  TIMESTAMPTZ,
    UNIQUE (asset_id, cve_id)
);

CREATE INDEX IF NOT EXISTS idx_assets_ip         ON asset.assets(ip_address);
CREATE INDEX IF NOT EXISTS idx_assets_hostname   ON asset.assets(hostname);
CREATE INDEX IF NOT EXISTS idx_assets_updated    ON asset.assets(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_vulns_asset       ON asset.vulnerabilities(asset_id);
CREATE INDEX IF NOT EXISTS idx_vulns_severity    ON asset.vulnerabilities(severity) WHERE remediated_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_vulns_cve         ON asset.vulnerabilities(cve_id);
