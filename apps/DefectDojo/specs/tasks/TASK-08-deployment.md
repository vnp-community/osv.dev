# TASK-08: Deployment & Operations

**Phase**: 8 — Production Readiness  
**Ước tính**: 8 giờ  
**Phụ thuộc**: TASK-07 (tests pass)  
**Output**: Dockerfile, Docker Compose, Makefile, monitoring, CI/CD

---

## Mục tiêu

Đóng gói ứng dụng thành Docker image, cấu hình deployment với Docker Compose và Kubernetes, thiết lập monitoring với Prometheus + Grafana.

---

## T-08.1: Dockerfile

**File**: `apps/DefectDojo/Dockerfile`  
**Ước tính**: 1h

```dockerfile
# ── Stage 1: Builder ──────────────────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

WORKDIR /workspace

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go.work and all module files first (for caching)
COPY services/go.work ./services/
COPY services/shared ./services/shared/
COPY services/auth-service/go.mod services/auth-service/go.sum ./services/auth-service/
COPY services/finding-service/go.mod services/finding-service/go.sum ./services/finding-service/
COPY services/product-service/go.mod services/product-service/go.sum ./services/product-service/
COPY services/scan-service/go.mod services/scan-service/go.sum ./services/scan-service/
COPY services/notification-service/go.mod services/notification-service/go.sum ./services/notification-service/
COPY services/report-service/go.mod services/report-service/go.sum ./services/report-service/
COPY services/integration-service/go.mod services/integration-service/go.sum ./services/integration-service/
COPY services/vulnerability-service/go.mod services/vulnerability-service/go.sum ./services/vulnerability-service/
COPY services/search-service/go.mod services/search-service/go.sum ./services/search-service/
COPY services/ingestion-service/go.mod services/ingestion-service/go.sum ./services/ingestion-service/
COPY services/ai-service/go.mod services/ai-service/go.sum ./services/ai-service/
COPY services/impact-service/go.mod services/impact-service/go.sum ./services/impact-service/
COPY apps/DefectDojo/go.mod apps/DefectDojo/go.sum ./apps/DefectDojo/

# Download dependencies
RUN cd /workspace/apps/DefectDojo && go mod download

# Copy all source code
COPY services ./services/
COPY apps/DefectDojo ./apps/DefectDojo/

# Build the binary
RUN cd /workspace && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
      -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
      -o /bin/defectdojo \
      ./apps/DefectDojo/cmd/defectdojo/

# ── Stage 2: Runner ───────────────────────────────────────────────────────────
FROM alpine:3.19

# Security: run as non-root
RUN adduser -D -s /bin/sh -u 1000 defectdojo

# TLS certificates for HTTPS external calls
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /bin/defectdojo /app/defectdojo

USER defectdojo

EXPOSE 8080 9090 9091

HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/defectdojo"]
```

**Tasks**:
- [ ] Multi-stage build (builder + runner)
- [ ] Non-root user
- [ ] Health check
- [ ] `docker build` thành công
- [ ] Image size < 50MB

---

## T-08.2: Docker Compose — Development

**File**: `apps/DefectDojo/docker-compose.yml`  
**Ước tính**: 1h

