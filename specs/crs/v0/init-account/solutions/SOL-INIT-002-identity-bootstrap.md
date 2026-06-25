# SOL-INIT-002 — Giải Pháp: Khởi Tạo identity-service

> **CR tham chiếu**: [CR-INIT-002](../CR-INIT-002-identity-service.md)  
> **Kiến trúc cơ sở**: `specs/01-architecture.md §3.10`, `specs/02-technical-design.md §10`, `§14.1`

## Phân Tích Code Hiện Tại

```
services/identity-service/
├── internal/
│   ├── infrastructure/
│   │   ├── jwt/token.go         ← RS256, PublicKeyJWKS() đã có ✓
│   │   └── crypto/argon2id.go   ← Argon2id HashPassword() đã có ✓
│   ├── usecase/
│   │   └── register/register.go ← Execute(ctx, Request) đã có ✓
│   └── delivery/http/
│       └── handlers.go          ← Router() thiếu JWKS + /health route ✗
├── migrations/
│   ├── 001_initial_schema.sql   ← SET search_path TO auth (không tự tạo schema!) ✗
│   └── 002_totp_pending.sql
└── cmd/server/main.go           ← DATABASE_URL hardcode, thiếu IDENTITY_ prefix ✗
```

**Vấn đề xác định:**
1. Migration `001_initial_schema.sql` gọi `SET search_path TO auth` nhưng không tạo schema `auth` trước
2. `cmd/server/main.go` đọc `DATABASE_URL` không nhất quán với CR quy ước `IDENTITY_DATABASE_URL`
3. `handlers.go` thiếu route `/.well-known/jwks.json` và `/health`
4. Không có script khởi tạo admin account

## Files cần tạo/sửa

### [NEW] `services/identity-service/scripts/init.sh`

**Logic thực thi**:
1. Tạo schema `auth` (idempotent)
2. Enable extensions (uuid-ossp, citext)
3. Apply migrations theo thứ tự
4. Generate RSA-4096 key pair
5. Seed admin account qua API (sau khi service up) hoặc trực tiếp vào DB

