# TASK-DD-007 — Finding Service DB Migrations

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-DD-007 |
| **Service** | `finding-service` |
| **CR** | CR-DD-001, CR-DD-004, CR-DD-005 |
| **Phase** | 1 — Foundation |
| **Priority** | 🔴 High |
| **Prerequisites** | TASK-DD-001 (đọc entity specs) |
| **Estimated effort** | 0.5 ngày |

## Context

Tạo tất cả database migrations cần thiết cho finding-service. Migrations phải idempotent (dùng `IF NOT EXISTS`, `IF NOT COLUMN EXISTS`) để chạy an toàn nhiều lần.

## Working Directory

```
/Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/migrations/
```

## Files to Create

```
migrations/
├── 002_product_fields_extension.sql        # CR-DD-001: product extra fields
├── 003_engagement_fields_extension.sql     # CR-DD-001: engagement extra fields
├── 004_test_fields_extension.sql           # CR-DD-001: test extra fields
├── 005_product_members.sql                 # CR-DD-001: RBAC members
├── 006_tool_configurations.sql             # CR-DD-001: external tool creds
├── 007_findings_fields_extension.sql       # CR-DD-004: finding extra fields
├── 008_finding_groups.sql                  # CR-DD-004: finding groups
├── 009_finding_notes.sql                   # CR-DD-004: finding notes/comments
├── 010_finding_files.sql                   # CR-DD-004: file attachments
├── 011_risk_acceptances.sql                # CR-DD-005: risk acceptance
└── 012_findings_indexes.sql                # performance indexes
```

## Implementation Spec

### `002_product_fields_extension.sql`

```sql
-- Mở rộng bảng products với DefectDojo fields
-- Idempotent: sử dụng IF NOT EXISTS checks

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='products' AND column_name='business_criticality') THEN
        ALTER TABLE products ADD COLUMN business_criticality VARCHAR(50);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='products' AND column_name='platform') THEN
        ALTER TABLE products ADD COLUMN platform VARCHAR(50);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='products' AND column_name='lifecycle') THEN
        ALTER TABLE products ADD COLUMN lifecycle VARCHAR(50);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='products' AND column_name='origin') THEN
        ALTER TABLE products ADD COLUMN origin VARCHAR(50);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='products' AND column_name='sla_configuration_id') THEN
        ALTER TABLE products ADD COLUMN sla_configuration_id UUID;
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='products' AND column_name='enable_simple_risk_acceptance') THEN
        ALTER TABLE products ADD COLUMN enable_simple_risk_acceptance BOOLEAN DEFAULT FALSE;
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='products' AND column_name='enable_full_risk_acceptance') THEN
        ALTER TABLE products ADD COLUMN enable_full_risk_acceptance BOOLEAN DEFAULT FALSE;
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='products' AND column_name='enable_product_tag_inheritance') THEN
        ALTER TABLE products ADD COLUMN enable_product_tag_inheritance BOOLEAN DEFAULT FALSE;
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='products' AND column_name='tags') THEN
        ALTER TABLE products ADD COLUMN tags TEXT[] DEFAULT '{}';
    END IF;
END $$;
```

### `003_engagement_fields_extension.sql`

```sql
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='engagements' AND column_name='engagement_type') THEN
        ALTER TABLE engagements ADD COLUMN engagement_type VARCHAR(50) DEFAULT 'Interactive';
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='engagements' AND column_name='status') THEN
        ALTER TABLE engagements ADD COLUMN status VARCHAR(50) DEFAULT 'In Progress';
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='engagements' AND column_name='build_id') THEN
        ALTER TABLE engagements ADD COLUMN build_id VARCHAR(255);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='engagements' AND column_name='commit_hash') THEN
        ALTER TABLE engagements ADD COLUMN commit_hash VARCHAR(255);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='engagements' AND column_name='branch_tag') THEN
        ALTER TABLE engagements ADD COLUMN branch_tag VARCHAR(255);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='engagements' AND column_name='source_code_management_uri') THEN
        ALTER TABLE engagements ADD COLUMN source_code_management_uri TEXT;
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='engagements' AND column_name='deduplication_on_engagement') THEN
        ALTER TABLE engagements ADD COLUMN deduplication_on_engagement BOOLEAN DEFAULT FALSE;
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='engagements' AND column_name='build_server_id') THEN
        ALTER TABLE engagements ADD COLUMN build_server_id UUID;
    END IF;
END $$;
```

### `004_test_fields_extension.sql`

