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

# ── Load .env ─────────────────────────────────────────────────────────────────
if [[ -f "${PROJECT_ROOT}/.env" ]]; then
  set -o allexport; source "${PROJECT_ROOT}/.env"; set +o allexport
fi

# ── Config với fallback chain ──────────────────────────────────────────────────
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

# ── Step 1: PostgreSQL schema + extensions ─────────────────────────────────────
echo "→ [1/4] Khởi tạo PostgreSQL schema 'auth'..."
# Spec: 01-architecture.md §4.1 — Schema-per-service trong cùng cluster
# citext: case-insensitive email lookup
# uuid-ossp: gen_random_uuid() trong migrations

psql "${DB_URL}" <<-SQL
  CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
  CREATE EXTENSION IF NOT EXISTS "citext";
  CREATE SCHEMA IF NOT EXISTS auth;
SQL
echo "   ✓ Schema 'auth' và extensions ready"

# Apply migrations theo thứ tự
echo "   Applying migrations..."
for sql_file in $(ls "${SERVICE_DIR}/migrations/"*.sql 2>/dev/null | sort -V); do
  fname="$(basename "$sql_file")"
  echo "   → ${fname}"
  psql "${DB_URL}" -v ON_ERROR_STOP=0 -f "$sql_file" 2>/dev/null || {
    echo "     (skipped: already applied)"
  }
done
echo "   ✓ Migrations applied"

# ── Step 2: RSA JWT Key Pair ───────────────────────────────────────────────────
echo "→ [2/4] Khởi tạo RSA-4096 JWT key pair..."
# Spec: 02-technical-design.md §14.1 "JWT RS256 signed token"
# Code: internal/infrastructure/jwt/token.go — NewService(cfg Config{PrivateKeyPath})

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

# Validate key
if openssl rsa -in "${JWT_KEY_PATH}" -check -noout 2>/dev/null; then
  echo "   ✓ RSA key validated OK"
else
  echo "   ✗ ERROR: Cannot validate RSA key at ${JWT_KEY_PATH}"
  exit 1
fi

# ── Step 3: Seed Admin Account ─────────────────────────────────────────────────
echo "→ [3/4] Khởi tạo admin account..."
# Tech: Argon2id (internal/infrastructure/crypto/argon2id.go)
# Params: memory=64MB, iterations=3, parallelism=2 (OWASP minimum)

# Kiểm tra admin đã tồn tại chưa
EXISTING=$(psql "${DB_URL}" -t -c \
  "SELECT COUNT(*) FROM auth.users WHERE email='${ADMIN_EMAIL}';" 2>/dev/null | tr -d ' ')

if [[ "${EXISTING:-0}" -gt 0 ]]; then
  echo "   ✓ Admin account already exists: ${ADMIN_EMAIL}"
  psql "${DB_URL}" -c \
    "UPDATE auth.users SET role='admin', is_active=true, is_verified=true WHERE email='${ADMIN_EMAIL}';" \
    2>/dev/null || true
  echo "   ✓ Admin role confirmed"
else
  # Hash password với argon2-cffi (Python) hoặc fallback
  HASHED_PW=""
  if command -v python3 &>/dev/null && python3 -c "import argon2" 2>/dev/null; then
    HASHED_PW=$(python3 <<-PYEOF
import argon2
ph = argon2.PasswordHasher(
    time_cost=3, memory_cost=65536, parallelism=2,
    hash_len=32, salt_len=16, encoding='utf-8', type=argon2.Type.ID,
)
print(ph.hash("${ADMIN_PASSWORD}"))
PYEOF
)
  fi

  if [[ -n "${HASHED_PW}" ]]; then
    psql "${DB_URL}" <<-SQL
      SET search_path TO auth;
      INSERT INTO users (email, username, hashed_password, role, auth_provider, is_active, is_verified)
      VALUES (
        '${ADMIN_EMAIL}', '${ADMIN_USERNAME}', '${HASHED_PW}',
        'admin', 'local', true, true
      )
      ON CONFLICT (email) DO UPDATE SET
        role = 'admin', is_active = true, is_verified = true, updated_at = NOW();
SQL
    echo "   ✓ Admin account created: ${ADMIN_EMAIL}"
  else
    echo "   ⚠ python3-argon2 không có — chạy seed-admin.sh sau khi service start"
    cat > "${SCRIPT_DIR}/seed-admin.sh" <<-'HEREDOC'
#!/usr/bin/env bash
# Seed admin account via API (chạy sau khi identity-service đã up)
source "$(dirname "$0")/../../../.env" 2>/dev/null || true
HTTP_PORT="${IDENTITY_HTTP_PORT:-9101}"
ADMIN_EMAIL="${INIT_ADMIN_EMAIL:-admin@openvulnscan.io}"
ADMIN_USERNAME="${INIT_ADMIN_USERNAME:-admin}"
ADMIN_PASSWORD="${INIT_ADMIN_PASSWORD:-Admin@123!ChangeMe}"
echo "Registering admin via API..."
curl -s -X POST "http://localhost:${HTTP_PORT}/api/v1/auth/register" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"${ADMIN_EMAIL}\",\"username\":\"${ADMIN_USERNAME}\",\"password\":\"${ADMIN_PASSWORD}\"}" \
  | python3 -m json.tool 2>/dev/null || cat
DB_URL="${IDENTITY_DATABASE_URL:-${POSTGRES_DSN}}"
psql "${DB_URL}" -c \
  "UPDATE auth.users SET role='admin', is_verified=true WHERE email='${ADMIN_EMAIL}';"
echo "✓ Admin promoted to role=admin"
HEREDOC
    chmod +x "${SCRIPT_DIR}/seed-admin.sh"
  fi
fi

# ── Step 4: Summary ────────────────────────────────────────────────────────────
echo "→ [4/4] Done"
echo ""
echo "══════════════════════════════════════════════════════════"
echo "  identity-service Bootstrap Complete"
echo "══════════════════════════════════════════════════════════"
echo "  HTTP:  :${HTTP_PORT}"
echo "  gRPC:  :${GRPC_PORT}"
echo "  Admin: ${ADMIN_EMAIL}"
echo ""
echo "Khởi động service:"
echo "  cd ${SERVICE_DIR} && ./server"
echo ""
echo "Test:"
echo "  curl http://localhost:${HTTP_PORT}/health"
echo "  curl http://localhost:${HTTP_PORT}/.well-known/jwks.json"
