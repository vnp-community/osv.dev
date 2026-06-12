-- 011_report.sql
-- report-service schema: reports
-- Source: services/report-service/migrations/001_initial_schema.sql

SET search_path TO report;

CREATE TABLE IF NOT EXISTS reports (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scan_id      UUID NOT NULL,          -- reference scan.scans(id) — no FK cross-schema
    user_id      UUID NOT NULL,          -- reference auth.users(id) — no FK cross-schema
    status       VARCHAR(20) NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending','generating','ready','failed')),
    storage_key  VARCHAR(500),           -- MinIO object key
    download_url TEXT,                   -- pre-signed URL
    error_msg    TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_reports_scan_id  ON report.reports(scan_id);
CREATE INDEX IF NOT EXISTS idx_reports_user_id  ON report.reports(user_id);
CREATE INDEX IF NOT EXISTS idx_reports_status   ON report.reports(status);
CREATE INDEX IF NOT EXISTS idx_reports_created  ON report.reports(created_at DESC);
