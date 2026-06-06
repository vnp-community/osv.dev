# OSV.dev — Deployment Model & CVE Management Specification

> **Version:** 1.0
> **Date:** 2026-06-03
> **Status:** Proposal
> **Author:** Engineering Team
> **Scope:** Mô hình triển khai hệ thống, quản lý nguồn CVE, phân loại CVE, quản lý kết nối

---

## 1. Tổng Quan Đề Xuất

Tài liệu này đề xuất **mô hình triển khai hoàn chỉnh** cho hệ thống OSV.dev dựa trên kiến trúc microservices hiện tại, tập trung vào ba nhóm vấn đề chính:

1. **Quản lý nguồn CVE** (Source Management) — Cấu hình, giám sát và điều phối các nguồn dữ liệu lỗ hổng
2. **Phân loại CVE** (CVE Classification) — Pipeline làm giàu, phân nhóm và đánh nhãn tự động
3. **Quản lý kết nối** (Connection Management) — Circuit breaker, retry, rate-limit và health check cho từng nguồn

---

## 2. Kiến Trúc Triển Khai Đề Xuất

### 2.1 High-Level Deployment Topology

```
┌──────────────────────────────────────────────────────────────────────────────────┐
│                              EXTERNAL DATA SOURCES (30+)                         │
│                                                                                  │
│  [Git Repos]  [GCS Buckets]  [REST APIs]  [NVD CVE API]  [Custom Feeds]        │
└────────────────────┬─────────────────────────────────────────────────────────────┘
                     │ Pull / Webhook
                     ▼
┌────────────────────────────────────────────┐
│         SOURCE MANAGEMENT LAYER            │
│  ┌──────────────────────────────────────┐  │
│  │  source-sync (Go)                    │  │
│  │  • Scheduler (cron per source)       │  │
│  │  • Connection pool & circuit breaker │  │
│  │  • Source health monitor             │  │
│  │  • Rate limiter per source           │  │
│  └──────────────────┬───────────────────┘  │
└─────────────────────┼──────────────────────┘
                      │ NATS JetStream (events)
                      ▼
┌────────────────────────────────────────────┐
│          INGESTION & PROCESSING LAYER       │
│  ┌─────────────┐   ┌────────────────────┐  │
│  │  ingestion  │   │  impact-analysis   │  │
│  │  (Go)       │   │  (Go)              │  │
│  │  • Validate │   │  • Git bisection   │  │
│  │  • Normalize│   │  • Version enum    │  │
│  │  • Dedupe   │   │  • Cherry-pick     │  │
│  └──────┬──────┘   └────────┬───────────┘  │
└─────────┼──────────────────┼──────────────┘
          │                  │
          ▼                  ▼
┌────────────────────────────────────────────┐
│           CLASSIFICATION LAYER              │
│  ┌──────────────┐  ┌─────────────────────┐ │
│  │ ai-enrichment│  │  alias-relations     │ │
│  │ (Go)         │  │  (Go)               │ │
│  │ • CVSS score │  │  • CVE↔GHSA mapping │ │
│  │ • Severity   │  │  • Alias groups     │ │
│  │ • Tags/labels│  │  • Upstream tracking│ │
│  │ • Embeddings │  │                     │ │
│  └──────┬───────┘  └──────────┬──────────┘ │
└─────────┼────────────────────┼─────────────┘
          │                    │
          ▼                    ▼
┌────────────────────────────────────────────┐
│              STORAGE LAYER                  │
│  ┌────────────┐  ┌──────────┐  ┌────────┐  │
│  │ Firestore  │  │  GCS /   │  │OpenSea-│  │
│  │ (metadata) │  │  S3      │  │rch     │  │
│  │            │  │  (blobs) │  │(search)│  │
│  └────────────┘  └──────────┘  └────────┘  │
└────────────────────────────────────────────┘
          │
          ▼
┌────────────────────────────────────────────┐
│              SERVING LAYER                  │
│  ┌──────────────┐  ┌────────────────────┐  │
│  │ vulnerability│  │  api-gateway        │  │
│  │ -query (Go)  │  │  (Go)              │  │
│  └──────────────┘  └────────────────────┘  │
└────────────────────────────────────────────┘
```

### 2.2 Môi Trường Triển Khai

