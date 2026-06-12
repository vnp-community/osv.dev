-- 002_auth.sql
-- auth-service schema: users, sessions, oauth_accounts, api_keys, audit_log
-- Source: services/auth-service/migrations/001_initial_schema.sql
-- Thay SET search_path thành schema-qualified names để tương thích với monolith

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

CREATE INDEX IF NOT EXISTS idx_users_email    ON auth.users (email);
CREATE INDEX IF NOT EXISTS idx_users_username ON auth.users (username);

-- ─── sessions ────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS sessions (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id               UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    refresh_token_hash    CHAR(64) NOT NULL,        -- SHA-256 hex of refresh token
    token_family          UUID NOT NULL,             -- for replay attack detection
    ip_address            INET,
    user_agent            TEXT,
    expires_at            TIMESTAMPTZ NOT NULL,
    revoked_at            TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_sessions_token_hash UNIQUE (refresh_token_hash)
);

CREATE INDEX IF NOT EXISTS idx_sessions_user_id     ON auth.sessions (user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_token_hash  ON auth.sessions (refresh_token_hash);
CREATE INDEX IF NOT EXISTS idx_sessions_token_family ON auth.sessions (token_family) WHERE revoked_at IS NULL;

-- ─── oauth_accounts ──────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS oauth_accounts (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    provider     VARCHAR(20) NOT NULL,       -- google|github
    provider_id  VARCHAR(255) NOT NULL,      -- provider's user ID
    email        CITEXT NOT NULL,
    name         VARCHAR(255),
    avatar_url   TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_oauth_accounts UNIQUE (provider, provider_id)
);

CREATE INDEX IF NOT EXISTS idx_oauth_accounts_user_id    ON auth.oauth_accounts (user_id);
CREATE INDEX IF NOT EXISTS idx_oauth_accounts_provider   ON auth.oauth_accounts (provider, provider_id);

-- ─── api_keys ────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS api_keys (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
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

CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON auth.api_keys (user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_prefix  ON auth.api_keys (prefix) WHERE revoked_at IS NULL;

-- ─── audit_log ───────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS audit_log (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID REFERENCES auth.users(id) ON DELETE SET NULL,
    action      VARCHAR(100) NOT NULL,
    ip_address  INET,
    user_agent  TEXT,
    details     JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_log_user_id    ON auth.audit_log (user_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_created_at ON auth.audit_log (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_action     ON auth.audit_log (action);
