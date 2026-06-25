-- Migration 014: Add missing columns discovered during production testing
-- These columns are referenced in finding_repo.go but were absent from the initial schema.

-- findings table: missing enrichment and assignment columns
ALTER TABLE findings ADD COLUMN IF NOT EXISTS epss_score       FLOAT;
ALTER TABLE findings ADD COLUMN IF NOT EXISTS epss_percentile  FLOAT;
ALTER TABLE findings ADD COLUMN IF NOT EXISTS is_kev           BOOLEAN DEFAULT false;
ALTER TABLE findings ADD COLUMN IF NOT EXISTS cvss_score       FLOAT;
ALTER TABLE findings ADD COLUMN IF NOT EXISTS cvss_vector      TEXT;
ALTER TABLE findings ADD COLUMN IF NOT EXISTS duplicate_finding_id UUID;
ALTER TABLE findings ADD COLUMN IF NOT EXISTS effort_for_fixing TEXT DEFAULT 'Low';
ALTER TABLE findings ADD COLUMN IF NOT EXISTS endpoint_id      UUID;
ALTER TABLE findings ADD COLUMN IF NOT EXISTS found_by         TEXT DEFAULT '';
ALTER TABLE findings ADD COLUMN IF NOT EXISTS source           TEXT DEFAULT '';
ALTER TABLE findings ADD COLUMN IF NOT EXISTS source_url       TEXT DEFAULT '';
ALTER TABLE findings ADD COLUMN IF NOT EXISTS assigned_to      UUID;
ALTER TABLE findings ADD COLUMN IF NOT EXISTS assigned_to_name  TEXT DEFAULT '';
ALTER TABLE findings ADD COLUMN IF NOT EXISTS assigned_to_email TEXT DEFAULT '';
ALTER TABLE findings ADD COLUMN IF NOT EXISTS planned_remediation_date    DATE;
ALTER TABLE findings ADD COLUMN IF NOT EXISTS planned_remediation_version TEXT DEFAULT '';
ALTER TABLE findings ADD COLUMN IF NOT EXISTS group_id         UUID;
ALTER TABLE findings ADD COLUMN IF NOT EXISTS asset_id         UUID;
ALTER TABLE findings ADD COLUMN IF NOT EXISTS asset_hostname   TEXT DEFAULT '';
ALTER TABLE findings ADD COLUMN IF NOT EXISTS asset_ip         TEXT DEFAULT '';
ALTER TABLE findings ADD COLUMN IF NOT EXISTS jira_issue_key   TEXT DEFAULT '';
ALTER TABLE findings ADD COLUMN IF NOT EXISTS jira_url         TEXT DEFAULT '';
ALTER TABLE findings ADD COLUMN IF NOT EXISTS product_name     TEXT DEFAULT '';

-- jira_configs: integration config per product
CREATE TABLE IF NOT EXISTS jira_configs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id  UUID REFERENCES products(id) ON DELETE CASCADE,
    server_url  TEXT NOT NULL DEFAULT '',
    username    TEXT NOT NULL DEFAULT '',
    api_token   TEXT NOT NULL DEFAULT '',
    project_key TEXT NOT NULL DEFAULT '',
    enabled     BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- jira_issues: tracks synced Jira tickets per finding
ALTER TABLE jira_issues ADD COLUMN IF NOT EXISTS jira_key  TEXT DEFAULT '';
ALTER TABLE jira_issues ADD COLUMN IF NOT EXISTS jira_id   TEXT DEFAULT '';
ALTER TABLE jira_issues ADD COLUMN IF NOT EXISTS issue_type TEXT DEFAULT 'Bug';

-- capec_patterns: add likelihood column referenced in taxonomy queries
ALTER TABLE capec_patterns ADD COLUMN IF NOT EXISTS likelihood TEXT DEFAULT 'Unknown';
