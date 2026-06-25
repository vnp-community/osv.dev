-- Migration: 002_add_epss_index.up.sql
-- TASK-GCV-006 / CR-GCV-002: Add EPSS columns and indexes for filter/sort support.
-- Idempotent: uses ALTER TABLE ... ADD COLUMN IF NOT EXISTS and CREATE INDEX IF NOT EXISTS.

-- Ensure EPSS scoring columns exist
ALTER TABLE cves ADD COLUMN IF NOT EXISTS epss            NUMERIC(8,7) DEFAULT NULL;
ALTER TABLE cves ADD COLUMN IF NOT EXISTS epss_percentile NUMERIC(8,7) DEFAULT NULL;
ALTER TABLE cves ADD COLUMN IF NOT EXISTS epss_updated_at TIMESTAMPTZ  DEFAULT NULL;

-- Descending index for EPSS sort and range filtering (WHERE epss IS NOT NULL ensures partial index)
CREATE INDEX IF NOT EXISTS idx_cves_epss
    ON cves(epss DESC NULLS LAST)
    WHERE epss IS NOT NULL;

-- Down (rollback):
-- DROP INDEX IF EXISTS idx_cves_epss;
-- ALTER TABLE cves DROP COLUMN IF EXISTS epss_updated_at;
-- ALTER TABLE cves DROP COLUMN IF EXISTS epss_percentile;
-- ALTER TABLE cves DROP COLUMN IF EXISTS epss;
