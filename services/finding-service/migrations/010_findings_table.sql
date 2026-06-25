-- Migration 010: findings table (was missing from earlier migrations)
-- Updated: 2026-06-25 — sync risk_acceptances with Go repo code
--   Added: name, product_id, notes, proof_file_key, reactivate_expired,
--          reactivate_note_text, restart_sla_on_reactivation, is_expired
--   Added: risk_acceptance_findings junction table
--   Changed: finding_id made optional (Go uses junction table)

BEGIN;

CREATE TABLE IF NOT EXISTS findings (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title               TEXT NOT NULL,
    description         TEXT NOT NULL DEFAULT '',
    mitigation          TEXT NOT NULL DEFAULT '',
    impact              TEXT NOT NULL DEFAULT '',
    "references"        TEXT NOT NULL DEFAULT '',
    severity            TEXT NOT NULL DEFAULT 'Low',
    numerical_severity  NUMERIC(5,2) NOT NULL DEFAULT 0.0,
    cve                 TEXT NOT NULL DEFAULT '',
    cwe                 TEXT NOT NULL DEFAULT '',
    vuln_id_from_tool   TEXT NOT NULL DEFAULT '',
    cvss_v3             TEXT NOT NULL DEFAULT '',
    cvss_v3_score       NUMERIC(4,1),
    active              BOOLEAN NOT NULL DEFAULT TRUE,
    verified            BOOLEAN NOT NULL DEFAULT FALSE,
    false_positive      BOOLEAN NOT NULL DEFAULT FALSE,
    duplicate           BOOLEAN NOT NULL DEFAULT FALSE,
    out_of_scope        BOOLEAN NOT NULL DEFAULT FALSE,
    is_mitigated        BOOLEAN NOT NULL DEFAULT FALSE,
    risk_accepted       BOOLEAN NOT NULL DEFAULT FALSE,
    date                DATE,
    mitigated_at        TIMESTAMPTZ,
    mitigated_by_id     UUID,
    sla_expiration_date DATE,
    test_id             UUID NOT NULL REFERENCES tests(id) ON DELETE CASCADE,
    engagement_id       UUID NOT NULL REFERENCES engagements(id) ON DELETE CASCADE,
    product_id          UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    component_name      TEXT NOT NULL DEFAULT '',
    component_version   TEXT NOT NULL DEFAULT '',
    service             TEXT NOT NULL DEFAULT '',
    file_path           TEXT NOT NULL DEFAULT '',
    line_number         INT,
    hash_code           TEXT NOT NULL DEFAULT '',
    tags                TEXT[] NOT NULL DEFAULT '{}',
    inherited_tags      TEXT[] NOT NULL DEFAULT '{}',
    last_status_update  TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_findings_test_id       ON findings(test_id);
CREATE INDEX IF NOT EXISTS idx_findings_engagement_id ON findings(engagement_id);
CREATE INDEX IF NOT EXISTS idx_findings_product_id    ON findings(product_id);
CREATE INDEX IF NOT EXISTS idx_findings_hash_code     ON findings(hash_code);
CREATE INDEX IF NOT EXISTS idx_findings_severity      ON findings(severity);
CREATE INDEX IF NOT EXISTS idx_findings_active        ON findings(active);
CREATE INDEX IF NOT EXISTS idx_findings_cve           ON findings(cve) WHERE cve != '';

-- Finding notes table
CREATE TABLE IF NOT EXISTS finding_notes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    finding_id  UUID NOT NULL REFERENCES findings(id) ON DELETE CASCADE,
    author_id   UUID NOT NULL,
    note        TEXT NOT NULL,
    private     BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_finding_notes_finding_id ON finding_notes(finding_id);

-- Finding groups table
CREATE TABLE IF NOT EXISTS finding_groups (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    test_id     UUID NOT NULL REFERENCES tests(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_finding_groups_test_id ON finding_groups(test_id);

-- ─── Risk Acceptances ──────────────────────────────────────────────────────────
-- Synced with: internal/infra/postgres/risk_acceptance_repo.go
-- Go INSERT columns: id, name, product_id, accepted_by_id, expiration_date, notes,
--                    proof_file_key, reactivate_expired, reactivate_note_text,
--                    restart_sla_on_reactivation, is_expired, created_at, updated_at
-- NOTE: finding_id column kept for backward compat (nullable) — actual links via
--       risk_acceptance_findings junction table.
CREATE TABLE IF NOT EXISTS risk_acceptances (
    id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                        TEXT NOT NULL DEFAULT '',
    product_id                  UUID REFERENCES products(id) ON DELETE SET NULL,
    accepted_by_id              UUID NOT NULL,
    expiration_date             DATE,
    notes                       TEXT NOT NULL DEFAULT '',
    note                        TEXT NOT NULL DEFAULT '',      -- legacy column kept for compat
    proof_file_key              TEXT NOT NULL DEFAULT '',
    reactivate_expired          BOOLEAN NOT NULL DEFAULT FALSE,
    reactivate_note_text        TEXT NOT NULL DEFAULT '',
    restart_sla_on_reactivation BOOLEAN NOT NULL DEFAULT FALSE,
    is_expired                  BOOLEAN NOT NULL DEFAULT FALSE,
    accepted_severity           VARCHAR(50) NOT NULL DEFAULT '',
    -- Legacy column: nullable (Go code now uses junction table for many-to-many)
    finding_id                  UUID REFERENCES findings(id) ON DELETE CASCADE,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_risk_acceptances_product_id  ON risk_acceptances(product_id);
CREATE INDEX IF NOT EXISTS idx_risk_acceptances_finding_id  ON risk_acceptances(finding_id);
CREATE INDEX IF NOT EXISTS idx_risk_acceptances_is_expired  ON risk_acceptances(is_expired);
CREATE INDEX IF NOT EXISTS idx_risk_acceptances_expiry      ON risk_acceptances(expiration_date) WHERE is_expired = FALSE;

-- Junction table: risk_acceptance_findings (many-to-many)
-- Matches: internal/infra/postgres/risk_acceptance_repo.go → AddFinding / loadFindingIDs
CREATE TABLE IF NOT EXISTS risk_acceptance_findings (
    risk_acceptance_id UUID NOT NULL REFERENCES risk_acceptances(id) ON DELETE CASCADE,
    finding_id         UUID NOT NULL REFERENCES findings(id) ON DELETE CASCADE,
    PRIMARY KEY (risk_acceptance_id, finding_id)
);
CREATE INDEX IF NOT EXISTS idx_raf_acceptance ON risk_acceptance_findings(risk_acceptance_id);
CREATE INDEX IF NOT EXISTS idx_raf_finding    ON risk_acceptance_findings(finding_id);

COMMIT;