| Môi trường | Mục đích | Hạ tầng | Scale |
|------------|---------|---------|-------|
| **Local Dev** | Phát triển và debug | Docker Compose | 1 replica/service |
| **Staging** | Integration test, QA | Kubernetes (1 node) | 1-2 replicas |
| **Production** | Live system | Kubernetes / GKE | Auto-scale |

---

## 3. Quản Lý Nguồn CVE (Source Management)

### 3.1 Mô Hình Cấu Hình Nguồn

Tất cả nguồn CVE được khai báo trong `source.yaml` theo cấu trúc thống nhất:

```yaml
# Cấu trúc nguồn chuẩn
- name: '<source-name>'               # ID duy nhất
  type: <0|1|2>                        # GIT=0, GCS_BUCKET=1, REST_API=2
  
  # --- Kết nối (tùy theo type) ---
  repo_url: 'https://...'             # type=0: Git URL
  repo_branch: 'main'                 # type=0: branch
  bucket: '<gcs-bucket>'              # type=1: GCS bucket
  rest_api_url: 'https://...'         # type=2: REST endpoint
  
  # --- Lọc dữ liệu ---
  db_prefix: ['CVE-']                 # Các prefix ID được chấp nhận
  accepted_ecosystems: ['PyPI', '*']  # Ecosystems được chấp nhận
  ignore_patterns: ['^(?!CVE-).*$']   # Regex để bỏ qua file
  directory_path: 'osv-output'        # Thư mục con trong nguồn
  extension: '.json'                  # Định dạng file
  
  # --- Hành vi ---
  versions_from_repo: false           # Lấy versions từ git tags?
  detect_cherrypicks: false           # Phát hiện cherry-pick?
  strict_validation: true            # Validate nghiêm ngặt?
  editable: false                     # Cho phép OSV tự sửa?
  
  # --- Hiển thị ---
  link: 'https://...'                 # Base URL nguồn
  human_link: 'https://...{{ BUG_ID }}' # URL human-readable (Jinja2)
```

### 3.2 Phân Loại Nguồn Theo Độ Tin Cậy

```
Tier 1 — AUTHORITATIVE (Nguồn chính thức):
  ├── cve-osv     (NVD/CVE.org — nguồn CVE gốc)
  ├── ghsa        (GitHub Security Advisory)
  ├── go          (Go Vulnerability Database)
  └── oss-fuzz    (Google OSS-Fuzz)

Tier 2 — TRUSTED (Nguồn đáng tin cậy):
  ├── python / PYSEC   (PyPI Advisory Database)
  ├── rust / RUSTSEC   (RustSec Advisory DB)
  ├── debian-*         (Debian Security)
  ├── ubuntu-*         (Ubuntu Security)
  └── redhat           (Red Hat Security)

Tier 3 — COMMUNITY (Nguồn cộng đồng):
  ├── malicious-packages (OSSF)
  ├── bitnami
  ├── drupal
  └── <other ecosystem-specific>

Tier 4 — EXTERNAL_VENDOR (Vendor tự báo cáo):
  ├── almalinux-*
  ├── rockylinux-*
  ├── azurelinux
  └── chainguard / minimos / ...
```

### 3.3 Chu Kỳ Đồng Bộ Theo Loại Nguồn

| Nguồn | Loại | Chu kỳ sync | Ưu tiên | Ghi chú |
|-------|------|-------------|---------|---------|
| `cve-osv` | Bucket | 15 phút | Cao | NVD feed cập nhật thường xuyên |
| `ghsa` | Git | 10 phút | Cao | Webhook có thể thay thế polling |
| `oss-fuzz` | Git | 30 phút | Cao | OSS-Fuzz có volume lớn |
| `go` | Bucket | 30 phút | Trung | Go VulnDB tốc độ vừa |
| `python`, `rust` | Git | 1 giờ | Trung | Cập nhật theo release mới |
| `debian-*`, `ubuntu-*` | Bucket/Git | 1 giờ | Trung | Release theo DSA/USN |
| `redhat` | REST | 2 giờ | Trung | RHSA cập nhật theo lô |
| `almalinux-*` | Git | 4 giờ | Thấp | Kế thừa từ RHEL |
| `chainguard`, `minimos` | REST | 6 giờ | Thấp | Vendor mới |
| Các nguồn còn lại | Mọi loại | 12 giờ | Thấp | Cập nhật ít thường xuyên |

### 3.4 Source Sync Service — Deployment Chi Tiết

