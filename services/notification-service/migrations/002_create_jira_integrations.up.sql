CREATE TABLE jira_integrations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id  UUID,
    server_url  TEXT NOT NULL,
    project_key VARCHAR(50) NOT NULL,
    issue_type  VARCHAR(50) DEFAULT 'Bug',
    api_token   TEXT,
    auto_create BOOLEAN DEFAULT FALSE,
    auto_sync   BOOLEAN DEFAULT FALSE,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE jira_issues (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    finding_id       UUID NOT NULL,
    integration_id   UUID REFERENCES jira_integrations(id) ON DELETE CASCADE,
    issue_key        VARCHAR(100),
    issue_url        TEXT,
    status           VARCHAR(50),
    synced_at        TIMESTAMPTZ,
    created_at       TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_jira_issues_finding_id ON jira_issues(finding_id);
CREATE INDEX idx_jira_issues_integration_id ON jira_issues(integration_id);
