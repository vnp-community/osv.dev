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
