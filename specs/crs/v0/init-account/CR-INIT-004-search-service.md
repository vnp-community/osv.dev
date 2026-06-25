# CR-INIT-004 — Khởi tạo search-service

## Mục tiêu

Sau khi chạy init, search-service phải:
1. Kết nối được với Redis (CPE browse cache)
2. OpenSearch index `vulnerabilities` được tạo (nếu dùng OpenSearch backend)
3. Backend được chọn đúng theo `SEARCH_BACKEND` env var

## Biến môi trường (đọc từ `.env`)

| Biến | Mô tả | Default |
|------|-------|---------|
| `REDIS_ADDR` | Redis host:port | `localhost:6379` |
| `REDIS_PASSWORD` | Redis password | `` (rỗng) |
| `OPENSEARCH_URL` | OpenSearch URL | `http://localhost:9200` |
| `OPENSEARCH_INDEX` | Tên index | `vulnerabilities` |
| `SEARCH_BACKEND` | Backend: `opensearch` \| `postgres` \| `mongo` \| `auto` | `auto` |
| `SEARCH_HTTP_PORT` | HTTP port | `8082` |
| `SEARCH_GRPC_PORT` | gRPC port | `50056` |
| `NATS_URL` | NATS URL (để nhận vuln events) | `nats://localhost:4222` |

## Các thay đổi cần thực hiện

### [NEW] `services/search-service/scripts/init.sh`

```bash
#!/usr/bin/env bash
# search-service bootstrap script
# 1. Kiểm tra kết nối Redis
# 2. Tạo OpenSearch index (nếu dùng opensearch backend)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Load .env
if [ -f "${SCRIPT_DIR}/../../../.env" ]; then
  set -o allexport
  source "${SCRIPT_DIR}/../../../.env"
  set +o allexport
fi

REDIS_ADDR="${REDIS_ADDR:-localhost:6379}"
OPENSEARCH_URL="${OPENSEARCH_URL:-http://localhost:9200}"
OPENSEARCH_INDEX="${OPENSEARCH_INDEX:-vulnerabilities}"
SEARCH_BACKEND="${SEARCH_BACKEND:-auto}"

echo "=== [search-service] Bootstrap Start ==="

# ── Step 1: Verify Redis connection ──────────────────────────────────────
echo "→ [1/2] Checking Redis connection..."
if redis-cli -h "${REDIS_ADDR%%:*}" -p "${REDIS_ADDR##*:}" \
   ${REDIS_PASSWORD:+-a "$REDIS_PASSWORD"} ping 2>/dev/null | grep -q "PONG"; then
  echo "   ✓ Redis connected at ${REDIS_ADDR}"
else
  echo "   ⚠ WARNING: Redis not available at ${REDIS_ADDR}"
  echo "   search-service will start but browse endpoint may not work"
fi

# ── Step 2: Create OpenSearch index ──────────────────────────────────────
echo "→ [2/2] OpenSearch index setup (backend: ${SEARCH_BACKEND})..."

if [ "$SEARCH_BACKEND" = "opensearch" ] || [ "$SEARCH_BACKEND" = "auto" ]; then
  if curl -s "${OPENSEARCH_URL}/_cluster/health" 2>/dev/null | grep -q '"status"'; then
    # Check if index exists
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
      "${OPENSEARCH_URL}/${OPENSEARCH_INDEX}")

    if [ "$HTTP_CODE" = "404" ]; then
      echo "   Creating index: ${OPENSEARCH_INDEX}..."
      curl -s -X PUT "${OPENSEARCH_URL}/${OPENSEARCH_INDEX}" \
        -H 'Content-Type: application/json' \
        -d '{
          "settings": {
            "number_of_shards": 1,
            "number_of_replicas": 0,
            "analysis": {
              "analyzer": {
                "cve_analyzer": {
                  "type": "custom",
                  "tokenizer": "standard",
                  "filter": ["lowercase", "stop"]
                }
              }
            }
          },
          "mappings": {
            "properties": {
              "id":          {"type": "keyword"},
              "cve_id":      {"type": "keyword"},
              "summary":     {"type": "text", "analyzer": "cve_analyzer"},
              "description": {"type": "text", "analyzer": "cve_analyzer"},
              "severity":    {"type": "keyword"},
              "cvss_score":  {"type": "float"},
              "published":   {"type": "date"},
              "modified":    {"type": "date"},
              "cpe":         {"type": "keyword"},
              "vendor":      {"type": "keyword"},
              "product":     {"type": "keyword"}
            }
          }
        }' | jq -c '{acknowledged: .acknowledged}' 2>/dev/null || true
      echo "   ✓ Index created: ${OPENSEARCH_INDEX}"
    else
      echo "   ✓ Index already exists: ${OPENSEARCH_INDEX}"
    fi
  else
    if [ "$SEARCH_BACKEND" = "auto" ]; then
      echo "   ℹ OpenSearch not available, will fallback to postgres backend"
    else
      echo "   ⚠ WARNING: OpenSearch not available at ${OPENSEARCH_URL}"
    fi
  fi
else
  echo "   ℹ Backend '${SEARCH_BACKEND}' — no OpenSearch index needed"
fi

echo ""
echo "=== [search-service] Bootstrap Complete ==="
echo "   HTTP: :${SEARCH_HTTP_PORT:-8082}"
echo "   gRPC: :${SEARCH_GRPC_PORT:-50056}"
echo "   Backend: ${SEARCH_BACKEND}"
echo ""
echo "Test:"
echo "  curl http://localhost:${SEARCH_HTTP_PORT:-8082}/health"
```

### [MODIFY] `services/search-service/cmd/server/main.go`

Cập nhật để đọc đúng tên biến và thêm Redis password:

```go
// Thay:
redisAddr := envOr("REDIS_ADDR", "localhost:6379")
redisClient := redis.NewClient(&redis.Options{Addr: redisAddr})

// Thành:
redisOpts := &redis.Options{
    Addr:     envOr("REDIS_ADDR", "localhost:6379"),
    Password: envOr("REDIS_PASSWORD", ""),
    DB:       0,
}
redisClient := redis.NewClient(redisOpts)

// Thêm HTTP port từ env:
httpPort := envOr("SEARCH_HTTP_PORT", envOr("HTTP_PORT", "8082"))
grpcPort := envOr("SEARCH_GRPC_PORT", envOr("GRPC_PORT", "50056"))
```

### [MODIFY] `services/search-service/internal/factory/factory.go` (nếu tồn tại)

Đảm bảo `FromEnv()` đọc đúng:

```go
// SEARCH_BACKEND env var → backend selector
func FromEnv() Backend {
    return Backend(envOr("SEARCH_BACKEND", "auto"))
}
```

## Acceptance Criteria

- [ ] `services/search-service/scripts/init.sh` tồn tại và executable
- [ ] Script không fail khi Redis hoặc OpenSearch chưa sẵn sàng (graceful warning)
- [ ] Khi OpenSearch available, index `vulnerabilities` được tạo với đúng mapping
- [ ] `GET /health` trả về 200
- [ ] `GET /browse/vendors` trả về danh sách (có thể rỗng nếu chưa có dữ liệu)
- [ ] Service start được với `SEARCH_BACKEND=auto`

## Kiểm tra nhanh

```bash
# 1. Init
./services/search-service/scripts/init.sh

# 2. Start
cd services/search-service
REDIS_ADDR=localhost:6379 SEARCH_BACKEND=auto ./server

# 3. Test
curl http://localhost:8082/health
curl http://localhost:8082/browse/vendors
```