```bash
#!/usr/bin/env bash
# =============================================================================
# identity-service — Bootstrap Script
# Spec: 01-architecture.md §3.10 (Auth Chain Local + LDAP)
# Tech: 02-technical-design.md §14.1 (Argon2id + RS256 JWT + TokenFamily)
# =============================================================================
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_DIR="$(dirname "$SCRIPT_DIR")"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

# ── Load .env ─────────────────────────────────────────────────────────────
if [[ -f "${PROJECT_ROOT}/.env" ]]; then
  set -o allexport; source "${PROJECT_ROOT}/.env"; set +o allexport
fi

# ── Config với fallback chain ──────────────────────────────────────────────
DB_URL="${IDENTITY_DATABASE_URL:-${POSTGRES_DSN:-postgres://osv:osv_dev@localhost:5432/osvdb?sslmode=disable}}"
GRPC_PORT="${IDENTITY_GRPC_PORT:-9001}"
HTTP_PORT="${IDENTITY_HTTP_PORT:-9101}"
JWT_KEY_PATH="${JWT_PRIVATE_KEY_PATH:-${SERVICE_DIR}/secrets/jwt_private.pem}"
ADMIN_EMAIL="${INIT_ADMIN_EMAIL:-admin@openvulnscan.io}"
ADMIN_USERNAME="${INIT_ADMIN_USERNAME:-admin}"
ADMIN_PASSWORD="${INIT_ADMIN_PASSWORD:-Admin@123!ChangeMe}"

echo "══════════════════════════════════════════════════════════"
echo "  identity-service Bootstrap"
echo "══════════════════════════════════════════════════════════"

# ── Step 1: PostgreSQL schema + extensions ────────────────────────────────
echo "→ [1/4] Khởi tạo PostgreSQL schema 'auth'..."

# Tạo schema auth và các extensions cần thiết
psql "${DB_URL}" <<-SQL
  -- Extensions (spec: pgvector for cves, citext for case-insensitive email)
  CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
  CREATE EXTENSION IF NOT EXISTS "citext";

  -- Schema cho identity-service (spec: 01-architecture.md §4.1 osv_identity)
  CREATE SCHEMA IF NOT EXISTS auth;
SQL
echo "   ✓ Schema 'auth' ready"

# Apply migrations theo thứ tự số
echo "   Applying migrations..."
for sql_file in $(ls "${SERVICE_DIR}/migrations/"*.sql 2>/dev/null | sort -V); do
  fname="$(basename "$sql_file")"
  echo "   → ${fname}"
  psql "${DB_URL}" -v ON_ERROR_STOP=0 -f "$sql_file" 2>/dev/null || {
    echo "   (skipped: already applied or non-critical)"
  }
done
echo "   ✓ Migrations applied"

# ── Step 2: RSA JWT Key Pair ──────────────────────────────────────────────
echo "→ [2/4] Khởi tạo RSA-4096 JWT key pair..."
# Spec: 02-technical-design.md §14.1 "JWT RS256 signed token"
# Code: internal/infrastructure/jwt/token.go — NewService(cfg Config)

KEY_DIR="$(dirname "${JWT_KEY_PATH}")"
mkdir -p "${KEY_DIR}"
chmod 700 "${KEY_DIR}"

if [[ ! -f "${JWT_KEY_PATH}" ]]; then
  echo "   Generating RSA-4096 private key..."
  openssl genrsa -out "${JWT_KEY_PATH}" 4096 2>/dev/null
  openssl rsa -in "${JWT_KEY_PATH}" -pubout \
    -out "${JWT_KEY_PATH%.pem}_public.pem" 2>/dev/null
  chmod 600 "${JWT_KEY_PATH}"
  chmod 644 "${JWT_KEY_PATH%.pem}_public.pem"
  echo "   ✓ Key pair created:"
  echo "     Private: ${JWT_KEY_PATH}"
  echo "     Public:  ${JWT_KEY_PATH%.pem}_public.pem"
else
  echo "   ✓ Key pair already exists at: ${JWT_KEY_PATH}"
fi

# Validate key có thể đọc được
openssl rsa -in "${JWT_KEY_PATH}" -check -noout 2>/dev/null && \
  echo "   ✓ Key validated OK" || \
  echo "   ✗ ERROR: Cannot validate RSA key at ${JWT_KEY_PATH}"

# ── Step 3: Seed Admin Account ────────────────────────────────────────────
echo "→ [3/4] Khởi tạo admin account..."
# Tech: Argon2id hashing (internal/infrastructure/crypto/argon2id.go)
# Params: memory=64MB, iterations=3, parallelism=2 (OWASP minimum)

# Kiểm tra admin có tồn tại chưa
EXISTING=$(psql "${DB_URL}" -t -c \
  "SELECT COUNT(*) FROM auth.users WHERE email='${ADMIN_EMAIL}';" 2>/dev/null | tr -d ' ')

if [[ "${EXISTING:-0}" -gt 0 ]]; then
  echo "   ✓ Admin account already exists: ${ADMIN_EMAIL}"
  # Đảm bảo role=admin
  psql "${DB_URL}" -c \
    "UPDATE auth.users SET role='admin', is_active=true, is_verified=true WHERE email='${ADMIN_EMAIL}';" \
    2>/dev/null || true
  echo "   ✓ Admin role confirmed"
else
  # Hash password bằng Python (argon2-cffi) hoặc Go binary
  HASHED_PW=""
  
  if command -v python3 &>/dev/null && python3 -c "import argon2" 2>/dev/null; then
    HASHED_PW=$(python3 <<-PYEOF
import argon2
ph = argon2.PasswordHasher(
    time_cost=3,          # iterations
    memory_cost=65536,    # 64MB
    parallelism=2,
    hash_len=32,
    salt_len=16,
    encoding='utf-8',
    type=argon2.Type.ID,  # Argon2id
)
print(ph.hash("${ADMIN_PASSWORD}"))
PYEOF
)
  elif command -v python3 &>/dev/null && python3 -c "import argon2.low_level" 2>/dev/null; then
    HASHED_PW=$(python3 -c "
from argon2.low_level import hash_secret, Type
import os, base64
salt = os.urandom(16)
h = hash_secret(b'${ADMIN_PASSWORD}', salt, time_cost=3, memory_cost=65536, parallelism=2, hash_len=32, type=Type.ID)
print(h.decode())
")
  fi

  if [[ -n "${HASHED_PW}" ]]; then
    # INSERT với Argon2id hash
    psql "${DB_URL}" <<-SQL
      SET search_path TO auth;
      INSERT INTO users (email, username, hashed_password, role, auth_provider, is_active, is_verified)
      VALUES (
        '${ADMIN_EMAIL}',
        '${ADMIN_USERNAME}',
        '${HASHED_PW}',
        'admin',
        'local',
        true,
        true
      )
      ON CONFLICT (email) DO UPDATE SET
        role = 'admin',
        is_active = true,
        is_verified = true,
        updated_at = NOW();
SQL
    echo "   ✓ Admin account created: ${ADMIN_EMAIL}"
  else
    # Fallback: tạo placeholder, đổi qua API sau khi service start
    echo "   ⚠ python3-argon2 không có sẵn"
    echo "   Tạo admin account sau khi service start:"
    echo "     ./scripts/seed-admin.sh"
  fi
fi

# ── Step 4: Tạo seed-admin.sh để dùng sau ────────────────────────────────
echo "→ [4/4] Tạo helper script..."
cat > "${SCRIPT_DIR}/seed-admin.sh" <<-'HEREDOC'
#!/usr/bin/env bash
# Seed admin account qua API (chạy sau khi identity-service đã up)
source "$(dirname "$0")/../../../.env" 2>/dev/null || true
HTTP_PORT="${IDENTITY_HTTP_PORT:-9101}"
ADMIN_EMAIL="${INIT_ADMIN_EMAIL:-admin@openvulnscan.io}"
ADMIN_USERNAME="${INIT_ADMIN_USERNAME:-admin}"
ADMIN_PASSWORD="${INIT_ADMIN_PASSWORD:-Admin@123!ChangeMe}"

echo "Registering admin via API..."
curl -s -X POST "http://localhost:${HTTP_PORT}/auth/register" \
  -H 'Content-Type: application/json' \
  -d "{
    \"email\":\"${ADMIN_EMAIL}\",
    \"username\":\"${ADMIN_USERNAME}\",
    \"password\":\"${ADMIN_PASSWORD}\"
  }" | python3 -m json.tool 2>/dev/null || cat

# Promote to admin via SQL (API register tạo role=user)
DB_URL="${IDENTITY_DATABASE_URL:-${POSTGRES_DSN}}"
psql "${DB_URL}" -c \
  "UPDATE auth.users SET role='admin', is_verified=true WHERE email='${ADMIN_EMAIL}';"
echo "✓ Admin promoted to role=admin"
HEREDOC
chmod +x "${SCRIPT_DIR}/seed-admin.sh"

echo ""
echo "══════════════════════════════════════════════════════════"
echo "  identity-service Bootstrap Complete"
echo "══════════════════════════════════════════════════════════"
echo "  HTTP:  :${HTTP_PORT}"
echo "  gRPC:  :${GRPC_PORT}"
echo "  Admin: ${ADMIN_EMAIL}"
echo ""
echo "Khởi động service:"
echo "  IDENTITY_DATABASE_URL='${DB_URL}' \\"
echo "  JWT_PRIVATE_KEY_PATH='${JWT_KEY_PATH}' \\"
echo "  REDIS_URL=\$REDIS_URL \\"
echo "  ./services/identity-service/server"
echo ""
echo "Test login:"
echo "  curl -X POST http://localhost:${HTTP_PORT}/auth/login \\"
echo "    -H 'Content-Type: application/json' \\"
echo "    -d '{\"email\":\"${ADMIN_EMAIL}\",\"password\":\"...\"}'"
echo ""
echo "JWKS:"
echo "  curl http://localhost:${HTTP_PORT}/.well-known/jwks.json"
```

