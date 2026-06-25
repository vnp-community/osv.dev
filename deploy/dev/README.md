# OSV Platform — Deployment Guide

## Cấu trúc

```
deploy/dev/
├── docker-compose.yml          ← Local dev (build from source)
├── docker-compose.server.yml   ← Server infra + pre-built binary (CI/CD)
├── .env.example                ← Template cho local dev
├── README.md                   ← Tài liệu này
├── ui-nginx.conf               ← Nginx config cho frontend
├── c12.openledger.vn.conf      ← Nginx config cho production server
└── server/
    ├── docker-compose.yml      ← Production server (172.20.2.48)
    ├── .env.example            ← Template cho production env
    ├── .env                    ← Actual secrets (không commit)
    └── deploy.sh               ← Build + rsync + restart script
```

## Local Development

### Yêu cầu

- Docker 24+ và Docker Compose v2
- Go 1.22+
- Node.js 20+ (cho UI)

### Khởi động infra + service

```bash
# 1. Tạo .env từ template
cp deploy/dev/.env.example deploy/dev/.env

# 2. Khởi động tất cả services (bao gồm infra)
docker compose -f deploy/dev/docker-compose.yml up -d

# 3. Kiểm tra trạng thái
docker compose -f deploy/dev/docker-compose.yml ps
docker compose -f deploy/dev/docker-compose.yml logs -f osv-server
```

### Hoặc chạy native (không Docker)

```bash
# 1. Khởi động chỉ infrastructure (PostgreSQL, Redis, MongoDB, NATS, OpenSearch)
docker compose -f deploy/dev/docker-compose.yml up -d postgres redis mongodb nats opensearch

# 2. Bootstrap (tạo schema, RSA keys, admin account)
cp .env.bootstrap .env
# Chỉnh JWT_SECRET và INIT_ADMIN_PASSWORD
./scripts/bootstrap.sh

# 3. Build + start tất cả services
export PATH=$PATH:/usr/local/go/bin
for svc in services/*/; do
  (cd "$svc" && go build -o server ./cmd/server/ 2>/dev/null && echo "Built: $svc" || echo "Skip: $svc")
done
(cd apps/osv && go build -o server ./cmd/server/)

./scripts/start-all.sh
```

### Endpoints (sau khi start)

| Service | URL |
|---------|-----|
| Gateway (apps/osv) | http://localhost:8080 |
| identity-service HTTP | http://localhost:9101 |
| JWKS endpoint | http://localhost:9101/.well-known/jwks.json |
| data-service HTTP | http://localhost:8082 |
| search-service HTTP | http://localhost:8083 |
| ranking-service HTTP | http://localhost:8088 |
| notification-service HTTP | http://localhost:8086 |
| ai-service gRPC | localhost:50052 |
| Frontend (UI) | http://localhost:3001 |
| NATS monitoring | http://localhost:8222 |
| OpenSearch | http://localhost:9200 |

### Test login

```bash
curl -X POST http://localhost:9101/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@openvulnscan.io","password":"Admin@123!ChangeMe"}'
```

---

## Server Deployment (172.20.2.48)

### Workflow

```bash
# Từ máy local (macOS/Linux):
./deploy/dev/server/deploy.sh               # build + sync + restart
./deploy/dev/server/deploy.sh --build-only  # chỉ compile
./deploy/dev/server/deploy.sh --sync-only   # chỉ sync + restart
./deploy/dev/server/deploy.sh --nginx-only  # chỉ push nginx config
```

### First-time server setup

```bash
# 1. SSH vào server
ssh ubuntu@172.20.2.48

# 2. Tạo thư mục
sudo mkdir -p /opt/osv/bin /opt/osv/secrets
sudo chown -R ubuntu:ubuntu /opt/osv

# 3. Tạo .env từ example
cp /opt/osv/.env.example /opt/osv/.env
nano /opt/osv/.env   # thay POSTGRES_PASSWORD, JWT_SECRET, INIT_ADMIN_PASSWORD

# 4. Tạo RSA key pair cho JWT
openssl genrsa -out /opt/osv/secrets/jwt_private.pem 4096
openssl rsa -in /opt/osv/secrets/jwt_private.pem -pubout \
    -out /opt/osv/secrets/jwt_public.pem
chmod 600 /opt/osv/secrets/jwt_private.pem

# 5. Deploy binary (từ máy local)
./deploy/dev/server/deploy.sh

# 6. Start services
cd /opt/osv
docker compose -f docker-compose.yml up -d

# 7. Verify
curl http://localhost:8080/health
curl http://localhost:9101/.well-known/jwks.json
```

---

## Infrastructure

| Service | Image | Port |
|---------|-------|------|
| PostgreSQL 16 + pgvector | `pgvector/pgvector:pg16` | 5432 |
| MongoDB 7 | `mongo:7` | 27017 |
| Redis 7 | `redis:7-alpine` | 6379 |
| NATS 2.10 JetStream | `nats:2.10-alpine` | 4222 / 8222 |
| OpenSearch 2.13 | `opensearchproject/opensearch:2.13.0` | 9200 |

**Lưu ý**: `pgvector/pgvector:pg16` thay thế `postgres:16-alpine` — bắt buộc vì `cves.embedding vector(1536)` trong data-service schema.

---

## Biến môi trường quan trọng

| Biến | Mô tả | Ví dụ |
|------|-------|-------|
| `JWT_SECRET` | HMAC secret cho gateway JWT validation | `openssl rand -hex 32` |
| `JWT_PRIVATE_KEY_PATH` | RSA-4096 key cho identity-service RS256 | `/run/secrets/jwt_private.pem` |
| `POSTGRES_DSN` | PostgreSQL connection string | `postgres://osv:...@postgres:5432/osvdb` |
| `IDENTITY_DATABASE_URL` | Per-service PostgreSQL DSN (overrides POSTGRES_DSN) | same as POSTGRES_DSN |
| `INIT_ADMIN_EMAIL` | Email admin được tạo tự động lần đầu | `admin@example.com` |
| `INIT_ADMIN_PASSWORD` | Password admin được hash bằng Argon2id | strong password |
| `AI_BACKEND` | LLM provider: `ollama`, `openai`, `vertex` | `ollama` |
| `ALIAS_GROUP_BACKEND` | Storage cho alias groups: `postgres`, `firestore` | `postgres` |
| `FORCE_INSECURE` | Bỏ qua JWT_SECRET check (local dev only) | `true` |

Xem đầy đủ tại: [`.env.bootstrap`](../../.env.bootstrap)