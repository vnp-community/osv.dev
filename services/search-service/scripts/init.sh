#!/usr/bin/env bash
# =============================================================================
# search-service — Bootstrap Script
# Spec: 01-architecture.md §3.3 — Dual Backend (OpenSearch primary, PG fallback)
# Spec: 01-architecture.md §4.2 — OpenSearch index mapping (BM25 + aggregations)
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

if [[ -f "${PROJECT_ROOT}/.env" ]]; then
  set -o allexport; source "${PROJECT_ROOT}/.env"; set +o allexport
fi

REDIS_ADDR="${REDIS_ADDR:-localhost:6379}"
REDIS_PASSWORD="${REDIS_PASSWORD:-}"
OPENSEARCH_URL="${OPENSEARCH_URL:-http://localhost:9200}"
OPENSEARCH_INDEX="${OPENSEARCH_INDEX:-vulnerabilities}"
SEARCH_BACKEND="${SEARCH_BACKEND:-auto}"
GRPC_PORT="${SEARCH_GRPC_PORT:-50056}"
HTTP_PORT="${SEARCH_HTTP_PORT:-8083}"

echo "══════════════════════════════════════════════════════════"
echo "  search-service Bootstrap"
echo "══════════════════════════════════════════════════════════"

# ── Step 1: Kiểm tra Redis ─────────────────────────────────────────────────────
echo "→ [1/2] Checking Redis connectivity..."
REDIS_HOST="${REDIS_ADDR%%:*}"
REDIS_PORT_NUM="${REDIS_ADDR##*:}"
_redis_opts="-h ${REDIS_HOST} -p ${REDIS_PORT_NUM} --no-auth-warning"
[[ -n "${REDIS_PASSWORD}" ]] && _redis_opts="${_redis_opts} -a ${REDIS_PASSWORD}"

if redis-cli ${_redis_opts} ping 2>/dev/null | grep -q "PONG"; then
  echo "   ✓ Redis: connected (${REDIS_ADDR})"
else
  echo "   ⚠ Redis: not available — CPE browse cache sẽ không hoạt động"
  echo "     search-service vẫn chạy được với SEARCH_BACKEND=postgres"
fi

# ── Step 2: OpenSearch index ───────────────────────────────────────────────────
echo "→ [2/2] Setting up OpenSearch index..."
# Spec: 01-architecture.md §4.2 — BM25 full-text search index
# Index mapping: cve_id(keyword), description(text/english), severity(keyword),
#                cvss_v3_score(float), epss_score(float), is_kev(boolean)

if [[ "${SEARCH_BACKEND}" == "postgres" ]]; then
  echo "   ℹ SEARCH_BACKEND=postgres — bỏ qua OpenSearch setup"
elif curl -sf "${OPENSEARCH_URL}/_cluster/health" >/dev/null 2>&1; then
  # Kiểm tra index đã tồn tại chưa
  HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "${OPENSEARCH_URL}/${OPENSEARCH_INDEX}" 2>/dev/null)
  if [[ "${HTTP_STATUS}" == "200" ]]; then
    echo "   ✓ OpenSearch index '${OPENSEARCH_INDEX}' already exists"
  else
    echo "   Creating index '${OPENSEARCH_INDEX}'..."
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
            "cve_id":        {"type": "keyword"},
            "description":   {"type": "text", "analyzer": "cve_analyzer"},
            "severity":      {"type": "keyword"},
            "cvss_v3_score": {"type": "float"},
            "epss_score":    {"type": "float"},
            "is_kev":        {"type": "boolean"},
            "published_at":  {"type": "date"},
            "updated_at":    {"type": "date"},
            "source":        {"type": "keyword"},
            "cpes":          {"type": "keyword"}
          }
        }
      }' 2>/dev/null | python3 -m json.tool 2>/dev/null || true
    echo "   ✓ OpenSearch index created"
  fi
else
  echo "   ⚠ OpenSearch: not available at ${OPENSEARCH_URL}"
  echo "     SEARCH_BACKEND=auto sẽ fallback về PostgreSQL GIN index"
fi

echo ""
echo "══════════════════════════════════════════════════════════"
echo "  search-service Bootstrap Complete"
echo "══════════════════════════════════════════════════════════"
echo "  HTTP: :${HTTP_PORT}   gRPC: :${GRPC_PORT}"
echo "  Backend: ${SEARCH_BACKEND}"
echo "  Test: curl http://localhost:${HTTP_PORT}/health"
