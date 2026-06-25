CREATE TABLE IF NOT EXISTS scans (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id  UUID,
    test_id     UUID,
    tool        VARCHAR(50) NOT NULL,
    status      VARCHAR(20) NOT NULL DEFAULT 'pending'
                CHECK (status IN ('pending','queued','running','completed','failed','cancelled')),
    targets     JSONB DEFAULT '[]',
    scan_type   VARCHAR(50),
    started_at  TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_by  UUID,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_scans_status     ON scans(status);
CREATE INDEX IF NOT EXISTS idx_scans_product_id ON scans(product_id);
CREATE INDEX IF NOT EXISTS idx_scans_created_at ON scans(created_at DESC);
