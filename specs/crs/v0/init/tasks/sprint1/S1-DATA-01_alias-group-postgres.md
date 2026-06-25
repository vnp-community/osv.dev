# S1-DATA-01 — Thêm PostgreSQL AliasGroup Repo (data-service)

## ✅ Execution Status: COMPLETED
- **Executed**: 2026-06-13
- **Result**: `go build` + `go vet` PASSED
- **Files Created**:
  - `migrations/005_create_alias_groups.up.sql`
  - `migrations/005_create_alias_groups.down.sql`
  - `internal/infra/persistence/postgres/alias_group_repo.go`
  - `internal/config/storage_config.go`
- **Key Decision**: Used 2-table design mirroring Firestore (alias_groups + alias_group_members)
- **Activation**: Set `ALIAS_GROUP_BACKEND=postgres` to switch from Firestore

## Metadata
- **Task ID**: S1-DATA-01
- **Service**: data-service
- **Sprint**: 1 (P0)
- **Ước tính**: 2-3 giờ
- **Dependencies**: Không có
- **Spec nguồn**: `specs/develop/02_data-service-upgrade.md` § "P0 — Thêm: PostgreSQL AliasGroup Repo"

## Context

```bash
# Đọc Firestore implementation để hiểu interface:
cat services/data-service/internal/infra/persistence/firestore/alias_group_repo.go

# Đọc domain interface:
cat services/data-service/internal/domain/repository/alias_group_repo.go

# Đọc PostgreSQL KEV repo để hiểu pattern:
cat services/data-service/internal/infra/persistence/postgres/kev_repo.go

# Xem migration files hiện có:
ls services/data-service/migrations/
```

## Goal

1. Tạo migration SQL cho bảng `alias_groups`
2. Tạo PostgreSQL implementation của `AliasGroupRepository`
3. Thêm config env var để chọn backend (Firestore vs PostgreSQL)

## Files to Create

### File 1: `services/data-service/migrations/005_create_alias_groups.up.sql`

```sql
-- 005_create_alias_groups.up.sql
-- Thêm table để lưu CVE alias groups, thay thế dần Firestore
-- Sử dụng IF NOT EXISTS để idempotent

CREATE TABLE IF NOT EXISTS alias_groups (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    primary_id  VARCHAR(30) NOT NULL UNIQUE,   -- canonical CVE ID (e.g., CVE-2024-12345)
    aliases     TEXT[]      NOT NULL DEFAULT '{}',  -- other IDs in this group
    sources     TEXT[]      NOT NULL DEFAULT '{}',  -- which sources reported this alias
    confirmed   BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- GIN index cho array search (tìm "is CVE-X an alias of something?")
CREATE INDEX IF NOT EXISTS idx_alias_groups_aliases
    ON alias_groups USING GIN(aliases);

CREATE INDEX IF NOT EXISTS idx_alias_groups_primary
    ON alias_groups(primary_id);

-- Auto-update updated_at
CREATE OR REPLACE FUNCTION update_alias_groups_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER alias_groups_updated_at
    BEFORE UPDATE ON alias_groups
    FOR EACH ROW
    EXECUTE FUNCTION update_alias_groups_updated_at();
```

### File 2: `services/data-service/migrations/005_create_alias_groups.down.sql`

```sql
DROP TRIGGER IF EXISTS alias_groups_updated_at ON alias_groups;
DROP FUNCTION IF EXISTS update_alias_groups_updated_at();
DROP TABLE IF EXISTS alias_groups;
```

### File 3: `services/data-service/internal/infra/persistence/postgres/alias_group_repo.go`

```go
package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/osv/data-service/internal/domain/repository"
	// Adjust import path based on actual domain package location:
	// "github.com/osv/data-service/internal/domain/aggregate" or similar
)

// AliasGroupRow is the DB row representation.
type AliasGroupRow struct {
	ID        string    `db:"id"`
	PrimaryID string    `db:"primary_id"`
	Aliases   []string  `db:"aliases"`
	Sources   []string  `db:"sources"`
	Confirmed bool      `db:"confirmed"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// PostgresAliasGroupRepo implements repository.AliasGroupRepository using PostgreSQL.
type PostgresAliasGroupRepo struct {
	db *pgxpool.Pool
}

// NewAliasGroupRepo creates a new PostgresAliasGroupRepo.
func NewAliasGroupRepo(db *pgxpool.Pool) *PostgresAliasGroupRepo {
	return &PostgresAliasGroupRepo{db: db}
}

// FindByMember finds the alias group containing the given CVE ID.
// The ID can be either the primary or one of the aliases.
func (r *PostgresAliasGroupRepo) FindByMember(ctx context.Context, id string) (*repository.AliasGroup, error) {
	query := `
		SELECT id, primary_id, aliases, sources, confirmed, created_at, updated_at
		FROM alias_groups
		WHERE primary_id = $1
		   OR $1 = ANY(aliases)
		LIMIT 1
	`
	var row AliasGroupRow
	err := r.db.QueryRow(ctx, query, id).Scan(
		&row.ID, &row.PrimaryID, &row.Aliases, &row.Sources,
		&row.Confirmed, &row.CreatedAt, &row.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // not found = nil, nil
		}
		return nil, err
	}
	return rowToAliasGroup(&row), nil
}