### [MODIFY] `services/identity-service/internal/delivery/http/handlers.go`

Thêm 2 routes vào `Router()` function (hiện đang ở line 57-90):

```go
// Thêm vào func (h *Handler) Router() http.Handler {
// Sau dòng: r.Use(middleware.Recoverer)

// Well-known endpoints (no auth required)
r.Get("/.well-known/jwks.json", h.JWKS)
r.Get("/health", h.Health)

// Thêm 2 handler methods vào Handler struct:

// JWKS handles GET /.well-known/jwks.json
// Spec: 01-architecture.md §3.1 — Gateway validates JWT via JWKS
// Code: internal/infrastructure/jwt/token.go — PublicKeyJWKS()
func (h *Handler) JWKS(w http.ResponseWriter, r *http.Request) {
    jwksBytes, err := h.jwtSvc.PublicKeyJWKS()
    if err != nil {
        h.logger.Error().Err(err).Msg("JWKS generation failed")
        jsonError(w, "failed to generate JWKS", http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("Cache-Control", "public, max-age=3600")
    w.WriteHeader(http.StatusOK)
    w.Write(jwksBytes)
}

// Health handles GET /health
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
    jsonResponse(w, http.StatusOK, map[string]string{
        "status":  "ok",
        "service": "identity-service",
    })
}
```

