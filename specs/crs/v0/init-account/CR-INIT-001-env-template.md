# CR-INIT-001 — File `.env` mẫu đầy đủ cho toàn hệ thống

## Mục tiêu

Tạo file `.env.bootstrap` tại root của project với toàn bộ biến môi trường cần thiết để **tất cả services** có thể khởi động và hoạt động ngay sau khi chạy `./bootstrap.sh`.

## File cần tạo/sửa

### [NEW] `.env.bootstrap` (root project)

File này là template đầy đủ. Người dùng copy thành `.env` và chỉnh sửa theo môi trường thực tế.

```bash
# =============================================================================
# OSV.dev — Bootstrap Environment Variables
# =============================================================================
# Hướng dẫn:
#   cp .env.bootstrap .env
#   # Chỉnh sửa các giá trị SECRET/PASSWORD theo môi trường của bạn
#   ./scripts/bootstrap.sh
# =============================================================================

# =============================================================================
# [CORE] PostgreSQL — Dùng cho: identity-service, data-service, notification-service
# =============================================================================
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=osv
POSTGRES_PASSWORD=osv_dev_secret_change_me
POSTGRES_DB=osvdb

# DSN tổng hợp (được dùng bởi nhiều services)
POSTGRES_DSN=postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable

# Per-service databases (mỗi service có schema riêng trong cùng một cluster)
# identity-service dùng schema "auth" trong osvdb
IDENTITY_DATABASE_URL=postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable&search_path=auth
# data-service dùng schema "vuln"
DATA_DATABASE_URL=postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable&search_path=vuln
# notification-service dùng schema "notification"
NOTIFICATION_DATABASE_URL=postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable

# =============================================================================
# [CORE] Redis — Dùng cho: identity-service (token cache), search-service (CPE cache)
# =============================================================================
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# URL dùng bởi identity-service
REDIS_URL=redis://:${REDIS_PASSWORD}@${REDIS_HOST}:${REDIS_PORT}/${REDIS_DB}
# Addr format dùng bởi search-service
REDIS_ADDR=${REDIS_HOST}:${REDIS_PORT}

# =============================================================================
# [CORE] MongoDB — Dùng cho: ranking-service
# =============================================================================
MONGO_URI=mongodb://localhost:27017
MONGO_DB=cvedb

# =============================================================================
# [CORE] NATS — Message bus cho event-driven communication
# =============================================================================
NATS_URL=nats://localhost:4222
# Đặt "true" để notification-service bắt buộc kết nối NATS (lỗi nếu không kết nối được)
NATS_ENABLED=false

# =============================================================================
# [CORE] OpenSearch — Dùng cho: search-service
# =============================================================================
OPENSEARCH_URL=http://localhost:9200
OPENSEARCH_INDEX=vulnerabilities
# Backend tìm kiếm: "opensearch" | "postgres" | "mongo" | "auto"
SEARCH_BACKEND=auto

# =============================================================================
# [IDENTITY SERVICE] — Authentication & Authorization
# Ports: HTTP=9101, gRPC=9001
# =============================================================================
# Port
IDENTITY_HTTP_PORT=9101
IDENTITY_GRPC_PORT=9001

# Đường dẫn đến RSA private key (PEM) để ký JWT
# Tạo bằng: openssl genrsa -out secrets/jwt_private.pem 4096
JWT_PRIVATE_KEY_PATH=secrets/jwt_private.pem

# JWT claims
JWT_ISSUER=https://auth.openvulnscan.io
JWT_AUDIENCE=openvulnscan

# Admin account mặc định — được tạo khi bootstrap lần đầu
INIT_ADMIN_EMAIL=admin@openvulnscan.io
INIT_ADMIN_USERNAME=admin
INIT_ADMIN_PASSWORD=Admin@123!ChangeMe

# OAuth2 providers (tùy chọn)
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=
GOOGLE_REDIRECT_URL=http://localhost:9101/api/v1/auth/oauth/google/callback

GITHUB_CLIENT_ID=
GITHUB_CLIENT_SECRET=
GITHUB_REDIRECT_URL=http://localhost:9101/api/v1/auth/oauth/github/callback

# LDAP (tùy chọn — bỏ trống nếu không dùng)
LDAP_HOST=
LDAP_PORT=389
LDAP_BASE_DN=
LDAP_USER_ATTR=uid
LDAP_USE_TLS=false

# =============================================================================
# [DATA SERVICE] — Vulnerability Data Platform
# Ports: HTTP=8080, gRPC=50053, Metrics=9092
# =============================================================================
DATA_HTTP_PORT=8080
DATA_GRPC_PORT=50053

# Storage backend cho alias groups: "firestore" | "postgres"
ALIAS_GROUP_BACKEND=postgres

# GCP (dùng khi ALIAS_GROUP_BACKEND=firestore)
GCP_PROJECT_ID=osv-local
FIRESTORE_EMULATOR_HOST=localhost:8200
GCS_BUCKET=osv-vulnerabilities-local

# Observability
OTLP_ENDPOINT=http://localhost:4318
OTEL_SAMPLING_RATE=1.0

# =============================================================================
# [SEARCH SERVICE] — Full-text Search Platform
# Ports: HTTP=8082, gRPC=50056, Metrics=9091
# =============================================================================
SEARCH_HTTP_PORT=8082
SEARCH_GRPC_PORT=50056

# =============================================================================
# [RANKING SERVICE] — CPE Ranking
# Port: HTTP=8088
# =============================================================================
RANKING_PORT=8088

# =============================================================================
# [NOTIFICATION SERVICE] — Alert & Webhook Platform
# Ports: HTTP=8086, gRPC=50063, Metrics=9094
# =============================================================================
NOTIFICATION_HTTP_PORT=8086
NOTIFICATION_GRPC_PORT=50063

# =============================================================================
# [AI SERVICE] — LLM Enrichment
# Port: gRPC=50052
# =============================================================================
AI_GRPC_PORT=50052

# Backend LLM: "ollama" | "vertex" | "openai"
AI_BACKEND=ollama
AI_MODEL=llama3
AI_BASE_URL=http://localhost:11434

# Vertex AI (chỉ khi AI_BACKEND=vertex)
VERTEX_PROJECT_ID=osv-local
VERTEX_LOCATION=us-central1

# OpenAI (chỉ khi AI_BACKEND=openai)
OPENAI_API_KEY=

# =============================================================================
# [GATEWAY SERVICE] — API Gateway
# Port: HTTP=8080
# =============================================================================
GATEWAY_HTTP_PORT=8080
GATEWAY_GRPC_PORT=9090

# Upstream service addresses (internal DNS / localhost)
DATA_SERVICE_HTTP=http://localhost:8080
SEARCH_SERVICE_HTTP=http://localhost:8082
NOTIFICATION_SERVICE_HTTP=http://localhost:8086

# =============================================================================
# [OSV APP — apps/osv] — Orchestrator / BFF Gateway
# Port: HTTP=8080
# =============================================================================
OSV_MODE=microservices
HTTP_PORT=8080

# gRPC upstreams
DATA_SERVICE_ADDR=localhost:50053
AI_SERVICE_ADDR=localhost:50052
FINDING_SERVICE_ADDR=localhost:50060
IDENTITY_SERVICE_ADDR=localhost:9001

# HTTP upstreams
NOTIFICATION_HTTP=http://localhost:8086
GATEWAY_HTTP=http://localhost:8080
DATA_SERVICE_HTTP=http://localhost:8080
SEARCH_SERVICE_HTTP=http://localhost:8082
RANKING_SERVICE_HTTP=http://localhost:8088
IDENTITY_SERVICE_HTTP=http://localhost:9101

# CORS
ALLOWED_ORIGINS=http://localhost:3001,http://localhost:5173

# JWT (dùng để validate token tại gateway)
JWKS_URL=http://localhost:9101/.well-known/jwks.json

# =============================================================================
# [OBSERVABILITY] — Grafana (monitoring)
# =============================================================================
GF_SECURITY_ADMIN_PASSWORD=admin_grafana_change_me
GF_AUTH_ANONYMOUS_ENABLED=true
```

## Các biến bắt buộc phải thay đổi trước khi production

| Biến | Lý do |
|------|-------|
| `POSTGRES_PASSWORD` | Không dùng default trong production |
| `INIT_ADMIN_PASSWORD` | Password admin mặc định phải thay |
| `JWT_PRIVATE_KEY_PATH` | Phải tạo key thực sự (xem CR-INIT-002) |
| `GF_SECURITY_ADMIN_PASSWORD` | Grafana admin password |

## Acceptance Criteria

- [ ] File `.env.bootstrap` tồn tại tại root project
- [ ] Tất cả biến từ tất cả services đều được cover
- [ ] Comment giải thích rõ ràng từng section
- [ ] Có hướng dẫn copy và sử dụng
