#!/bin/bash
# Usage: ./scripts/migrate.sh [up|down]
# Applies all SQL migrations in order to DATABASE_URL.

set -e

DIRECTION=${1:-up}
MIGRATIONS_DIR="$(dirname "$0")/../migrations"

if [ -z "$DATABASE_URL" ]; then
    echo "ERROR: DATABASE_URL is not set"
    exit 1
fi

echo "=== GlobalCVE Migrations: ${DIRECTION} ==="

if [ "$DIRECTION" = "down" ]; then
    FILES=$(ls "$MIGRATIONS_DIR"/*.down.sql 2>/dev/null | sort -r)
else
    FILES=$(ls "$MIGRATIONS_DIR"/*.up.sql 2>/dev/null | sort)
fi

for f in $FILES; do
    echo "  Applying $f..."
    psql "$DATABASE_URL" -f "$f"
done

echo "=== Done ==="
