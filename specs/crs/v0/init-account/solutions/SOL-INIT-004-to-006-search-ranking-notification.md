# SOL-INIT-004 — Giải Pháp: Khởi Tạo search-service

> **CR tham chiếu**: [CR-INIT-004](../CR-INIT-004-search-service.md)  
> **Kiến trúc cơ sở**: `specs/01-architecture.md §3.3`, `§4.2`, `specs/02-technical-design.md §11`

## Phân Tích Code Hiện Tại

```
services/search-service/
├── cmd/server/main.go
│   ├── REDIS_ADDR  ← đọc đúng ✓
│   ├── HTTP_PORT   ← thiếu SEARCH_ prefix ✗
│   └── GRPC_PORT   ← thiếu SEARCH_ prefix ✗
└── internal/
    └── (search backend factory)
```

**Kiến trúc từ spec**:
- Spec §3.3: "Dual Backend Architecture — OpenSearch primary, PostgreSQL GIN fallback"
- Spec §4.2: OpenSearch index `cves` với mapping cụ thể (cve_id, description, severity, epss_score, is_kev...)
- Spec §11.1: `SearchUseCase` — try OpenSearch first, fallback to PostgreSQL
- Redis: dùng cho browse/CPE cache (`osv:cpe_dict`)

**Vấn đề:**
1. `SEARCH_HTTP_PORT`, `SEARCH_GRPC_PORT` chưa được đọc (chỉ dùng `HTTP_PORT`, `GRPC_PORT`)
2. OpenSearch index mapping từ spec khác với CR-INIT-004 (index tên là `cves` trong spec, `vulnerabilities` trong CR)
3. Không có init script
4. REDIS_PASSWORD không được đọc vào redis.Options

## Files cần tạo/sửa

### [NEW] `services/search-service/scripts/init.sh`

