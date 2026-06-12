# Infrastructure & Dependencies

## 1. Stack cơ sở hạ tầng

| Component      | Vai trò                                        | Version đề xuất  |
|----------------|------------------------------------------------|------------------|
| **PostgreSQL** | Primary database (scans, findings, users, assets) | 15+           |
| **Redis**      | JWT session revocation, rate limiting, cache  | 7+               |
| **NATS JetStream** | Async message bus giữa các modules         | 2.10+            |
| **MinIO / S3** | Lưu PDF reports, nmap raw output               | Latest           |

---

## 2. Packages Go được dùng (từ services gốc)

### Từ `shared/pkg`

```go
github.com/osv/shared/pkg/config        // Viper-based config loader
github.com/osv/shared/pkg/database      // pgx connection pool wrapper
github.com/osv/shared/pkg/logger        // zerolog setup
github.com/osv/shared/pkg/nats          // NATS JetStream client
github.com/osv/shared/pkg/grpcutil      // gRPC server/client helpers
github.com/osv/shared/pkg/middleware     // HTTP middleware (logging, tracing)
github.com/osv/shared/pkg/health        // /healthz endpoint
github.com/osv/shared/pkg/resilience    // circuit breaker, retry
github.com/osv/shared/pkg/severity      // CVSS → severity mapping
github.com/osv/shared/pkg/pagination    // Cursor-based pagination
github.com/osv/shared/pkg/errors        // Domain errors
github.com/osv/shared/pkg/observability // OpenTelemetry tracing/metrics
```

### Từ `shared/proto`

```go
github.com/osv/shared/proto/auth        // AuthService proto + generated code
github.com/osv/shared/proto/scan        // ScanService proto + generated code
github.com/osv/shared/proto/finding     // FindingService proto
github.com/osv/shared/proto/product     // ProductService proto (assets)
github.com/osv/shared/proto/reporter    // ReportService proto
github.com/osv/shared/proto/cve         // CVE proto
```

---

## 3. External dependencies

```go
// HTTP
github.com/go-chi/chi/v5              // Router
github.com/go-chi/cors                // CORS middleware
github.com/go-chi/httprate            // Rate limiting

// Auth
github.com/golang-jwt/jwt/v5          // JWT
golang.org/x/oauth2                    // Google OAuth2
github.com/pquerna/otp                // TOTP/MFA

// Database
github.com/jackc/pgx/v5               // PostgreSQL driver
github.com/jackc/pgx/v5/stdlib        // database/sql compat

// Cache
github.com/redis/go-redis/v9          // Redis client

// Messaging
github.com/nats-io/nats.go            // NATS client + JetStream

// Crypto
golang.org/x/crypto                    // bcrypt

// Logging
github.com/rs/zerolog                  // Structured logging

// gRPC
google.golang.org/grpc                 // gRPC framework
google.golang.org/protobuf             // Protobuf

// Utilities
github.com/google/uuid                 // UUID generation
github.com/robfig/cron/v3              // Cron scheduler

// PDF (report-service)
// (tùy theo implementation của report-service)

// External API
// OSV API: https://api.osv.dev/v1/query (HTTP client, không cần library)
```

---

## 4. Docker Compose

```yaml
# docker-compose.yml
version: "3.9"
services:

  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_USER: openvulnscan
      POSTGRES_PASSWORD: secret
      POSTGRES_DB: openvulnscan
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    command: redis-server --appendonly yes

  nats:
    image: nats:2.10-alpine
    ports:
      - "4222:4222"   # Client connections
      - "8222:8222"   # HTTP monitoring
    command: -js -m 8222  # Enable JetStream

  minio:
    image: minio/minio:latest
    ports:
      - "9000:9000"
      - "9001:9001"
    environment:
      MINIO_ROOT_USER: minioadmin
      MINIO_ROOT_PASSWORD: minioadmin
    command: server /data --console-address ":9001"
    volumes:
      - minio_data:/data

  openvulnscan:
    build: .
    ports:
      - "8080:8080"
      - "9090:9090"   # gRPC (optional)
    environment:
      - DATABASE_URL=postgres://openvulnscan:secret@postgres:5432/openvulnscan
      - REDIS_URL=redis://redis:6379
      - NATS_URL=nats://nats:4222
      - MINIO_ENDPOINT=minio:9000
      - JWT_SECRET=change-in-production
    depends_on:
      - postgres
      - redis
      - nats
      - minio

volumes:
  postgres_data:
  minio_data:
```

