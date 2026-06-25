-- Migration 002: Row Level Security policies for audit_events

BEGIN;

-- ── Create application role ───────────────────────────────────────────────────
DO $$ BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'audit_app_role') THEN
        CREATE ROLE audit_app_role;
    END IF;
END $$;

-- ── Enable RLS ────────────────────────────────────────────────────────────────
ALTER TABLE audit_events ENABLE ROW LEVEL SECURITY;

-- Superuser bypass (still has full access, needed for admin)
ALTER TABLE audit_events FORCE ROW LEVEL SECURITY;

-- ── Block UPDATE and DELETE at database level ─────────────────────────────────
-- Even if the application code has a bug, the DB will reject any mutations.
CREATE POLICY audit_no_update ON audit_events
    FOR UPDATE USING (FALSE);

CREATE POLICY audit_no_delete ON audit_events
    FOR DELETE USING (FALSE);

-- ── Allow INSERT and SELECT for the application role ─────────────────────────
CREATE POLICY audit_insert ON audit_events
    FOR INSERT TO audit_app_role WITH CHECK (TRUE);

CREATE POLICY audit_select ON audit_events
    FOR SELECT TO audit_app_role USING (TRUE);

-- Grant privileges
GRANT SELECT, INSERT ON audit_events TO audit_app_role;

COMMIT;
