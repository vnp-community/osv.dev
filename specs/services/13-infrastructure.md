# Infrastructure & Platform Specification

> **Version:** 1.0 | **Status:** Proposed | **Priority:** P0  
> **Platform:** Cloud-Agnostic (Primary: GCP, Secondary: AWS/Azure ready)  
> **Orchestration:** Kubernetes (GKE)

---

## 1. Infrastructure Overview

### 1.1 Platform Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        GCP / Cloud Platform                          │
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │                    GKE Cluster (Autopilot)                  │    │
│  │                                                              │    │
│  │  Namespaces:                                                 │    │
│  │  ├── osv-gateway        (API Gateway)                        │    │
│  │  ├── osv-query          (Query + Search + BFF)               │    │
│  │  ├── osv-pipeline       (Source Sync + Ingestion + Impact)   │    │
│  │  ├── osv-ai             (AI Enrichment)                      │    │
│  │  ├── osv-relations      (Alias + Notification)               │    │
│  │  ├── osv-platform       (NATS, Redis, OpenSearch)            │    │
│  │  └── osv-observability  (OTel, Prometheus, Grafana)          │    │
│  │                                                              │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                      │
│  ┌──────────────┐  ┌──────────────┐  ┌────────────────────────┐    │
│  │  Cloud       │  │  Cloud       │  │  Firestore             │    │
│  │  Storage     │  │  Pub/Sub     │  │  (primary DB)          │    │
│  │  (GCS)       │  │  (compat)    │  │                        │    │
│  └──────────────┘  └──────────────┘  └────────────────────────┘    │
│                                                                      │
│  ┌──────────────┐  ┌──────────────┐  ┌────────────────────────┐    │
│  │  Vertex AI   │  │  Secret      │  │  Cloud Armor           │    │
│  │  (embeddings │  │  Manager     │  │  (WAF + DDoS)          │    │
│  │   + Gemini)  │  │              │  │                        │    │
│  └──────────────┘  └──────────────┘  └────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 2. Kubernetes Resources Per Service

### 2.1 Standard Service Deployment Template

```yaml
# templates/service-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${SERVICE_NAME}
  namespace: ${NAMESPACE}
  labels:
    app: ${SERVICE_NAME}
    version: ${VERSION}
    part-of: osv
spec:
  replicas: ${MIN_REPLICAS}  # Start with min, HPA scales up
  selector:
    matchLabels:
      app: ${SERVICE_NAME}
  template:
    metadata:
      labels:
        app: ${SERVICE_NAME}
        version: ${VERSION}
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9090"
        prometheus.io/path: "/metrics"
    spec:
      serviceAccountName: ${SERVICE_NAME}-sa
      containers:
        - name: ${SERVICE_NAME}
          image: ${IMAGE}:${VERSION}
          ports:
            - name: grpc
              containerPort: 50051
            - name: http
              containerPort: 8080
            - name: metrics
              containerPort: 9090
          env:
            - name: SERVICE_NAME
              value: ${SERVICE_NAME}
            - name: SERVICE_VERSION
              valueFrom:
                fieldRef:
                  fieldPath: metadata.labels['version']
            - name: POD_NAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
            - name: NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
          envFrom:
            - configMapRef:
                name: ${SERVICE_NAME}-config
            - secretRef:
                name: ${SERVICE_NAME}-secrets
          resources:
            requests:
              cpu: "100m"
              memory: "256Mi"
            limits:
              cpu: "2000m"
              memory: "2Gi"
          livenessProbe:
            httpGet:
              path: /health/live
              port: 8080
            initialDelaySeconds: 10
            periodSeconds: 15
          readinessProbe:
            httpGet:
              path: /health/ready
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 10
          lifecycle:
            preStop:
              exec:
                command: ["/bin/sh", "-c", "sleep 5"]
```

### 2.2 HPA Configuration Per Service

```yaml
# HPA for each service
---
# API Gateway: Scale based on RPS
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: api-gateway-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: api-gateway
  minReplicas: 3
  maxReplicas: 50
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 60
    - type: Pods
      pods:
        metric:
          name: gateway_requests_per_second
        target:
          type: AverageValue
          averageValue: "1000"

---
# Query Service: Scale based on CPU + custom metric
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: vulnerability-query-hpa
spec:
  minReplicas: 3
  maxReplicas: 30
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70

---
# Impact Analysis: Scale based on queue depth
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: impact-analysis-hpa
spec:
  minReplicas: 1
  maxReplicas: 10
  metrics:
    - type: External
      external:
        metric:
          name: nats_stream_pending_messages
          selector:
            matchLabels:
              stream: osv-vuln-imported
        target:
          type: AverageValue
          averageValue: "100"
```