```sql
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='tests' AND column_name='scan_type') THEN
        ALTER TABLE tests ADD COLUMN scan_type VARCHAR(255);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='tests' AND column_name='percent_complete') THEN
        ALTER TABLE tests ADD COLUMN percent_complete INTEGER DEFAULT 0
            CHECK (percent_complete >= 0 AND percent_complete <= 100);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='tests' AND column_name='build_id') THEN
        ALTER TABLE tests ADD COLUMN build_id VARCHAR(255);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='tests' AND column_name='commit_hash') THEN
        ALTER TABLE tests ADD COLUMN commit_hash VARCHAR(255);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='tests' AND column_name='branch_tag') THEN
        ALTER TABLE tests ADD COLUMN branch_tag VARCHAR(255);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='tests' AND column_name='version') THEN
        ALTER TABLE tests ADD COLUMN version VARCHAR(100);
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='tests' AND column_name='tags') THEN
        ALTER TABLE tests ADD COLUMN tags TEXT[] DEFAULT '{}';
    END IF;
END $$;
```

### `005_product_members.sql`

```sql
CREATE TABLE IF NOT EXISTS product_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL,
    user_id UUID NOT NULL,
    role_id VARCHAR(50) NOT NULL
        CHECK (role_id IN ('Owner', 'Maintainer', 'Writer', 'API Importer', 'Reader')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_product_user UNIQUE (product_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_product_members_product ON product_members(product_id);
CREATE INDEX IF NOT EXISTS idx_product_members_user ON product_members(user_id);

CREATE TABLE IF NOT EXISTS product_type_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_type_id UUID NOT NULL,
    user_id UUID NOT NULL,
    role_id VARCHAR(50) NOT NULL
        CHECK (role_id IN ('Owner', 'Maintainer', 'Writer', 'API Importer', 'Reader')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_product_type_user UNIQUE (product_type_id, user_id)
);
```

### `006_tool_configurations.sql`

```sql
CREATE TABLE IF NOT EXISTS tool_configurations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(200) NOT NULL,
    description TEXT,
    tool_type VARCHAR(100),
    url TEXT,
    auth_type VARCHAR(50)
        CHECK (auth_type IN ('api_key', 'http_basic', 'ssh', 'bearer')),
    username VARCHAR(200),
    password_enc TEXT,   -- AES-256-GCM encrypted
    api_key_enc TEXT,    -- AES-256-GCM encrypted
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_tool_config_type ON tool_configurations(tool_type);
```

### `007_findings_fields_extension.sql`

```sql
-- State machine fields
DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns
    WHERE table_name='findings' AND column_name='false_p') THEN
    ALTER TABLE findings ADD COLUMN false_p BOOLEAN DEFAULT FALSE; END IF; END $$;

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns
    WHERE table_name='findings' AND column_name='out_of_scope') THEN
    ALTER TABLE findings ADD COLUMN out_of_scope BOOLEAN DEFAULT FALSE; END IF; END $$;

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns
    WHERE table_name='findings' AND column_name='risk_accepted') THEN
    ALTER TABLE findings ADD COLUMN risk_accepted BOOLEAN DEFAULT FALSE; END IF; END $$;

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns
    WHERE table_name='findings' AND column_name='duplicate_finding_id') THEN
    ALTER TABLE findings ADD COLUMN duplicate_finding_id UUID REFERENCES findings(id); END IF; END $$;

-- Dedup fields
DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns
    WHERE table_name='findings' AND column_name='hash_code') THEN
    ALTER TABLE findings ADD COLUMN hash_code VARCHAR(64); END IF; END $$;

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns
    WHERE table_name='findings' AND column_name='vuln_id_from_tool') THEN
    ALTER TABLE findings ADD COLUMN vuln_id_from_tool VARCHAR(255); END IF; END $$;

-- CVSS v4
DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns
    WHERE table_name='findings' AND column_name='cvss_v4') THEN
    ALTER TABLE findings ADD COLUMN cvss_v4 VARCHAR(255); END IF; END $$;

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns
    WHERE table_name='findings' AND column_name='cvss_v4_score') THEN
    ALTER TABLE findings ADD COLUMN cvss_v4_score DECIMAL(4,1); END IF; END $$;

-- Group + context
DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns
    WHERE table_name='findings' AND column_name='finding_group_id') THEN
    ALTER TABLE findings ADD COLUMN finding_group_id UUID; END IF; END $$;

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns
    WHERE table_name='findings' AND column_name='source_code') THEN
    ALTER TABLE findings ADD COLUMN source_code TEXT; END IF; END $$;

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns
    WHERE table_name='findings' AND column_name='inherited_tags') THEN
    ALTER TABLE findings ADD COLUMN inherited_tags TEXT[] DEFAULT '{}'; END IF; END $$;

-- SLA
DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns
    WHERE table_name='findings' AND column_name='sla_expiration_date') THEN
    ALTER TABLE findings ADD COLUMN sla_expiration_date DATE; END IF; END $$;
```

