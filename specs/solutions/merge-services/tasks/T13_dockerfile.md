# T13 — Dockerfiles

**Phase**: 13
**Depends on**: T12
**Estimated effort**: 1 hour

---

## Mục tiêu

Tạo/cập nhật `Dockerfile` cho tất cả 8 core services với multi-stage build chuẩn.

---

## Tác vụ chi tiết

### Template Dockerfile chuẩn

```dockerfile
# Dockerfile template cho tất cả Go services
# Multi-stage build: builder + minimal runtime

# Stage 1: Build
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy shared dependencies first (better cache)
COPY ../shared/pkg ./shared/pkg
COPY ../shared/proto ./shared/proto

# Copy service source
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /service ./cmd/server/

# Stage 2: Runtime
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /service /service

EXPOSE 8080 50051

ENTRYPOINT ["/service"]
```

### Bước 1: Tạo Dockerfile cho từng service

```bash
SVC_ROOT="/Users/binhnt/Lab/sec/cve/osv.dev/services"

create_dockerfile() {
  local SVC="$1"
  local HTTP_PORT="$2"
  local GRPC_PORT="$3"

  cat > "$SVC_ROOT/$SVC/Dockerfile" << EOF
# ============================================
# ${SVC} — Multi-stage Docker build
# ============================================

# Stage 1: Builder
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy shared libraries (cached layer)
COPY shared/pkg ./shared/pkg
COPY shared/proto ./shared/proto

# Copy service
COPY services/${SVC}/go.mod services/${SVC}/go.sum ./services/${SVC}/
WORKDIR /build/services/${SVC}
RUN go mod download

COPY services/${SVC}/ .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \\
    go build -ldflags="-s -w" -o /bin/service ./cmd/server/

# Stage 2: Runtime (minimal distroless)
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /bin/service /service
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

EXPOSE ${HTTP_PORT} ${GRPC_PORT}

ENTRYPOINT ["/service"]
EOF
  echo "Created Dockerfile for $SVC"
}

create_dockerfile "identity-service"    8081 50051
create_dockerfile "data-service"        8082 50052
create_dockerfile "search-service"      8083 50053
create_dockerfile "scan-service"        8084 50054
create_dockerfile "finding-service"     8085 50055
create_dockerfile "ai-service"          8086 50056
create_dockerfile "notification-service" 8087 50057
create_dockerfile "gateway-service"     8080 0
```

### Bước 2: Tạo .dockerignore cho từng service

```bash
DOCKERIGNORE_CONTENT="# .dockerignore
*.md
*.test
*_test.go
.git
.gitignore
specs/
deploy/
docs/
"

for svc in identity-service data-service search-service scan-service \
           finding-service ai-service notification-service gateway-service; do
  echo "$DOCKERIGNORE_CONTENT" > "$SVC_ROOT/$svc/.dockerignore"
  echo "Created .dockerignore for $svc"
done
```

### Bước 3: Tạo docker-compose.yml cho local development