### 2.3 Resource Sizing Per Service

| Service | Min Replicas | Max Replicas | CPU Request | Memory Request |
|---------|-------------|-------------|-------------|----------------|
| API Gateway | 3 | 50 | 200m | 512Mi |
| Vulnerability Query | 3 | 30 | 500m | 1Gi |
| Ingestion | 2 | 20 | 500m | 1Gi |
| Impact Analysis | 1 | 10 | 1000m | 2Gi |
| Version Index | 1 | 5 | 500m | 1Gi |
| Source Sync | 1 | 3 | 200m | 512Mi |
| Notification | 1 | 10 | 200m | 512Mi |
| Search | 2 | 20 | 500m | 1Gi |
| Web BFF | 2 | 20 | 200m | 512Mi |
| AI Enrichment | 1 | 10 | 500m | 2Gi |
| Alias Relations | 1 | 5 | 200m | 512Mi |

---

## 3. NATS JetStream Configuration

### 3.1 Cluster Setup

```yaml
# NATS JetStream cluster (3 nodes for HA)
# Deployed via NATS Helm chart or NATS Operator

nats:
  cluster:
    enabled: true
    replicas: 3
  
  jetstream:
    enabled: true
    memStorage:
      enabled: true
      size: 2Gi
    fileStorage:
      enabled: true
      size: 50Gi
      storageClass: premium-rwo
```

### 3.2 Stream Definitions

```go
// Platform configuration for NATS streams

streams := []nats.StreamConfig{
    {
        Name:     "OSV-EVENTS",
        Subjects: []string{"osv.>"},
        Retention: nats.LimitsPolicy,
        MaxAge:    7 * 24 * time.Hour,  // 7 days retention
        Storage:   nats.FileStorage,
        Replicas:  3,
        MaxBytes:  50 * 1024 * 1024 * 1024,  // 50GB
    },
}

// Consumer definitions (durable, pull-based)
consumers := map[string]nats.ConsumerConfig{
    "ingestion-service": {
        Durable:       "ingestion-service",
        FilterSubject: "osv.source.change.>",
        AckPolicy:     nats.AckExplicitPolicy,
        MaxDeliver:    5,
        AckWait:       30 * time.Minute,  // Long for slow processing
    },
    "impact-analysis": {
        Durable:       "impact-analysis",
        FilterSubject: "osv.vuln.imported",
        AckPolicy:     nats.AckExplicitPolicy,
        MaxDeliver:    3,
        AckWait:       30 * time.Minute,
    },
    "ai-enrichment": {
        Durable:       "ai-enrichment",
        FilterSubject: "osv.vuln.imported",
        AckPolicy:     nats.AckExplicitPolicy,
        MaxDeliver:    3,
        AckWait:       5 * time.Minute,
    },
    "search-indexer": {
        Durable:       "search-indexer",
        FilterSubject: "osv.vuln.>",
        AckPolicy:     nats.AckExplicitPolicy,
        MaxDeliver:    5,
        AckWait:       2 * time.Minute,
    },
    "notification": {
        Durable:       "notification",
        FilterSubject: "osv.vuln.>",
        AckPolicy:     nats.AckExplicitPolicy,
        MaxDeliver:    10,
        AckWait:       30 * time.Second,
    },
    "alias-service": {
        Durable:       "alias-service",
        FilterSubject: "osv.vuln.imported",
        AckPolicy:     nats.AckExplicitPolicy,
        MaxDeliver:    3,
        AckWait:       5 * time.Minute,
    },
}
```

### 3.3 Dead Letter Queue

```go
// DLQ for failed messages (after MaxDeliver retries)
dlqStream := nats.StreamConfig{
    Name:     "OSV-DLQ",
    Subjects: []string{"osv.dlq.>"},
    MaxAge:   30 * 24 * time.Hour,  // 30 days
    Storage:  nats.FileStorage,
}

// Alert when DLQ has messages
// Operator investigates and replays or discards
```

---

## 4. Redis Configuration

### 4.1 Redis Cluster (HA)