```bash
#!/usr/bin/env bash
# =============================================================================
# search-service — Bootstrap Script
# Spec: 01-architecture.md §3.3 (Dual Backend: OpenSearch primary, PG fallback)
# Spec: 01-architecture.md §4.2 (OpenSearch index mapping)
# Tech: 02-technical-design.md §11 (SearchUseCase dual backend logic)
# =============================================================================
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

if [[ -f "${PROJECT_ROOT}/.env" ]]; then
  set -o allexport; source "${PROJECT_ROOT}/.env"; set +o allexport
fi

REDIS_HOST="${REDIS_HOST:-localhost}"
REDIS_PORT="${REDIS_PORT:-6379}"
REDIS_ADDR="${REDIS_ADDR:-${REDIS_HOST}:${REDIS_PORT}}"
REDIS_PASSWORD="${REDIS_PASSWORD:-}"

OPENSEARCH_URL="${OPENSEARCH_URL:-http://localhost:9200}"
# Spec §4.2 dùng index name "cves", CR dùng "vulnerabilities"
# Giữ configurable để compat cả 2
OPENSEARCH_INDEX="${OPENSEARCH_INDEX:-vulnerabilities}"
SEARCH_BACKEND="${SEARCH_BACKEND:-auto}"
SEARCH_HTTP_PORT="${SEARCH_HTTP_PORT:-8083}"
SEARCH_GRPC_PORT="${SEARCH_GRPC_PORT:-50056}"

echo "══════════════════════════════════════════════════════════"
echo "  search-service Bootstrap"
echo "══════════════════════════════════════════════════════════"
echo "  Backend:   ${SEARCH_BACKEND}"
echo "  OpenSearch: ${OPENSEARCH_URL}"

# ── Step 1: Verify Redis ──────────────────────────────────────────────────
echo "→ [1/2] Kiểm tra Redis (dùng cho CPE browse cache)..."
# Spec §4.3: Key pattern osv:cpe_dict — 24h TTL cho CPE dictionary cache

_redis_host="${REDIS_ADDR%%:*}"
_redis_port="${REDIS_ADDR##*:}"
_redis_opts="-h ${_redis_host} -p ${_redis_port}"
[[ -n "${REDIS_PASSWORD}" ]] && _redis_opts="${_redis_opts} -a ${REDIS_PASSWORD}"

if redis-cli ${_redis_opts} ping 2>/dev/null | grep -q "PONG"; then
  echo "   ✓ Redis connected at ${REDIS_ADDR}"
else
  echo "   ⚠ Redis unavailable at ${REDIS_ADDR}"
  echo "   Browse endpoint (/browse/vendors, /browse/products) sẽ bị ảnh hưởng"
fi

# ── Step 2: OpenSearch index ──────────────────────────────────────────────
echo "→ [2/2] OpenSearch index setup..."

# Spec §4.2 — mapping từ architecture document:
# cve_id: keyword, description: text/english, severity: keyword
# cvss_v3_score: float, epss_score: float, is_kev: boolean
# vendors: keyword, products: keyword, cwe_ids: keyword, published_at: date

OS_AVAILABLE=false
if curl -s --max-time 5 "${OPENSEARCH_URL}/_cluster/health" 2>/dev/null | grep -q '"status"'; then
  OS_AVAILABLE=true
fi

if [[ "${SEARCH_BACKEND}" == "opensearch" ]] || \
   [[ "${SEARCH_BACKEND}" == "auto" && "${OS_AVAILABLE}" == "true" ]]; then
  
  if [[ "${OS_AVAILABLE}" == "false" ]]; then
    echo "   ✗ OpenSearch không available tại ${OPENSEARCH_URL}"
    echo "   Đặt SEARCH_BACKEND=postgres để bỏ qua OpenSearch"
    exit 1
  fi
  
  # Check/create index
  HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "${OPENSEARCH_URL}/${OPENSEARCH_INDEX}")
  
  if [[ "${HTTP_CODE}" == "404" ]]; then
    echo "   Tạo index: ${OPENSEARCH_INDEX}..."
    
    # Mapping từ spec §4.2
    curl -s -X PUT "${OPENSEARCH_URL}/${OPENSEARCH_INDEX}" \
      -H 'Content-Type: application/json' \
      -d '{
        "settings": {
          "number_of_shards": 1,
          "number_of_replicas": 0,
          "index.max_result_window": 50000
        },
        "mappings": {
          "properties": {
            "cve_id":        {"type": "keyword"},
            "description":   {"type": "text", "analyzer": "english"},
            "severity":      {"type": "keyword"},
            "cvss_v3_score": {"type": "float"},
            "cvss_v3_vector":{"type": "keyword", "index": false},
            "epss_score":    {"type": "float"},
            "epss_percentile":{"type": "float"},
            "is_kev":        {"type": "boolean"},
            "is_exploit":    {"type": "boolean"},
            "known_ransomware": {"type": "boolean"},
            "vendors":       {"type": "keyword"},
            "products":      {"type": "keyword"},
            "cpe_list":      {"type": "keyword"},
            "cwe_ids":       {"type": "keyword"},
            "data_source":   {"type": "keyword"},
            "published_at":  {"type": "date"},
            "modified_at":   {"type": "date"}
          }
        }
      }' | python3 -m json.tool 2>/dev/null | grep -E "acknowledged|status" || true
    
    echo "   ✓ Index created: ${OPENSEARCH_INDEX}"
  else
    echo "   ✓ Index already exists: ${OPENSEARCH_INDEX}"
  fi
  
elif [[ "${SEARCH_BACKEND}" == "auto" ]]; then
  echo "   ℹ OpenSearch không available — fallback sang PostgreSQL GIN index"
  echo "   (Spec §11.1: SearchUseCase fallback logic)"
fi

echo ""
echo "══════════════════════════════════════════════════════════"
echo "  search-service Bootstrap Complete"
echo "══════════════════════════════════════════════════════════"
echo "  HTTP:    :${SEARCH_HTTP_PORT}"
echo "  gRPC:    :${SEARCH_GRPC_PORT}"
echo "  Backend: ${SEARCH_BACKEND}"
echo ""
echo "Test:"
echo "  curl http://localhost:${SEARCH_HTTP_PORT}/health"
echo "  curl 'http://localhost:${SEARCH_HTTP_PORT}/api/v1/cves/search?q=log4j'"
echo "  curl http://localhost:${SEARCH_HTTP_PORT}/browse/vendors"
```

### [MODIFY] `services/search-service/cmd/server/main.go`

