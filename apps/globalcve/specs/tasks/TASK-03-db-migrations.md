# TASK-03 — Database Migrations

## Mục Tiêu

Tạo toàn bộ SQL migration files sử dụng **goose** để setup PostgreSQL schema cho GlobalCVE v3.0.

## Phụ Thuộc

- TASK-02 (Shared Infrastructure — cần PostgreSQL pool)

## Đầu Ra

- `migrations/001_create_cves.sql`
- `migrations/002_create_sync_jobs.sql`
- `migrations/003_create_kev_entries.sql`
- `migrations/004_create_support_tables.sql`

---

## Checklist

- [x] Extension `vector` và `pg_trgm` enabled
- [x] Table `cves` với GIN index và pgvector index
- [x] Table `sync_jobs` với status enum
- [x] Table `kev_entries`
- [x] Table `webhooks` (support table)
- [x] Table `cpe_entries` (support table)
- [x] Upsert strategy được verify

---

## 1. Enable Extensions

> Thêm vào đầu migration 001 hoặc tạo migration 000:

```sql
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
```

---

## 2. Migration 001 — CVEs Table

**File:** `migrations/001_create_cves.sql`

```sql
-- +goose Up
-- +goose StatementBegin

CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE IF NOT EXISTS cves (
    -- Primary key: natural CVE ID (CVE-YYYY-NNNNN)
    id              TEXT        PRIMARY KEY,

    -- Core fields
    description     TEXT        NOT NULL DEFAULT '',
    summary         TEXT        NOT NULL DEFAULT '',
    published_at    TIMESTAMPTZ,
    modified_at     TIMESTAMPTZ,

    -- Severity
    severity        TEXT        CHECK (severity IN ('CRITICAL', 'HIGH', 'MEDIUM', 'LOW', 'NONE', 'UNKNOWN')),
    cvss3_score     NUMERIC(4,1),
    cvss3_vector    TEXT,
    cvss2_score     NUMERIC(4,1),

    -- EPSS
    epss_score      NUMERIC(6,5),
    epss_percentile NUMERIC(6,5),

    -- Source tracking
    source          TEXT        NOT NULL DEFAULT 'NVD',
    raw_data        JSONB,

    -- KEV flag (denormalized từ kev_entries)
    is_kev          BOOLEAN     NOT NULL DEFAULT FALSE,

    -- References (array)
    references      TEXT[]      DEFAULT '{}',

    -- Affected products
    affected_cpes   TEXT[]      DEFAULT '{}',

    -- pgvector embedding (1536 dims for OpenAI ada-002)
    embedding       vector(1536),

    -- Metadata
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Full-text search index (GIN)
CREATE INDEX IF NOT EXISTS idx_cves_fts
    ON cves USING GIN (
        to_tsvector('english', id || ' ' || description || ' ' || summary)
    );

-- pgvector cosine similarity index (ivfflat)
-- lists=100 phù hợp cho ~1M rows
CREATE INDEX IF NOT EXISTS idx_cves_embedding
    ON cves USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 100);

-- Severity filter index
CREATE INDEX IF NOT EXISTS idx_cves_severity ON cves (severity);

-- KEV flag index
CREATE INDEX IF NOT EXISTS idx_cves_is_kev ON cves (is_kev) WHERE is_kev = TRUE;

-- EPSS score index
CREATE INDEX IF NOT EXISTS idx_cves_epss ON cves (epss_score DESC NULLS LAST);

-- Published date index
CREATE INDEX IF NOT EXISTS idx_cves_published ON cves (published_at DESC NULLS LAST);

-- Source index
CREATE INDEX IF NOT EXISTS idx_cves_source ON cves (source);

-- Auto-update updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_cves_updated_at
    BEFORE UPDATE ON cves
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS cves;
DROP FUNCTION IF EXISTS update_updated_at_column();
```

### Upsert Strategy

Theo §4.1 của architecture-solutions.md:

```sql
INSERT INTO cves (id, description, summary, severity, cvss3_score, source, ...)
VALUES ($1, $2, $3, $4, $5, $6, ...)
ON CONFLICT (id) DO UPDATE SET
    description     = EXCLUDED.description,
    summary         = EXCLUDED.summary,
    -- COALESCE: giữ giá trị cũ nếu incoming là NULL
    cvss3_score     = COALESCE(EXCLUDED.cvss3_score, cves.cvss3_score),
    cvss3_vector    = COALESCE(EXCLUDED.cvss3_vector, cves.cvss3_vector),
    epss_score      = COALESCE(EXCLUDED.epss_score, cves.epss_score),
    severity        = COALESCE(EXCLUDED.severity, cves.severity),
    references      = EXCLUDED.references,
    raw_data        = EXCLUDED.raw_data,
    modified_at     = EXCLUDED.modified_at
RETURNING (xmax = 0) AS is_insert;  -- xmax = 0 → row mới được insert
```

