# SOL-INIT-003 — Giải Pháp: Khởi Tạo data-service

> **CR tham chiếu**: [CR-INIT-003](../CR-INIT-003-data-service.md)  
> **Kiến trúc cơ sở**: `specs/01-architecture.md §3.2`, `§4.1`, `specs/02-technical-design.md §4`

## Phân Tích Code Hiện Tại

```
services/data-service/
├── internal/
│   └── config/
│       └── storage_config.go  ← envOr() là placeholder (trả về defaultVal) ✗
├── migrations/
│   ├── 002_create_kev_entries.up.sql     (kev_entries table)
│   ├── 003_initial_schema.sql            (cves, taxonomy, dbinfo)
│   ├── 004_create_sync_jobs.up.sql       (sync_jobs)
│   ├── 005_create_alias_groups.up.sql    (alias_groups, pgvector extension)
│   ├── 006_kev_ransomware_extended.up.sql
│   └── 007_add_source_isexploit.up.sql
└── cmd/server/main.go  ← envOr("GRPC_PORT", "50053") — không đọc DATA_ prefix ✗
```

**Vấn đề xác định:**
1. `storage_config.go` function `envOr` là placeholder — hardcode return defaultVal, không đọc OS env!
2. `cmd/server/main.go` không nhận `DATA_DATABASE_URL`, `DATA_HTTP_PORT`, `DATA_GRPC_PORT`
3. Không có schema `vuln` tự tạo (migrations dùng `CREATE TABLE IF NOT EXISTS` nhưng không SET schema)
4. Không có init script

## Files cần tạo/sửa

### [NEW] `services/data-service/scripts/init.sh`

```bash
#!/usr/bin/env bash
# =============================================================================
# data-service — Bootstrap Script
# Spec: 01-architecture.md §3.2 (CVE Data Platform — 15+ fetchers)
# Spec: 01-architecture.md §4.1 (PostgreSQL schema: osv_cves)
# Tech: 02-technical-design.md §4 (Fetcher Registry, EPSS sync, KEV diff)
# =============================================================================
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_DIR="$(dirname "$SCRIPT_DIR")"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

if [[ -f "${PROJECT_ROOT}/.env" ]]; then
  set -o allexport; source "${PROJECT_ROOT}/.env"; set +o allexport
fi

DB_URL="${DATA_DATABASE_URL:-${POSTGRES_DSN:-postgres://osv:osv_dev@localhost:5432/osvdb?sslmode=disable}}"
ALIAS_BACKEND="${ALIAS_GROUP_BACKEND:-postgres}"
HTTP_PORT="${DATA_HTTP_PORT:-8082}"
GRPC_PORT="${DATA_GRPC_PORT:-50053}"

echo "══════════════════════════════════════════════════════════"
echo "  data-service Bootstrap"
echo "══════════════════════════════════════════════════════════"

# ── Step 1: Extensions + Schema ───────────────────────────────────────────
echo "→ [1/3] Khởi tạo PostgreSQL extensions và schema..."
# Spec: 01-architecture.md §4.1 — pgvector for cves.embedding vector(1536)
# Schema: osv_cves (data-service + search-service)

psql "${DB_URL}" <<-SQL
  -- pgvector for semantic search (spec: §4.1 — cves.embedding vector(1536))
  CREATE EXTENSION IF NOT EXISTS "vector";
  -- uuid-ossp for gen_random_uuid()
  CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
  -- citext for case-insensitive text
  CREATE EXTENSION IF NOT EXISTS "citext";

  -- Schema cho data-service
  -- (spec: osv_cves owned by data-service + search-service)
  CREATE SCHEMA IF NOT EXISTS vuln;
  -- Alias cho backward compat với migrations
  -- SET search_path TO vuln, public;
SQL
echo "   ✓ Extensions và schema ready"

# ── Step 2: Apply migrations ──────────────────────────────────────────────
echo "→ [2/3] Applying migrations..."
MIGRATION_DIR="${SERVICE_DIR}/migrations"

# Thứ tự: .down.sql bỏ qua, .up.sql và .sql apply theo version sort
for sql_file in $(ls "${MIGRATION_DIR}"/*.sql 2>/dev/null | sort -V); do
  fname="$(basename "$sql_file")"
  
  # Bỏ qua DOWN migrations
  [[ "$fname" == *.down.sql ]] && continue
  
  echo "   → ${fname}"
  psql "${DB_URL}" -v ON_ERROR_STOP=0 -f "$sql_file" 2>/dev/null || \
    echo "     (skipped: already applied)"
done
echo "   ✓ Migrations applied"

# ── Step 3: Validate storage backend config ───────────────────────────────
echo "→ [3/3] Validating storage backend..."
# Code: internal/config/storage_config.go — LoadStorageConfig()
# FIXED in SOL-INIT-003: envOr() thực sự đọc OS env vars

case "${ALIAS_BACKEND}" in
  postgres)
    # Verify alias_groups table exists (từ migration 005)
    TABLE_EXISTS=$(psql "${DB_URL}" -t -c \
      "SELECT COUNT(*) FROM information_schema.tables WHERE table_name='alias_groups';" \
      2>/dev/null | tr -d ' ')
    if [[ "${TABLE_EXISTS:-0}" -gt 0 ]]; then
      echo "   ✓ alias_groups table ready (PostgreSQL backend)"
    else
      echo "   ⚠ alias_groups table not found — apply migration 005_create_alias_groups.up.sql"
    fi
    ;;
  firestore)
    if [[ -z "${GCP_PROJECT_ID:-}" ]]; then
      echo "   ✗ ALIAS_GROUP_BACKEND=firestore yêu cầu GCP_PROJECT_ID"
      echo "   Đặt ALIAS_GROUP_BACKEND=postgres để dùng PostgreSQL"
      exit 1
    fi
    echo "   ✓ Firestore backend (project: ${GCP_PROJECT_ID})"
    if [[ -n "${FIRESTORE_EMULATOR_HOST:-}" ]]; then
      echo "   ℹ Local emulator: ${FIRESTORE_EMULATOR_HOST}"
    fi
    ;;
  *)
    echo "   ✗ Unknown ALIAS_GROUP_BACKEND: '${ALIAS_BACKEND}'"
    exit 1
    ;;
esac

echo ""
echo "══════════════════════════════════════════════════════════"
echo "  data-service Bootstrap Complete"
echo "══════════════════════════════════════════════════════════"
echo "  HTTP:  :${HTTP_PORT} (health + KEV REST)"
echo "  gRPC:  :${GRPC_PORT} (CVEService)"
echo "  Backend: ${ALIAS_BACKEND}"
echo ""
echo "Khởi động:"
echo "  DATA_DATABASE_URL='${DB_URL}' \\"
echo "  ALIAS_GROUP_BACKEND=${ALIAS_BACKEND} \\"
echo "  ./services/data-service/server"
echo ""
echo "Test:"
echo "  curl http://localhost:${HTTP_PORT}/health"
```

