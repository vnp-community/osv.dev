-- Migration 002: test_imports — records each scan import/reimport operation
-- Idempotent (safe to run multiple times via IF NOT EXISTS)

BEGIN;

CREATE TABLE IF NOT EXISTS test_imports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    test_id UUID NOT NULL,
    import_type VARCHAR(20) NOT NULL
        CONSTRAINT test_imports_type_check
        CHECK (import_type IN ('import', 'reimport')),
    version VARCHAR(100),
    branch_tag VARCHAR(255),
    build_id VARCHAR(255),
    commit_hash VARCHAR(255),
    new_findings INTEGER NOT NULL DEFAULT 0,
    closed_findings INTEGER NOT NULL DEFAULT 0,
    reactivated INTEGER NOT NULL DEFAULT 0,
    untouched INTEGER NOT NULL DEFAULT 0,
    scan_file_key TEXT,          -- MinIO/S3 object key for the raw scan file
    import_settings JSONB NOT NULL DEFAULT '{}',
    requestor_user_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_test_imports_test_id
    ON test_imports(test_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_test_imports_created
    ON test_imports(created_at DESC);

COMMIT;
