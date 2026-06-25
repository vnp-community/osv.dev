-- migrations/002_totp_pending.sql
-- Additive: adds pending_totp_secret to users table.
-- mfa_totp_secret and mfa_enabled already exist per entity.User definition.

ALTER TABLE users ADD COLUMN IF NOT EXISTS pending_totp_secret VARCHAR(100);
