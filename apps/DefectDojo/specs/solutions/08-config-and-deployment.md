# Configuration & Deployment

## Environment Variables

### Core Infrastructure

```bash
# Database
POSTGRES_URL=postgres://defectdojo:password@localhost:5432/defectdojo?sslmode=disable

# NATS JetStream
NATS_URL=nats://localhost:4222

# Redis (session store, rate limiting)
REDIS_URL=redis://localhost:6379

# OpenSearch (full-text search)
OPENSEARCH_URL=http://localhost:9200

# Security
JWT_SECRET=your-super-secret-jwt-key-min-256-bits
JIRA_ENCRYPTION_KEY=32-byte-aes-key-for-jira-password
```

### Service Ports

```bash
# HTTP REST API (DefectDojo v2 compatible)
HTTP_PORT=8080

# gRPC (for external gRPC clients)
GRPC_PORT=9090
```

### AI Service

```bash
AI_BACKEND=ollama          # ollama | openai | azure | anthropic
AI_MODEL=llama3            # model name
AI_BASE_URL=http://localhost:11434  # Ollama URL
AI_API_KEY=sk-...          # OpenAI/Azure/Anthropic API key
```

### Authentication

```bash
JWT_EXPIRY=24h              # JWT token expiry
REFRESH_EXPIRY=7d           # Refresh token expiry

# OAuth2 Providers (optional)
OAUTH_GOOGLE_KEY=your-google-client-id
OAUTH_GOOGLE_SECRET=your-google-client-secret
OAUTH_GITHUB_KEY=your-github-client-id
OAUTH_GITHUB_SECRET=your-github-client-secret
OAUTH_GITLAB_KEY=your-gitlab-client-id
OAUTH_GITLAB_SECRET=your-gitlab-client-secret
```

### Notification Channels

```bash
# Email (SMTP)
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=notifications@yourcompany.com
SMTP_PASSWORD=your-smtp-password
SMTP_FROM=DefectDojo <notifications@yourcompany.com>
SMTP_USE_TLS=true

# Slack
SLACK_TOKEN=xoxb-your-slack-token

# MS Teams
TEAMS_WEBHOOK_URL=https://outlook.office.com/webhook/...
```

### Observability

```bash
# Prometheus metrics
METRICS_ENABLED=true
METRICS_PORT=9091

# Structured logging
LOG_LEVEL=info              # debug | info | warn | error
LOG_FORMAT=json             # json | console

# OpenTelemetry tracing (optional)
OTEL_ENABLED=false
OTEL_EXPORTER_OTLP_ENDPOINT=http://jaeger:4317
OTEL_SERVICE_NAME=defectdojo-go
```

## Docker Compose — Production

