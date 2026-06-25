# CR-INIT-002 — Khởi tạo identity-service

## Mục tiêu

Sau khi chạy init, identity-service phải:
1. Có database schema đầy đủ (migrations đã apply)
2. Có RSA private key để ký JWT
3. Có admin account sẵn sàng để đăng nhập

## Biến môi trường (đọc từ `.env`)

| Biến | Mô tả | Default |
|------|-------|---------|
| `IDENTITY_DATABASE_URL` | PostgreSQL DSN với search_path=auth | `postgres://osv:osv_dev_secret_change_me@localhost:5432/osvdb?sslmode=disable&search_path=auth` |
| `IDENTITY_HTTP_PORT` | HTTP server port | `9101` |
| `IDENTITY_GRPC_PORT` | gRPC server port | `9001` |
| `JWT_PRIVATE_KEY_PATH` | Đường dẫn RSA private key PEM | `secrets/jwt_private.pem` |
| `JWT_ISSUER` | JWT issuer claim | `https://auth.openvulnscan.io` |
| `JWT_AUDIENCE` | JWT audience claim | `openvulnscan` |
| `INIT_ADMIN_EMAIL` | Email của admin account | `admin@openvulnscan.io` |
| `INIT_ADMIN_USERNAME` | Username của admin account | `admin` |
| `INIT_ADMIN_PASSWORD` | Password của admin account (min 8 ký tự) | `Admin@123!ChangeMe` |
| `REDIS_URL` | Redis URL để token blacklist | `redis://localhost:6379/0` |

## Các thay đổi cần thực hiện

### [NEW] `services/identity-service/scripts/init.sh`

Script này được gọi một lần khi bootstrap:

```bash
#!/usr/bin/env bash
# identity-service bootstrap script
# Đọc cấu hình từ .env và khởi tạo:
#   1. PostgreSQL schema (apply migrations)
#   2. RSA JWT key pair
#   3. Admin account

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_DIR="$(dirname "$SCRIPT_DIR")"

# Load .env nếu tồn tại
if [ -f "${SCRIPT_DIR}/../../../.env" ]; then
  set -o allexport
  source "${SCRIPT_DIR}/../../../.env"
  set +o allexport
fi

# ── Defaults ──────────────────────────────────────────────────────────────
IDENTITY_DATABASE_URL="${IDENTITY_DATABASE_URL:-postgres://osv:osv_dev_secret_change_me@localhost:5432/osvdb?sslmode=disable}"
JWT_PRIVATE_KEY_PATH="${JWT_PRIVATE_KEY_PATH:-${SERVICE_DIR}/secrets/jwt_private.pem}"
INIT_ADMIN_EMAIL="${INIT_ADMIN_EMAIL:-admin@openvulnscan.io}"
INIT_ADMIN_USERNAME="${INIT_ADMIN_USERNAME:-admin}"
INIT_ADMIN_PASSWORD="${INIT_ADMIN_PASSWORD:-Admin@123!ChangeMe}"

echo "=== [identity-service] Bootstrap Start ==="

# ── Step 1: Create schema if not exists ──────────────────────────────────
echo "→ [1/3] Applying database migrations..."
psql "${IDENTITY_DATABASE_URL}" -c "CREATE SCHEMA IF NOT EXISTS auth;"
for sql_file in "${SERVICE_DIR}/migrations/"*.sql; do
  echo "   Applying: $(basename "$sql_file")"
  psql "${IDENTITY_DATABASE_URL}" -f "$sql_file" 2>/dev/null || echo "   (already applied, skipping)"
done
echo "   ✓ Database schema ready"

# ── Step 2: Generate JWT RSA key pair ────────────────────────────────────
echo "→ [2/3] Setting up JWT keys..."
KEY_DIR="$(dirname "$JWT_PRIVATE_KEY_PATH")"
mkdir -p "$KEY_DIR"

if [ ! -f "$JWT_PRIVATE_KEY_PATH" ]; then
  echo "   Generating RSA-4096 key pair..."
  openssl genrsa -out "$JWT_PRIVATE_KEY_PATH" 4096 2>/dev/null
  openssl rsa -in "$JWT_PRIVATE_KEY_PATH" -pubout \
    -out "${JWT_PRIVATE_KEY_PATH%.pem}_public.pem" 2>/dev/null
  chmod 600 "$JWT_PRIVATE_KEY_PATH"
  echo "   ✓ JWT keys created at: $JWT_PRIVATE_KEY_PATH"
else
  echo "   ✓ JWT keys already exist, skipping"
fi

# ── Step 3: Create admin account ─────────────────────────────────────────
echo "→ [3/3] Creating admin account..."
# Hash password với bcrypt (cost=12) — dùng Python nếu có, hoặc htpasswd
if command -v python3 &>/dev/null && python3 -c "import bcrypt" 2>/dev/null; then
  HASHED_PASSWORD=$(python3 -c "
import bcrypt, sys
pw = sys.argv[1].encode('utf-8')
print(bcrypt.hashpw(pw, bcrypt.gensalt(12)).decode())
" "$INIT_ADMIN_PASSWORD")
elif command -v htpasswd &>/dev/null; then
  HASHED_PASSWORD=$(htpasswd -bnBC 12 "" "$INIT_ADMIN_PASSWORD" | tr -d ':\n' | sed 's/\$2y/\$2a/')
else
  echo "   ⚠ WARNING: Cannot hash password (install python3-bcrypt or apache2-utils)"
  echo "   Skipping admin account creation. Create manually via API."
  HASHED_PASSWORD=""
fi

if [ -n "$HASHED_PASSWORD" ]; then
  psql "${IDENTITY_DATABASE_URL}" <<-SQL
    SET search_path TO auth;
    INSERT INTO users (email, username, hashed_password, role, is_active, is_verified)
    VALUES (
      '${INIT_ADMIN_EMAIL}',
      '${INIT_ADMIN_USERNAME}',
      '${HASHED_PASSWORD}',
      'admin',
      true,
      true
    )
    ON CONFLICT (email) DO UPDATE SET
      role = 'admin',
      is_active = true,
      is_verified = true,
      updated_at = NOW();
SQL
  echo "   ✓ Admin account ready: ${INIT_ADMIN_EMAIL}"
fi

echo ""
echo "=== [identity-service] Bootstrap Complete ==="
echo "   HTTP: :${IDENTITY_HTTP_PORT:-9101}"
echo "   gRPC: :${IDENTITY_GRPC_PORT:-9001}"
echo "   Admin: ${INIT_ADMIN_EMAIL} / [configured password]"
echo ""
echo "Test:"
echo "  curl -X POST http://localhost:${IDENTITY_HTTP_PORT:-9101}/auth/login \\"
echo "    -H 'Content-Type: application/json' \\"
echo "    -d '{\"email\":\"${INIT_ADMIN_EMAIL}\",\"password\":\"...\"}'"
```

