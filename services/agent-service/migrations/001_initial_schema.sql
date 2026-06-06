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
