-- 001_extensions.sql
-- PostgreSQL extensions cần thiết cho OpenVulnScan
-- Run trước tất cả các migration khác

-- UUID generation (PostgreSQL 14+: gen_random_uuid() đã built-in, không cần extension)
-- Nhưng một số services dùng uuid-ossp nên install an toàn
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- CITEXT: case-insensitive text (dùng cho email)
CREATE EXTENSION IF NOT EXISTS "citext";

-- pgvector: vector similarity search (dùng cho CVE embedding search)
CREATE EXTENSION IF NOT EXISTS "vector";

-- pg_trgm: fuzzy text search
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- btree_gin: cho GIN indexes trên non-JSONB types
CREATE EXTENSION IF NOT EXISTS "btree_gin";

-- ── Schemas ──────────────────────────────────────────────────────────────────
-- Mỗi service có schema riêng để tránh naming conflicts
CREATE SCHEMA IF NOT EXISTS auth;
CREATE SCHEMA IF NOT EXISTS scan;
CREATE SCHEMA IF NOT EXISTS agent;
CREATE SCHEMA IF NOT EXISTS asset;
CREATE SCHEMA IF NOT EXISTS cve;
CREATE SCHEMA IF NOT EXISTS report;