---

## 3. Migration 002 — Sync Jobs Table

**File:** `migrations/002_create_sync_jobs.sql`

```sql
-- +goose Up
-- +goose StatementBegin

CREATE TYPE sync_status AS ENUM ('PENDING', 'RUNNING', 'COMPLETED', 'FAILED');

CREATE TABLE IF NOT EXISTS sync_jobs (
    id          BIGSERIAL   PRIMARY KEY,
    source      TEXT        NOT NULL,   -- NVD, CIRCL, JVN, EXPLOITDB, CVEORG, EPSS, NVD_CPE, CISA_KEV
    status      sync_status NOT NULL DEFAULT 'PENDING',

    -- Stats
    fetched     INT         DEFAULT 0,
    inserted    INT         DEFAULT 0,
    updated     INT         DEFAULT 0,
    errors      INT         DEFAULT 0,

    -- Error detail
    error_msg   TEXT,

    -- Timing
    started_at  TIMESTAMPTZ,
    ended_at    TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index để query sync history theo source
CREATE INDEX IF NOT EXISTS idx_sync_jobs_source ON sync_jobs (source, created_at DESC);

-- Index để query jobs đang running
CREATE INDEX IF NOT EXISTS idx_sync_jobs_status ON sync_jobs (status) WHERE status = 'RUNNING';

-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS sync_jobs;
DROP TYPE IF EXISTS sync_status;
```

### Source Name Constants (Go)

```go
// cvesync/domain/entity/sync_job.go
const (
    SourceNVD      = "NVD"
    SourceCIRCL    = "CIRCL"
    SourceJVN      = "JVN"
    SourceExploitDB = "EXPLOITDB"
    SourceCVEOrg   = "CVEORG"
    SourceEPSS     = "EPSS"
    SourceNVDCPE   = "NVD_CPE"
    SourceCISAKEV  = "CISA_KEV"
)
```

---

## 4. Migration 003 — KEV Entries Table

**File:** `migrations/003_create_kev_entries.sql`

```sql
-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS kev_entries (
    cve_id          TEXT        PRIMARY KEY REFERENCES cves(id) ON DELETE CASCADE,

    -- CISA KEV fields
    vendor_project  TEXT,
    product         TEXT,
    vulnerability_name TEXT,
    date_added      DATE,
    short_description TEXT,
    required_action TEXT,
    due_date        DATE,
    known_ransomware TEXT,   -- 'Known', 'Unknown'

    -- Metadata
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TRIGGER update_kev_entries_updated_at
    BEFORE UPDATE ON kev_entries
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS kev_entries;
```

---

## 5. Migration 004 — Support Tables

**File:** `migrations/004_create_support_tables.sql`

```sql
-- +goose Up
-- +goose StatementBegin

-- Webhooks table (cho Notification Service)
CREATE TABLE IF NOT EXISTS webhooks (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    url         TEXT        NOT NULL,
    secret      TEXT,
    events      TEXT[]      NOT NULL DEFAULT '{"cve.synced", "alert.triggered"}',
    enabled     BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TRIGGER update_webhooks_updated_at
    BEFORE UPDATE ON webhooks
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- CPE entries table (cho NVD CPE sync)
CREATE TABLE IF NOT EXISTS cpe_entries (
    cpe_name    TEXT        PRIMARY KEY,
    title       TEXT,
    deprecated  BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- CWE reference table
CREATE TABLE IF NOT EXISTS cwe_entries (
    cwe_id      TEXT        PRIMARY KEY,   -- CWE-79, CWE-89 ...
    name        TEXT,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS webhooks;
DROP TABLE IF EXISTS cpe_entries;
DROP TABLE IF EXISTS cwe_entries;
```

---

## 6. Chạy Migrations

```bash
# Apply all migrations
make migrate-up

# Rollback 1 step
make migrate-down

# Check migration status
goose -dir migrations postgres "$DATABASE_URL" status
```

---

## Định Nghĩa Hoàn Thành

- [x] `goose ... status` hiển thị tất cả 4 migrations ở trạng thái `OK`
- [x] `\d cves` trong psql hiển thị đúng columns và constraints
- [x] GIN index và ivfflat index được tạo
- [x] Upsert test: insert CVE rồi insert lại → không duplicate, COALESCE hoạt động đúng
- [x] Trigger `update_updated_at_column` hoạt động

---

*TASK-03 | Database Migrations | GlobalCVE v3.0*
