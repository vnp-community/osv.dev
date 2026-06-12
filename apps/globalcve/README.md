# GlobalCVE Monolithic App

A high-performance monolithic Go application that consolidates all GlobalCVE services into a single deployable binary. Each service runs as an independent goroutine, communicating via direct function calls, HTTP, and NATS JetStream.

## Architecture

```
                    Port 8080 (single external entry point)
                         │
                    API Gateway
                    /     |     \
              (direct)  (proxy) (proxy)
                 │        │       │
           CVE Search  KEV Svc  Notification
              :8081     :8083     :8084
                 │
            CVE Sync (background)
              :8082
```

## Quick Start

### 1. Start Infrastructure
```bash
docker-compose up -d
```

### 2. Run the app
```bash
cp .env.example .env
# Edit .env with your settings
make run
```

### 3. Try the API
```bash
# Search CVEs
curl "http://localhost:8080/api/v2/cves?query=log4j&severity=CRITICAL&limit=5"

# Get by ID
curl "http://localhost:8080/api/v2/cves/CVE-2021-44228"

# Check KEV status
curl "http://localhost:8080/api/v2/kev/check?ids=CVE-2021-44228,CVE-2023-44487"

# Get KEV stats
curl "http://localhost:8080/api/v2/kev/stats"

# Health
curl "http://localhost:8080/health"
```

## Services

| Service | Port | Description |
|---------|------|-------------|
| API Gateway | 8080 | Single external entry point, CORS, rate limiting |
| CVE Search | 8081 | Search, filter, paginate CVEs from PostgreSQL |
| CVE Sync | 8082 | Background sync from NVD, CIRCL, JVN, ExploitDB, CVE.org |
| KEV Service | 8083 | CISA KEV catalog, bulk check, stats |
| Notification | 8084 | Webhook management, NATS event dispatch |
| Metrics | 9090 | Prometheus metrics |

## Data Sources

| Source | Frequency | Data |
|--------|-----------|------|
| NVD | Every 2h | CVE records, CVSS scores |
| CIRCL | Every 6h | CVE enrichment |
| JVN | Every 1h | Japanese vulnerability database |
| ExploitDB | Daily 2am | Public exploits |
| CVE.org | Every 12h | Official CVE records |
| EPSS | Daily 3am | Exploit Prediction Scoring System |
| CISA KEV | Every 6h | Known Exploited Vulnerabilities |

## Build

```bash
make build
./build/globalcve-mono --config config/config.yaml
```

## Configuration

Copy and edit `config/config.yaml`. All values can be overridden via environment variables using UPPER_SNAKE_CASE format.

Key environment variables:
- `DATABASE_URL` — PostgreSQL connection URL
- `REDIS_URL` — Redis connection URL
- `OPENSEARCH_URL` — OpenSearch URL (optional, falls back to PostgreSQL GIN)
- `NATS_URL` — NATS JetStream URL (optional, events disabled if unavailable)
- `NVD_API_KEY` — NVD API key for higher rate limits

## API Reference

### CVE Search

```
GET /api/v2/cves
  ?query=log4j          # keyword or CVE-YYYY-NNNN
  &severity=CRITICAL    # CRITICAL|HIGH|MEDIUM|LOW
  &source=NVD           # NVD|CIRCL|JVN|EXPLOITDB|CVE.ORG
  &sort=newest          # newest|oldest|cvss_desc|epss_desc
  &page=0               # 0-indexed
  &limit=50             # 1-100
  &kev=true             # only KEV entries
  &min_epss=0.5         # minimum EPSS score

GET /api/v2/cves/{CVE-ID}
```

### KEV Service

```
GET /api/v2/kev                      # list KEV entries
GET /api/v2/kev/{CVE-ID}             # single KEV entry
GET /api/v2/kev/check?ids=CVE-...,CVE-...  # bulk check
GET /api/v2/kev/stats                # statistics
```

### Sync Management (requires auth)

```
GET  /api/v2/sync/status
POST /api/v2/sync/trigger
POST /api/v2/sync/trigger/{source}   # NVD|CIRCL|JVN|EXPLOITDB|CVE.ORG|EPSS
```

### Webhooks (requires auth)

```
GET    /api/v2/webhooks
POST   /api/v2/webhooks     {"url": "...", "events": ["alert.triggered"]}
DELETE /api/v2/webhooks/{id}
```

## License

See root LICENSE file.