```yaml
# services/source-sync — Cấu hình triển khai đề xuất
Service: source-sync
Role: Orchestrator lịch sync cho tất cả nguồn

Replicas: 1 (singleton — tránh duplicate scheduling)
Resources:
  CPU: 0.5 vCPU
  Memory: 512Mi

Config (config.yaml):
  sync_interval_default: 1h
  sync_intervals:
    cve-osv: 15m
    ghsa: 10m
    oss-fuzz: 30m
    go: 30m
  
  circuit_breaker:
    threshold: 5          # Số lỗi liên tiếp trước khi mở circuit
    timeout: 5m           # Thời gian chờ trước khi thử lại
    half_open_max: 1      # Số request thử khi half-open
  
  rate_limiter:
    global_rps: 50        # Request/giây toàn cục
    per_source_rps: 5     # Request/giây mỗi nguồn
  
  http:
    timeout: 60s
    retry_max: 3
    retry_backoff: exponential
    retry_backoff_base: 2s
    retry_backoff_max: 30s

Publishes to: NATS JetStream "source.sync.{source_name}"
```

---

## 4. Phân Loại CVE (CVE Classification Pipeline)

### 4.1 Sơ Đồ Pipeline Phân Loại

```
                    Raw CVE Record
                          │
              ┌───────────▼───────────┐
              │   Schema Validation   │
              │   (OSV Schema v1.7.5) │
              └───────────┬───────────┘
                          │ Valid
              ┌───────────▼───────────┐
              │   Deduplication       │
              │   • By CVE ID         │
              │   • By Alias group    │
              │   • By content hash   │
              └───────────┬───────────┘
                          │ New/Changed
        ┌─────────────────┼─────────────────┐
        │                 │                 │
        ▼                 ▼                 ▼
┌───────────────┐ ┌───────────────┐ ┌──────────────────┐
│  Ecosystem    │ │  Severity     │ │  Alias Resolution │
│  Classification│ │  Scoring     │ │  • CVE ↔ GHSA    │
│  • Ecosystem  │ │  • CVSS v3/4 │ │  • CVE ↔ PYSEC   │
│    detection  │ │  • EPSS score│ │  • Cross-source   │
│  • Package    │ │  • Risk tier │ │    deduplication  │
│    mapping    │ │  • KEV check │ │                   │
└───────┬───────┘ └───────┬───────┘ └────────┬─────────┘
        └─────────────────┼─────────────────-┘
                          │
              ┌───────────▼───────────┐
              │   AI Enrichment       │
              │   • Tag generation    │
              │   • Summary (LLM)     │
              │   • Vector embedding  │
              │   • Attack pattern    │
              └───────────┬───────────┘
                          │
              ┌───────────▼───────────┐
              │  Version Impact       │
              │  • Git bisection      │
              │  • Affected versions  │
              │  • Cherry-pick detect │
              └───────────┬───────────┘
                          │
              ┌───────────▼───────────┐
              │  Storage & Indexing   │
              │  • Firestore          │
              │  • GCS (JSON)         │
              │  • OpenSearch         │
              └───────────────────────┘
```

### 4.2 Phân Loại Theo Ecosystem

```yaml
# Mapping ecosystem → ưu tiên xử lý
ecosystem_priority:
  critical:    # Xử lý ngay
    - PyPI
    - npm
    - Go
    - Maven
    - crates.io
    - NuGet
  
  high:        # Xử lý trong 5 phút
    - Debian
    - Ubuntu
    - Red Hat
    - Alpine
    - Android
  
  normal:      # Xử lý trong 1 giờ
    - Packagist
    - RubyGems
    - Hackage
    - crates.io
    - OSS-Fuzz
  
  low:         # Xử lý theo batch
    - AlmaLinux
    - Rocky Linux
    - Bitnami
    - openSUSE
    - openEuler
```

### 4.3 Phân Loại Theo Mức Độ Nghiêm Trọng

```
┌──────────────────────────────────────────────────────────┐
│                  CVE SEVERITY TIERS                       │
│                                                          │
│  CRITICAL  CVSS ≥ 9.0 + KEV                            │
│  ──────────────────────────────────────────────────────  │
│  HIGH      CVSS 7.0–8.9 hoặc CVSS ≥ 9.0 (no KEV)      │
│  ──────────────────────────────────────────────────────  │
│  MEDIUM    CVSS 4.0–6.9                                 │
│  ──────────────────────────────────────────────────────  │
│  LOW       CVSS < 4.0                                   │
│  ──────────────────────────────────────────────────────  │
│  UNKNOWN   Chưa có CVSS score                           │
└──────────────────────────────────────────────────────────┘

KEV = CISA Known Exploited Vulnerabilities Catalog
```

