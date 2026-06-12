-- Consolidated schema for identity-service
-- Generated: 2026-06-12T15:25:30Z
-- DO NOT EDIT — regenerate by running: cat migrations/*.sql > migrations/schema_all.sql


-- ============================================
-- Migration: 001_initial_schema.sql
-- ============================================
-- auth service initial schema
-- Run: psql $DATABASE_URL -f 001_initial_schema.sql

SET search_path TO auth;

CREATE TABLE IF NOT EXISTS users (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email                  CITEXT NOT NULL,
    username               VARCHAR(100) NOT NULL,
    hashed_password        TEXT,                           -- NULL for OAuth-only
    role                   VARCHAR(20) NOT NULL DEFAULT 'user',
    auth_provider          VARCHAR(20) NOT NULL DEFAULT 'local',
    mfa_enabled            BOOLEAN NOT NULL DEFAULT FALSE,
    mfa_totp_secret        TEXT,                           -- encrypted
    is_active              BOOLEAN NOT NULL DEFAULT TRUE,
    is_verified            BOOLEAN NOT NULL DEFAULT FALSE,
    failed_login_attempts  INT NOT NULL DEFAULT 0,
    last_login_at          TIMESTAMPTZ,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_users_email    UNIQUE (email),
    CONSTRAINT uq_users_username UNIQUE (username),
    CONSTRAINT chk_users_role    CHECK (role IN ('admin', 'user', 'readonly', 'agent'))
);

CREATE INDEX idx_users_email    ON users (email);
CREATE INDEX idx_users_username ON users (username);

-- ─── sessions ────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS sessions (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id               UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_hash    CHAR(64) NOT NULL,        -- SHA-256 hex of refresh token
    token_family          UUID NOT NULL,             -- for replay attack detection
    ip_address            INET,
    user_agent            TEXT,
    expires_at            TIMESTAMPTZ NOT NULL,
    revoked_at            TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_sessions_token_hash UNIQUE (refresh_token_hash)
);

CREATE INDEX idx_sessions_user_id     ON sessions (user_id);
CREATE INDEX idx_sessions_token_hash  ON sessions (refresh_token_hash);
CREATE INDEX idx_sessions_token_family ON sessions (token_family) WHERE revoked_at IS NULL;

-- ─── oauth_accounts ──────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS oauth_accounts (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider     VARCHAR(20) NOT NULL,       -- google|github
    provider_id  VARCHAR(255) NOT NULL,      -- provider's user ID
    email        CITEXT NOT NULL,
    name         VARCHAR(255),
    avatar_url   TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_oauth_accounts UNIQUE (provider, provider_id)
);

CREATE INDEX idx_oauth_accounts_user_id    ON oauth_accounts (user_id);
CREATE INDEX idx_oauth_accounts_provider   ON oauth_accounts (provider, provider_id);

-- ─── api_keys ────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS api_keys (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         VARCHAR(255) NOT NULL,
    key_hash     CHAR(64) NOT NULL,          -- SHA-256 hex, never store raw key
    prefix       VARCHAR(12) NOT NULL,       -- "ovs_" + 8 chars, for lookup
    permissions  TEXT[] NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ,
    expires_at   TIMESTAMPTZ,               -- NULL = never expires
    revoked_at   TIMESTAMPTZ,              -- NULL = active

    CONSTRAINT uq_api_keys_hash   UNIQUE (key_hash),
    CONSTRAINT uq_api_keys_prefix UNIQUE (prefix)
);

CREATE INDEX idx_api_keys_user_id ON api_keys (user_id);
CREATE INDEX idx_api_keys_prefix  ON api_keys (prefix) WHERE revoked_at IS NULL;

-- ─── audit_log ───────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS audit_log (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID REFERENCES users(id) ON DELETE SET NULL,
    action      VARCHAR(100) NOT NULL,    -- register|login|logout|token_refresh|api_key_created
    ip_address  INET,
    user_agent  TEXT,
    details     JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_log_user_id    ON audit_log (user_id);
CREATE INDEX idx_audit_log_created_at ON audit_log (created_at DESC);
CREATE INDEX idx_audit_log_action     ON audit_log (action);
