# T12 — Migrations Consolidation

**Phase**: 12
**Depends on**: T11
**Estimated effort**: 1 hour

---

## Mục tiêu

Kiểm tra và đảm bảo migrations trong tất cả services được đánh số liên tục, không bị trùng. Tạo migration tổng hợp cho từng service.

---

## Tác vụ chi tiết

### Bước 1: Kiểm tra migrations hiện tại

```bash
SVC_ROOT="/Users/binhnt/Lab/sec/cve/osv.dev/services"

for svc in identity-service data-service scan-service finding-service notification-service; do
  echo "=== $svc/migrations ==="
  ls -la "$SVC_ROOT/$svc/migrations/" 2>/dev/null || echo "(no migrations dir)"
done
```

### Bước 2: Đánh số lại migrations nếu có gaps hoặc conflicts

```bash
# Script renumber migrations sequentially
renumber_migrations() {
  local MIG_DIR="$1"
  local i=1
  for f in $(ls "$MIG_DIR"/*.sql 2>/dev/null | sort); do
    BASENAME=$(basename "$f")
    # Extract description part (remove leading number)
    DESC=$(echo "$BASENAME" | sed 's/^[0-9]*_//')
    NEW_NAME="$(printf '%03d' $i)_${DESC}"
    if [ "$BASENAME" != "$NEW_NAME" ]; then
      mv "$MIG_DIR/$BASENAME" "$MIG_DIR/$NEW_NAME"
      echo "  Renamed: $BASENAME → $NEW_NAME"
    fi
    i=$((i + 1))
  done
}

for svc in identity-service data-service scan-service finding-service notification-service; do
  echo "Renumbering $svc migrations..."
  renumber_migrations "$SVC_ROOT/$svc/migrations"
done
```

### Bước 3: Tạo migration summary cho từng service

```bash
# Tạo README trong migrations/ mô tả từng migration
for svc in identity-service data-service scan-service finding-service notification-service; do
  MIG_DIR="$SVC_ROOT/$svc/migrations"

  cat > "$MIG_DIR/README.md" << EOF
# ${svc} Migrations

Run all migrations with:
\`\`\`bash
psql \$DATABASE_URL -f migrations/001_*.sql
# ... or use a migration tool like golang-migrate
\`\`\`

## Migration Files
EOF

  for f in $(ls "$MIG_DIR"/*.sql 2>/dev/null | sort); do
    echo "- \`$(basename $f)\`" >> "$MIG_DIR/README.md"
  done

  echo "Created migration README for $svc"
done
```

### Bước 4: Tạo consolidated schema script

```bash
# Tạo consolidated SQL script cho mỗi service (hữu ích cho dev/test)
for svc in identity-service data-service scan-service finding-service notification-service; do
  MIG_DIR="$SVC_ROOT/$svc/migrations"
  OUT="$MIG_DIR/schema_all.sql"

  echo "-- Consolidated schema for $svc" > "$OUT"
  echo "-- Generated: $(date)" >> "$OUT"
  echo "" >> "$OUT"

  for f in $(ls "$MIG_DIR"/*.sql 2>/dev/null | sort); do
    if [ "$(basename $f)" != "schema_all.sql" ]; then
      echo "" >> "$OUT"
      echo "-- ============================================" >> "$OUT"
      echo "-- $(basename $f)" >> "$OUT"
      echo "-- ============================================" >> "$OUT"
      cat "$f" >> "$OUT"
    fi
  done

  echo "Created $OUT"
done
```

### Bước 5: Xác nhận không có migrations trùng lặp

```bash
for svc in identity-service data-service scan-service finding-service notification-service; do
  MIG_DIR="$SVC_ROOT/$svc/migrations"
  echo "=== $svc ==="
  ls "$MIG_DIR"/*.sql 2>/dev/null | sort | head -20
done
```

---

## Điều kiện hoàn thành

- [ ] Tất cả migrations đánh số liên tục (001, 002, 003...)
- [ ] Không có số bị trùng trong cùng một service
- [ ] `migrations/README.md` tồn tại trong mỗi service
- [ ] `migrations/schema_all.sql` tồn tại (consolidated)

---

## Commit message

```
chore(migrations): consolidate and renumber all migrations

- Renumbered migrations sequentially for all services
- Added migrations/README.md per service
- Added consolidated schema_all.sql for dev/test convenience
```