```yaml
version: "3.9"

x-logging: &logging
  logging:
    driver: "json-file"
    options:
      max-size: "20m"
      max-file: "3"

services:
  # ── Infrastructure ──────────────────────────────────────────────────────────
  postgres:
    image: pgvector/pgvector:pg16
    environment:
      POSTGRES_USER: defectdojo
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-changeme}
      POSTGRES_DB: defectdojo
    volumes:
      - pgdata:/var/lib/postgresql/data
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U defectdojo"]
      interval: 5s
      timeout: 5s
      retries: 10
    <<: *logging

  redis:
    image: redis:7-alpine
    command: redis-server --save 60 1 --loglevel warning
    volumes:
      - redisdata:/data
    ports:
      - "6379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
    <<: *logging

  nats:
    image: nats:2.10-alpine
    command: ["-js", "-m", "8222", "--store_dir", "/data"]
    volumes:
      - natsdata:/data
    ports:
      - "4222:4222"
      - "8222:8222"
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8222/healthz"]
      interval: 5s
    <<: *logging

  opensearch:
    image: opensearchproject/opensearch:2
    environment:
      - discovery.type=single-node
      - DISABLE_SECURITY_PLUGIN=true
      - "OPENSEARCH_JAVA_OPTS=-Xms512m -Xmx512m"
    volumes:
      - opensearchdata:/usr/share/opensearch/data
    ports:
      - "9200:9200"
    healthcheck:
      test: ["CMD-SHELL", "curl -s http://localhost:9200/_cluster/health | grep -q '\"status\":\"green\"'"]
      interval: 10s
      timeout: 5s
      retries: 10
    <<: *logging

  # ── Application ──────────────────────────────────────────────────────────────
  defectdojo:
    build:
      context: ../..
      dockerfile: apps/DefectDojo/Dockerfile
    ports:
      - "${HTTP_PORT:-8080}:8080"
      - "${GRPC_PORT:-9090}:9090"
      - "9091:9091"    # Prometheus metrics
    env_file:
      - .env
    environment:
      POSTGRES_URL: postgres://defectdojo:${POSTGRES_PASSWORD:-changeme}@postgres/defectdojo?sslmode=disable
      NATS_URL: nats://nats:4222
      REDIS_URL: redis://redis:6379
      OPENSEARCH_URL: http://opensearch:9200
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
      nats:
        condition: service_healthy
      opensearch:
        condition: service_healthy
    <<: *logging

  # ── Monitoring (optional, comment out if not needed) ─────────────────────────
  prometheus:
    image: prom/prometheus:v2.51.0
    volumes:
      - ./monitoring/prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - prometheusdata:/prometheus
    ports:
      - "9090:9090"

  grafana:
    image: grafana/grafana:10.4.0
    environment:
      GF_SECURITY_ADMIN_PASSWORD: ${GRAFANA_PASSWORD:-admin}
    volumes:
      - grafanadata:/var/lib/grafana
      - ./monitoring/grafana:/etc/grafana/provisioning
    ports:
      - "3000:3000"
    depends_on:
      - prometheus

volumes:
  pgdata:
  redisdata:
  natsdata:
  opensearchdata:
  prometheusdata:
  grafanadata:
```

**Tasks**:
- [ ] Docker Compose với healthchecks
- [ ] `docker-compose up -d` thành công
- [ ] App healthy sau 60s
- [ ] `GET http://localhost:8080/health` returns 200

---

## T-08.3: Prometheus Configuration

**File**: `apps/DefectDojo/monitoring/prometheus.yml`  
**Ước tính**: 0.5h

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

rule_files:
  - "alerts.yml"

scrape_configs:
  - job_name: 'defectdojo'
    static_configs:
      - targets: ['defectdojo:9091']
    metrics_path: '/metrics'

  - job_name: 'postgres'
    static_configs:
      - targets: ['postgres-exporter:9187']

  - job_name: 'nats'
    static_configs:
      - targets: ['nats:8222']
    metrics_path: '/metrics'
```

**File**: `apps/DefectDojo/monitoring/alerts.yml`

```yaml
groups:
  - name: defectdojo
    rules:
      - alert: ServiceDown
        expr: dd_service_health == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "DefectDojo service {{ $labels.service }} is down"

      - alert: HighSLABreachRate
        expr: rate(dd_sla_breaches_total[1h]) > 10
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High SLA breach rate detected"

      - alert: ScanProcessingSlowdown
        expr: histogram_quantile(0.95, rate(dd_scan_processing_seconds_bucket[5m])) > 300
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Scan processing is slow (p95 > 5 minutes)"
```

---

## T-08.4: Grafana Dashboard

**File**: `apps/DefectDojo/monitoring/grafana/dashboards/defectdojo.json`  
**Ước tính**: 1h

Dashboard panels:
- [ ] Service health grid (1=green/0=red per service)
- [ ] Findings created per hour (by severity)
- [ ] Scan processing time (p50, p95, p99)
- [ ] API request rate (by endpoint)
- [ ] API error rate (4xx, 5xx)
- [ ] NATS message throughput
- [ ] Active SLA breaches
- [ ] DB connection pool usage

---

## T-08.5: CI/CD Pipeline

**File**: `apps/DefectDojo/.github/workflows/ci.yml` (hoặc tương đương)  
**Ước tính**: 1h

```yaml
name: DefectDojo Go CI

on:
  push:
    paths:
      - 'apps/DefectDojo/**'
      - 'services/**'
  pull_request:
    paths:
      - 'apps/DefectDojo/**'
      - 'services/**'

