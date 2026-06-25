-- Migration 009: Findings table extensions + dedup/state fields + performance indexes
-- Adds state machine fields, dedup fingerprint, CVSS v4, group link, SLA column
-- Adds performance indexes for dedup queries and SLA computations

BEGIN;

-- ── State machine fields ───────────────────────────────────────────────────────
-- These support the 6-state finding lifecycle (active/mitigated/FP/OOS/RA/duplicate)

ALTER TABLE findings
    ADD COLUMN IF NOT EXISTS false_positive BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS out_of_scope BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS risk_accepted BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS last_status_update TIMESTAMPTZ;

-- ── Deduplication fields ───────────────────────────────────────────────────────

ALTER TABLE findings
    ADD COLUMN IF NOT EXISTS hash_code VARCHAR(64),
    ADD COLUMN IF NOT EXISTS vuln_id_from_tool VARCHAR(500),
    ADD COLUMN IF NOT EXISTS duplicate_finding_id UUID REFERENCES findings(id) ON DELETE SET NULL;

-- ── CVSS v4 support ────────────────────────────────────────────────────────────

ALTER TABLE findings
    ADD COLUMN IF NOT EXISTS cvss_v4 VARCHAR(255),
    ADD COLUMN IF NOT EXISTS cvss_v4_score DECIMAL(4,1);

-- ── Group + context ────────────────────────────────────────────────────────────

ALTER TABLE findings
    ADD COLUMN IF NOT EXISTS finding_group_id UUID,
    ADD COLUMN IF NOT EXISTS source_code TEXT,
    ADD COLUMN IF NOT EXISTS inherited_tags TEXT[] NOT NULL DEFAULT '{}';

-- ── SLA date ──────────────────────────────────────────────────────────────────

ALTER TABLE findings
    ADD COLUMN IF NOT EXISTS sla_expiration_date DATE;

-- ── Mitigated-by audit ────────────────────────────────────────────────────────

ALTER TABLE findings
    ADD COLUMN IF NOT EXISTS mitigated_by_id UUID;

-- ── Performance Indexes ────────────────────────────────────────────────────────

-- Dedup: hash_code lookup (primary dedup mechanism)
CREATE INDEX IF NOT EXISTS idx_findings_hash_code ON findings(hash_code)
    WHERE hash_code IS NOT NULL;

-- Dedup: hash_code + product_id (product-scoped dedup)
CREATE INDEX IF NOT EXISTS idx_findings_hash_product ON findings(hash_code, product_id)
    WHERE hash_code IS NOT NULL;

-- Dedup: hash_code + engagement_id (engagement-scoped dedup)
CREATE INDEX IF NOT EXISTS idx_findings_hash_engagement ON findings(hash_code, engagement_id)
    WHERE hash_code IS NOT NULL;

-- Dedup: vuln_id_from_tool + product_id (unique_id algorithm)
CREATE INDEX IF NOT EXISTS idx_findings_vuln_id_product ON findings(vuln_id_from_tool, product_id)
    WHERE vuln_id_from_tool IS NOT NULL;

-- SLA computation: active findings approaching SLA breach
CREATE INDEX IF NOT EXISTS idx_findings_sla_active ON findings(sla_expiration_date, severity)
    WHERE active = TRUE AND sla_expiration_date IS NOT NULL;

-- State-based product dashboard queries
CREATE INDEX IF NOT EXISTS idx_findings_product_active_sev ON findings(product_id, active, severity)
    WHERE active = TRUE;

-- False-positive history: fast FP lookups for auto-marking
CREATE INDEX IF NOT EXISTS idx_findings_fp_hash ON findings(hash_code)
    WHERE false_positive = TRUE AND hash_code IS NOT NULL;

-- Report generation: product findings by severity
CREATE INDEX IF NOT EXISTS idx_findings_product_severity ON findings(product_id, severity, is_mitigated);

-- Test-scoped finding cleanup (CloseOldFindings)
CREATE INDEX IF NOT EXISTS idx_findings_test_active ON findings(test_id, active)
    WHERE active = TRUE;

COMMIT;
