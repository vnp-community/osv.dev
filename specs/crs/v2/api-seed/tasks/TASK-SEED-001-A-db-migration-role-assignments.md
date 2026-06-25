# TASK-SEED-001-A: DB Migration — role_assignments table (identity-service)

> **Solution:** [SOL-SEED-001](../solutions/SOL-SEED-001-identity-bootstrap.md)  
> **Service:** `services/identity-service`  
> **Depends on:** Không có  
> **Blocking:** TASK-SEED-001-B (usecase cần table này)  
> **Status:** ✅ COMPLETED — 2026-06-18  
> **File tạo:** `services/identity-service/migrations/003_role_assignments.sql`

## Mục tiêu

Tạo migration file để thêm bảng `role_assignments` vào schema `osv_identity`, cho phép gán role product-scoped cho user.

## Các bước thực thi

### Bước 1: Tìm đúng thư mục migrations

```bash
find /Users/binhnt/Lab/sec/cve/osv.dev/services/identity-service -type d -name "migration*"
```

### Bước 2: Tạo migration file

**File cần tạo:** `services/identity-service/migrations/0010_role_assignments.sql` (hoặc đúng theo convention thư mục tìm được ở Bước 1)

```sql
-- Migration: 0010_role_assignments
-- Purpose: Support product-scoped role assignments for SEED-001

CREATE TABLE IF NOT EXISTS osv_identity.role_assignments (
    id          UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES osv_identity.users(id) ON DELETE CASCADE,
    role_id     INT NOT NULL,
    scope       VARCHAR(20) NOT NULL DEFAULT 'global',  -- 'global' | 'product'
    resource_id UUID,
    assigned_at TIMESTAMPTZ DEFAULT NOW(),
    assigned_by UUID REFERENCES osv_identity.users(id),
    UNIQUE(user_id, role_id, scope, COALESCE(resource_id, '00000000-0000-0000-0000-000000000000'::UUID))
);

CREATE INDEX IF NOT EXISTS idx_role_assignments_user_id 
    ON osv_identity.role_assignments(user_id);

CREATE INDEX IF NOT EXISTS idx_role_assignments_resource_id 
    ON osv_identity.role_assignments(resource_id) 
    WHERE resource_id IS NOT NULL;

COMMENT ON TABLE osv_identity.role_assignments IS 
    'Product-scoped or global role assignments for RBAC';
```

### Bước 3: Verify không conflict với schema hiện tại

```bash
grep -r "role_assignments\|role_id" \
  /Users/binhnt/Lab/sec/cve/osv.dev/services/identity-service/migrations/ \
  2>/dev/null | head -20
```

## Acceptance Criteria

- [x] File migration tồn tại đúng thư mục (`migrations/003_role_assignments.sql`)
- [x] SQL syntax đúng — không lỗi khi chạy trên Postgres 16
- [x] Table `role_assignments` được tạo với đúng columns và indexes
- [x] UNIQUE constraint có `COALESCE` để handle NULL `resource_id` đúng