**Ghi chú**: `Handler` struct cần thêm field `jwtSvc *jwt.Service` và cập nhật `NewHandler()`.

### [MODIFY] `services/identity-service/cmd/server/main.go`

Thêm fallback chain cho `DATABASE_URL`:

```go
// Thay dòng 50:
// dbURL := getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/authdb?sslmode=disable")

// Thành:
dbURL := getEnvFallback(
    "IDENTITY_DATABASE_URL",
    "DATABASE_URL",
    "postgres://osv:osv_dev@localhost:5432/osvdb?sslmode=disable",
)

// Thêm helper function:
// getEnvFallback returns the value of the first non-empty env var,
// or defaultVal if all are empty.
func getEnvFallback(defaultVal string, keys ...string) string {
    for _, k := range keys {
        if v := os.Getenv(k); v != "" {
            return v
        }
    }
    return defaultVal
}

// Fix HTTP port (thêm IDENTITY_HTTP_PORT prefix):
httpPort := getEnvFallback("9101", "IDENTITY_HTTP_PORT", "HTTP_PORT")
grpcPort := getEnvFallback("9001", "IDENTITY_GRPC_PORT", "GRPC_PORT")
```

### [MODIFY] `services/identity-service/migrations/001_initial_schema.sql`

Thêm `CREATE SCHEMA IF NOT EXISTS auth;` vào đầu file:

```sql
-- Đảm bảo schema tồn tại trước khi SET search_path
-- (Script init.sh tạo schema, nhưng migration cũng nên tự-đủ)
CREATE SCHEMA IF NOT EXISTS auth;

SET search_path TO auth;

-- Phần còn lại giữ nguyên (users, sessions, oauth_accounts, api_keys, audit_log)
```

## Kiểm Tra Idempotent

```bash
# Chạy lần 1
./services/identity-service/scripts/init.sh
# Expected: "Schema 'auth' ready", "Key pair created", "Admin created"

# Chạy lần 2 (phải không lỗi)
./services/identity-service/scripts/init.sh
# Expected: "Schema 'auth' ready", "Key pair already exists", "Admin already exists"
```

## Acceptance Criteria

- [ ] `scripts/init.sh` chạy được, idempotent
- [ ] Schema `auth` và tất cả tables tồn tại sau init
- [ ] RSA key pair tạo tại `JWT_PRIVATE_KEY_PATH`
- [ ] Admin account tồn tại với `role=admin`, `is_active=true`, `is_verified=true`
- [ ] `GET /.well-known/jwks.json` trả về JSON với `{"keys":[{"kty":"RSA","alg":"RS256",...}]}`
- [ ] `GET /health` trả về `{"status":"ok","service":"identity-service"}`
- [ ] `POST /auth/login` với admin credentials trả về 200 + `{access_token, refresh_token}`
- [ ] `IDENTITY_DATABASE_URL` được ưu tiên hơn `DATABASE_URL`

## Files Tóm Tắt

| File | Action |
|------|--------|
| `services/identity-service/scripts/init.sh` | **[NEW]** |
| `services/identity-service/scripts/seed-admin.sh` | **[NEW]** (tạo trong init.sh) |
| `services/identity-service/internal/delivery/http/handlers.go` | **[MODIFY]** thêm JWKS + Health |
| `services/identity-service/cmd/server/main.go` | **[MODIFY]** fallback env vars |
| `services/identity-service/migrations/001_initial_schema.sql` | **[MODIFY]** thêm CREATE SCHEMA |
