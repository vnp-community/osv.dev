# GlobalCVE v3.0 — Monolithic Go App: Architectural Solutions

## 1. Tổng Quan Kiến Trúc

GlobalCVE v3.0 chuyển từ kiến trúc **Next.js serverless** sang **monolithic Go app** với các "module services" là **goroutine độc lập** hoạt động trong cùng một process.

```
┌─────────────────────────────────────────────────────────────────────┐
│              globalcve-mono (Single Go Binary — Port 8080)           │
│                                                                       │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │                    main.go — errgroup + signal               │    │
│  └──────────────────────┬──────────────────────────────────────┘    │
│                         │                                            │
│           Shared Infrastructure (injected via DI)                    │
│    PostgreSQL+pgvector | Redis | OpenSearch | NATS JetStream         │
│                                                                       │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────┐  │
│  │ API Gateway  │ │ CVE Search   │ │ CVE Sync     │ │ KEV Svc  │  │
│  │ :8080        │ │ :8081        │ │ :8082        │ │ :8083    │  │
│  │ Goroutine    │ │ Goroutine    │ │ Goroutine    │ │ Goroutine│  │
│  └──────┬───────┘ └──────▲───────┘ └──────┬───────┘ └────┬─────┘  │
│         │ Direct call     │                │ NATS          │ NATS   │
│         │ (same process)  │                │               │        │
│         └─────────────────┘                └───────────────┘        │
│                                                                       │
│  ┌──────────────┐                                                    │
│  │ Notification │  NATS subscriber: alert.triggered                 │
│  │ :8084        │  Dispatches webhooks                               │
│  │ Goroutine    │                                                    │
│  └──────────────┘                                                    │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 2. Giải Pháp: Inter-Service Communication

### 2.1 Direct Function Call (Zero-latency — trong process)

**Khi nào dùng:** API Gateway → CVE Search Service (hot path — mọi API request)

**Pattern:**
```go
// Gateway inject CVE Search handler trực tiếp — không qua mạng
gatewaySvc := gateway.New(cfg, redis, cveSearchSvc.Handler())
```

**Ưu điểm:**
- Zero network overhead
- Type-safe function call
- Không cần serialization/deserialization

### 2.2 HTTP Internal Proxy (Loose coupling)

**Khi nào dùng:** Gateway → KEV Service, Gateway → Notification Service, Gateway → CVE Sync admin

**Pattern:**
```go
// Gateway proxy đến internal HTTP server của service khác
r.Get("/kev", proxyTo("http://localhost:8083"))
```

Mỗi service expose một HTTP server nội bộ. Gateway làm reverse proxy với `httputil.ReverseProxy`.

**Ưu điểm:**
- Services có thể tách ra thành microservice sau này chỉ cần đổi URL
- Load balancing có thể được thêm vào sau

### 2.3 NATS JetStream (Event-driven — async)

**Khi nào dùng:** Background notifications, cache invalidation, cross-service data propagation

**Event flow:**
```
CVE Sync ──(cve.synced)──→ [JetStream: CVE_EVENTS] ──→ Notification
KEV Svc  ──(kev.updated)──→ [JetStream: KEV_EVENTS] ──→ CVE Sync (mark is_kev)
CVE Sync ──(alert.triggered)──→ [JetStream: ALERT_EVENTS] ──→ Notification
```

**Streams configuration:**
| Stream | Subject | Retention |
|--------|---------|-----------|
| `CVE_EVENTS` | `cve.>` | 24h |
| `KEV_EVENTS` | `kev.>` | 24h |
| `ALERT_EVENTS` | `alert.>` | 48h |

---

## 3. Giải Pháp: Codebase Reuse

### 3.1 Domain Entities — Adapted

| Original | Adapted | Thay Đổi |
|----------|---------|----------|
| `vulnerability-service/entity/cve.go` | `cvesearch/domain/entity/cve.go` | Bỏ bson tags, thêm pgvector embedding |
| `vulnerability-service/domain/kev/` | `kevservice/domain/entity/kev.go` | Consolidate fields từ spec |
| `ingestion-service/domain/sync_job.go` | `cvesync/domain/entity/sync_job.go` | Thêm SourceName constants mới |

### 3.2 Fetchers — Adapted (MongoDB → PostgreSQL)

| Original | Adapted | Thay Đổi |
|----------|---------|----------|
| `ingestion-service/fetcher/nvd_cve.go` | `cvesync/fetcher/nvd_cve.go` | `UpsertBatch` → PostgreSQL |
| `ingestion-service/fetcher/epss.go` | `cvesync/fetcher/epss.go` | `UpdateEPSS` → PostgreSQL |
| `ingestion-service/fetcher/mitre_capec.go` | (tương lai) | — |

### 3.3 New Fetchers — Ported from TypeScript

| TypeScript Source | Go Implementation |
|------------------|------------------|
| `src/app/api/cves/route.ts` (CIRCL) | `fetcher/circl.go` |
| `src/lib/jvn.ts` | `fetcher/jvn.go` |
| `src/lib/exploitdb.ts` | `fetcher/exploitdb.go` |
| `src/app/api/cves/route.ts` (CVE.org) | `fetcher/cveorg.go` |
| `src/lib/kev.ts` | `kevservice/adapter/cisa/client.go` |

---

## 4. Giải Pháp: Database

### 4.1 PostgreSQL Schema Key Design Decisions

**CVEs Table — Single table, no sharding**
- `id TEXT PRIMARY KEY` — CVE-YYYY-NNNN format (natural key)
- GIN index for full-text search: `to_tsvector('english', id || description || summary)`
- ivfflat index for pgvector: `embedding <=> $1` (cosine similarity)
- `is_kev BOOLEAN` — denormalized từ kev_entries (performance)
- `severity TEXT CHECK (...)` — enum constraint ở DB level

**Upsert Strategy:**
```sql
INSERT INTO cves (...) VALUES (...)
ON CONFLICT (id) DO UPDATE SET
  description = EXCLUDED.description,
  -- COALESCE preserves existing value if incoming is NULL
  cvss3_score = COALESCE(EXCLUDED.cvss3_score, cves.cvss3_score),
  ...
