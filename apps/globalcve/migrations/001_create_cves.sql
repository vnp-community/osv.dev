-- +goose Up
-- +goose StatementBegin

CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS cves (
    id              TEXT        PRIMARY KEY,          -- "CVE-2021-44228"
    description     TEXT        NOT NULL DEFAULT '',
    summary         TEXT        NOT NULL DEFAULT '',
    severity        TEXT        NOT NULL DEFAULT 'UNKNOWN'
                                CHECK (severity IN ('CRITICAL', 'HIGH', 'MEDIUM', 'LOW', 'UNKNOWN')),
    published       TIMESTAMPTZ,
    modified        TIMESTAMPTZ,
    source          TEXT        NOT NULL DEFAULT 'NVD'
                                CHECK (source IN ('NVD', 'CIRCL', 'JVN', 'EXPLOITDB', 'CVE.ORG', 'ARCHIVE')),
    is_kev          BOOLEAN     NOT NULL DEFAULT FALSE,
    link            TEXT        NOT NULL DEFAULT '',
    cvss_score      NUMERIC(4,1),                     -- CVSS v2
    cvss3_score     NUMERIC(4,1),                     -- CVSS v3
    cvss_vector     TEXT        NOT NULL DEFAULT '',
    cvss3_vector    TEXT        NOT NULL DEFAULT '',
    epss            NUMERIC(8,6),                     -- 0.000000 - 1.000000
    epss_percentile NUMERIC(8,6),
    vendors         TEXT[]      NOT NULL DEFAULT '{}',
    products        TEXT[]      NOT NULL DEFAULT '{}',
    cwe             TEXT[]      NOT NULL DEFAULT '{}',
    references      TEXT[]      NOT NULL DEFAULT '{}',
    embedding       vector(1536),                     -- pgvector for semantic search
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Performance indexes
CREATE INDEX IF NOT EXISTS idx_cves_severity   ON cves(severity);
CREATE INDEX IF NOT EXISTS idx_cves_published  ON cves(published DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS idx_cves_modified   ON cves(modified DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS idx_cves_source     ON cves(source);
CREATE INDEX IF NOT EXISTS idx_cves_kev        ON cves(is_kev) WHERE is_kev = TRUE;
CREATE INDEX IF NOT EXISTS idx_cves_epss       ON cves(epss DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS idx_cves_cvss3      ON cves(cvss3_score DESC NULLS LAST);

-- Full-text search index (GIN)
CREATE INDEX IF NOT EXISTS idx_cves_fts ON cves
    USING GIN (to_tsvector('english', id || ' ' || description || ' ' || summary));

-- pgvector semantic search index (ivfflat for fast approximate nearest neighbor)
CREATE INDEX IF NOT EXISTS idx_cves_embedding ON cves
    USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 100);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_cves_embedding;
DROP INDEX IF EXISTS idx_cves_fts;
DROP INDEX IF EXISTS idx_cves_cvss3;
DROP INDEX IF EXISTS idx_cves_epss;
DROP INDEX IF EXISTS idx_cves_kev;
DROP INDEX IF EXISTS idx_cves_source;
DROP INDEX IF EXISTS idx_cves_modified;
DROP INDEX IF EXISTS idx_cves_published;
DROP INDEX IF EXISTS idx_cves_severity;
DROP TABLE IF EXISTS cves;
DROP EXTENSION IF EXISTS vector;
-- +goose StatementEnd
