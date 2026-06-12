-- CVE sync service: sync_jobs table

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

CREATE INDEX IF NOT EXISTS idx_sync_jobs_source      ON sync_jobs(source);
CREATE INDEX IF NOT EXISTS idx_sync_jobs_started_at  ON sync_jobs(started_at DESC NULLS LAST);
