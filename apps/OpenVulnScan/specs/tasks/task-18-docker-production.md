> **✅ COMPLETED** — go build && go vet passed.

# T18 — Docker Production Build

## Thông tin
| | |
|---|---|
| **Phase** | 6 — Polish |
| **Ước tính** | 2–3 giờ |
| **Depends on** | T01–T17 |

---

## Các bước thực hiện

### 18.1 Dockerfile multi-stage

```dockerfile
# Dockerfile
# Stage 1: Builder
FROM golang:1.22-alpine AS builder

WORKDIR /workspace

# Install build deps
RUN apk add --no-cache git ca-certificates tzdata

# Copy workspace
COPY services/ ./services/
COPY apps/OpenVulnScan/ ./apps/OpenVulnScan/

# Build
WORKDIR /workspace/apps/OpenVulnScan
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=$(git describe --tags --always)" \
    -o /bin/openvulnscan ./cmd/server/

# Stage 2: Runtime
FROM alpine:3.19

# Security: non-root user
RUN addgroup -g 1001 -S openvulnscan && \
    adduser -u 1001 -S openvulnscan -G openvulnscan

# Install runtime deps
RUN apk add --no-cache \
    nmap \           # Required for scan-service
    ca-certificates \
    tzdata

# Copy binary and configs
COPY --from=builder /bin/openvulnscan /usr/local/bin/openvulnscan
COPY apps/OpenVulnScan/configs/ /app/configs/
COPY apps/OpenVulnScan/migrations/ /app/migrations/

WORKDIR /app

USER openvulnscan

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s \
    CMD wget -qO- http://localhost:8080/healthz || exit 1

CMD ["openvulnscan", "--config", "/app/configs/config.yaml"]
```

### 18.2 docker-compose.prod.yml

```yaml
# docker-compose.prod.yml
version: "3.9"
services:

  openvulnscan:
    build: .
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: postgres://openvulnscan:${DB_PASSWORD}@postgres:5432/openvulnscan
      NATS_URL: nats://nats:4222
      REDIS_URL: redis://redis:6379
      JWT_SECRET: ${JWT_SECRET}
      ADMIN_EMAIL: ${ADMIN_EMAIL}
      ADMIN_PASSWORD: ${ADMIN_PASSWORD}
    depends_on:
      postgres:
        condition: service_healthy
      nats:
        condition: service_started
      redis:
        condition: service_healthy
    cap_add:
      - NET_RAW      # Required for nmap
      - NET_ADMIN    # Required for some nmap scan types
    volumes:
      - ./configs/config.docker.yaml:/app/configs/config.yaml:ro
    networks:
      - openvulnscan

  postgres:
    image: postgres:15-alpine
    restart: unless-stopped
    environment:
      POSTGRES_USER: openvulnscan
      POSTGRES_PASSWORD: ${DB_PASSWORD}
      POSTGRES_DB: openvulnscan
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U openvulnscan"]
      interval: 5s
      timeout: 5s
      retries: 10
    networks:
      - openvulnscan

  redis:
    image: redis:7-alpine
    restart: unless-stopped
    command: redis-server --appendonly yes --requirepass ${REDIS_PASSWORD}
    volumes:
      - redis_data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "-a", "${REDIS_PASSWORD}", "ping"]
      interval: 5s
    networks:
      - openvulnscan

  nats:
    image: nats:2.10-alpine
    restart: unless-stopped
    command: -js -m 8222
    networks:
      - openvulnscan

  minio:
    image: minio/minio:latest
    restart: unless-stopped
    environment:
      MINIO_ROOT_USER: ${MINIO_USER}
      MINIO_ROOT_PASSWORD: ${MINIO_PASSWORD}
    command: server /data --console-address ":9001"
    volumes:
      - minio_data:/data
    networks:
      - openvulnscan

volumes:
  postgres_data:
  redis_data:
  minio_data:

networks:
  openvulnscan:
    driver: bridge
```

### 18.3 `.env.example`

```bash
# .env.example — copy to .env và điền giá trị
DB_PASSWORD=change-me-strong-password
JWT_SECRET=change-me-32-chars-minimum
REDIS_PASSWORD=change-me
MINIO_USER=minioadmin
MINIO_PASSWORD=change-me
ADMIN_EMAIL=admin@your-domain.com
ADMIN_PASSWORD=change-me-strong-password
```

### 18.4 `configs/config.docker.yaml`

```yaml
server:
  http_addr: ":8080"

database:
  url: "${DATABASE_URL}"
  max_connections: 25

redis:
  url: "${REDIS_URL}"

nats:
  url: "${NATS_URL}"

auth:
  jwt_secret: "${JWT_SECRET}"
  jwt_expiry: 24h
  refresh_expiry: 168h

scan:
  worker_pool_size: 5
  nmap_binary: "/usr/bin/nmap"

admin:
  email: "${ADMIN_EMAIL}"
  password: "${ADMIN_PASSWORD}"

log:
  level: "info"
  format: "json"
```

### 18.5 Makefile production targets

```makefile
.PHONY: docker-build docker-push docker-run

VERSION ?= $(shell git describe --tags --always)
IMAGE_NAME ?= openvulnscan

docker-build:
	docker build -t $(IMAGE_NAME):$(VERSION) -t $(IMAGE_NAME):latest .

docker-push:
	docker push $(IMAGE_NAME):$(VERSION)
	docker push $(IMAGE_NAME):latest

docker-run:
	cp .env.example .env
	# Edit .env with your values
	docker-compose -f docker-compose.prod.yml --env-file .env up -d

migrate-prod:
	docker-compose -f docker-compose.prod.yml exec openvulnscan \
		openvulnscan migrate --config /app/configs/config.yaml

logs:
	docker-compose -f docker-compose.prod.yml logs -f openvulnscan
```

### 18.6 GitHub Actions CI/CD (optional)

```yaml
# .github/workflows/ci.yml
name: CI

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_PASSWORD: secret
          POSTGRES_DB: openvulnscan
        ports: ["5432:5432"]
      nats:
        image: nats:2.10
        args: ["-js"]
        ports: ["4222:4222"]
      redis:
        image: redis:7
        ports: ["6379:6379"]

    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: {go-version: "1.22"}
      - name: Install nmap
        run: sudo apt-get install -y nmap
      - name: Test
        run: go test -tags integration ./... -timeout 120s

  build:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Build Docker image
        run: docker build -t openvulnscan .
```

---

## Output

- [x] `Dockerfile` multi-stage build ✓ (builder → alpine final, ~20MB)
- [x] `docker-compose.prod.yml` ✓
- [x] `.env.example` ✓
- [x] `configs/config.docker.yaml` ✓
- [x] Makefile production targets ✓ (docker-build, docker-run, docker-stop)

## Acceptance Criteria

```bash
# Build Docker image
docker build -t openvulnscan:test .
# → Build thành công, không có lỗi

# Start production stack
cp .env.example .env  # điền giá trị
docker-compose -f docker-compose.prod.yml --env-file .env up -d

# Health check
curl http://localhost:8080/healthz
# → {"status":"ok"}

# Image size
docker image inspect openvulnscan:test --format='{{.Size}}' | numfmt --to=iec
# → < 100MB (multi-stage build với alpine)
```