### 4.4 Auto-Tagging System

```yaml
# Các tag tự động sinh ra khi import CVE
auto_tags:
  attack_vector:
    - NETWORK        # AV:N trong CVSS
    - ADJACENT       # AV:A
    - LOCAL          # AV:L
    - PHYSICAL       # AV:P
  
  impact_type:
    - RCE            # Remote Code Execution
    - PRIVESC        # Privilege Escalation
    - DOS            # Denial of Service
    - INFO_DISC      # Information Disclosure
    - SQLI           # SQL Injection
    - XSS            # Cross-Site Scripting
    - SSRF           # Server-Side Request Forgery
  
  source_tier:
    - tier-1-authoritative
    - tier-2-trusted
    - tier-3-community
    - tier-4-vendor
  
  status:
    - has-fix        # Có phiên bản vá
    - no-fix         # Chưa có vá
    - withdrawn      # Đã rút lại
    - disputed       # Đang tranh luận
    - kev            # Trong danh sách KEV
```

### 4.5 AI Enrichment Service — Chi Tiết Phân Loại

```yaml
# services/ai-enrichment cấu hình đề xuất
Providers:
  primary: vertex-ai    # Google Vertex AI (production)
  secondary: ollama     # Local LLM (dev/staging)
  fallback: openai      # OpenAI (backup)

Models:
  embedding: text-embedding-004    # Vector embeddings
  classification: gemini-1.5-flash # Tag/category classification
  summarization: gemini-1.5-pro    # Long description → summary

Tasks:
  1. tag_generation:
     input: CVE description + affected packages
     output: [attack_vector, impact_type, cwe_ids, keywords]
     
  2. severity_assessment:
     input: CVSS vector + KEV status + package popularity
     output: exploitability_score ∈ [0, 1]
     
  3. summary_generation:
     input: full CVE details + references
     output: 2-3 câu tóm tắt kỹ thuật
     
  4. vector_embedding:
     input: summary + tags + package names
     output: 768-dim float32 vector (cho semantic search)
```

---

## 5. Quản Lý Kết Nối (Connection Management)

### 5.1 Kiến Trúc Connection Pool

```
┌─────────────────────────────────────────────────────────┐
│              CONNECTION MANAGER                          │
│                                                         │
│  ┌─────────────────────────────────────────────────┐   │
│  │          PER-SOURCE CONNECTION POOL              │   │
│  │                                                  │   │
│  │  Source A ──► [Conn 1] [Conn 2] [Conn 3]       │   │
│  │  Source B ──► [Conn 1] [Conn 2]                │   │
│  │  Source C ──► [Conn 1]                         │   │
│  │                                                  │   │
│  │  Max connections per source: configurable        │   │
│  └─────────────────────────────────────────────────┘   │
│                                                         │
│  ┌─────────────────────────────────────────────────┐   │
│  │           CIRCUIT BREAKER (per source)           │   │
│  │                                                  │   │
│  │  CLOSED → [5 failures] → OPEN → [5min] → HALF   │   │
│  │                                         ↑        │   │
│  │                             HALF → [1 success] ──┘  │
│  │                             HALF → [failure] → OPEN │   │
│  └─────────────────────────────────────────────────┘   │
│                                                         │
│  ┌─────────────────────────────────────────────────┐   │
│  │              RATE LIMITER (token bucket)          │   │
│  │                                                  │   │
│  │  Global:    50 RPS                               │   │
│  │  Git clone: 10 concurrent                        │   │
│  │  REST API:  5 RPS/source                         │   │
│  │  GCS:       100 RPS/source                       │   │
│  └─────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

### 5.2 Connection Configuration Per Source Type

#### Type 0 — Git Repository
```yaml
git_connection:
  transport: https   # hoặc ssh
  timeout: 300s      # Git clone/fetch timeout
  depth: 0           # Full clone (0 = unlimited)
  max_concurrent: 5  # Số clone song song tối đa
  
  credentials:
    type: ssh_key    # hoặc token, none
    key_path: /secrets/git-ssh-key
    # hoặc
    type: token
    token_env: GIT_TOKEN
  
  retry:
    max_attempts: 3
    backoff: exponential
    initial_delay: 5s
    max_delay: 60s
  
  circuit_breaker:
    failure_threshold: 3
    recovery_timeout: 10m