```go
// Thêm SEARCH_ prefix cho ports:
httpPort := envOrDefault("SEARCH_HTTP_PORT", envOrDefault("HTTP_PORT", "8083"))
grpcPort := envOrDefault("SEARCH_GRPC_PORT", envOrDefault("GRPC_PORT", "50056"))

// Thêm REDIS_PASSWORD support:
redisOpts := &redis.Options{
    Addr:     envOrDefault("REDIS_ADDR", "localhost:6379"),
    Password: envOrDefault("REDIS_PASSWORD", ""),   // THÊM
    DB:       0,
}
redisClient := redis.NewClient(redisOpts)

// Cập nhật health handler:
mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    fmt.Fprintf(w, `{"status":"ok","service":"search-service","backend":"%s"}`,
        envOrDefault("SEARCH_BACKEND", "auto"))
})
```

## Quan Hệ Với Các Service Khác

```
data-service  ──publish NATS ingestion.cve.synced──►  search-service
                                                       (index CVE vào OpenSearch)
apps/osv (gateway) ──route /v1/ ──► search-service:8083
                   ──route /api/v2/cves/search ──► search-service:8083
```

## Acceptance Criteria

- [ ] `scripts/init.sh` chạy được, graceful khi OpenSearch/Redis unavailable
- [ ] OpenSearch index `vulnerabilities` được tạo với đúng mapping từ spec §4.2
- [ ] `SEARCH_HTTP_PORT`, `SEARCH_GRPC_PORT` được đọc từ env
- [ ] `REDIS_PASSWORD` được đọc vào `redis.Options`
- [ ] `SEARCH_BACKEND=auto` hoạt động: dùng OpenSearch nếu có, fallback PG
- [ ] `GET /health` trả về `{"status":"ok","service":"search-service","backend":"..."}`
- [ ] `GET /browse/vendors` trả về danh sách

## Files Tóm Tắt

| File | Action |
|------|--------|
| `services/search-service/scripts/init.sh` | **[NEW]** |
| `services/search-service/cmd/server/main.go` | **[MODIFY]** SEARCH_ ports, REDIS_PASSWORD |

---

# SOL-INIT-005 — Giải Pháp: Khởi Tạo ranking-service

> **CR tham chiếu**: [CR-INIT-005](../CR-INIT-005-ranking-service.md)  
> **Kiến trúc cơ sở**: `specs/01-architecture.md §2.2` (Port: 8088), `§3.11`

## Phân Tích Code Hiện Tại

```
services/ranking-service/
├── cmd/server/main.go
│   └── PORT env var ← không có RANKING_PORT, không có /health ✗
└── internal/
    └── delivery/http/
        └── (router thiếu /health) ✗
```

## Files cần tạo/sửa

### [NEW] `services/ranking-service/scripts/init.sh`

```bash
#!/usr/bin/env bash
# =============================================================================
# ranking-service — Bootstrap Script
# Spec: 01-architecture.md §2.2 (Port 8088), §3.11 (CPE popularity ranking)
# Storage: MongoDB (cvedb.ranking collection)
# =============================================================================
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

if [[ -f "${PROJECT_ROOT}/.env" ]]; then
  set -o allexport; source "${PROJECT_ROOT}/.env"; set +o allexport
fi

MONGO_URI="${MONGO_URI:-mongodb://localhost:27017}"
MONGO_DB="${MONGO_DB:-cvedb}"
RANKING_PORT="${RANKING_PORT:-8088}"

echo "══════════════════════════════════════════════════════════"
echo "  ranking-service Bootstrap"
echo "══════════════════════════════════════════════════════════"

# ── Step 1: Verify MongoDB ────────────────────────────────────────────────
echo "→ [1/2] Kiểm tra MongoDB..."
MONGO_CMD=""
command -v mongosh &>/dev/null && MONGO_CMD="mongosh"
command -v mongo   &>/dev/null && [[ -z "${MONGO_CMD}" ]] && MONGO_CMD="mongo"

if [[ -n "${MONGO_CMD}" ]]; then
  if ${MONGO_CMD} --quiet --eval "db.runCommand({ping:1}).ok" "${MONGO_URI}" 2>/dev/null | grep -q "1"; then
    echo "   ✓ MongoDB connected at ${MONGO_URI}"
  else
    echo "   ⚠ MongoDB không available tại ${MONGO_URI}"
    echo "   ranking-service sẽ thử kết nối lại khi start"
  fi
else
  echo "   ℹ mongosh/mongo không có sẵn — bỏ qua MongoDB check"
fi

# ── Step 2: Create indexes ────────────────────────────────────────────────
echo "→ [2/2] Tạo MongoDB indexes..."

MIGRATION_JS="${SERVICE_DIR:-$(dirname "$SCRIPT_DIR")}/scripts/001_ranking_indexes.js"

# Inline index creation
INDEX_JS='
db = db.getSiblingDB("'"${MONGO_DB}"'");

// Unique index trên CPE (primary lookup key)
db.ranking.createIndex(
  { "cpe": 1 },
  { unique: true, name: "ranking_cpe_unique", background: true }
);

// Index trên rank group cho filtering
db.ranking.createIndex(
  { "rank.group": 1 },
  { name: "ranking_group", background: true }
);

// Index trên score cho sorting
db.ranking.createIndex(
  { "rank.score": -1 },
  { name: "ranking_score_desc", background: true }
);

print("Indexes created successfully");
'

if [[ -n "${MONGO_CMD}" ]]; then
  echo "${INDEX_JS}" | ${MONGO_CMD} --quiet "${MONGO_URI}" 2>/dev/null || \
    echo "   (indexes may already exist)"
  echo "   ✓ Indexes ready"
else
  echo "   ⚠ Bỏ qua index creation (mongosh không có sẵn)"
fi

echo ""
echo "══════════════════════════════════════════════════════════"
echo "  ranking-service Bootstrap Complete"
echo "══════════════════════════════════════════════════════════"
echo "  HTTP: :${RANKING_PORT}"
echo "  MongoDB: ${MONGO_URI}/${MONGO_DB}"
echo ""
echo "Test:"
echo "  curl http://localhost:${RANKING_PORT}/health"
```

