-- 009_cve.sql
-- vulnerability-service schema: cves, cve_references, cve_affected_packages, package_cve_cache
-- Source: services/vulnerability-service/migrations/001_initial_schema.sql

SET search_path TO cve;

CREATE TABLE IF NOT EXISTS cves (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cve_id           VARCHAR(30) NOT NULL UNIQUE,
    summary          TEXT NOT NULL DEFAULT '',
    description      TEXT NOT NULL DEFAULT '',
    severity         VARCHAR(20) NOT NULL DEFAULT 'none',
    cvss_v3_score    NUMERIC(4,1) DEFAULT 0,
    cvss_v3_vector   VARCHAR(200),
    cvss_v2_score    NUMERIC(4,1) DEFAULT 0,
    epss             NUMERIC(7,6) DEFAULT 0,
    epss_percentile  NUMERIC(7,6) DEFAULT 0,
    remediation      TEXT,
    published_at     TIMESTAMPTZ,
    updated_at       TIMESTAMPTZ,
    last_fetched_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sources          TEXT[] NOT NULL DEFAULT '{}',
    embedding        vector(768),
    embedding_model  VARCHAR(100),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS cve_references (
    id     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cve_id VARCHAR(30) NOT NULL REFERENCES cve.cves(cve_id) ON DELETE CASCADE,
    url    TEXT NOT NULL,
    type   VARCHAR(50)
);

CREATE TABLE IF NOT EXISTS cve_affected_packages (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cve_id        VARCHAR(30) NOT NULL REFERENCES cve.cves(cve_id) ON DELETE CASCADE,
    ecosystem     VARCHAR(50) NOT NULL,
    package_name  VARCHAR(255) NOT NULL,
    versions      TEXT[] NOT NULL DEFAULT '{}',
    fixed_version VARCHAR(100)
);

CREATE TABLE IF NOT EXISTS package_cve_cache (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ecosystem    VARCHAR(50) NOT NULL,
    package_name VARCHAR(255) NOT NULL,
    version      VARCHAR(100),
    cve_ids      TEXT[] NOT NULL DEFAULT '{}',
    fetched_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (ecosystem, package_name, version)
);

-- Sync jobs (ingestion-service)
CREATE TABLE IF NOT EXISTS sync_jobs (
    id           BIGSERIAL   PRIMARY KEY,
    source       TEXT        NOT NULL,
    status       TEXT        NOT NULL DEFAULT 'PENDING'
                             CHECK (status IN ('PENDING','RUNNING','COMPLETED','FAILED')),
    started_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    synced       INT         NOT NULL DEFAULT 0,
    skipped      INT         NOT NULL DEFAULT 0,
    errors       INT         NOT NULL DEFAULT 0,
    error_msg    TEXT        NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_cves_severity     ON cve.cves(severity);
CREATE INDEX IF NOT EXISTS idx_cves_cvss         ON cve.cves(cvss_v3_score DESC);
CREATE INDEX IF NOT EXISTS idx_cves_published    ON cve.cves(published_at DESC);
CREATE INDEX IF NOT EXISTS idx_cves_fetched      ON cve.cves(last_fetched_at DESC);
CREATE INDEX IF NOT EXISTS idx_affected_pkg      ON cve.cve_affected_packages(ecosystem, package_name);
CREATE INDEX IF NOT EXISTS idx_sync_jobs_source  ON cve.sync_jobs(source);
CREATE INDEX IF NOT EXISTS idx_sync_jobs_started ON cve.sync_jobs(started_at DESC NULLS LAST);

-- NOTE: Create IVFFlat index AFTER >= 100 rows:
-- CREATE INDEX idx_cves_embedding ON cve.cves
--   USING ivfflat (embedding vector_cosine_ops) WITH (lists=100);
