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
