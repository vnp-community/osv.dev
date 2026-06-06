# Task T12 — Infrastructure Setup

> **Priority:** P0 | **Phase:** 0 (đầu tiên) | **Spec:** `specs/services/13-infrastructure.md`  
> **Note:** Deploy hết trước khi bắt đầu bất kỳ service nào

## Mục Tiêu
Cung cấp toàn bộ hạ tầng: Kubernetes (GKE), NATS JetStream, Redis, OpenSearch, Observability stack.

## Thứ Tự Deploy

```
1. GKE Cluster + Namespaces
2. NATS JetStream (3-node cluster)
3. Redis Cluster (3 master + 3 replicas)
4. OpenSearch (3 master + 3 data nodes)
5. OpenTelemetry Collector
6. Prometheus + Grafana
7. Jaeger/Tempo (tracing backend)
8. Network Policies (mTLS)
```

## Namespaces Kubernetes

```yaml
namespaces:
  osv-gateway:       # API Gateway
  osv-query:         # Query + Search + BFF
  osv-pipeline:      # Source Sync + Ingestion + Impact Analysis
  osv-ai:            # AI Enrichment
  osv-relations:     # Alias + Notification
  osv-platform:      # NATS, Redis, OpenSearch
  osv-observability: # OTel, Prometheus, Grafana, Jaeger
```

## NATS JetStream Config

```yaml
# Deploy via NATS Helm chart hoặc NATS Operator
nats:
  cluster:
    enabled: true
    replicas: 3
  jetstream:
    enabled: true
    fileStorage:
      enabled: true
      size: 50Gi
      storageClass: premium-rwo
```

### NATS Streams & Consumers

```go
// Stream: OSV-EVENTS (tất cả domain events)
nats.StreamConfig{
    Name:     "OSV-EVENTS",
    Subjects: []string{"osv.>"},
    MaxAge:   7 * 24 * time.Hour,
    Storage:  nats.FileStorage,
    Replicas: 3,
    MaxBytes: 50 * 1024 * 1024 * 1024,  // 50GB
}

// Stream: OSV-DLQ (failed messages)
nats.StreamConfig{
    Name:     "OSV-DLQ",
    Subjects: []string{"osv.dlq.>"},
    MaxAge:   30 * 24 * time.Hour,
}

// Durable Consumers:
// ingestion-service:  FilterSubject="osv.source.change.>", MaxDeliver=5, AckWait=30min
// impact-analysis:    FilterSubject="osv.vuln.imported",   MaxDeliver=3, AckWait=30min
// ai-enrichment:      FilterSubject="osv.vuln.imported",   MaxDeliver=3, AckWait=5min
// search-indexer:     FilterSubject="osv.vuln.>",          MaxDeliver=5, AckWait=2min
// notification:       FilterSubject="osv.vuln.>",          MaxDeliver=10, AckWait=30s
// alias-service:      FilterSubject="osv.vuln.imported",   MaxDeliver=3, AckWait=5min
```

## Redis Cluster Config

```yaml
# 3 masters + 3 replicas (GCP Memorystore hoặc self-hosted)
redis:
  cluster:
    enabled: true
    replicas: 1  # 1 replica per master
  persistence:
    enabled: true
    size: 20Gi
  resources:
    requests: { cpu: "500m", memory: "4Gi" }
    limits: { cpu: "2000m", memory: "8Gi" }
```

### Redis Key Namespacing

```
osv:query:cache:{query_hash}:{page}          TTL: 5min
osv:vuln:id:{vuln_id}                        TTL: 1h
osv:search:cache:{search_hash}               TTL: 30s
osv:stats:ecosystem:{date}                   TTL: 24h
osv:ratelimit:{client_id}:{endpoint}:{min}   TTL: 1min
osv:auth:token:{token_hash}                  TTL: 5min
osv:embed:{vuln_id}                          TTL: 7d
osv:idem:{content_hash}                      TTL: 24h
```

## OpenSearch Config

```yaml
opensearch:
  master:
    replicas: 3
    resources:
      requests: { cpu: "500m", memory: "2Gi" }
      limits: { cpu: "2000m", memory: "4Gi" }
    persistence.size: 20Gi
  data:
    replicas: 3
    resources:
      requests: { cpu: "1000m", memory: "4Gi" }
      limits: { cpu: "4000m", memory: "8Gi" }
    persistence.size: 200Gi
    storageClass: premium-rwo
```