```

#### Type 1 — GCS Bucket
```yaml
gcs_connection:
  project: osv-production
  credentials: workload_identity   # GKE Workload Identity
  max_concurrent_blobs: 20        # Parallel blob reads
  timeout_per_blob: 30s
  
  retry:
    max_attempts: 5
    backoff: exponential
    initial_delay: 1s
    max_delay: 30s
  
  # Không cần circuit breaker — GCS highly available
```

#### Type 2 — REST API
```yaml
rest_connection:
  timeout: 60s
  max_body_size: 100MB   # Giới hạn response size
  follow_redirects: true
  max_redirects: 5
  
  headers:
    User-Agent: "OSV-dev/1.0 (+https://osv.dev)"
    Accept: "application/json"
  
  retry:
    max_attempts: 3
    backoff: exponential
    initial_delay: 2s
    max_delay: 30s
    retry_on: [429, 500, 502, 503, 504]
  
  rate_limit:
    rps: 5              # Tối đa 5 request/giây
    burst: 10           # Burst ngắn cho phép đến 10 RPS
  
  circuit_breaker:
    failure_threshold: 5
    recovery_timeout: 5m
```

### 5.3 Health Check & Monitoring Per Source

```yaml
# Trạng thái nguồn và cách theo dõi
source_health:
  check_interval: 5m
  
  metrics:
    - source_sync_duration_seconds     # Thời gian sync mỗi nguồn
    - source_sync_records_total        # Số record sync được
    - source_sync_errors_total         # Số lỗi
    - source_sync_last_success_time    # Timestamp sync thành công cuối
    - source_circuit_breaker_state     # CLOSED=0, OPEN=1, HALF=2
    - source_connection_pool_usage     # % pool đang dùng
  
  alerts:
    - name: SourceSyncFailed
      condition: source_sync_errors_total > 3 (5m window)
      severity: warning
      
    - name: SourceNotSynced
      condition: time_since_last_success > 2 * sync_interval
      severity: critical
      
    - name: CircuitBreakerOpen
      condition: source_circuit_breaker_state == 1
      severity: warning
```

### 5.4 Source Registry & Lifecycle Management

```
Source Lifecycle States:
  
  CONFIGURED ──► INITIALIZING ──► ACTIVE
                      │               │
                      ▼               ▼
                   FAILED        DEGRADED ──► ACTIVE (recovery)
                      │               │
                      └───────────────▼
                               DISABLED (manual)
                                   │
                                   ▼
                               REMOVED
```

```yaml
# API quản lý nguồn (internal admin API)
endpoints:
  GET  /admin/sources              # List tất cả nguồn và trạng thái
  GET  /admin/sources/{name}       # Chi tiết nguồn
  POST /admin/sources/{name}/sync  # Trigger sync thủ công
  PUT  /admin/sources/{name}       # Cập nhật config nguồn
  POST /admin/sources/{name}/pause # Tạm dừng sync
  POST /admin/sources/{name}/resume # Tiếp tục sync
  GET  /admin/sources/{name}/logs  # Import logs
  GET  /admin/sources/{name}/stats # Thống kê
```

---

## 6. Infrastructure Triển Khai

### 6.1 Kubernetes Deployment (Production)

```yaml
# Namespace và resource allocation đề xuất
namespace: osv-system

