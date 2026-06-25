-- Migration: 003_role_assignments
-- Purpose: Support product-scoped role assignments for SEED-001 API
-- Schema: auth (matching existing 001_initial_schema.sql convention)
-- Run: psql $DATABASE_URL -f 003_role_assignments.sql

SET search_path TO auth;

CREATE TABLE IF NOT EXISTS role_assignments (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id     INT NOT NULL,
    scope       VARCHAR(20) NOT NULL DEFAULT 'global'
                CHECK (scope IN ('global', 'product')),
    resource_id UUID,       -- NULL when scope = 'global'
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    assigned_by UUID REFERENCES users(id) ON DELETE SET NULL,

    -- Unique per user+role+scope+resource (COALESCE handles NULL resource_id)
    CONSTRAINT uq_role_assignments UNIQUE (
        user_id,
        role_id,
        scope,
        COALESCE(resource_id, '00000000-0000-0000-0000-000000000000'::UUID)
    )
);

CREATE INDEX IF NOT EXISTS idx_role_assignments_user_id
    ON role_assignments(user_id);

CREATE INDEX IF NOT EXISTS idx_role_assignments_resource_id
    ON role_assignments(resource_id)
    WHERE resource_id IS NOT NULL;

COMMENT ON TABLE role_assignments IS
    'Product-scoped or global role assignments for SEED-001 RBAC support';
COMMENT ON COLUMN role_assignments.scope IS
    '''global'' = applies to all resources; ''product'' = scoped to resource_id';
COMMENT ON COLUMN role_assignments.resource_id IS
    'UUID of the product or resource. NULL when scope = ''global''';
