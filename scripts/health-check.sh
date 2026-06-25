#!/usr/bin/env bash
# =============================================================================
# OSV Platform — Health Check Script
# Kiểm tra tất cả services sau khi start
# =============================================================================
source "$(dirname "$0")/../.env" 2>/dev/null || true

GREEN='\033[0;32m'; RED='\033[0;31m'; YELLOW='\033[1;33m'; NC='\033[0m'

echo "════════════════════════════════════════"
echo "  OSV Platform Health Check"
echo "  $(date '+%Y-%m-%d %H:%M:%S')"
echo "════════════════════════════════════════"

PASS=0; FAIL=0

check_http() {
  local name="$1" url="$2"
  resp=$(curl -s --max-time 3 "${url}" 2>/dev/null || echo '{}')
  if echo "${resp}" | python3 -c "import sys,json; d=json.load(sys.stdin); assert d.get('status')=='ok'" 2>/dev/null; then
    echo -e "  ${GREEN}✓${NC} ${name}"
    PASS=$((PASS + 1))
  else
    echo -e "  ${RED}✗${NC} ${name}: not responding (${url})"
    FAIL=$((FAIL + 1))
  fi
}

check_http "identity-service     :${IDENTITY_HTTP_PORT:-9101}" \
           "http://localhost:${IDENTITY_HTTP_PORT:-9101}/health"
check_http "data-service         :${DATA_HTTP_PORT:-8082}" \
           "http://localhost:${DATA_HTTP_PORT:-8082}/health"
check_http "search-service       :${SEARCH_HTTP_PORT:-8083}" \
           "http://localhost:${SEARCH_HTTP_PORT:-8083}/health"
check_http "ranking-service      :${RANKING_PORT:-8088}" \
           "http://localhost:${RANKING_PORT:-8088}/health"
check_http "notification-service :${NOTIFICATION_HTTP_PORT:-8086}" \
           "http://localhost:${NOTIFICATION_HTTP_PORT:-8086}/health"
check_http "apps/osv gateway     :${HTTP_PORT:-8080}" \
           "http://localhost:${HTTP_PORT:-8080}/health"

echo ""
echo "JWKS check (identity-service):"
if curl -s --max-time 3 "http://localhost:${IDENTITY_HTTP_PORT:-9101}/.well-known/jwks.json" \
   2>/dev/null | python3 -c "import sys,json; d=json.load(sys.stdin); assert len(d.get('keys',[]))>0" 2>/dev/null; then
  echo -e "  ${GREEN}✓${NC} JWKS available ($(curl -s "http://localhost:${IDENTITY_HTTP_PORT:-9101}/.well-known/jwks.json" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('keys',[])), 'key(s)')" 2>/dev/null || echo '?'))"
else
  echo -e "  ${YELLOW}⚠${NC} JWKS not available"
fi

echo ""
echo "Admin login test:"
LOGIN_RESP=$(curl -s --max-time 5 -X POST \
  "http://localhost:${IDENTITY_HTTP_PORT:-9101}/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"${INIT_ADMIN_EMAIL:-admin@openvulnscan.io}\",\"password\":\"${INIT_ADMIN_PASSWORD:-Admin@123!ChangeMe}\"}" \
  2>/dev/null || echo '{}')
if echo "${LOGIN_RESP}" | python3 -c "import sys,json; d=json.load(sys.stdin); assert 'access_token' in d" 2>/dev/null; then
  echo -e "  ${GREEN}✓${NC} Login OK — access_token received"
else
  echo -e "  ${YELLOW}⚠${NC} Login not ready (start identity-service first)"
fi

echo ""
echo "════════════════════════════════════════"
echo "  Results: ${PASS} passed, ${FAIL} failed"
echo "════════════════════════════════════════"
[[ ${FAIL} -eq 0 ]]