### OpenSearch Index Strategy

```
Indices: osv-vulnerabilities-{YYYY.MM}  (monthly time-based)
Alias:   osv-vulnerabilities             (points to current month)
Settings:
  - number_of_shards: 3
  - number_of_replicas: 1
  - refresh_interval: 5s
  - index.knn: true  (enable vector search)
```

## OpenTelemetry Collector Config

```yaml
# otel-collector-config.yaml
receivers:
  otlp:
    protocols:
      grpc: { endpoint: 0.0.0.0:4317 }
      http: { endpoint: 0.0.0.0:4318 }

processors:
  batch: { timeout: 5s, send_batch_size: 1000 }
  memory_limiter: { limit_mib: 512 }

exporters:
  jaeger: { endpoint: jaeger-collector:14250 }
  prometheus: { endpoint: 0.0.0.0:8889 }
  googlecloud: { project: ${GCP_PROJECT} }

service.pipelines:
  traces:   { receivers: [otlp], processors: [...], exporters: [jaeger] }
  metrics:  { receivers: [otlp], processors: [...], exporters: [prometheus] }
  logs:     { receivers: [otlp], processors: [...], exporters: [googlecloud] }
```

## Standard Kubernetes Deployment Template

```yaml
# Áp dụng cho mọi service
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    metadata:
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9090"
        prometheus.io/path: "/metrics"
    spec:
      containers:
        - ports:
            - name: grpc    containerPort: 50051
            - name: http    containerPort: 8080
            - name: metrics containerPort: 9090
          livenessProbe:
            httpGet: { path: /health/live, port: 8080 }
            initialDelaySeconds: 10
            periodSeconds: 15
          readinessProbe:
            httpGet: { path: /health/ready, port: 8080 }
            initialDelaySeconds: 5
            periodSeconds: 10
          lifecycle:
            preStop:
              exec: { command: ["/bin/sh", "-c", "sleep 5"] }
```

## HPA Per Service

```
API Gateway:         min=3, max=50, CPU target=60%
Vulnerability Query: min=3, max=30, CPU target=70%
Ingestion:          min=2, max=20
Impact Analysis:    min=1, max=10 (scale by NATS pending messages)
Source Sync:        min=1, max=3
Notification:       min=1, max=10
Search:             min=2, max=20
Web BFF:            min=2, max=20
AI Enrichment:      min=1, max=10
Alias Relations:    min=1, max=5
Version Index:      min=1, max=5
```

## Network Policies

```yaml
# Chỉ osv-gateway namespace được kết nối tới backend services
# Chỉ osv-observability được scrape metrics port 9090
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: backend-policy
  namespace: osv-query
spec:
  podSelector: {}
  ingress:
    - from: [{ namespaceSelector: { matchLabels: { name: osv-gateway } } }]
      ports: [{ port: 50051 }]
    - from: [{ namespaceSelector: { matchLabels: { name: osv-observability } } }]
      ports: [{ port: 9090 }]
```

## mTLS (Istio)

```yaml
# Enable strict mTLS cho all inter-service traffic
apiVersion: security.istio.io/v1beta1
kind: PeerAuthentication
metadata:
  name: default
  namespace: osv-production
spec:
  mtls: { mode: STRICT }
```

## Prometheus Alerts Quan Trọng

```yaml
groups:
  - name: osv-critical
    rules:
      - alert: APIGatewayHighErrorRate
        expr: rate(gateway_requests_total{status=~"5.."}[5m]) > 0.01
        for: 5m

      - alert: IngestionLagHigh
        expr: nats_stream_pending_messages{stream="OSV-EVENTS"} > 10000
        for: 10m

      - alert: FirestoreWriteFailures
        expr: rate(firestore_write_errors_total[5m]) > 0
        for: 2m

  - name: osv-warning
    rules:
      - alert: QueryServiceHighLatency
        expr: histogram_quantile(0.99, rate(grpc_server_handling_seconds_bucket[5m])) > 1
```

## Terraform Structure