### [MODIFY] `services/ranking-service/cmd/server/main.go`

```go
// Thêm RANKING_PORT fallback:
port := envDefault("RANKING_PORT", envDefault("PORT", "8088"))

// Thêm /health route:
router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    fmt.Fprintf(w, `{"status":"ok","service":"ranking-service","port":"%s"}`, port)
})
```

## Acceptance Criteria

- [ ] `scripts/init.sh` chạy được, graceful khi MongoDB unavailable
- [ ] MongoDB indexes: `ranking_cpe_unique`, `ranking_group`, `ranking_score_desc`
- [ ] `RANKING_PORT` được đọc từ env (fallback `PORT`)
- [ ] `GET /health` trả về 200

## Files Tóm Tắt

| File | Action |
|------|--------|
| `services/ranking-service/scripts/init.sh` | **[NEW]** |
| `services/ranking-service/cmd/server/main.go` | **[MODIFY]** RANKING_PORT + /health |

---

# SOL-INIT-006 — Giải Pháp: Khởi Tạo notification-service

> **CR tham chiếu**: [CR-INIT-006](../CR-INIT-006-notification-service.md)  
> **Kiến trúc cơ sở**: `specs/01-architecture.md §3.7`, `specs/02-technical-design.md §8`

## Phân Tích Code Hiện Tại

```
services/notification-service/
├── migrations/
│   ├── 001_dd_tables.sql
│   ├── 002_create_jira_integrations.up.sql
│   ├── 003_notification_defectdojo_events.sql
│   ├── 004_globalcve_001_create_webhooks.up.sql
│   ├── 005_notification_rules.up.sql
│   ├── 007_inapp_notifications.up.sql
│   ├── 008_inapp_alerts_extensions.up.sql
│   └── 202606161830_init_notification.sql   ← latest
└── cmd/server/main.go
    ├── DATABASE_URL  ← không support NOTIFICATION_ prefix ✗
    └── HTTP_PORT     ← không support NOTIFICATION_ prefix ✗
```

## Files cần tạo/sửa

### [NEW] `services/notification-service/scripts/init.sh`

