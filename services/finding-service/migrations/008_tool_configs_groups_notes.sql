-- Migration 008: Tool configurations + finding groups + notes

BEGIN;

-- ── Tool Configurations ────────────────────────────────────────────────────────
-- Stores external tool credentials (encrypted). Used by build_server_id, etc.

CREATE TABLE tool_configurations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(200) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    tool_type VARCHAR(100) NOT NULL,  -- e.g. 'GitHub', 'Jira', 'Jenkins'
    url TEXT NOT NULL DEFAULT '',
    auth_type VARCHAR(50) NOT NULL DEFAULT 'api_key'
        CONSTRAINT tool_auth_type_check CHECK (auth_type IN ('api_key', 'http_basic', 'ssh', 'bearer')),
    username VARCHAR(200) NOT NULL DEFAULT '',
    password_enc TEXT NOT NULL DEFAULT '',  -- AES-256-GCM encrypted
    api_key_enc TEXT NOT NULL DEFAULT '',   -- AES-256-GCM encrypted
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_tool_configs_type ON tool_configurations(tool_type);

-- Now wire FKs from engagements
ALTER TABLE engagements
    ADD CONSTRAINT fk_engagement_build_server
        FOREIGN KEY (build_server_id) REFERENCES tool_configurations(id) ON DELETE SET NULL,
    ADD CONSTRAINT fk_engagement_orch_engine
        FOREIGN KEY (orchestration_engine_id) REFERENCES tool_configurations(id) ON DELETE SET NULL;

-- ── Finding Groups ─────────────────────────────────────────────────────────────
-- Groups related findings (same vulnerability across multiple components)

CREATE TABLE finding_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(500) NOT NULL,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    -- Aggregate stats (denormalized for performance)
    finding_count INT NOT NULL DEFAULT 0,
    -- Auto-computed on upsert via trigger (see trigger below)
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_finding_groups_product ON finding_groups(product_id);

-- ── Notes ──────────────────────────────────────────────────────────────────────
-- Analyst notes attached to findings/engagements/tests

CREATE TABLE notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id UUID NOT NULL,
    entry TEXT NOT NULL,
    private BOOLEAN NOT NULL DEFAULT FALSE,
    note_type VARCHAR(100),
    date_time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    edited BOOLEAN NOT NULL DEFAULT FALSE,
    editor_id UUID,
    edit_time TIMESTAMPTZ,
    -- Polymorphic FK (only one of these is set)
    finding_id    UUID REFERENCES findings(id) ON DELETE CASCADE,
    engagement_id UUID REFERENCES engagements(id) ON DELETE CASCADE,
    test_id       UUID REFERENCES tests(id) ON DELETE CASCADE,
    -- Must reference exactly one entity
    CONSTRAINT notes_single_entity_check CHECK (
        (finding_id IS NOT NULL)::int +
        (engagement_id IS NOT NULL)::int +
        (test_id IS NOT NULL)::int = 1
    )
);
CREATE INDEX idx_notes_finding    ON notes(finding_id) WHERE finding_id IS NOT NULL;
CREATE INDEX idx_notes_engagement ON notes(engagement_id) WHERE engagement_id IS NOT NULL;
CREATE INDEX idx_notes_test       ON notes(test_id) WHERE test_id IS NOT NULL;

-- ── Finding → Group association ────────────────────────────────────────────────
-- finding.finding_group_id (already exists in findings table) is the FK
ALTER TABLE findings
    ADD CONSTRAINT fk_findings_group
        FOREIGN KEY (finding_group_id) REFERENCES finding_groups(id) ON DELETE SET NULL;

COMMIT;
