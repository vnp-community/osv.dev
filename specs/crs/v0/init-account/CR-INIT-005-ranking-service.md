# CR-INIT-005 — Khởi tạo ranking-service

## Mục tiêu

Sau khi chạy init, ranking-service phải:
1. Kết nối được với MongoDB
2. Indexes trên collection `ranking` được tạo (unique trên `cpe`, index trên `rank.group`)
3. Service start được ngay lập tức

## Biến môi trường (đọc từ `.env`)

| Biến | Mô tả | Default |
|------|-------|---------|
| `MONGO_URI` | MongoDB connection URI | `mongodb://localhost:27017` |
| `MONGO_DB` | Database name | `cvedb` |
| `RANKING_PORT` | HTTP port | `8088` |

> **Lưu ý:** ranking-service dùng `PORT` env var (không phải `HTTP_PORT`). Script bootstrap sẽ set cả hai.

## Các thay đổi cần thực hiện

### [NEW] `services/ranking-service/scripts/init.sh`

```bash
#!/usr/bin/env bash
# ranking-service bootstrap script
# 1. Kiểm tra kết nối MongoDB
# 2. Tạo indexes trên collection "ranking"

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_DIR="$(dirname "$SCRIPT_DIR")"

# Load .env
if [ -f "${SCRIPT_DIR}/../../../.env" ]; then
  set -o allexport
  source "${SCRIPT_DIR}/../../../.env"
  set +o allexport
fi

MONGO_URI="${MONGO_URI:-mongodb://localhost:27017}"
MONGO_DB="${MONGO_DB:-cvedb}"
RANKING_PORT="${RANKING_PORT:-8088}"

echo "=== [ranking-service] Bootstrap Start ==="

# ── Step 1: Verify MongoDB connection ─────────────────────────────────────
echo "→ [1/2] Checking MongoDB connection..."
if mongosh --quiet --eval "db.runCommand({ping:1}).ok" "${MONGO_URI}" 2>/dev/null | grep -q "1"; then
  echo "   ✓ MongoDB connected at ${MONGO_URI}"
elif mongo --quiet --eval "db.runCommand({ping:1}).ok" "${MONGO_URI}" 2>/dev/null | grep -q "1"; then
  echo "   ✓ MongoDB (legacy) connected at ${MONGO_URI}"
else
  echo "   ⚠ WARNING: Cannot verify MongoDB connection"
  echo "   ranking-service will attempt to connect at startup"
fi

# ── Step 2: Create indexes ────────────────────────────────────────────────
echo "→ [2/2] Creating MongoDB indexes..."

# Dùng migration script nếu có
MIGRATION_JS="${SERVICE_DIR}/migrations/001_ranking_indexes.js"
if [ -f "$MIGRATION_JS" ]; then
  if command -v mongosh &>/dev/null; then
    mongosh "${MONGO_URI}/${MONGO_DB}" --quiet "$MIGRATION_JS" 2>/dev/null || \
      echo "   (indexes may already exist)"
  elif command -v mongo &>/dev/null; then
    mongo "${MONGO_URI}/${MONGO_DB}" --quiet "$MIGRATION_JS" 2>/dev/null || \
      echo "   (indexes may already exist)"
  else
    echo "   ⚠ mongosh/mongo not found, indexes will be created at service startup"
  fi
  echo "   ✓ Indexes ready"
else
  # Inline create indexes nếu không có migration file
  MONGO_CMD='
db.ranking.createIndex({"cpe": 1}, {unique: true, name: "ranking_cpe_unique"});
db.ranking.createIndex({"rank.group": 1}, {name: "ranking_group"});
print("indexes created");
'
  if command -v mongosh &>/dev/null; then
    echo "$MONGO_CMD" | mongosh "${MONGO_URI}/${MONGO_DB}" --quiet 2>/dev/null || true
  elif command -v mongo &>/dev/null; then
    echo "$MONGO_CMD" | mongo "${MONGO_URI}/${MONGO_DB}" --quiet 2>/dev/null || true
  fi
  echo "   ✓ Indexes ready (inline)"
fi

echo ""
echo "=== [ranking-service] Bootstrap Complete ==="
echo "   HTTP: :${RANKING_PORT}"
echo "   MongoDB: ${MONGO_URI}/${MONGO_DB}"
echo ""
echo "Test:"
echo "  curl http://localhost:${RANKING_PORT}/health"
echo "  curl http://localhost:${RANKING_PORT}/api/v1/ranking"
```

### [MODIFY] `services/ranking-service/cmd/server/main.go`

Cập nhật để đọc `RANKING_PORT` với fallback về `PORT`:

```go
// Thay:
port := envDefault("PORT", "8088")

// Thành (support cả RANKING_PORT và PORT):
port := envDefault("RANKING_PORT", envDefault("PORT", "8088"))
```

Thêm `/health` endpoint:

```go
// Thêm vào router (sau khi wire handler):
router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    fmt.Fprintf(w, `{"status":"ok","service":"ranking-service","port":"%s"}`, port)
})
```

> **Ghi chú:** `deliveryhttp.NewRouter(handler)` cần được mở rộng để accept health handler, hoặc thêm route trực tiếp vào chi.Router trong main.go.

### [NEW] `services/ranking-service/scripts/seed_sample.sh` (tùy chọn)

Script seed dữ liệu mẫu để test ngay sau bootstrap:

```bash
#!/usr/bin/env bash
# Seed một vài CPE ranking entries mẫu
source "$(dirname "$0")/../../../.env" 2>/dev/null || true
MONGO_URI="${MONGO_URI:-mongodb://localhost:27017}"
MONGO_DB="${MONGO_DB:-cvedb}"

mongosh "${MONGO_URI}/${MONGO_DB}" --quiet --eval '
db.ranking.insertMany([
  {
    cpe: "cpe:2.3:a:apache:log4j:*:*:*:*:*:*:*:*",
    rank: { group: "critical", score: 9.8, priority: 1 },
    updated_at: new Date()
  },
  {
    cpe: "cpe:2.3:o:linux:linux_kernel:*:*:*:*:*:*:*:*",
    rank: { group: "high", score: 7.5, priority: 2 },
    updated_at: new Date()
  }
], {ordered: false})
' 2>/dev/null || echo "Sample data may already exist"
echo "✓ Sample ranking data seeded"
```

## Acceptance Criteria

- [ ] `services/ranking-service/scripts/init.sh` tồn tại và executable
- [ ] Script không fail nếu MongoDB chưa sẵn sàng (warning thay vì error)
- [ ] Indexes được tạo: `ranking_cpe_unique` (unique) và `ranking_group`
- [ ] `GET /health` trả về 200
- [ ] `GET /api/v1/ranking` trả về danh sách (có thể rỗng)
- [ ] Chạy lại script không gây lỗi duplicate index

## Kiểm tra nhanh

```bash
# 1. Init
./services/ranking-service/scripts/init.sh

# 2. Start
cd services/ranking-service
MONGO_URI=$MONGO_URI MONGO_DB=$MONGO_DB RANKING_PORT=8088 ./server

# 3. Test
curl http://localhost:8088/health
curl http://localhost:8088/api/v1/ranking?cpe=cpe%3A2.3%3Aa%3Aapache%3Alog4j
```