---

## 5. Cấu hình ứng dụng

```yaml
# configs/config.yaml
server:
  http_addr: ":8080"
  grpc_addr: ":9090"    # Optional
  read_timeout: 30s
  write_timeout: 30s

database:
  url: "postgres://openvulnscan:secret@localhost:5432/openvulnscan"
  max_connections: 25
  max_idle_connections: 5

redis:
  url: "redis://localhost:6379"
  db: 0

nats:
  url: "nats://localhost:4222"
  streams:
    - name: SCAN_EVENTS
      subjects: ["scan.*"]
      retention: WorkQueuePolicy
      replicas: 1
    - name: AGENT_EVENTS
      subjects: ["agent.*"]
      retention: WorkQueuePolicy
    - name: NOTIFY_EVENTS
      subjects: ["notification.*"]
      retention: WorkQueuePolicy

storage:
  type: minio    # hoặc "local" cho dev
  endpoint: "localhost:9000"
  bucket: "openvulnscan"
  access_key: "minioadmin"
  secret_key: "minioadmin"

auth:
  jwt_secret: "change-in-production"
  jwt_expiry: 24h
  refresh_expiry: 168h    # 7 days
  google_client_id: ""
  google_client_secret: ""
  google_redirect_url: "http://localhost:8080/api/v1/auth/google/callback"

scan:
  worker_pool_size: 5     # Concurrent scan workers
  nmap_binary: "/usr/bin/nmap"
  zap_api_url: "http://zap:8090"
  default_timeout: 300    # seconds
  max_targets_per_scan: 100

admin:
  email: "admin@openvulnscan.local"
  password: "admin123"    # Thay đổi sau khi deploy!

siem:
  enabled: false
  host: ""
  port: 514
  protocol: "udp"    # hoặc "tcp"

notification:
  email:
    enabled: false
    smtp_host: ""
    smtp_port: 587
    from: "openvulnscan@example.com"
  webhook:
    enabled: false
    url: ""
    secret: ""

log:
  level: "info"    # debug|info|warn|error
  format: "json"   # json|pretty
```

---

## 6. Dockerfile

```dockerfile
# Build stage
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git protoc

WORKDIR /build

# Copy go.work và toàn bộ services (để resolve replace directives)
COPY services/ ./services/
COPY apps/OpenVulnScan/ ./apps/OpenVulnScan/

WORKDIR /build/apps/OpenVulnScan
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o openvulnscan ./cmd/server/

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache nmap ca-certificates tzdata

WORKDIR /app

COPY --from=builder /build/apps/OpenVulnScan/openvulnscan .
COPY --from=builder /build/apps/OpenVulnScan/configs/ ./configs/
COPY --from=builder /build/apps/OpenVulnScan/web/ ./web/

EXPOSE 8080 9090

ENTRYPOINT ["./openvulnscan"]
```

---

## 7. Database Migrations

Database schema được tổng hợp từ migrations của các services:

```
migrations/
├── 001_create_users.sql          ← từ auth-service/migrations
├── 002_create_scans.sql          ← từ scan-service/migrations
├── 003_create_findings.sql       ← từ finding-service/migrations
├── 004_create_assets.sql         ← từ product-service/migrations
├── 005_create_cves.sql           ← từ vulnerability-service/migrations
├── 006_create_notifications.sql  ← từ notification-service/migrations
├── 007_create_agent_reports.sql  ← từ ingestion-service/migrations
├── 008_create_tags.sql           ← NEW (từ Python models/tag.py)
├── 009_create_siem_config.sql    ← NEW (từ Python models/siem_config.py)
└── 010_create_scheduled_scans.sql ← từ scan-service/migrations
```

Chạy migration với `golang-migrate` hoặc script custom trong `cmd/server/main.go`.

---

## 8. Yêu cầu hệ thống (Runtime)

| Yêu cầu        | Chi tiết                                           |
|----------------|----------------------------------------------------|
| OS             | Linux (production), macOS/Windows (development)   |
| `nmap`         | Phải được cài sẵn trên host hoặc trong container  |
| `OWASP ZAP`    | Optional, chỉ cần cho web scan (`scan_type=web`)  |
| RAM tối thiểu  | 512MB (không có ZAP), 2GB (với ZAP)               |
| Network        | Quyền thực hiện port scan (có thể cần root/CAP_NET_RAW) |