```
infrastructure/
├── modules/
│   ├── gke/           # GKE Autopilot cluster
│   ├── nats/          # NATS JetStream on GKE
│   ├── redis/         # Redis Cluster or Memorystore
│   ├── opensearch/    # OpenSearch cluster
│   ├── firestore/     # Firestore setup + indexes
│   ├── gcs/           # GCS buckets
│   ├── iam/           # Service accounts + Workload Identity
│   ├── networking/    # VPC, subnets, Cloud Armor
│   ├── observability/ # OTel, Prometheus, Grafana
│   └── secrets/       # Secret Manager
├── environments/
│   ├── dev/main.tf
│   ├── staging/main.tf
│   └── prod/main.tf
└── helm/
    ├── api-gateway/ vulnerability-query/ ingestion/ ...
```

## GCS Buckets Cần Tạo

```
gs://osv-vulnz/            # Vulnerability JSON blobs
gs://osv-repo-cache/       # Git repo caches
gs://osv-backups/          # Firestore exports
gs://osv-public-import-logs/  # Public import quality logs
gs://cve-osv-conversion/   # CVE source (external, existing)
gs://go-vulndb/            # Go vuln DB (external, existing)
```

## CI/CD Pipeline (Cloud Build)

```yaml
steps:
  - name: 'golang:1.22'
    entrypoint: 'make'
    args: ['test-unit']
  - name: 'golang:1.22'
    entrypoint: 'make'
    args: ['test-integration']
  - name: 'ghcr.io/aquasecurity/trivy'
    args: ['fs', '--exit-code', '1', '--severity', 'HIGH,CRITICAL', '.']
  - name: 'gcr.io/cloud-builders/docker'
    args: ['build', '-t', 'gcr.io/$PROJECT_ID/${SERVICE_NAME}:${SHORT_SHA}', ...]
  - name: 'gcr.io/cloud-builders/docker'
    args: ['push', ...]
```

## Checklist Thực Thi

> **Status: ✅ COMPLETED (Infrastructure Code)** — 2026-06-01

**Phase 0.1 — GKE & Base:**
- [x] Terraform module: GKE Autopilot cluster (`infrastructure/modules/gke/`)
- [x] Namespaces + resource quotas (defined in Terraform)
- [x] Istio mTLS (`infrastructure/k8s/istio/peer-authentication.yaml`)
- [x] Network Policies (`infrastructure/k8s/network-policies/`)

**Phase 0.2 — Platform Services:**
- [x] Terraform NATS JetStream module (`infrastructure/modules/nats/`)
- [x] NATS streams: OSV-EVENTS, OSV-DLQ (defined in stream config)
- [x] Terraform Redis module (`infrastructure/modules/redis/`)
- [x] Terraform OpenSearch module (`infrastructure/modules/opensearch/`)
- [x] OpenSearch kNN index template (defined in module)

**Phase 0.3 — Observability:**
- [x] OTel Collector config (`infrastructure/helm/observability/otel-collector-config.yaml`)
- [x] Terraform observability module (`infrastructure/modules/observability/`)
- [x] Grafana dashboards skeleton

**Phase 0.4 — GCP Resources:**
- [x] Terraform GCS buckets module (`infrastructure/modules/gcs/`)
- [x] Terraform IAM + Workload Identity (`infrastructure/modules/iam/`)
- [x] Dev environment main.tf (`infrastructure/environments/dev/main.tf`)
- [x] Feature flags config (`infrastructure/config/feature-flags.yaml`)

**Phase 0.5 — CI/CD:**
- [x] `tools/smoke-test/main.go` — endpoint sanity check
- [ ] Deploy to actual GCP (apply Terraform)
- [ ] Create Artifact Registry repository
- [ ] Setup Cloud Build triggers (per-service)
- [ ] Setup Secret Manager with initial secrets
- [ ] Create GCP Pub/Sub topics
- [ ] Enable Firestore (Native mode) in GCP Console

**Validation:**
- [ ] NATS: pub/sub test on OSV-EVENTS stream
- [ ] Redis: ping + SET/GET test
- [ ] OpenSearch: create test index + document
- [ ] OTel: verify traces appear in Jaeger/Cloud Trace
- [ ] Prometheus: verify metrics scraping
- [ ] Network policies: verify namespace isolation