services:
  # --- Core Query Path (High Priority) ---
  api-gateway:
    replicas: 3
    cpu: "500m–2000m"
    memory: "256Mi–1Gi"
    autoscale: HPA (CPU > 70%)
    
  vulnerability-query:
    replicas: 2–5
    cpu: "500m–2000m"
    memory: "512Mi–2Gi"
    autoscale: HPA (RPS-based)
    
  search:
    replicas: 2
    cpu: "500m–1000m"
    memory: "512Mi–1Gi"
    
  web-bff:
    replicas: 2
    cpu: "250m–500m"
    memory: "256Mi–512Mi"

  # --- Data Pipeline (Medium Priority) ---
  ingestion:
    replicas: 2–10
    cpu: "500m–2000m"
    memory: "512Mi–2Gi"
    autoscale: HPA (NATS queue depth)
    
  impact-analysis:
    replicas: 2–8
    cpu: "1000m–4000m"    # CPU-intensive (git operations)
    memory: "1Gi–4Gi"
    autoscale: HPA (queue depth)
    
  ai-enrichment:
    replicas: 1–4
    cpu: "500m–2000m"
    memory: "512Mi–2Gi"
    autoscale: HPA (queue depth)

  # --- Background Services (Low Priority) ---
  source-sync:
    replicas: 1            # Singleton
    cpu: "250m–500m"
    memory: "256Mi–512Mi"
    
  alias-relations:
    replicas: 1–2
    cpu: "250m–500m"
    memory: "256Mi–512Mi"
    
  version-index-controller:
    replicas: 1
    cpu: "250m"
    memory: "256Mi"
    
  version-index-worker:
    replicas: 3–10
    cpu: "500m–2000m"
    memory: "512Mi–2Gi"
    autoscale: HPA (queue depth)
    
  notification:
    replicas: 1–2
    cpu: "250m–500m"
    memory: "256Mi–512Mi"
```

### 6.2 Infrastructure Components

```yaml
# Hạ tầng cần thiết
infrastructure:

  message_broker:
    type: NATS JetStream       # Dev/Staging/Production
    alternative: Cloud Pub/Sub # Production GCP native
    config:
      jetstream: enabled
      max_memory: 1GB
      max_file: 20GB
      replicas: 3              # HA mode
    
  cache:
    type: Redis 7.x
    mode: standalone           # Dev
    mode_prod: Redis Cluster   # Production
    memory: 512Mi–2Gi
    eviction: allkeys-lru
    
  search_engine:
    type: OpenSearch 2.x
    mode: single-node          # Dev
    mode_prod: 3-node cluster  # Production
    memory: 2Gi/node
    storage: 100Gi/node
    
  database:
    type: Firestore / Firestore Emulator
    mode_dev: Emulator (port 8200)
    mode_prod: Cloud Firestore (native)
    
  object_storage:
    type: GCS / MinIO
    mode_dev: MinIO (local)
    mode_prod: Google Cloud Storage
    buckets:
      - osv-vulnerabilities       # Curated exports
      - osv-source-snapshots      # Git repo caches
      - osv-import-logs           # Import failure logs
      
  observability:
    tracing: OpenTelemetry Collector → Jaeger/Cloud Trace
    metrics: Prometheus → Grafana
    logging: Structured JSON → Cloud Logging / ELK
```

### 6.3 Deployment Environments Detail

#### Local Development (Docker Compose)

```bash
# Khởi động infrastructure
docker-compose --profile infra up -d

# Khởi động tất cả services
docker-compose --profile all up -d

# Chỉ khởi động một service cụ thể
docker-compose up -d source-sync vulnerability-query

# Xem logs
docker-compose logs -f source-sync

# Port mặc định:
# API Gateway:    http://localhost:8080
# Web BFF:        http://localhost:8090
# Grafana:        http://localhost:3000
# Prometheus:     http://localhost:9091
# NATS Monitor:   http://localhost:8222
# OpenSearch:     http://localhost:9200
```

#### Staging (Kubernetes - Single Node)

```bash
# Namespace setup
kubectl create namespace osv-staging

# Deploy infrastructure
helm install osv-infra ./deployment/helm/infra \
  -n osv-staging \
  -f deployment/values/staging-infra.yaml

# Deploy services
helm install osv-services ./deployment/helm/services \
  -n osv-staging \
  -f deployment/values/staging-services.yaml

# Scale về minimum
kubectl scale deployment -n osv-staging --all --replicas=1
```

#### Production (GKE)

```bash
# Sử dụng Terraform + Cloud Deploy
cd deployment/terraform
terraform workspace select production
terraform apply -auto-approve

# Deploy via Cloud Deploy pipeline
gcloud deploy releases create release-$(date +%Y%m%d-%H%M%S) \
  --delivery-pipeline=osv-pipeline \
  --region=us-central1 \
  --images=...
```

---

## 7. CVE Source Onboarding Playbook

### 7.1 Quy Trình Thêm Nguồn Mới

```
Step 1: Phân tích nguồn
  ├── Xác định loại nguồn (Git/GCS/REST)
  ├── Đánh giá tần suất cập nhật
  ├── Xác định format dữ liệu (OSV Schema? Custom?)
  ├── Kiểm tra authentication cần thiết
  └── Đánh giá Tier (1-4)

