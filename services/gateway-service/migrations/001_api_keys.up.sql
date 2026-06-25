-- migrations/001_api_keys.up.sql
-- TASK-GCV-008 / CR-GCV-008: Create api_keys table for gateway-service API key authentication.
-- Idempotent: uses IF NOT EXISTS throughout.

CREATE TABLE IF NOT EXISTS api_keys (
    id              TEXT        PRIMARY KEY,
    key_hash        TEXT        NOT NULL UNIQUE,   -- SHA-256(plaintext_key), never store plain key
    owner_id        TEXT        NOT NULL,
    description     TEXT        NOT NULL DEFAULT '',
    scopes          TEXT[]      NOT NULL DEFAULT '{}',
    rate_limit      INT         DEFAULT NULL,      -- req/min override; NULL = global tier default
    last_used_at    TIMESTAMPTZ DEFAULT NULL,
    expires_at      TIMESTAMPTZ DEFAULT NULL,      -- NULL = no expiry
    is_active       BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Fast hash-based lookup (primary auth path)
CREATE UNIQUE INDEX IF NOT EXISTS idx_api_keys_hash
    ON api_keys(key_hash);

-- Owner listing (management API)
CREATE INDEX IF NOT EXISTS idx_api_keys_owner_active
    ON api_keys(owner_id)
    WHERE is_active = TRUE;

COMMENT ON TABLE api_keys IS 'API keys issued for programmatic access to the OSV platform.';
COMMENT ON COLUMN api_keys.key_hash IS 'SHA-256 hex-encoded hash of the plain API key. Plain text is never stored.';
COMMENT ON COLUMN api_keys.scopes IS 'Permission scopes: cve:read, kev:read, webhook:write, sync:admin, read:all';
COMMENT ON COLUMN api_keys.rate_limit IS 'Requests per minute override. NULL = use global default for tier.';

-- Down (rollback):
-- DROP TABLE IF EXISTS api_keys;
