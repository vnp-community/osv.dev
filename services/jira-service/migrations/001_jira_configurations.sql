-- Migration 001: JIRA configurations

BEGIN;

CREATE TABLE IF NOT EXISTS jira_configurations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL UNIQUE,
    url VARCHAR(2048) NOT NULL,
    username VARCHAR(255) NOT NULL,
    password_enc TEXT NOT NULL,       -- AES-256-GCM encrypted token
    project_key VARCHAR(50) NOT NULL,
    issue_type_id VARCHAR(50) NOT NULL DEFAULT '10001',
    issue_type_fields JSONB NOT NULL DEFAULT '{}',
    default_assignee VARCHAR(255),
    find_severity_field VARCHAR(255),
    find_url_field VARCHAR(255),
    push_notes BOOLEAN NOT NULL DEFAULT FALSE,
    push_all_issues BOOLEAN NOT NULL DEFAULT FALSE,
    enable_deduplication BOOLEAN NOT NULL DEFAULT TRUE,
    priority_mapping JSONB NOT NULL DEFAULT '{
        "Critical": "Highest",
        "High": "High",
        "Medium": "Medium",
        "Low": "Low",
        "Info": "Lowest"
    }',
    webhook_secret VARCHAR(255),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_jira_config_product ON jira_configurations(product_id);

COMMIT;
