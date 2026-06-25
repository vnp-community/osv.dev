# TASK-SEED-005-A: DB Migration — assets schema (asset-service)

> **Solution:** [SOL-SEED-005](../solutions/SOL-SEED-005-assets-scan.md)  
> **Service:** `services/asset-service`  
> **Depends on:** Không có  
> **Blocking:** TASK-SEED-005-B  
> **Status:** ✅ COMPLETED — 2026-06-19  
> **Files tạo/sửa:**  
> - `services/asset-service/migrations/0001_init.sql` (tồn tại) — đầy đủ schema `osv_asset`, bảng `assets` (INET, GIN index), `asset_vulnerabilities` (FK cascade), `scan_schedules`

## Mục tiêu

Tạo schema và bảng cho asset-service nếu chưa tồn tại. Theo architecture, schema `osv_asset` với INET type và GIN index cho tags.

## Bước 1: Kiểm tra schema hiện có

```bash
# Tìm migrations hiện có trong asset-service
find /Users/binhnt/Lab/sec/cve/osv.dev/services/asset-service \
  -type d -name "migration*" -o -name "*.sql" 2>/dev/null

# Tìm asset entity/model hiện tại
find /Users/binhnt/Lab/sec/cve/osv.dev/services/asset-service \
  -name "*.go" | xargs grep -l "type Asset struct\|osv_asset" 2>/dev/null | head -10

# Kiểm tra nếu đã có table assets
grep -rn "CREATE TABLE.*assets\|osv_asset" \
  /Users/binhnt/Lab/sec/cve/osv.dev/services/asset-service/ 2>/dev/null | head -10
```

## Bước 2: Tạo migration (CHỈ nếu chưa có)

**File:** `services/asset-service/migrations/0001_init.sql` (hoặc đúng số thứ tự)

```sql
-- Migration: init asset schema
CREATE SCHEMA IF NOT EXISTS osv_asset;

CREATE TABLE IF NOT EXISTS osv_asset.assets (
    id            UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    ip_address    INET UNIQUE,
    hostname      VARCHAR(255),
    os            VARCHAR(100),
    mac_address   VARCHAR(17),
    services      JSONB DEFAULT '[]'::JSONB,
    tags          TEXT[] DEFAULT '{}',
    labels        JSONB DEFAULT '{}'::JSONB,
    risk_score    NUMERIC(4,2) DEFAULT 0,
    finding_count INT DEFAULT 0,
    status        VARCHAR(20) DEFAULT 'active'
                  CHECK (status IN ('active','inactive','decommissioned')),
    last_seen_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ DEFAULT NOW(),
    updated_at    TIMESTAMPTZ DEFAULT NOW()
);

-- GIN index cho fast tag filtering
CREATE INDEX IF NOT EXISTS idx_assets_tags 
    ON osv_asset.assets USING GIN(tags);

CREATE INDEX IF NOT EXISTS idx_assets_status 
    ON osv_asset.assets(status);

-- Vulnerabilities injected manually (SEED-005)
CREATE TABLE IF NOT EXISTS osv_asset.asset_vulnerabilities (
    id          UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    asset_id    UUID NOT NULL REFERENCES osv_asset.assets(id) ON DELETE CASCADE,
    cve_id      VARCHAR(50) NOT NULL,
    severity    VARCHAR(10) NOT NULL
                CHECK (severity IN ('critical','high','medium','low','none')),
    cvss        NUMERIC(4,2),
    detected_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_asset_vulns_asset_id 
    ON osv_asset.asset_vulnerabilities(asset_id);

CREATE INDEX IF NOT EXISTS idx_asset_vulns_cve_id 
    ON osv_asset.asset_vulnerabilities(cve_id);

COMMENT ON TABLE osv_asset.assets IS 
    'Network assets registry — created by scans (NATS) or manually via API (SEED-005)';
COMMENT ON TABLE osv_asset.asset_vulnerabilities IS 
    'Vulnerabilities manually injected into assets for seeding';
```

## Acceptance Criteria

-[x] Schema `osv_asset` tồn tại
-[x] Bảng `assets` có INET type cho `ip_address`, UNIQUE constraint
-[x] GIN index cho `tags` column
-[x] Bảng `asset_vulnerabilities` với FK cascade
-[x] SQL không lỗi trên Postgres 16