Step 2: Cấu hình trong source.yaml
  ├── Thêm entry với đầy đủ field
  ├── Thiết lập ignore_patterns phù hợp
  ├── Cấu hình strict_validation
  └── Review với team security

Step 3: Test với source_test.yaml
  ├── Chạy importer ở dry-run mode
  ├── Kiểm tra sample records
  ├── Validate schema compliance
  └── Kiểm tra alias detection

Step 4: Deploy to Staging
  ├── Enable source với replicas=1
  ├── Monitor logs 24h
  ├── Verify record counts
  └── Check import error rate

Step 5: Production rollout
  ├── Announce trong team channel
  ├── Enable với gradual rollout
  ├── Monitor metrics 48h
  └── Tài liệu hóa nguồn mới
```

### 7.2 Checklist Nguồn Mới

```markdown
**Pre-conditions:**
- [ ] Nguồn có format OSV Schema hoặc có converter?
- [ ] Đã xác nhận update frequency?
- [ ] Authentication đã được setup?
- [ ] Deletion safety threshold đã cấu hình?

**source.yaml Config:**
- [ ] `name` unique, lowercase, dấu gạch ngang
- [ ] `type` đúng (0/1/2)
- [ ] `db_prefix` không trùng với nguồn khác
- [ ] `accepted_ecosystems` chính xác
- [ ] `ignore_patterns` regex đúng
- [ ] `strict_validation` đặt true nếu Tier 1/2
- [ ] `human_link` Jinja2 template đúng syntax

**Monitoring:**
- [ ] Alert đã được cấu hình
- [ ] Dashboard đã thêm source mới
- [ ] SLA document đã cập nhật
```

---

## 8. Security Model

### 8.1 Network Security

```
External Sources
      │
      │ HTTPS/SSH (outbound only)
      ▼
┌─────────────────────────────┐
│     source-sync service     │
│  (egress to internet)       │
└─────────────────────────────┘
      │ Internal gRPC/NATS
      ▼
┌─────────────────────────────┐
│   Internal service mesh     │
│   (mTLS - all services)     │
└─────────────────────────────┘
      │
      ▼
┌─────────────────────────────┐
│   api-gateway               │
│   (ingress từ internet)     │
│   JWT auth + rate limit     │
└─────────────────────────────┘
```

### 8.2 Secret Management

```yaml
# Secrets cần quản lý
secrets:
  git_ssh_keys:
    - oss-fuzz-ssh-key        # SSH key cho oss-fuzz repo
    - v8-ssh-key              # SSH key cho V8/Chromium
    store: Kubernetes Secrets / GCP Secret Manager
    
  api_tokens:
    - nvd-api-key             # NVD CVE API key
    - github-token            # Tránh rate limiting GitHub API
    store: GCP Secret Manager
    
  service_accounts:
    - gcs-reader-sa           # Đọc GCS buckets
    - gcs-writer-sa           # Ghi GCS buckets
    - firestore-sa            # Firestore access
    store: Workload Identity (GKE)
    
  encryption:
    - at_rest: Google-managed keys (GCS, Firestore)
    - in_transit: TLS 1.3 minimum
    - internal: mTLS (Istio / Linkerd)
```

### 8.3 Data Integrity Controls

```yaml
# Kiểm soát tính toàn vẹn dữ liệu
integrity_controls:
  
  import_validation:
    - schema_validation: true       # OSV Schema v1.7.5
    - id_format_check: true         # Đúng prefix theo source
    - timestamp_sanity: true        # published <= modified <= now
    - size_limit: 10MB/record       # Giới hạn kích thước
  
  deletion_safety:
    threshold: 10%                  # Từ chối xóa > 10% trong 1 lần
    require_confirmation: true      # Admin confirm nếu vượt ngưỡng
    audit_log: true                 # Ghi log mọi xóa hàng loạt
  
  audit_trail:
    all_imports: true               # Log mọi record được import
    all_modifications: true         # Log mọi thay đổi
    all_deletions: true             # Log mọi xóa
    retention: 90_days
```

---

## 9. Observability Stack

### 9.1 Metrics Dashboard Đề Xuất

```
Grafana Dashboards:

1. "CVE Source Health" Dashboard
   ├── Source sync status (heatmap: green=OK, red=error)
   ├── Records ingested per source (bar chart, 24h)
   ├── Sync duration per source (line chart)
   ├── Circuit breaker states (gauge per source)
   └── Error rate per source (line chart)