jobs:
  test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: pgvector/pgvector:pg16
        env:
          POSTGRES_PASSWORD: test
          POSTGRES_DB: defectdojo_test
        options: >-
          --health-cmd pg_isready
          --health-interval 5s
          --health-retries 10
      nats:
        image: nats:2.10-alpine
        options: "--entrypoint nats-server -- -js"
      redis:
        image: redis:7-alpine
        options: >-
          --health-cmd "redis-cli ping"
          --health-interval 5s

    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26'

      - name: Unit tests
        run: |
          cd apps/DefectDojo
          go test ./... -v -timeout 60s

      - name: Integration tests
        run: |
          cd apps/DefectDojo
          go test ./tests/integration/... -tags=integration -timeout 300s
        env:
          TEST_POSTGRES_URL: postgres://postgres:test@localhost:5432/defectdojo_test?sslmode=disable
          TEST_NATS_URL: nats://localhost:4222
          TEST_REDIS_URL: redis://localhost:6379

      - name: Build Docker image
        run: |
          docker build -f apps/DefectDojo/Dockerfile -t defectdojo-go:test .
```

---

## T-08.6: Kubernetes Manifests (Optional)

**Directory**: `apps/DefectDojo/k8s/`  
**Ước tính**: 1h

Files:
- [ ] `k8s/namespace.yaml`
- [ ] `k8s/secret.yaml` (template, values from Vault/external secrets)
- [ ] `k8s/configmap.yaml`
- [ ] `k8s/deployment.yaml`
- [ ] `k8s/service.yaml`
- [ ] `k8s/ingress.yaml`
- [ ] `k8s/hpa.yaml` (HorizontalPodAutoscaler — scale replicas nếu cần)
- [ ] `k8s/pdb.yaml` (PodDisruptionBudget)

**Helm chart** (alternative):
```bash
mkdir -p apps/DefectDojo/helm/defectdojo-go/{templates,charts}
```

---

## T-08.7: Operations Runbook

**File**: `apps/DefectDojo/OPERATIONS.md`  
**Ước tính**: 1h

Nội dung:
- [ ] **Quick Start**: `docker-compose up -d` → ready in 60s
- [ ] **First User Setup**: How to create admin user
- [ ] **Import Scan**: curl example
- [ ] **Backup**: pg_dump procedure
- [ ] **Health Check**: How to verify all services healthy
- [ ] **Log Aggregation**: How to query structured logs
- [ ] **Scaling**: Khi nào cần scale và scale cái gì
- [ ] **Troubleshooting**:
  - Finding service down → check postgres connectivity
  - NATS reconnect loop → check NATS server health
  - Slow imports → check scan worker pool size

---

## T-08.8: Production Checklist

Kiểm tra trước khi deploy production:

```
Security:
- [ ] JWT_SECRET >= 64 chars, randomly generated
- [ ] JIRA_ENCRYPTION_KEY is 32 bytes AES key
- [ ] PostgreSQL password is strong
- [ ] SMTP credentials are app-specific password
- [ ] Non-root Docker user
- [ ] TLS certificate configured

Performance:
- [ ] PostgreSQL max_connections tuned
- [ ] NATS max_file_store limited
- [ ] App resource limits set (CPU, Memory)
- [ ] Worker pool size tuned for scan throughput

Reliability:
- [ ] Health checks pass
- [ ] Graceful shutdown works (SIGTERM → 30s drain)
- [ ] Database backup configured
- [ ] NATS data persisted (not ephemeral)
- [ ] Log aggregation configured (Loki/ELK/CloudWatch)

Monitoring:
- [ ] Prometheus scraping metrics
- [ ] Alertmanager configured
- [ ] Grafana dashboard imported
- [ ] PagerDuty/OpsGenie integrated
```

---

## Definition of Done — TASK-08

- [ ] T-08.1 Dockerfile builds (<50MB image)
- [ ] T-08.2 Docker Compose: `up -d` → app healthy in 60s
- [ ] T-08.3 Prometheus config với alert rules
- [ ] T-08.4 Grafana dashboard với 8+ panels
- [ ] T-08.5 CI pipeline runs unit + integration tests
- [ ] T-08.6 K8s manifests (deployment, service, ingress)
- [ ] T-08.7 Operations runbook
- [ ] T-08.8 Production checklist completed
- [ ] `docker-compose up` → `GET /health` returns `{"status":"ok"}`
- [ ] All metrics visible in Prometheus at `:9091/metrics`
