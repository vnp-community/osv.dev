-- Migration 002: DefectDojo RBAC extensions for auth-service
-- Applies on top of 001_initial_schema.sql (existing OSV auth schema).

BEGIN;

-- Extend the users table with DefectDojo-specific fields.
ALTER TABLE users ADD COLUMN IF NOT EXISTS first_name VARCHAR(150) NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_name VARCHAR(150) NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_staff BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_superuser BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_locked BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS force_password_change BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS global_role_id SMALLINT
    CHECK (global_role_id IS NULL OR global_role_id IN (1, 2, 3, 4, 5));

-- Scoped role assignments (user ↔ role ↔ scope).
CREATE TABLE IF NOT EXISTS role_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id SMALLINT NOT NULL CHECK (role_id IN (1, 2, 3, 4, 5)),
    scope VARCHAR(20) NOT NULL CHECK (scope IN ('global', 'product_type', 'product')),
    resource_id UUID,            -- NULL for global scope
    resource_type VARCHAR(50),   -- "product" | "product_type" | "" for global
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, scope, resource_id)
);

CREATE INDEX IF NOT EXISTS idx_role_assignments_user
    ON role_assignments(user_id);
CREATE INDEX IF NOT EXISTS idx_role_assignments_resource
    ON role_assignments(resource_id) WHERE resource_id IS NOT NULL;

-- SSO provider configurations (SAML, LDAP, OIDC).
CREATE TABLE IF NOT EXISTS sso_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type VARCHAR(10) NOT NULL CHECK (type IN ('oidc', 'saml', 'ldap')),
    name VARCHAR(100) NOT NULL UNIQUE,
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    config JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMIT;
