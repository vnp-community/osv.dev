#!/bin/bash
# GlobalCVE v2.0 Smoke Tests
# Usage: ./scripts/smoke-test.sh
# Requires: curl, jq, all services running

set -e

BASE_GW="http://localhost:8080"
BASE_SEARCH="http://localhost:8081"
BASE_SYNC="http://localhost:8082"
BASE_KEV="http://localhost:8083"
BASE_NOTIF="http://localhost:8084"

PASS=0
FAIL=0

check() {
    local name="$1"
    local result="$2"
    if [ "$result" = "0" ]; then
        echo "  ✅ $name"
        PASS=$((PASS + 1))
    else
        echo "  ❌ $name"
        FAIL=$((FAIL + 1))
    fi
}

echo "=== Smoke Test: GlobalCVE v2.0 ==="

echo ""
echo "--- 1. Health Checks ---"
curl -sf "$BASE_GW/health"     > /dev/null 2>&1; check "api-gateway health"      $?
curl -sf "$BASE_SEARCH/health" > /dev/null 2>&1; check "cve-search-service health" $?
curl -sf "$BASE_SYNC/health"   > /dev/null 2>&1; check "cve-sync-service health"  $?
curl -sf "$BASE_KEV/health"    > /dev/null 2>&1; check "kev-service health"        $?
curl -sf "$BASE_NOTIF/health"  > /dev/null 2>&1; check "notification-service health" $?

echo ""
echo "--- 2. KEV Sync ---"
curl -sf -X POST "$BASE_KEV/internal/kev/sync" > /dev/null 2>&1; check "kev sync trigger" $?
sleep 3
KEV_COUNT=$(curl -sf "$BASE_KEV/api/v2/kev/stats" | jq -r '.total // 0' 2>/dev/null || echo "0")
echo "  ℹ️  KEV entries: $KEV_COUNT"

echo ""
echo "--- 3. CVE Count ---"
CVE_COUNT=$(curl -sf "$BASE_SEARCH/internal/cves/count" | jq -r '.count // 0' 2>/dev/null || echo "0")
echo "  ℹ️  CVE count: $CVE_COUNT"

echo ""
echo "--- 4. Search via Gateway ---"
HTTP_CODE=$(curl -sf -o /dev/null -w "%{http_code}" "$BASE_GW/api/v2/cves?limit=1" 2>/dev/null || echo "000")
[ "$HTTP_CODE" = "200" ] && check "gateway /api/v2/cves" 0 || check "gateway /api/v2/cves ($HTTP_CODE)" 1

echo ""
echo "--- 5. KEV via Gateway ---"
HTTP_CODE=$(curl -sf -o /dev/null -w "%{http_code}" "$BASE_GW/api/v2/kev?limit=1" 2>/dev/null || echo "000")
[ "$HTTP_CODE" = "200" ] && check "gateway /api/v2/kev" 0 || check "gateway /api/v2/kev ($HTTP_CODE)" 1

echo ""
echo "--- 6. Auth Enforcement ---"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_GW/api/v2/webhooks" 2>/dev/null || echo "000")
[ "$HTTP_CODE" = "401" ] && check "webhook endpoint enforces auth (401)" 0 || check "webhook should return 401, got $HTTP_CODE" 1

echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="
[ "$FAIL" = "0" ] && echo "✅ All smoke tests passed!" || exit 1
