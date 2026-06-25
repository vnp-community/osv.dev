-- Migration: SEED-004 — triage_entries table
-- Stores persistent CVE triage decisions (user-authored, client-seeded)
-- These override the in-memory VEX-based triage from entity.TriageEntry.
-- Schema: osv_cves (same as existing CVE tables)

CREATE TABLE IF NOT EXISTS osv_cves.triage_entries (
    id             UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    cve_id         VARCHAR(50)  NOT NULL,
    remarks        VARCHAR(30)  NOT NULL
                   CHECK (remarks IN ('NewFound','Unexplored','Confirmed','Mitigated','FalsePositive','NotAffected')),
    comments       TEXT,
    justification  VARCHAR(255),
    response       TEXT[]       DEFAULT '{}',
    triaged_by     UUID         NOT NULL,
    triaged_at     TIMESTAMPTZ  DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  DEFAULT NOW(),
    UNIQUE(cve_id)
);

CREATE INDEX IF NOT EXISTS idx_triage_entries_cve_id ON osv_cves.triage_entries(cve_id);
CREATE INDEX IF NOT EXISTS idx_triage_entries_remarks ON osv_cves.triage_entries(remarks);
CREATE INDEX IF NOT EXISTS idx_triage_entries_triaged_by ON osv_cves.triage_entries(triaged_by);
