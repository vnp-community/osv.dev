-- Migration 001: scan-orchestrator initial schema

BEGIN;

CREATE TABLE scan_imports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    test_id UUID NOT NULL,
    engagement_id UUID NOT NULL,
    product_id UUID NOT NULL,
    scan_type VARCHAR(200) NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'pending',
    file_key TEXT,
    options JSONB NOT NULL DEFAULT '{}',
    result JSONB,
    error_msg TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_scan_imports_test ON scan_imports(test_id);
CREATE INDEX idx_scan_imports_product ON scan_imports(product_id);
CREATE INDEX idx_scan_imports_status ON scan_imports(status, created_at DESC);

COMMIT;