```yaml
# apps/DefectDojo/docker-compose.prod.yml
version: "3.9"

x-logging: &logging
  logging:
    driver: "json-file"
    options:
      max-size: "50m"
      max-file: "5"

services:
  # ── Infrastructure ──────────────────────────────────────────────────────────
  postgres:
    image: pgvector/pgvector:pg16
    restart: unless-stopped
    environment:
      POSTGRES_USER: ${POSTGRES_USER:-defectdojo}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      POSTGRES_DB: ${POSTGRES_DB:-defectdojo}
    volumes:
      - pgdata:/var/lib/postgresql/data
      - ./scripts/init-db.sql:/docker-entrypoint-initdb.d/init.sql
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U defectdojo"]
      interval: 10s
      timeout: 5s
      retries: 5
    <<: *logging

  redis:
    image: redis:7-alpine
    restart: unless-stopped
    command: redis-server --requirepass ${REDIS_PASSWORD} --save 60 1
    volumes:
      - redisdata:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
    <<: *logging

  nats:
    image: nats:2.10-alpine
    restart: unless-stopped
    command:
      - "-js"
      - "-m"
      - "8222"
      - "--store_dir"
      - "/data"
      - "--max_file_store"
      - "10GB"
    volumes:
      - natsdata:/data
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8222/healthz"]
      interval: 10s
    <<: *logging

  opensearch:
    image: opensearchproject/opensearch:2
    restart: unless-stopped
    environment:
      - discovery.type=single-node
      - DISABLE_SECURITY_PLUGIN=true
      - "OPENSEARCH_JAVA_OPTS=-Xms1g -Xmx1g"
      - bootstrap.memory_lock=true
    ulimits:
      memlock:
        soft: -1
        hard: -1
    volumes:
      - opensearchdata:/usr/share/opensearch/data
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9200/_cluster/health"]
      interval: 30s
    <<: *logging

  # ── Application ──────────────────────────────────────────────────────────────
  defectdojo:
    image: ghcr.io/your-org/defectdojo-go:${VERSION:-latest}
    restart: unless-stopped
    ports:
      - "${HTTP_PORT:-8080}:8080"
      - "${GRPC_PORT:-9090}:9090"
    environment:
      POSTGRES_URL: postgres://${POSTGRES_USER:-defectdojo}:${POSTGRES_PASSWORD}@postgres/${POSTGRES_DB:-defectdojo}?sslmode=disable
      NATS_URL: nats://nats:4222
      REDIS_URL: redis://:${REDIS_PASSWORD}@redis:6379
      OPENSEARCH_URL: http://opensearch:9200
      JWT_SECRET: ${JWT_SECRET}
      JIRA_ENCRYPTION_KEY: ${JIRA_ENCRYPTION_KEY}
      AI_BACKEND: ${AI_BACKEND:-ollama}
      AI_MODEL: ${AI_MODEL:-llama3}
      AI_BASE_URL: ${AI_BASE_URL:-http://ollama:11434}
      SMTP_HOST: ${SMTP_HOST}
      SMTP_PORT: ${SMTP_PORT:-587}
      SMTP_USER: ${SMTP_USER}
      SMTP_PASSWORD: ${SMTP_PASSWORD}
      SLACK_TOKEN: ${SLACK_TOKEN}
      LOG_LEVEL: ${LOG_LEVEL:-info}
      LOG_FORMAT: json
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
      nats:
        condition: service_healthy
      opensearch:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 60s
    <<: *logging

  # ── Nginx Reverse Proxy ──────────────────────────────────────────────────────
  nginx:
    image: nginx:1.25-alpine
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx/nginx.conf:/etc/nginx/nginx.conf:ro
      - ./nginx/ssl:/etc/nginx/ssl:ro
    depends_on:
      - defectdojo
    <<: *logging

  # ── Monitoring ───────────────────────────────────────────────────────────────
  prometheus:
    image: prom/prometheus:v2.51.0
    restart: unless-stopped
    volumes:
      - ./monitoring/prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - prometheusdata:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.retention.time=30d'
    <<: *logging

  grafana:
    image: grafana/grafana:10.4.0
    restart: unless-stopped
    environment:
      GF_SECURITY_ADMIN_PASSWORD: ${GRAFANA_PASSWORD:-admin}
      GF_INSTALL_PLUGINS: grafana-piechart-panel
    volumes:
      - grafanadata:/var/lib/grafana
      - ./monitoring/grafana/provisioning:/etc/grafana/provisioning
    ports:
      - "3000:3000"
    <<: *logging

volumes:
  pgdata:
  redisdata:
  natsdata:
  opensearchdata:
  prometheusdata:
  grafanadata:
```

## Kubernetes Deployment

```yaml
# apps/DefectDojo/k8s/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: defectdojo-go
  labels:
    app: defectdojo
spec:
  replicas: 1    # Monolith — single replica (state in DB/NATS/Redis)
  selector:
    matchLabels:
      app: defectdojo
  template:
    metadata:
      labels:
        app: defectdojo
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9091"
    spec:
      containers:
      - name: defectdojo
        image: ghcr.io/your-org/defectdojo-go:latest
        ports:
        - containerPort: 8080
          name: http
        - containerPort: 9090
          name: grpc
        - containerPort: 9091
          name: metrics
        envFrom:
        - secretRef:
            name: defectdojo-secrets
        - configMapRef:
            name: defectdojo-config
        resources:
          requests:
            memory: "512Mi"
            cpu: "250m"
          limits:
            memory: "2Gi"
            cpu: "2000m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 60
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /health/ready
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
---
apiVersion: v1
kind: Service
metadata:
  name: defectdojo-go
spec:
  selector:
    app: defectdojo
  ports:
  - name: http
    port: 80
    targetPort: 8080
  - name: grpc
    port: 9090
    targetPort: 9090
  type: ClusterIP
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: defectdojo-go
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
spec:
  rules:
  - host: defectdojo.yourcompany.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: defectdojo-go
            port:
              number: 80
```

## Nginx Configuration

