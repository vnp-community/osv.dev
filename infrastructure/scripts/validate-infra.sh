#!/usr/bin/env bash
# validate-infra.sh — Quick validation of all infrastructure components
# Usage: NATS_URL=nats://... REDIS_URL=redis://... OPENSEARCH_URL=http://... ./validate-infra.sh

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

NATS_URL="${NATS_URL:-nats://localhost:4222}"
REDIS_URL="${REDIS_URL:-redis://localhost:6379}"
OPENSEARCH_URL="${OPENSEARCH_URL:-http://localhost:9200}"
JAEGER_URL="${JAEGER_URL:-http://localhost:16686}"

pass() { echo -e "${GREEN}✓${NC} $1"; }
fail() { echo -e "${RED}✗${NC} $1"; FAILURES=$((FAILURES+1)); }
warn() { echo -e "${YELLOW}⚠${NC} $1"; }

FAILURES=0

echo "=== OSV Infrastructure Validation ==="
echo ""

# ── NATS ────────────────────────────────────────────────────────────────────
echo "--- NATS JetStream ---"
if nats server ping --server="$NATS_URL" 2>/dev/null; then
  pass "NATS server reachable"
  
  # Check OSV-EVENTS stream exists
  if nats stream info OSV-EVENTS --server="$NATS_URL" 2>/dev/null | grep -q "OSV-EVENTS"; then
    pass "Stream OSV-EVENTS exists"
  else
    fail "Stream OSV-EVENTS NOT found — run setup-nats-streams"
  fi
  
  # Check OSV-DLQ stream exists  
  if nats stream info OSV-DLQ --server="$NATS_URL" 2>/dev/null | grep -q "OSV-DLQ"; then
    pass "Stream OSV-DLQ exists"
  else
    fail "Stream OSV-DLQ NOT found"
  fi
  
  # Publish test message
  if nats pub osv.test.ping '{"test": true}' --server="$NATS_URL" 2>/dev/null; then
    pass "NATS pub/sub working"
  else
    fail "NATS pub/sub failed"
  fi
else
  fail "NATS server NOT reachable at $NATS_URL"
fi

echo ""

# ── Redis ────────────────────────────────────────────────────────────────────
echo "--- Redis ---"
REDIS_HOST=$(echo "$REDIS_URL" | sed 's|redis://||' | cut -d: -f1)
REDIS_PORT=$(echo "$REDIS_URL" | sed 's|redis://||' | cut -d: -f2)
REDIS_PORT="${REDIS_PORT:-6379}"

if redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" ping 2>/dev/null | grep -q "PONG"; then
  pass "Redis PING successful"
  
  # SET/GET test
  redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" SET osv:infra:test "ok" EX 60 >/dev/null 2>&1
  VAL=$(redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" GET osv:infra:test 2>/dev/null)
  if [ "$VAL" = "ok" ]; then
    pass "Redis SET/GET working"
    redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" DEL osv:infra:test >/dev/null 2>&1
  else
    fail "Redis SET/GET failed"
  fi
else
  fail "Redis NOT reachable at $REDIS_HOST:$REDIS_PORT"
fi

echo ""

# ── OpenSearch ───────────────────────────────────────────────────────────────
echo "--- OpenSearch ---"
if curl -sf "$OPENSEARCH_URL/_cluster/health" | python3 -c "import sys,json; d=json.load(sys.stdin); sys.exit(0 if d['status'] in ['green','yellow'] else 1)" 2>/dev/null; then
  HEALTH=$(curl -sf "$OPENSEARCH_URL/_cluster/health" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['status'])")
  pass "OpenSearch cluster health: $HEALTH"
  
  # Create test document
  curl -sf -X PUT "$OPENSEARCH_URL/osv-infra-test/_doc/1" \
    -H 'Content-Type: application/json' \
    -d '{"test": true, "ts": "'"$(date -u +%Y-%m-%dT%H:%M:%SZ)"'"}' >/dev/null
  
  # Read it back
  if curl -sf "$OPENSEARCH_URL/osv-infra-test/_doc/1" | python3 -c "import sys,json; d=json.load(sys.stdin); sys.exit(0 if d.get('found') else 1)" 2>/dev/null; then
    pass "OpenSearch index/read working"
    curl -sf -X DELETE "$OPENSEARCH_URL/osv-infra-test" >/dev/null 2>&1
  else
    fail "OpenSearch document read failed"
  fi
else
  fail "OpenSearch NOT reachable at $OPENSEARCH_URL"
fi

echo ""

# ── Summary ──────────────────────────────────────────────────────────────────
echo "=== Summary ==="
if [ "$FAILURES" -eq 0 ]; then
  echo -e "${GREEN}All checks passed!${NC}"
  exit 0
else
  echo -e "${RED}$FAILURES check(s) failed.${NC}"
  exit 1
fi
