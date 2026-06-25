-- Migration 004: DefectDojo import pipeline — scan report storage + import history

BEGIN;

-- ── Import history ─────────────────────────────────────────────────────────────
-- Records each scan import operation's outcome and file reference.

CREATE TABLE test_imports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    test_id UUID NOT NULL,           -- references finding-service tests.id
    engagement_id UUID,              -- references finding-service engagements.id
    product_id UUID,                 -- references finding-service products.id
    import_type VARCHAR(20) NOT NULL DEFAULT 'import'
        CONSTRAINT test_imports_type_check CHECK (import_type IN ('import', 'reimport')),
    scan_type VARCHAR(200) NOT NULL,
    version VARCHAR(200) NOT NULL DEFAULT '',
    branch_tag VARCHAR(200) NOT NULL DEFAULT '',
    build_id VARCHAR(200) NOT NULL DEFAULT '',
    commit_hash VARCHAR(200) NOT NULL DEFAULT '',
    -- Outcome counts
    new_findings INT NOT NULL DEFAULT 0,
    closed_findings INT NOT NULL DEFAULT 0,
    reactivated_findings INT NOT NULL DEFAULT 0,
    untouched_findings INT NOT NULL DEFAULT 0,
    -- File reference
    scan_file_key TEXT NOT NULL DEFAULT '', -- MinIO/S3 object key
    import_settings JSONB NOT NULL DEFAULT '{}',
    requestor_user_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_test_imports_test ON test_imports(test_id);
CREATE INDEX idx_test_imports_product ON test_imports(product_id) WHERE product_id IS NOT NULL;
CREATE INDEX idx_test_imports_created ON test_imports(created_at DESC);

-- ── Scan file metadata ─────────────────────────────────────────────────────────
-- Tracks uploaded scan files in object storage for audit/replay.

CREATE TABLE scan_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    test_import_id UUID REFERENCES test_imports(id) ON DELETE CASCADE,
    object_key TEXT NOT NULL,          -- MinIO/S3 key
    original_filename VARCHAR(500) NOT NULL DEFAULT '',
    content_type VARCHAR(100) NOT NULL DEFAULT 'application/json',
    size_bytes BIGINT NOT NULL DEFAULT 0,
    checksum_sha256 VARCHAR(64),       -- hex SHA-256 of file content
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_scan_files_import ON scan_files(test_import_id);

-- ── Hash-code index for deduplication ─────────────────────────────────────────
-- Enables fast lookup of existing findings by SHA-256 hash code.
-- Stored in scan-service for dedup engine without cross-service DB joins.
-- Keeps only the latest hash→finding_id mapping.

CREATE TABLE finding_hash_cache (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hash_code CHAR(64) NOT NULL,       -- SHA-256 hex
    finding_id UUID NOT NULL,          -- finding-service finding ID
    product_id UUID NOT NULL,
    engagement_id UUID,                -- NULL = product-level dedup
    status VARCHAR(20) NOT NULL DEFAULT 'active'
        CONSTRAINT hash_cache_status_check CHECK (status IN ('active', 'mitigated', 'false_positive', 'risk_accepted', 'out_of_scope', 'duplicate')),
    vuln_id_from_tool TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (hash_code, product_id)
);

CREATE INDEX idx_hash_cache_hash_product ON finding_hash_cache(hash_code, product_id);
CREATE INDEX idx_hash_cache_engagement ON finding_hash_cache(engagement_id) WHERE engagement_id IS NOT NULL;
CREATE INDEX idx_hash_cache_vuln_id ON finding_hash_cache(vuln_id_from_tool) WHERE vuln_id_from_tool != '';

-- ── Dedup configuration per product ───────────────────────────────────────────
-- Product-level dedup settings overriding global defaults.

CREATE TABLE product_dedup_settings (
    product_id UUID PRIMARY KEY,
    algorithm VARCHAR(30) NOT NULL DEFAULT 'hash_code'
        CONSTRAINT dedup_algorithm_check CHECK (algorithm IN ('hash_code', 'unique_id_from_tool', 'legacy')),
    false_positive_history BOOLEAN NOT NULL DEFAULT FALSE,
    max_duplicates INT NOT NULL DEFAULT 10,
    delete_duplicates BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMIT;