2. "CVE Pipeline Throughput" Dashboard
   ├── Records/hour through pipeline (line)
   ├── NATS queue depths (line)
   ├── Processing latency P50/P95/P99 (histogram)
   ├── Impact analysis duration (heatmap)
   └── AI enrichment queue (bar)

3. "API Performance" Dashboard
   ├── RPS per endpoint (line)
   ├── Latency P50/P95/P99 (histogram)
   ├── Error rate 4xx/5xx (line)
   ├── Active connections (gauge)
   └── Cache hit rates (gauge)

4. "CVE Classification" Dashboard
   ├── CVEs by severity tier (pie)
   ├── CVEs by ecosystem (bar)
   ├── New CVEs per day (line, 30d)
   ├── KEV coverage (gauge)
   └── AI tagging accuracy (gauge)
```

### 9.2 Alerts

```yaml
# Critical Alerts (PagerDuty / immediate response)
critical_alerts:
  - name: APIErrorRateHigh
    condition: http_request_error_rate > 5% (5m)
    
  - name: SourceSyncAllFailed
    condition: all sources sync_last_success > 2h
    
  - name: FirestoreUnavailable
    condition: firestore_connection_errors > 0 (1m)
    
  - name: NATSQueueDepthCritical
    condition: nats_queue_depth > 100000

# Warning Alerts (Slack notification)
warning_alerts:
  - name: SourceSyncDelayed
    condition: source_sync_last_success > 2 * scheduled_interval
    
  - name: CircuitBreakerOpen
    condition: any circuit_breaker_state == OPEN
    
  - name: ImpactAnalysisQueueGrowing
    condition: impact_analysis_queue > 10000 (30m trend)
    
  - name: AIEnrichmentBacklogged
    condition: ai_enrichment_queue > 5000
```

---

## 10. Rollout Plan

### 10.1 Phase 1: Foundation (Tuần 1-2)

```
[ ] Deploy infrastructure (NATS, Redis, OpenSearch, Firestore emulator)
[ ] Deploy core services (vulnerability-query, api-gateway)
[ ] Migrate dữ liệu từ Python stack sang Go services
[ ] Verify API compatibility (backward compatible)
[ ] Setup monitoring dashboards
[ ] Load testing với production data snapshot
```

### 10.2 Phase 2: Source Management (Tuần 3-4)

```
[ ] Deploy source-sync service
[ ] Cấu hình tất cả 30+ nguồn từ source.yaml
[ ] Thiết lập circuit breakers và rate limiters
[ ] Enable sync với Tier 1 sources trước (CVE, GHSA, Go, OSS-Fuzz)
[ ] Monitor 72h, tune schedules
[ ] Enable Tier 2-4 sources dần dần
[ ] Thiết lập source health alerts
```

### 10.3 Phase 3: Classification (Tuần 5-6)

```
[ ] Deploy ai-enrichment service
[ ] Pull Ollama model cho local dev
[ ] Cấu hình Vertex AI cho production
[ ] Enable AI tagging cho new CVEs
[ ] Backfill tags cho existing records (batch job)
[ ] Deploy alias-relations service
[ ] Verify cross-source deduplication
[ ] Enable severity auto-classification
```

### 10.4 Phase 4: Production Hardening (Tuần 7-8)

```
[ ] Enable mTLS giữa tất cả services
[ ] Setup GCP Secret Manager integration
[ ] Enable Workload Identity cho GCS/Firestore access
[ ] Load test production scale
[ ] Chaos engineering (inject failures)
[ ] DR drill (restore từ backup)
[ ] Documentation hoàn chỉnh
[ ] Team training
```

---

## 11. Tham Khảo

| Tài liệu | Đường dẫn |
|----------|-----------|
| Architecture Overview | [specs/01-architecture.md](./01-architecture.md) |
| Technical Design | [specs/02-technical-design.md](./02-technical-design.md) |
| Service Specs | [specs/services/](./services/) |
| Source Config | [source.yaml](../source.yaml) |
| Docker Compose | [docker-compose.yml](../docker-compose.yml) |
| Infrastructure | [specs/services/13-infrastructure.md](./services/13-infrastructure.md) |

---

*Tài liệu được tạo ngày 2026-06-03 — dựa trên phân tích kiến trúc hiện tại tại `/services/`, `docker-compose.yml`, và `source.yaml`.*
