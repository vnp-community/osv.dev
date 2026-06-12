-- JIRA integration initial schema
BEGIN;

CREATE TABLE jira_configurations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID UNIQUE,  -- NULL = global config
    url TEXT NOT NULL,
    username TEXT NOT NULL,
    api_token_encrypted TEXT NOT NULL,  -- AES-256-GCM encrypted, never plaintext
    project_key VARCHAR(50) NOT NULL,
    issue_type VARCHAR(100) NOT NULL DEFAULT 'Bug',
    default_assignee_id VARCHAR(200),
    labels TEXT[] DEFAULT '{}',
    issue_priority JSONB DEFAULT '{"Critical":"Highest","High":"High","Medium":"Medium","Low":"Low"}',
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    webhook_secret TEXT,  -- HMAC secret for incoming JIRA webhooks
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE jira_issue_mappings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    finding_id UUID NOT NULL UNIQUE,
    jira_config_id UUID NOT NULL REFERENCES jira_configurations(id),
    jira_issue_id VARCHAR(50) NOT NULL,
    jira_key VARCHAR(50) NOT NULL,
    jira_status VARCHAR(100),
    jira_url TEXT,
    last_synced_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_jira_mappings_finding ON jira_issue_mappings(finding_id);
CREATE INDEX idx_jira_mappings_key ON jira_issue_mappings(jira_key);

COMMIT;