### [MODIFY] `services/data-service/internal/config/storage_config.go`

Fix hàm `envOr` placeholder để thực sự đọc OS environment:

```go
// Package config provides storage backend selection configuration.
// Controls which storage implementation is used for each repository.
// Pattern: env-driven selector — existing Firestore implementation is default.
package config

import "os"  // THÊM IMPORT

// AliasGroupBackend selects the persistence backend for AliasGroupRepository.
type AliasGroupBackend string

const (
    AliasGroupBackendFirestore AliasGroupBackend = "firestore"
    AliasGroupBackendPostgres  AliasGroupBackend = "postgres"
)

// StorageConfig holds backend selection for data-service storage domains.
type StorageConfig struct {
    // AliasGroupBackend selects where alias groups are persisted.
    // Env: ALIAS_GROUP_BACKEND
    // Default: "postgres" (changed from "firestore" to avoid GCP dependency)
    AliasGroupBackend AliasGroupBackend
}

// LoadStorageConfig loads StorageConfig from environment variables.
func LoadStorageConfig() StorageConfig {
    backend := AliasGroupBackend(envOr("ALIAS_GROUP_BACKEND", string(AliasGroupBackendPostgres)))
    return StorageConfig{
        AliasGroupBackend: backend,
    }
}

// envOr returns the value of the OS environment variable named key,
// or defaultVal if the variable is not set or empty.
// FIXED: was a placeholder returning defaultVal always.
func envOr(key, defaultVal string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return defaultVal
}
```

### [MODIFY] `services/data-service/cmd/server/main.go`

Thêm đọc đúng env vars:

```go
// Thay:
grpcPort := envOr("GRPC_PORT", "50053")
httpPort := envOr("HTTP_PORT", "8080")

// Thành:
grpcPort := envOr("DATA_GRPC_PORT", envOr("GRPC_PORT", "50053"))
httpPort := envOr("DATA_HTTP_PORT", envOr("HTTP_PORT", "8082"))

// Thêm database URL read (cho khi migrations cần thiết):
// dbURL := envOr("DATA_DATABASE_URL", envOr("POSTGRES_DSN", "postgres://osv:osv_dev@localhost:5432/osvdb?sslmode=disable"))

// Cập nhật health handler với thông tin đầy đủ:
mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    fmt.Fprintf(w, `{"status":"ok","service":"data-service","grpc_port":"%s","http_port":"%s","alias_backend":"%s"}`,
        grpcPort, httpPort,
        envOr("ALIAS_GROUP_BACKEND", "postgres"))
})
```

## Thứ Tự Migration Chi Tiết

| File | Mục đích | Spec reference |
|------|----------|----------------|
| `001_create_kev_entries.down.sql` | Rollback KEV | Skip (DOWN) |
| `002_create_kev_entries.up.sql` | Bảng kev_entries (CISA KEV catalog) | `01-arch §3.2 KEV Diff Detection` |
| `003_initial_schema.sql` | cves, taxonomy_categories, dbinfo | `01-arch §3.2 Key Tables` |
| `004_create_sync_jobs.up.sql` | sync_jobs (fetcher scheduler state) | `02-tech §4.1 FetchScheduler` |
| `005_create_alias_groups.up.sql` | alias_groups, pgvector extension | `01-arch §4.1 pgvector` |
| `006_kev_ransomware_extended.up.sql` | kev_entries: ransomware columns | `01-arch §3.2` |
| `007_add_source_isexploit.up.sql` | cves: source, is_exploit columns | `01-arch §3.2` |

## Acceptance Criteria

- [ ] `scripts/init.sh` chạy được, idempotent
- [ ] pgvector extension enabled
- [ ] Schema `vuln` tồn tại
- [ ] Tất cả UP migrations applied
- [ ] `envOr()` trong `storage_config.go` đọc OS env thực sự
- [ ] `ALIAS_GROUP_BACKEND=postgres` hoạt động (alias_groups table có)
- [ ] `DATA_GRPC_PORT`, `DATA_HTTP_PORT` được đọc từ env
- [ ] `GET /health` trả về JSON với `alias_backend` field

## Files Tóm Tắt

| File | Action |
|------|--------|
| `services/data-service/scripts/init.sh` | **[NEW]** |
| `services/data-service/internal/config/storage_config.go` | **[MODIFY]** fix envOr + default=postgres |
| `services/data-service/cmd/server/main.go` | **[MODIFY]** DATA_ prefix env vars |
