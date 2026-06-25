-- +migrate Up
CREATE TABLE IF NOT EXISTS finding_notes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    finding_id  UUID NOT NULL REFERENCES findings(id) ON DELETE CASCADE,
    content     TEXT NOT NULL,
    created_by  VARCHAR(255) NOT NULL,  -- user email
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_finding_notes_finding_id ON finding_notes(finding_id);

-- +migrate Down
DROP TABLE IF EXISTS finding_notes;
