-- Migration 013: reports table for finding-service
-- Idempotent via IF NOT EXISTS

BEGIN;

CREATE TABLE IF NOT EXISTS reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL,
    title VARCHAR(300) NOT NULL,
    format VARCHAR(10) NOT NULL
        CONSTRAINT reports_format_check
        CHECK (format IN ('pdf', 'xlsx', 'json', 'csv')),
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CONSTRAINT reports_status_check
        CHECK (status IN ('pending', 'generating', 'completed', 'failed')),
    engagement_id UUID,
    test_id UUID,
    severities TEXT[] NOT NULL DEFAULT '{}',
    active_only BOOLEAN NOT NULL DEFAULT TRUE,
    storage_key TEXT,           -- MinIO/S3 object key
    file_size_bytes BIGINT,
    generated_by UUID,          -- user who requested
    generated_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,     -- auto-delete after 30 days
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_reports_product
    ON reports(product_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_reports_expires
    ON reports(expires_at)
    WHERE status = 'completed';

CREATE INDEX IF NOT EXISTS idx_reports_user
    ON reports(generated_by, created_at DESC);

COMMIT;
