-- migrations/001_create_kev_entries.up.sql
-- KEV service: CISA Known Exploited Vulnerabilities catalog

CREATE TABLE IF NOT EXISTS kev_entries (
    cve_id              TEXT        PRIMARY KEY,
    vendor_project      TEXT        NOT NULL DEFAULT '',
    product             TEXT        NOT NULL DEFAULT '',
    vulnerability_name  TEXT        NOT NULL DEFAULT '',
    date_added          DATE,
    due_date            DATE,
    notes               TEXT        NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_kev_date_added ON kev_entries(date_added DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS idx_kev_vendor     ON kev_entries(vendor_project);
CREATE INDEX IF NOT EXISTS idx_kev_product    ON kev_entries(product);
