# TASK-INIT-001 — Tạo file `.env.bootstrap`

> **Solution**: [SOL-INIT-001](../solutions/SOL-INIT-001-env-bootstrap.md)  
> **Phạm vi**: Tạo mới 1 file, sửa 1 file  
> **Độ phức tạp**: Thấp — chỉ tạo/chỉnh sửa file text

---

## Mô tả

Tạo file `.env.bootstrap` tổng hợp tất cả biến môi trường cho toàn bộ hệ thống, và bổ sung các biến còn thiếu vào `apps/osv/.env.example`.

---

## Các bước thực thi

### Bước 1 — Tạo `.env.bootstrap`

**Tạo file mới**: `/Users/binhnt/Lab/sec/cve/osv.dev/.env.bootstrap`

Nội dung đầy đủ như trong SOL-INIT-001, bao gồm tất cả các section:

```
# PostgreSQL
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=osv
POSTGRES_PASSWORD=osv_dev_secret_CHANGE_ME
POSTGRES_DB=osvdb
POSTGRES_DSN=postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable
IDENTITY_DATABASE_URL=postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable
DATA_DATABASE_URL=postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable
NOTIFICATION_DATABASE_URL=postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0
REDIS_URL=redis://:${REDIS_PASSWORD}@${REDIS_HOST}:${REDIS_PORT}/${REDIS_DB}
REDIS_ADDR=${REDIS_HOST}:${REDIS_PORT}

# MongoDB
MONGO_URI=mongodb://localhost:27017
MONGO_DB=cvedb

# NATS
NATS_URL=nats://localhost:4222
NATS_ENABLED=false

# OpenSearch
OPENSEARCH_URL=http://localhost:9200
OPENSEARCH_INDEX=vulnerabilities
SEARCH_BACKEND=auto

# identity-service
IDENTITY_HTTP_PORT=9101
IDENTITY_GRPC_PORT=9001
JWT_PRIVATE_KEY_PATH=secrets/jwt_private.pem
JWT_ISSUER=https://auth.openvulnscan.io
JWT_AUDIENCE=openvulnscan
INIT_ADMIN_EMAIL=admin@openvulnscan.io
INIT_ADMIN_USERNAME=admin
INIT_ADMIN_PASSWORD=Admin@123!ChangeMe
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=
GOOGLE_REDIRECT_URL=http://localhost:9101/api/v1/auth/oauth/google/callback
GITHUB_CLIENT_ID=
GITHUB_CLIENT_SECRET=
GITHUB_REDIRECT_URL=http://localhost:9101/api/v1/auth/oauth/github/callback
LDAP_HOST=
LDAP_PORT=389
LDAP_BASE_DN=dc=example,dc=com
LDAP_USER_ATTR=uid
LDAP_USE_TLS=false
LDAP_SKIP_VERIFY=false

# data-service
DATA_HTTP_PORT=8082
DATA_GRPC_PORT=50053
ALIAS_GROUP_BACKEND=postgres
GCP_PROJECT_ID=osv-local
FIRESTORE_EMULATOR_HOST=localhost:8200
GCS_BUCKET=osv-vulnerabilities-local
OTLP_ENDPOINT=http://localhost:4318
OTEL_SAMPLING_RATE=1.0

# search-service
SEARCH_HTTP_PORT=8083
SEARCH_GRPC_PORT=50056

# ranking-service
RANKING_PORT=8088

# notification-service
NOTIFICATION_HTTP_PORT=8086
NOTIFICATION_GRPC_PORT=50063
SMTP_HOST=
SMTP_PORT=587
SMTP_USER=
SMTP_PASSWORD=

# ai-service
AI_GRPC_PORT=50052
AI_BACKEND=ollama
AI_MODEL=llama3
AI_BASE_URL=http://localhost:11434
VERTEX_PROJECT_ID=
VERTEX_LOCATION=us-central1
OPENAI_API_KEY=

# gateway-service
GATEWAY_HTTP_PORT=8080
GATEWAY_GRPC_PORT=9090
DATA_SERVICE_HTTP=http://localhost:8082
SEARCH_SERVICE_HTTP=http://localhost:8083
NOTIFICATION_SERVICE_HTTP=http://localhost:8086

# apps/osv
OSV_MODE=microservices
HTTP_PORT=8080
DATA_SERVICE_ADDR=localhost:50053
AI_SERVICE_ADDR=localhost:50052
FINDING_SERVICE_ADDR=localhost:50060
IDENTITY_SERVICE_ADDR=localhost:9001
NOTIFICATION_HTTP=http://localhost:8086
GATEWAY_HTTP=http://localhost:8080
RANKING_SERVICE_HTTP=http://localhost:8088
IDENTITY_SERVICE_HTTP=http://localhost:9101
JWKS_URL=http://localhost:9101/.well-known/jwks.json
JWT_SECRET=CHANGE_ME_use_openssl_rand_hex_32
ALLOWED_ORIGINS=http://localhost:3001,http://localhost:5173

# Observability
GF_SECURITY_ADMIN_PASSWORD=admin_grafana_CHANGE_ME
GF_AUTH_ANONYMOUS_ENABLED=true

# Bootstrap flags
FORCE_INSECURE=false
SKIP_INFRA_CHECK=false
BOOTSTRAP_TIMEOUT=30
```

### Bước 2 — Cập nhật `apps/osv/.env.example`

**Đọc** file hiện tại: `/Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/.env.example`

**Thêm vào cuối file** (nếu chưa có):

```bash
# ── EmbeddedInfra (required khi OSV_MODE=microservices) ─────────────────
POSTGRES_DSN=postgres://osv:osv_dev_secret_CHANGE_ME@localhost:5432/osvdb?sslmode=disable
REDIS_ADDR=redis://localhost:6379

# JWT signing secret cho embedded mode
# Lấy từ: openssl rand -hex 32
JWT_SECRET=CHANGE_ME_use_openssl_rand_hex_32

# JWKS endpoint của identity-service
JWKS_URL=http://localhost:9101/.well-known/jwks.json
JWT_AUDIENCE=openvulnscan
JWT_ISSUER=https://auth.openvulnscan.io
```

---

## Acceptance Criteria

- [ ] File `.env.bootstrap` tồn tại tại `/Users/binhnt/Lab/sec/cve/osv.dev/.env.bootstrap`
- [ ] Có đủ sections cho tất cả services (identity, data, search, ranking, notification, ai, gateway, osv)
- [ ] `apps/osv/.env.example` có đủ `POSTGRES_DSN`, `REDIS_ADDR`, `JWT_SECRET`, `JWKS_URL`

---

## Ghi chú thực thi

- Không cần `set -e` hay bất kỳ logic phức tạp — chỉ tạo/sửa file text
- Kiểm tra `apps/osv/.env.example` trước khi append để tránh duplicate