```yaml
# Redis Cluster: 3 masters + 3 replicas
# Or use GCP Memorystore for Redis (managed)

redis:
  cluster:
    enabled: true
    replicas: 1      # 1 replica per master
  
  sentinel:
    enabled: false   # Not needed with cluster mode
  
  persistence:
    enabled: true
    size: 20Gi
  
  resources:
    requests:
      cpu: "500m"
      memory: "4Gi"
    limits:
      cpu: "2000m"
      memory: "8Gi"
```

### 4.2 Key Namespacing

```
Redis Key Conventions:
  osv:query:cache:{query_hash}:{page}         → Query results (TTL: 5min)
  osv:vuln:id:{vuln_id}                       → Vulnerability JSON (TTL: 1h)
  osv:search:cache:{search_hash}              → Search results (TTL: 30s)
  osv:stats:ecosystem:{date}                  → Ecosystem counts (TTL: 24h)
  osv:ratelimit:{client_id}:{endpoint}:{min}  → Rate limit counters (TTL: 1min)
  osv:auth:token:{token_hash}                 → Auth token cache (TTL: 5min)
  osv:embed:{vuln_id}                         → Embedding cache (TTL: 7d)
  osv:idem:{content_hash}                     → Idempotency keys (TTL: 24h)
```

---

## 5. OpenSearch Configuration

### 5.1 Cluster Setup

```yaml
# OpenSearch: 3 master + 3 data nodes
opensearch:
  master:
    replicas: 3
    resources:
      requests:
        cpu: "500m"
        memory: "2Gi"
      limits:
        cpu: "2000m"
        memory: "4Gi"
    persistence:
      size: 20Gi
  
  data:
    replicas: 3
    resources:
      requests:
        cpu: "1000m"
        memory: "4Gi"
      limits:
        cpu: "4000m"
        memory: "8Gi"
    persistence:
      size: 200Gi
      storageClass: premium-rwo
```

### 5.2 Index Strategy

```
Indices:
  osv-vulnerabilities-{YYYY.MM}    # Monthly indices (time-based)
  osv-vulnerabilities-aliases       # Points to current month

Settings:
  - number_of_shards: 3
  - number_of_replicas: 1
  - refresh_interval: 5s           # Balance between freshness and performance
  - index.knn: true                # Enable vector search
```

---

## 6. Observability Stack

### 6.1 OpenTelemetry Collector

```yaml
# otel-collector-config.yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  batch:
    timeout: 5s
    send_batch_size: 1000
  
  memory_limiter:
    limit_mib: 512
  
  resource:
    attributes:
      - key: environment
        value: production
        action: upsert

exporters:
  # Traces → Jaeger or GCP Cloud Trace
  jaeger:
    endpoint: jaeger-collector:14250
    tls:
      insecure: true
  
  # Metrics → Prometheus
  prometheus:
    endpoint: 0.0.0.0:8889
  
  # Logs → GCP Cloud Logging
  googlecloud:
    project: ${GCP_PROJECT}

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, batch, resource]
      exporters: [jaeger]
    
    metrics:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [prometheus]
    
    logs:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [googlecloud]
```

### 6.2 Prometheus Alerts

```yaml
# Critical alerts (page immediately)
groups:
  - name: osv-critical
    rules:
      - alert: APIGatewayHighErrorRate
        expr: rate(gateway_requests_total{status=~"5.."}[5m]) > 0.01
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "API Gateway error rate > 1%"
      
      - alert: IngestionLagHigh
        expr: nats_stream_pending_messages{stream="OSV-EVENTS"} > 10000
        for: 10m
        labels:
          severity: critical
        annotations:
          summary: "NATS ingestion lag > 10K messages"
      
      - alert: DatastoreWriteFailures
        expr: rate(firestore_write_errors_total[5m]) > 0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "Firestore write errors detected"

  - name: osv-warning
    rules:
      - alert: QueryServiceHighLatency
        expr: histogram_quantile(0.99, rate(grpc_server_handling_seconds_bucket[5m])) > 1
        labels:
          severity: warning
```

### 6.3 Grafana Dashboard Structure

```
Dashboards:
  ├── OSV Overview             → All services at a glance
  ├── API Gateway              → Request rate, errors, latency per endpoint
  ├── Vulnerability Pipeline   → Import rate, Kafka lag, processing time
  ├── Query Service            → Cache hit rate, query breakdown, latency
  ├── Search Service           → Search latency, index staleness, results/query
  ├── AI Enrichment            → Enrichment queue, LLM latency, cost tracking
  ├── Infrastructure           → NATS, Redis, OpenSearch health
  └── SLO Dashboard            → Service level objectives vs actual
```