```nginx
# apps/DefectDojo/nginx/nginx.conf

events {
    worker_connections 1024;
}

http {
    upstream defectdojo {
        server defectdojo:8080;
        keepalive 32;
    }

    server {
        listen 80;
        server_name defectdojo.yourcompany.com;
        return 301 https://$host$request_uri;
    }

    server {
        listen 443 ssl http2;
        server_name defectdojo.yourcompany.com;

        ssl_certificate     /etc/nginx/ssl/cert.pem;
        ssl_certificate_key /etc/nginx/ssl/key.pem;
        ssl_protocols TLSv1.2 TLSv1.3;

        client_max_body_size 100M;  # For scan file uploads

        location / {
            proxy_pass http://defectdojo;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
            proxy_read_timeout 300s;    # For large report generation
            proxy_send_timeout 300s;
        }

        location /api/v2/import-scan-results/ {
            proxy_pass http://defectdojo;
            client_max_body_size 500M;  # Large scan files
            proxy_read_timeout 600s;
        }
    }
}
```

## Monitoring — Prometheus Config

```yaml
# apps/DefectDojo/monitoring/prometheus.yml
global:
  scrape_interval: 15s

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

  - job_name: 'redis'
    static_configs:
      - targets: ['redis-exporter:9121']
```

## Key Metrics

```go
// apps/DefectDojo/internal/metrics/metrics.go

package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
    // Service health
    ServiceHealthGauge = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{Name: "dd_service_health", Help: "Service health (1=healthy, 0=unhealthy)"},
        []string{"service"},
    )
    
    // Finding metrics
    FindingsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "dd_findings_total", Help: "Total findings created"},
        []string{"severity", "product_id"},
    )
    FindingsSLABreachTotal = prometheus.NewCounter(
        prometheus.CounterOpts{Name: "dd_sla_breaches_total", Help: "Total SLA breaches"},
    )
    
    // Scan metrics
    ScansTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "dd_scans_total", Help: "Total scans imported"},
        []string{"scan_type", "status"},
    )
    ScanDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "dd_scan_duration_seconds",
            Help:    "Scan processing duration",
            Buckets: []float64{1, 5, 10, 30, 60, 120, 300},
        },
        []string{"scan_type"},
    )
    
    // API metrics
    HTTPRequestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "dd_http_requests_total", Help: "Total HTTP requests"},
        []string{"method", "path", "status"},
    )
    HTTPRequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "dd_http_request_duration_seconds",
            Help:    "HTTP request duration",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method", "path"},
    )
    
    // NATS metrics
    NATSMessagesTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "dd_nats_messages_total", Help: "Total NATS messages"},
        []string{"subject", "status"},
    )
)

func init() {
    prometheus.MustRegister(
        ServiceHealthGauge,
        FindingsTotal,
        FindingsSLABreachTotal,
        ScansTotal,
        ScanDuration,
        HTTPRequestsTotal,
        HTTPRequestDuration,
        NATSMessagesTotal,
    )
}
```

## .env.example

```bash
# apps/DefectDojo/.env.example

# ── Required ──────────────────────────────────────────────
POSTGRES_URL=postgres://defectdojo:changeme@localhost:5432/defectdojo?sslmode=disable
JWT_SECRET=change-me-to-a-very-long-random-secret-string-please

# ── Recommended ───────────────────────────────────────────
NATS_URL=nats://localhost:4222
REDIS_URL=redis://localhost:6379
OPENSEARCH_URL=http://localhost:9200
HTTP_PORT=8080
GRPC_PORT=9090

# ── JIRA Integration (optional) ───────────────────────────
JIRA_ENCRYPTION_KEY=32-bytes-random-key-here-fill-this

# ── AI Triage (optional) ──────────────────────────────────
AI_BACKEND=ollama
AI_MODEL=llama3
AI_BASE_URL=http://localhost:11434

# ── Email Notifications (optional) ────────────────────────
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your-email@gmail.com
SMTP_PASSWORD=your-app-password

# ── Slack Notifications (optional) ────────────────────────
SLACK_TOKEN=xoxb-your-slack-bot-token

# ── Logging ───────────────────────────────────────────────
LOG_LEVEL=info
LOG_FORMAT=json
```

## Makefile

```makefile
# apps/DefectDojo/Makefile

.PHONY: build run test docker-build docker-up

build:
	go build -o bin/defectdojo ./cmd/defectdojo/

run:
	go run ./cmd/defectdojo/

test:
	go test ./... -v -timeout 60s

test-integration:
	go test ./... -tags=integration -v -timeout 300s

docker-build:
	docker build -f Dockerfile -t defectdojo-go:dev ../../

docker-up:
	docker-compose -f docker-compose.prod.yml up -d

docker-down:
	docker-compose -f docker-compose.prod.yml down

migrate:
	go run ./cmd/defectdojo/ --migrate-only

proto:
	cd ../../services/shared/proto && buf generate

lint:
	golangci-lint run ./...
```
