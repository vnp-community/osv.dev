-- Migration: 006_user_invitations.sql (was numbered 007 in spec, using 006 to follow sequence)
-- TASK-HC-014: Invitation token table for user invitation flow with email.

CREATE TABLE IF NOT EXISTS user_invitations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email       VARCHAR(255) NOT NULL,
    token       VARCHAR(128) NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '48 hours'),
    accepted_at TIMESTAMPTZ,
    invited_by  UUID,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_invitations_token   ON user_invitations(token) WHERE accepted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_invitations_user    ON user_invitations(user_id);
CREATE INDEX IF NOT EXISTS idx_invitations_expires ON user_invitations(expires_at) WHERE accepted_at IS NULL;