---

## 7. Security Configuration

### 7.1 Network Policies

```yaml
# Only allow gateway to reach backend services
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: backend-policy
  namespace: osv-query
spec:
  podSelector: {}
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              name: osv-gateway
      ports:
        - port: 50051    # gRPC
    - from:
        - namespaceSelector:
            matchLabels:
              name: osv-observability
      ports:
        - port: 9090     # Metrics scraping
```

### 7.2 mTLS Between Services (Istio / Linkerd)

```yaml
# Enable mTLS for all inter-service communication
apiVersion: security.istio.io/v1beta1
kind: PeerAuthentication
metadata:
  name: default
  namespace: osv-production
spec:
  mtls:
    mode: STRICT    # All inter-service traffic must use mTLS
```

### 7.3 Secrets Management

```
Approach: Layered secrets management

1. Development:
   - .env files (git-ignored)
   - Local Vault dev server

2. Staging/Production:
   - GCP Secret Manager (primary)
   - Kubernetes Secrets (K8s-native, encrypted at rest)
   - Workload Identity (GKE → GCP services, no credentials needed)

Secret rotation:
   - API keys: 90-day rotation
   - Database passwords: 180-day rotation
   - TLS certificates: Auto-renewed via cert-manager

Access pattern:
   - Services use Workload Identity to access GCP services
   - Secrets mounted as environment variables via ESO (External Secrets Operator)
```

---

## 8. CI/CD Pipeline

### 8.1 Build Pipeline (Cloud Build)

```yaml
# cloudbuild.yaml
steps:
  # 1. Unit tests
  - name: 'golang:1.22'
    entrypoint: 'make'
    args: ['test-unit']
  
  # 2. Integration tests (with test containers)
  - name: 'golang:1.22'
    entrypoint: 'make'
    args: ['test-integration']
  
  # 3. Security scan
  - name: 'ghcr.io/aquasecurity/trivy'
    args: ['fs', '--exit-code', '1', '--severity', 'HIGH,CRITICAL', '.']
  
  # 4. Build containers
  - name: 'gcr.io/cloud-builders/docker'
    args: ['build', '-t', 'gcr.io/$PROJECT_ID/${SERVICE_NAME}:${SHORT_SHA}', './services/${SERVICE_NAME}']
  
  # 5. Push to Artifact Registry
  - name: 'gcr.io/cloud-builders/docker'
    args: ['push', 'gcr.io/$PROJECT_ID/${SERVICE_NAME}:${SHORT_SHA}']
  
  # 6. Deploy to staging
  - name: 'gcr.io/google.com/cloudsdktool/cloud-sdk'
    args: ['run', 'deploy', 'staging']
```

### 8.2 Deployment Strategy

```
Environments:
  dev    → auto-deploy on every merge to main
  staging → auto-deploy with smoke tests
  prod   → manual approval gate

Rollout strategy per environment:
  dev:     RollingUpdate (maxSurge: 100%, maxUnavailable: 0%)
  staging: RollingUpdate (maxSurge: 50%, maxUnavailable: 0%)
  prod:    Blue-Green with manual traffic shift (5% → 25% → 50% → 100%)

Canary analysis (via Flagger):
  - Duration: 30 minutes
  - Metrics: error rate < 1%, p99 latency < 500ms
  - Auto-rollback on threshold breach
```

---

## 9. Disaster Recovery

### 9.1 RTO / RPO Targets

| Component | RTO | RPO |
|-----------|-----|-----|
| API Gateway | 5 min | 0 (stateless) |
| Query Service | 5 min | 0 (stateless) |
| Firestore | 30 min | 1 min (PITR) |
| NATS | 15 min | 5 min (JetStream persistence) |
| Redis | 10 min | 0 (cache, rebuild-able) |
| OpenSearch | 60 min | 30 min (snapshots) |

### 9.2 Backup Strategy

```
Firestore:
  - Daily exports to GCS (gs://osv-backups/firestore/)
  - Point-in-time recovery enabled (7 days)

NATS JetStream:
  - File storage on persistent disk (replicated 3x)
  - Stream snapshots to GCS daily

OpenSearch:
  - Snapshot repository on GCS
  - Automated snapshots every 6h
  - Retention: 30 days

GCS (vulnerability JSON):
  - Multi-region storage (already redundant)
  - Versioning enabled (30-day version retention)
```

---

## 10. Cost Optimization