```bash
cat > "$SVC_ROOT/../deploy/dev/docker-compose.yml" << 'EOF'
version: '3.9'

services:
  # Infrastructure
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: osv
      POSTGRES_PASSWORD: osv_dev
      POSTGRES_DB: osv
    ports: ["5432:5432"]
    volumes:
      - postgres_data:/var/lib/postgresql/data

  mongodb:
    image: mongo:7
    ports: ["27017:27017"]
    volumes:
      - mongo_data:/data/db

  redis:
    image: redis:7-alpine
    ports: ["6379:6379"]

  nats:
    image: nats:2.10-alpine
    command: ["-js", "-m", "8222"]
    ports: ["4222:4222", "8222:8222"]

  elasticsearch:
    image: docker.elastic.co/elasticsearch/elasticsearch:8.13.0
    environment:
      - discovery.type=single-node
      - xpack.security.enabled=false
      - ES_JAVA_OPTS=-Xms512m -Xmx512m
    ports: ["9200:9200"]

  # Core Services
  identity-service:
    build:
      context: ../..
      dockerfile: services/identity-service/Dockerfile
    ports: ["8081:8081", "50051:50051"]
    environment:
      POSTGRES_DSN: "postgres://osv:osv_dev@postgres:5432/osv"
      REDIS_ADDR: "redis:6379"
      JWT_SECRET: "dev-secret-change-in-prod"
    depends_on: [postgres, redis]

  data-service:
    build:
      context: ../..
      dockerfile: services/data-service/Dockerfile
    ports: ["8082:8082", "50052:50052"]
    environment:
      POSTGRES_DSN: "postgres://osv:osv_dev@postgres:5432/osv"
      MONGO_URI: "mongodb://mongodb:27017"
      NATS_URL: "nats://nats:4222"
    depends_on: [postgres, mongodb, nats]

  search-service:
    build:
      context: ../..
      dockerfile: services/search-service/Dockerfile
    ports: ["8083:8083", "50053:50053"]
    environment:
      ES_ADDR: "http://elasticsearch:9200"
      REDIS_ADDR: "redis:6379"
      NATS_URL: "nats://nats:4222"
    depends_on: [elasticsearch, redis, nats]

  scan-service:
    build:
      context: ../..
      dockerfile: services/scan-service/Dockerfile
    ports: ["8084:8084", "50054:50054"]
    environment:
      POSTGRES_DSN: "postgres://osv:osv_dev@postgres:5432/osv"
      REDIS_ADDR: "redis:6379"
      NATS_URL: "nats://nats:4222"
    depends_on: [postgres, redis, nats]

  finding-service:
    build:
      context: ../..
      dockerfile: services/finding-service/Dockerfile
    ports: ["8085:8085", "50055:50055"]
    environment:
      POSTGRES_DSN: "postgres://osv:osv_dev@postgres:5432/osv"
      MONGO_URI: "mongodb://mongodb:27017"
      NATS_URL: "nats://nats:4222"
    depends_on: [postgres, mongodb, nats]

  ai-service:
    build:
      context: ../..
      dockerfile: services/ai-service/Dockerfile
    ports: ["8086:8086", "50056:50056"]
    environment:
      REDIS_ADDR: "redis:6379"
      NATS_URL: "nats://nats:4222"
      OPENAI_API_KEY: "${OPENAI_API_KEY}"
    depends_on: [redis, nats]

  notification-service:
    build:
      context: ../..
      dockerfile: services/notification-service/Dockerfile
    ports: ["8087:8087", "50057:50057"]
    environment:
      POSTGRES_DSN: "postgres://osv:osv_dev@postgres:5432/osv"
      NATS_URL: "nats://nats:4222"
    depends_on: [postgres, nats]

  gateway-service:
    build:
      context: ../..
      dockerfile: services/gateway-service/Dockerfile
    ports: ["8080:8080"]
    environment:
      IDENTITY_SERVICE_ADDR: "identity-service:50051"
      DATA_SERVICE_ADDR: "data-service:50052"
      SEARCH_SERVICE_ADDR: "search-service:50053"
      SCAN_SERVICE_ADDR: "scan-service:50054"
      FINDING_SERVICE_ADDR: "finding-service:50055"
      AI_SERVICE_ADDR: "ai-service:50056"
      NOTIFICATION_SERVICE_ADDR: "notification-service:50057"
      REDIS_ADDR: "redis:6379"
    depends_on:
      - identity-service
      - data-service
      - search-service
      - scan-service
      - finding-service
      - ai-service
      - notification-service

volumes:
  postgres_data:
  mongo_data:
EOF
echo "Created docker-compose.yml for local development"
```

### Bước 4: Build test (optional — cần Docker daemon)

```bash
# Test build một service (optional)
cd /Users/binhnt/Lab/sec/cve/osv.dev
docker build -f services/identity-service/Dockerfile -t osv/identity-service:test . 2>&1 | tail -20
```

---

## Điều kiện hoàn thành

- [ ] `Dockerfile` tồn tại trong tất cả 8 services
- [ ] `.dockerignore` tồn tại trong tất cả 8 services
- [ ] `deploy/dev/docker-compose.yml` với tất cả 8 services + infrastructure
- [ ] Docker build thành công cho ít nhất 1 service (smoke test)

---

## Commit message

```
feat(docker): add Dockerfiles and docker-compose for all 8 services

- Multi-stage build (golang:1.26-alpine + distroless runtime)
- .dockerignore per service
- docker-compose.yml for local development with all dependencies
- All services exposed on correct ports
```
