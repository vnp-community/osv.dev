# ✅ COMPLETED — TASK-DD-011 — Scan Service DB Migrations

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-DD-011 |
| **Service** | `scan-service` |
| **CR** | CR-DD-002 |
| **Phase** | 1 — Foundation |
| **Priority** | 🔴 High |
| **Prerequisites** | — (độc lập) |
| **Estimated effort** | 0.5 ngày |

## Context

Tạo migrations cho `test_imports` table (import history) trong scan-service.

## Working Directory

```
/Users/binhnt/Lab/sec/cve/osv.dev/services/scan-service/migrations/
```

## Files to Create

```
migrations/
└── 002_test_imports.sql
```

## Implementation

### `002_test_imports.sql`

```sql
-- test_imports: records each scan import/reimport operation
CREATE TABLE IF NOT EXISTS test_imports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    test_id UUID NOT NULL,
    import_type VARCHAR(20) NOT NULL
        CHECK (import_type IN ('import', 'reimport')),
    version VARCHAR(100),
    branch_tag VARCHAR(255),
    build_id VARCHAR(255),
    commit_hash VARCHAR(255),
    new_findings INTEGER DEFAULT 0,
    closed_findings INTEGER DEFAULT 0,
    reactivated INTEGER DEFAULT 0,
    untouched INTEGER DEFAULT 0,
    scan_file_key TEXT,          -- MinIO object key for the raw scan file
    import_settings JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_test_imports_test_id
    ON test_imports(test_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_test_imports_created
    ON test_imports(created_at DESC);
```

## Acceptance Criteria

- [x] Migration idempotent (chạy 2 lần không fail)
- [x] `test_imports` table created với đủ columns
- [x] `import_type` CHECK constraint rejects values không phải 'import'|'reimport'
- [x] Index trên `(test_id, created_at DESC)` tạo thành công

## Implementation Status: ✅ DONE

> `services/scan-service/migrations/002_test_imports.sql` — đã tạo đầy đủ
> Table: `test_imports` với id, test_id, import_type (CHECK), version, branch_tag, build_id, commit_hash,
> new_findings, closed_findings, reactivated, untouched, scan_file_key, import_settings (JSONB)
> Indexes: `idx_test_imports_test_id` (test_id, created_at DESC), `idx_test_imports_created` (created_at DESC)