// Upsert creates or updates an alias group.
func (r *PostgresAliasGroupRepo) Upsert(ctx context.Context, group *repository.AliasGroup) error {
	query := `
		INSERT INTO alias_groups (primary_id, aliases, sources, confirmed)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (primary_id) DO UPDATE SET
			aliases   = ARRAY(SELECT DISTINCT UNNEST(alias_groups.aliases || EXCLUDED.aliases)),
			sources   = ARRAY(SELECT DISTINCT UNNEST(alias_groups.sources || EXCLUDED.sources)),
			confirmed = alias_groups.confirmed OR EXCLUDED.confirmed,
			updated_at = NOW()
	`
	_, err := r.db.Exec(ctx, query,
		group.PrimaryID,
		group.Aliases,
		group.Sources,
		group.Confirmed,
	)
	return err
}

// Merge merges secondary alias group into primary.
func (r *PostgresAliasGroupRepo) Merge(ctx context.Context, primaryID, secondaryID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Add secondaryID as alias of primary
	query := `
		UPDATE alias_groups
		SET aliases = ARRAY(SELECT DISTINCT UNNEST(aliases || ARRAY[$2::text])),
			updated_at = NOW()
		WHERE primary_id = $1
	`
	if _, err := tx.Exec(ctx, query, primaryID, secondaryID); err != nil {
		return err
	}

	// Move aliases from secondary group to primary
	query2 := `
		UPDATE alias_groups
		SET aliases = (
			SELECT ARRAY(
				SELECT DISTINCT UNNEST(p.aliases || s.aliases || ARRAY[s.primary_id])
			)
			FROM alias_groups p, alias_groups s
			WHERE p.primary_id = $1 AND s.primary_id = $2
		)
		WHERE primary_id = $1
	`
	if _, err := tx.Exec(ctx, query2, primaryID, secondaryID); err != nil {
		return err
	}

	// Delete secondary group (or mark it as merged)
	query3 := `DELETE FROM alias_groups WHERE primary_id = $1`
	if _, err := tx.Exec(ctx, query3, secondaryID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// rowToAliasGroup converts a DB row to domain entity.
// Adjust the return type based on actual domain struct.
func rowToAliasGroup(row *AliasGroupRow) *repository.AliasGroup {
	return &repository.AliasGroup{
		ID:        row.ID,
		PrimaryID: row.PrimaryID,
		Aliases:   row.Aliases,
		Sources:   row.Sources,
		Confirmed: row.Confirmed,
	}
}
```

**Note**: Đọc actual `repository.AliasGroup` struct trước và điều chỉnh field names cho khớp.

### File 4: `services/data-service/internal/config/storage_config.go`

```go
package config

// AliasGroupBackend defines the storage backend for alias groups.
type AliasGroupBackend string

const (
	// AliasGroupBackendFirestore uses Google Firestore (current default).
	AliasGroupBackendFirestore AliasGroupBackend = "firestore"

	// AliasGroupBackendPostgres uses PostgreSQL (new addition).
	AliasGroupBackendPostgres AliasGroupBackend = "postgres"
)

// StorageConfig holds backend selection for each storage domain.
type StorageConfig struct {
	// AliasGroupBackend selects where alias groups are persisted.
	// Env: ALIAS_GROUP_BACKEND (default: "firestore")
	AliasGroupBackend AliasGroupBackend `env:"ALIAS_GROUP_BACKEND" default:"firestore"`
}
```

## Files to Extend

### Extend: `cmd/server/main.go`

Tìm chỗ wire alias_group_repo và thêm switch:

```go
// Đọc config:
storageCfg := config.StorageConfig{} // load from env

// Chọn backend:
var aliasGroupRepo domain.AliasGroupRepository // adjust type name
switch storageCfg.AliasGroupBackend {
case config.AliasGroupBackendPostgres:
    aliasGroupRepo = postgres_infra.NewAliasGroupRepo(pgPool)
default:
    // Giữ nguyên Firestore (existing code)
    aliasGroupRepo = firestore_infra.NewAliasGroupRepo(firestoreClient)
}
```

## Verification

```bash
# Run migration:
cd services/data-service
go run ./cmd/migrate/... up  # hoặc lệnh migrate tương đương

# Build check:
go build ./...

# Test insert:
# Set ALIAS_GROUP_BACKEND=postgres
# Run data-service và check logs có dùng postgres repo không
```

## Notes

- Đọc `domain/repository/alias_group_repo.go` để biết chính xác interface methods và types
- Nếu `AliasGroup` struct ở domain layer khác với assumption, điều chỉnh `rowToAliasGroup()`
- Migration sử dụng `IF NOT EXISTS` — an toàn để chạy nhiều lần