```
Strategies:

1. Compute:
   - GKE Autopilot: pay per pod CPU/memory (no node management)
   - Spot/Preemptible nodes for non-critical workloads:
     - Impact Analysis (can retry)
     - Version Index workers
     - AI Enrichment batch jobs

2. Storage:
   - GCS Nearline for old vulnerability JSON (> 90 days)
   - GCS Archive for backups (> 365 days)
   - OpenSearch warm tier for old indices

3. AI:
   - Cache embeddings aggressively (7-day TTL)
   - Use Gemini Flash instead of Pro for classification
   - Batch AI operations (process in bulk, not per-vuln)
   - Self-hosted Ollama for development/testing

4. Network:
   - Internal load balancers for service-to-service
   - Cloud CDN for static website assets
   - Cloud Armor to block bad traffic early (save backend costs)

Monthly Cost Estimate (rough):
  GKE Autopilot:     ~$2,000
  Firestore:          ~$500
  Redis Memorystore:  ~$400
  OpenSearch:         ~$800
  NATS (self-hosted): ~$200
  GCS:                ~$300
  Vertex AI:          ~$500 (with caching)
  Misc (LB, CDN):    ~$300
  ─────────────────────────
  Total:              ~$5,000/month
```

---

## 11. Terraform Module Structure

```
infrastructure/
├── modules/
│   ├── gke/                    # GKE Autopilot cluster
│   ├── nats/                   # NATS JetStream on GKE
│   ├── redis/                  # Redis Cluster or Memorystore
│   ├── opensearch/             # OpenSearch cluster
│   ├── firestore/              # Firestore setup + indexes
│   ├── gcs/                    # GCS buckets
│   ├── iam/                    # Service accounts + Workload Identity
│   ├── networking/             # VPC, subnets, Cloud Armor
│   ├── observability/          # OTel, Prometheus, Grafana
│   └── secrets/                # Secret Manager
├── environments/
│   ├── dev/
│   │   └── main.tf
│   ├── staging/
│   │   └── main.tf
│   └── prod/
│       └── main.tf
├── helm/
│   ├── api-gateway/
│   ├── vulnerability-query/
│   ├── ingestion/
│   ├── impact-analysis/
│   ├── version-index/
│   ├── source-sync/
│   ├── notification/
│   ├── search/
│   ├── web-bff/
│   ├── ai-enrichment/
│   └── alias-relations/
└── scripts/
    ├── deploy.sh
    ├── rollback.sh
    └── smoke-test.sh
```

---

## 9. Implementation Status

> **Status:** ✅ Infrastructure Defined | **Updated:** 2026-06-01

### Implemented
- [x] `infrastructure/terraform/gcp/` — Full Terraform config for GKE, Cloud Run, Firestore, Pub/Sub, Redis
- [x] `infrastructure/kubernetes/base/` — Service, Deployment, HPA YAML for all 11 services
- [x] `infrastructure/kubernetes/overlays/` — Kustomize overlays for staging + production
- [x] `infrastructure/kubernetes/helm/` — Helm charts: NATS cluster (3 nodes), OpenSearch (3 nodes, 2 replica), Redis Sentinel
- [x] `infrastructure/config/feature-flags.yaml` — Feature flag definitions for phased migration
- [x] `infrastructure/monitoring/alerts/alerts.yaml` — P0/P1/P2/P3 SLO alert definitions
- [x] `infrastructure/monitoring/dashboards/` — Grafana dashboard definitions (per-service + overview)
- [x] `infrastructure/monitoring/logging/` — Structured logging configuration
- [x] `infrastructure/scripts/setup-gcp.sh` + `setup-secrets.sh` — Initial GCP provisioning scripts

### Pending (Deployment gates)
- [ ] GCP project IAM roles + service accounts setup
- [ ] Secret Manager values provisioned (API keys, NATS TLS, etc.)
- [ ] NATS JetStream streams created: `OSV_EVENTS` (20 subjects), `OSV_TASKS`
- [ ] OpenSearch indices created + mapping applied
- [ ] GKE cluster provisioned; namespaces created
- [ ] Terraform state backend configured (GCS bucket)
- [ ] CI/CD pipelines (Cloud Build / GitHub Actions)

### Notes
- All K8s manifests use `resources: requests/limits` matching GKE node pool specs
- Terraform modules are modular (separate: gke.tf, firestore.tf, redis.tf, nats.tf)
- OpenSearch cluster uses 3 nodes across 3 AZs for HA
