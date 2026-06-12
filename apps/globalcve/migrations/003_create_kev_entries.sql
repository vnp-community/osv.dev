-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS kev_entries (
    cve_id              TEXT        PRIMARY KEY,
    vendor_project      TEXT        NOT NULL DEFAULT '',
    product             TEXT        NOT NULL DEFAULT '',
    vulnerability_name  TEXT        NOT NULL DEFAULT '',
    short_description   TEXT        NOT NULL DEFAULT '',
    required_action     TEXT        NOT NULL DEFAULT '',
    date_added          DATE,
    due_date            DATE,
    known_ransomware    BOOLEAN     NOT NULL DEFAULT FALSE,
    notes               TEXT        NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_kev_date_added    ON kev_entries(date_added DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS idx_kev_vendor        ON kev_entries(vendor_project);
CREATE INDEX IF NOT EXISTS idx_kev_ransomware    ON kev_entries(known_ransomware) WHERE known_ransomware = TRUE;

-- View for fast KEV statistics
CREATE OR REPLACE VIEW kev_stats AS
SELECT
    COUNT(*) AS total,
    COUNT(*) FILTER (WHERE date_added >= NOW() - '7 days'::interval) AS added_last_7_days,
    COUNT(*) FILTER (WHERE date_added >= NOW() - '30 days'::interval) AS added_last_30_days;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP VIEW IF EXISTS kev_stats;
DROP INDEX IF EXISTS idx_kev_ransomware;
DROP INDEX IF EXISTS idx_kev_vendor;
DROP INDEX IF EXISTS idx_kev_date_added;
DROP TABLE IF EXISTS kev_entries;
-- +goose StatementEnd
