-- migrations/001_create_kev_entries.down.sql
DROP TABLE IF EXISTS kev_entries CASCADE;
DROP INDEX IF EXISTS idx_kev_date_added;
DROP INDEX IF EXISTS idx_kev_vendor;
DROP INDEX IF EXISTS idx_kev_product;
