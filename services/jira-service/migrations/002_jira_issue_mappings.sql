-- Migration 002: JIRA issue mappings (finding ↔ JIRA issue)

BEGIN;

CREATE TABLE IF NOT EXISTS jira_issue_mappings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    finding_id UUID NOT NULL UNIQUE,  -- 1 finding → 1 JIRA issue
    jira_configuration_id UUID REFERENCES jira_configurations(id) ON DELETE SET NULL,
    jira_id VARCHAR(100) NOT NULL,    -- internal JIRA issue ID
    jira_key VARCHAR(100) NOT NULL,   -- display key e.g. "PROJ-123"
    jira_url TEXT NOT NULL,           -- browser URL to the issue
    jira_status VARCHAR(100),         -- "To Do" | "In Progress" | "Done"
    jira_priority VARCHAR(50),
    synced BOOLEAN NOT NULL DEFAULT TRUE,
    last_sync_at TIMESTAMPTZ,
    sync_error TEXT,                  -- last sync error message if any
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_jira_mapping_key ON jira_issue_mappings(jira_key);
CREATE INDEX IF NOT EXISTS idx_jira_mapping_finding ON jira_issue_mappings(finding_id);
CREATE INDEX IF NOT EXISTS idx_jira_mapping_config ON jira_issue_mappings(jira_configuration_id)
    WHERE jira_configuration_id IS NOT NULL;

-- Table to track bidirectional sync events
CREATE TABLE IF NOT EXISTS jira_sync_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mapping_id UUID NOT NULL REFERENCES jira_issue_mappings(id),
    direction VARCHAR(10) NOT NULL CHECK (direction IN ('push', 'pull')),
    status VARCHAR(20) NOT NULL CHECK (status IN ('success', 'failed', 'skipped')),
    error_message TEXT,
    synced_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_jira_sync_mapping ON jira_sync_log(mapping_id, synced_at DESC);

COMMIT;
