-- services/ai-service/migrations/001_epss_scores.sql
-- EPSS scores table: stores time-series EPSS data per CVE.
-- ADDITIVE: new table, no existing schema modified.

CREATE TABLE IF NOT EXISTS epss_scores (
    id          BIGSERIAL     PRIMARY KEY,
    cve_id      VARCHAR(30)   NOT NULL,
    score       DOUBLE PRECISION NOT NULL,
    percentile  DOUBLE PRECISION NOT NULL DEFAULT 0,
    fetched_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

-- Index for CVE-specific history
CREATE INDEX IF NOT EXISTS idx_epss_cve_id
    ON epss_scores(cve_id, fetched_at DESC);

-- Latest EPSS view (simplifies queries)
CREATE OR REPLACE VIEW epss_latest AS
    SELECT DISTINCT ON (cve_id)
        cve_id, score, percentile, fetched_at
    FROM epss_scores
    ORDER BY cve_id, fetched_at DESC;