```bash
#!/usr/bin/env bash
# =============================================================================
# notification-service — Bootstrap Script
# Spec: 01-architecture.md §3.7 (5-channel dispatch: Email/Slack/Teams/Webhook/In-app)
# Spec: 01-architecture.md §4.4 (NATS consumer: FINDINGS, SLA, KEV streams)
# Tech: 02-technical-design.md §8 (ChannelDispatcher, WebhookSender HMAC)
# =============================================================================
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_DIR="$(dirname "$SCRIPT_DIR")"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

if [[ -f "${PROJECT_ROOT}/.env" ]]; then
  set -o allexport; source "${PROJECT_ROOT}/.env"; set +o allexport
fi

DB_URL="${NOTIFICATION_DATABASE_URL:-${POSTGRES_DSN:-postgres://osv:osv_dev@localhost:5432/osvdb?sslmode=disable}}"
NATS_ENABLED="${NATS_ENABLED:-false}"
NATS_URL="${NATS_URL:-nats://localhost:4222}"
HTTP_PORT="${NOTIFICATION_HTTP_PORT:-8086}"
GRPC_PORT="${NOTIFICATION_GRPC_PORT:-50063}"

echo "══════════════════════════════════════════════════════════"
echo "  notification-service Bootstrap"
echo "══════════════════════════════════════════════════════════"

# ── Step 1: Schema + extensions ───────────────────────────────────────────
echo "→ [1/3] Khởi tạo schema 'notif'..."
psql "${DB_URL}" <<-SQL
  CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
  CREATE SCHEMA IF NOT EXISTS notif;
SQL

# ── Step 2: Apply migrations ──────────────────────────────────────────────
echo "→ [2/3] Applying migrations..."
for sql_file in $(ls "${SERVICE_DIR}/migrations/"*.sql 2>/dev/null | sort -V); do
  fname="$(basename "$sql_file")"
  [[ "$fname" == *.down.sql ]] && continue
  echo "   → ${fname}"
  psql "${DB_URL}" -v ON_ERROR_STOP=0 -f "$sql_file" 2>/dev/null || \
    echo "     (skipped)"
done
echo "   ✓ Migrations applied"

# ── Step 3: NATS check (optional) ─────────────────────────────────────────
echo "→ [3/3] NATS check (NATS_ENABLED=${NATS_ENABLED})..."
# Spec §4.4: Consumer groups: notification-service → FINDINGS, SLA, KEV

if [[ "${NATS_ENABLED}" == "true" ]]; then
  NATS_HOST="${NATS_URL#nats://}"
  NATS_HOST="${NATS_HOST%%/*}"
  if nc -z "${NATS_HOST%%:*}" "${NATS_HOST##*:}" 2>/dev/null; then
    echo "   ✓ NATS available at ${NATS_URL}"
  else
    echo "   ✗ NATS không available — NATS_ENABLED=true yêu cầu NATS running"
    exit 1
  fi
else
  echo "   ℹ NATS_ENABLED=false — service chạy mà không có event consumption"
  echo "   Webhook delivery vẫn hoạt động. Để nhận CVE events, set NATS_ENABLED=true"
fi

echo ""
echo "══════════════════════════════════════════════════════════"
echo "  notification-service Bootstrap Complete"
echo "══════════════════════════════════════════════════════════"
echo "  HTTP:  :${HTTP_PORT}"
echo "  gRPC:  :${GRPC_PORT}"
echo ""
echo "Test:"
echo "  curl http://localhost:${HTTP_PORT}/health"
```

### [MODIFY] `services/notification-service/cmd/server/main.go`

```go
// Thêm NOTIFICATION_ prefix fallbacks:
dbURL := envOr("NOTIFICATION_DATABASE_URL",
    envOr("DATABASE_URL",
        envOr("POSTGRES_DSN", "postgres://osv:osv_dev@localhost:5432/osvdb?sslmode=disable")))

httpPort := envOr("NOTIFICATION_HTTP_PORT", envOr("HTTP_PORT", "8086"))
grpcPort := envOr("NOTIFICATION_GRPC_PORT", envOr("GRPC_PORT", "50063"))

// Thêm /health route:
mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    fmt.Fprintf(w, `{"status":"ok","service":"notification-service"}`)
})
```

## Acceptance Criteria

- [ ] `scripts/init.sh` chạy được, idempotent
- [ ] Schema `notif` tồn tại
- [ ] Webhooks, notification_rules, in_app_alerts tables tồn tại
- [ ] `NATS_ENABLED=false` — service start được dù NATS không chạy
- [ ] `GET /health` trả về 200

## Files Tóm Tắt

| File | Action |
|------|--------|
| `services/notification-service/scripts/init.sh` | **[NEW]** |
| `services/notification-service/cmd/server/main.go` | **[MODIFY]** NOTIFICATION_ prefix + /health |