RETURNING (xmax = 0) AS is_insert  -- xmax = 0 means new row
```

### 4.2 Redis Cache Keys

| Pattern | TTL | Content |
|---------|-----|---------|
| `search:{md5(query+params)}` | 5 min | JSON search results |
| `cve:{CVE-ID}` | 60 min | JSON CVE record |
| `rl:ip:{ip}` | rolling window | Rate limit counter |

---

## 5. Giải Pháp: Goroutine Lifecycle Management

### 5.1 errgroup + signal context

```go
// main.go pattern
ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer cancel()

g, gctx := errgroup.WithContext(ctx)
g.Go(func() error { return cveSyncSvc.Start(gctx) })
g.Go(func() error { return cveSearchSvc.Start(gctx) })
g.Go(func() error { return kevSvc.Start(gctx) })
g.Go(func() error { return notifSvc.Start(gctx) })
g.Go(func() error { return gatewaySvc.Start(gctx) })
g.Wait()
```

### 5.2 Graceful Shutdown Flow

```
SIGTERM →
  1. context cancelled (gctx.Done())
  2. Each service's HTTP server: server.Shutdown(15s timeout)
  3. Cron schedulers: cron.Stop().Done()
  4. NATS consumers: consumer.Stop()
  5. errgroup.Wait() → return nil
  6. Deferred: pool.Close(), redis.Close(), nats.Close()