### `008_finding_groups.sql`

```sql
CREATE TABLE IF NOT EXISTS finding_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    test_id UUID NOT NULL,
    jira_issue_key VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_finding_groups_test ON finding_groups(test_id);
```

### `009_finding_notes.sql`

```sql
CREATE TABLE IF NOT EXISTS finding_notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    finding_id UUID NOT NULL,
    author_id UUID NOT NULL,
    content TEXT NOT NULL CHECK (content <> ''),
    edit_count INTEGER DEFAULT 0,
    is_private BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_finding_notes_finding ON finding_notes(finding_id, created_at DESC);
```

### `010_finding_files.sql`

```sql
CREATE TABLE IF NOT EXISTS finding_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    finding_id UUID NOT NULL,
    filename VARCHAR(255) NOT NULL,
    mime_type VARCHAR(100),
    size_bytes BIGINT,
    storage_key TEXT NOT NULL,
    uploaded_by_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_finding_files_finding ON finding_files(finding_id);
```

### `011_risk_acceptances.sql`

```sql
CREATE TABLE IF NOT EXISTS risk_acceptances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(300) NOT NULL,
    product_id UUID NOT NULL,
    accepted_by_id UUID NOT NULL,
    expiration_date DATE,
    notes TEXT,
    proof_file_key TEXT,
    reactivate_expired BOOLEAN DEFAULT FALSE,
    reactivate_note_text TEXT,
    restart_sla_on_reactivation BOOLEAN DEFAULT FALSE,
    is_expired BOOLEAN DEFAULT FALSE,
    finding_ids UUID[] DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ra_product ON risk_acceptances(product_id);
CREATE INDEX IF NOT EXISTS idx_ra_expiry ON risk_acceptances(expiration_date)
    WHERE is_expired = FALSE AND expiration_date IS NOT NULL;
```

### `012_findings_indexes.sql`

```sql
-- Dedup performance indexes
CREATE INDEX IF NOT EXISTS idx_findings_hash ON findings(hash_code)
    WHERE hash_code IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_findings_vuln_id ON findings(vuln_id_from_tool, product_id)
    WHERE vuln_id_from_tool IS NOT NULL;

-- SLA performance
CREATE INDEX IF NOT EXISTS idx_findings_sla_date ON findings(sla_expiration_date)
    WHERE active = TRUE AND sla_expiration_date IS NOT NULL;

-- State-based queries
CREATE INDEX IF NOT EXISTS idx_findings_active_product ON findings(product_id, active, severity)
    WHERE active = TRUE;

-- Report generation
CREATE INDEX IF NOT EXISTS idx_findings_product_severity ON findings(product_id, severity, is_mitigated);

-- False positive history lookup
CREATE INDEX IF NOT EXISTS idx_findings_fp_hash ON findings(hash_code, false_p)
    WHERE false_p = TRUE AND hash_code IS NOT NULL;
```

## Acceptance Criteria

- [x] Tất cả migrations chạy thành công trên fresh database
- [x] Migrations idempotent: chạy lại 2 lần không fail (IF NOT EXISTS)
- [x] `products` table có tất cả các columns mới (business_criticality, platform, lifecycle, origin, sla_configuration_id, tags)
- [x] `engagements` table có: engagement_type, status, build_id, commit_hash, branch_tag, deduplication_on_engagement
- [x] `tests` table có: scan_type, percent_complete, build_id, commit_hash, branch_tag, version, tags
- [x] `product_members` table với UNIQUE constraint (product_id, user_id) và role CHECK
- [x] `tool_configurations` table với encrypted credential columns
- [x] `findings` table có: hash_code, vuln_id_from_tool, false_positive, out_of_scope, risk_accepted, finding_group_id, sla_expiration_date
- [x] `finding_groups`, `finding_notes`, `finding_files` tables tạo thành công
- [x] `risk_acceptances` table với proper indexes
- [x] All performance indexes created (hash, vuln_id, sla_date, active_product, fp_hash)

## Implementation Status: ✅ DONE

> Migrations 001-009 tại `finding-service/migrations/`:
> - 001-003: base finding, product, engagement/test tables
> - 004-006: risk_acceptance, SLA config, member/SLA fields
> - 007-008: tool_configs, groups, notes
> - **009**: `findings` extensions (false_positive, hash_code, vuln_id, sla_date, duplicate, CVSS v4) + 10 performance indexes