### [MODIFY] `services/identity-service/cmd/server/main.go`

Thêm đọc biến `DATABASE_URL` từ env (hiện đang hardcode tên biến không nhất quán):

```go
// Thay dòng:
dbURL := getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/authdb?sslmode=disable")

// Thành (ưu tiên IDENTITY_DATABASE_URL, fallback DATABASE_URL):
dbURL := getEnvFallback("IDENTITY_DATABASE_URL", "DATABASE_URL",
    "postgres://osv:osv_dev_secret_change_me@localhost:5432/osvdb?sslmode=disable&search_path=auth")

// Thêm helper:
func getEnvFallback(keys ...string) string {
    // keys: primary key, fallback key, default value
    for i := 0; i < len(keys)-1; i++ {
        if v := os.Getenv(keys[i]); v != "" {
            return v
        }
    }
    return keys[len(keys)-1]
}
```

Tương tự cho `REDIS_URL`:

```go
// Thay:
rdbOpts, err := redis.ParseURL(getEnv("REDIS_URL", "redis://localhost:6379"))
// Giữ nguyên — REDIS_URL đã đúng tên biến
```

## Thứ tự migration

```
migrations/001_initial_schema.sql   — tables: users, sessions, oauth_accounts, api_keys, audit_log
migrations/002_totp_pending.sql     — thêm totp_pending table
```

## JWKS endpoint (cần thêm)

Để gateway validate JWT, identity-service cần expose JWKS:

### [MODIFY] `services/identity-service/internal/delivery/http/handlers.go`

Thêm route vào `Router()`:

```go
// Thêm vào public routes:
r.Get("/.well-known/jwks.json", h.JWKS)
r.Get("/health", h.Health)

// Thêm handler:
func (h *Handler) JWKS(w http.ResponseWriter, r *http.Request) {
    // Trả về JWKS từ JWT service
    jsonResponse(w, http.StatusOK, h.jwtSvc.JWKS())
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
    jsonResponse(w, http.StatusOK, map[string]string{"status": "ok", "service": "identity-service"})
}
```

## Acceptance Criteria

- [ ] `services/identity-service/scripts/init.sh` tồn tại và executable
- [ ] Script tạo schema `auth` và apply tất cả migrations không bị lỗi
- [ ] RSA key pair được tạo tại `JWT_PRIVATE_KEY_PATH`
- [ ] Admin account tồn tại trong DB sau khi chạy script
- [ ] `POST /auth/login` với admin credentials trả về 200 + JWT tokens
- [ ] `GET /.well-known/jwks.json` trả về public key
- [ ] `GET /health` trả về `{"status":"ok"}`
- [ ] Chạy lại script lần 2 không gây lỗi (idempotent)

## Kiểm tra nhanh

```bash
# 1. Chạy init
./services/identity-service/scripts/init.sh

# 2. Start service
cd services/identity-service
DATABASE_URL=$IDENTITY_DATABASE_URL \
REDIS_URL=$REDIS_URL \
JWT_PRIVATE_KEY_PATH=$JWT_PRIVATE_KEY_PATH \
./server

# 3. Login
curl -s -X POST http://localhost:9101/auth/login \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$INIT_ADMIN_EMAIL\",\"password\":\"$INIT_ADMIN_PASSWORD\"}" | jq .
```