```

### 5.3 Error Handling

- Nếu **một service goroutine lỗi**, `errgroup` cancel context cho tất cả goroutines khác → graceful shutdown
- NATS/OpenSearch không available → fail-open (warn, tiếp tục)
- Database không available → fail-fast (app không start)

---

## 6. Giải Pháp: API Compatibility

### 6.1 GlobalCVE API Routes (Port 8080)

| Method | Path | Handler | Auth |
|--------|------|---------|------|
| GET | `/health` | Gateway aggregate health | No |
| GET | `/api/v2/cves` | CVE Search (direct call) | No |
| GET | `/api/v2/cves/{id}` | CVE GetByID (direct call) | No |
| GET | `/api/v2/kev` | KEV List (proxy) | No |
| GET | `/api/v2/kev/{id}` | KEV GetByID (proxy) | No |
| GET | `/api/v2/kev/check?ids=` | KEV BulkCheck (proxy) | No |
| GET | `/api/v2/kev/stats` | KEV Stats (proxy) | No |
| GET | `/api/v2/webhooks` | List webhooks (proxy) | Yes |
| POST | `/api/v2/webhooks` | Create webhook (proxy) | Yes |
| DELETE | `/api/v2/webhooks/{id}` | Delete webhook (proxy) | Yes |
| GET | `/api/v2/sync/status` | Sync status (proxy) | Yes |
| POST | `/api/v2/sync/trigger` | Trigger all sync (proxy) | Yes |
| POST | `/api/v2/sync/trigger/{source}` | Trigger one source (proxy) | Yes |

### 6.2 CVE Search Query Parameters

Tương thích với Next.js API đang có:

| Parameter | Type | Description |
|-----------|------|-------------|
| `query` | string | Keyword / CVE ID |
| `severity` | CRITICAL/HIGH/MEDIUM/LOW | Severity filter |
| `source` | NVD/CIRCL/JVN/... | Data source filter |
| `sort` | newest/oldest/cvss_desc/epss_desc | Sort order |
| `page` | int | Page number (0-indexed) |
| `limit` | int | Page size (1-100, default 50) |
| `kev` | bool | Only KEV entries |
| `min_epss` | float | Minimum EPSS score |

---

## 7. Giải Pháp: Scheduler

### 7.1 Cron Schedule (robfig/cron v3 — với seconds field)

| Source | Schedule | Frequency |
|--------|----------|-----------|
| NVD CVE | `0 0 */2 * * *` | Mỗi 2 giờ |
| JVN RSS | `0 0 * * * *` | Mỗi 1 giờ |
| CIRCL | `0 0 */6 * * *` | Mỗi 6 giờ |
| ExploitDB | `0 0 2 * * *` | Hàng ngày 2am |
| CVE.org | `0 0 */12 * * *` | Mỗi 12 giờ |
| EPSS | `0 0 3 * * *` | Hàng ngày 3am |
| NVD CPE | `0 0 4 * * 0` | Chủ nhật 4am |
| CAPEC/CWE | `0 0 5 * * 0` | Chủ nhật 5am |
| CISA KEV | `0 0 */6 * * *` | Mỗi 6 giờ |

### 7.2 Sync Orchestrator Pattern

```go
// Parallel sync across all sources
func (o *Orchestrator) SyncAll(ctx context.Context) ([]*SyncResult, error) {
    g, gctx := errgroup.WithContext(ctx)
    for _, f := range o.fetchers {
        f := f
        g.Go(func() error {
            result := o.syncOne(gctx, f, FetchOptions{})
            // Non-fatal: capture in SyncResult.Err
            return nil
        })
    }
    g.Wait()
    ...
}
```

Mỗi sync được track trong bảng `sync_jobs` với status PENDING → RUNNING → COMPLETED/FAILED.

---

## 8. Thư Mục Cấu Trúc Cuối Cùng

```
osv.dev/apps/globalcve/
├── cmd/
│   └── main.go                          # Entry point
├── config/
│   └── config.yaml                      # Config file
├── internal/
│   ├── app/                             # Lifecycle management
│   │   └── app.go
│   ├── config/                          # Config loader (Viper)
│   │   └── config.go
│   ├── events/                          # NATS event types
│   │   └── events.go
│   ├── infra/                           # Shared infrastructure
│   │   ├── postgres/pool.go
│   │   ├── redis/client.go
│   │   ├── nats/client.go
│   │   └── opensearch/client.go
│   ├── cvesearch/                       # CVE Search Service goroutine
│   │   ├── domain/{entity,repository}/
│   │   ├── adapter/{postgres,redis}/
│   │   ├── usecase/search.go
│   │   ├── http/handler.go
│   │   └── service.go
│   ├── cvesync/                         # CVE Sync Service goroutine
│   │   ├── domain/{entity,repository}/
│   │   ├── fetcher/{fetcher,nvd_cve,circl,jvn,exploitdb,cveorg,epss}.go
│   │   ├── adapter/postgres/{cve_repo,sync_repo}.go
│   │   ├── usecase/orchestrator.go
│   │   ├── scheduler/scheduler.go
│   │   └── service.go
│   ├── kevservice/                      # KEV Service goroutine
│   │   ├── domain/{entity,repository}/
│   │   ├── adapter/{postgres,cisa}/
│   │   ├── usecase/usecase.go
│   │   └── service.go
│   ├── notification/                    # Notification Service goroutine
│   │   └── service.go
│   └── gateway/                         # API Gateway goroutine
│       └── service.go
├── migrations/                          # goose SQL migrations
│   ├── 001_create_cves.sql
│   ├── 002_create_sync_jobs.sql
│   ├── 003_create_kev_entries.sql
│   └── 004_create_support_tables.sql
├── docker-compose.yml                   # Infrastructure
├── .env.example
├── Makefile
├── go.mod
└── go.sum
```

---

## 9. Tương Lai: Scale Path

Khi cần scale ra microservices:
1. Mỗi goroutine service đã có HTTP server riêng → chỉ cần tách ra và deploy độc lập
2. API Gateway đang dùng `proxyTo()` → chỉ cần đổi URL từ `localhost:808X` sang service URL thật
3. Direct call (Gateway → CVE Search) → thay bằng gRPC client (proto files đã có tại `shared/proto`)
4. NATS events không thay đổi

---

*Solutions v1.0 | 2026-06-09 | GlobalCVE Monolithic Go App*
