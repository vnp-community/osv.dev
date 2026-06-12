# TASK-05 — CVE Sync Service

## Mục Tiêu

Implement **CVE Sync Service** — goroutine service quản lý việc fetch và sync CVE data từ nhiều nguồn (NVD, CIRCL, JVN, ExploitDB, CVE.org, EPSS) vào PostgreSQL, với scheduler cron và NATS event publishing.

## Phụ Thuộc

- TASK-03 (Database Migrations — tables `cves`, `sync_jobs` phải tồn tại)
- TASK-02 (NATS Client — để publish events)

## Đầu Ra

- `internal/cvesync/domain/entity/cve.go` — CVE ingest entity
- `internal/cvesync/domain/entity/sync_job.go` — SyncJob entity
- `internal/cvesync/domain/repository/cve_repository.go`
- `internal/cvesync/domain/repository/sync_repository.go`
- `internal/cvesync/adapter/postgres/cve_repo.go` — Upsert adapter
- `internal/cvesync/adapter/postgres/sync_repo.go`
- `internal/cvesync/fetcher/fetcher.go` — Fetcher interface
- `internal/cvesync/fetcher/nvd_cve.go`
- `internal/cvesync/fetcher/circl.go`
- `internal/cvesync/fetcher/jvn.go`
- `internal/cvesync/fetcher/exploitdb.go`
- `internal/cvesync/fetcher/cveorg.go`
- `internal/cvesync/fetcher/epss.go`
- `internal/cvesync/usecase/orchestrator.go`
- `internal/cvesync/scheduler/scheduler.go`
- `internal/cvesync/service.go`

---

## Checklist

- [x] Fetcher interface
- [x] NVD CVE fetcher (ported từ ingestion-service)
- [x] CIRCL fetcher (ported từ TypeScript)
- [x] JVN RSS fetcher
- [x] ExploitDB fetcher
- [x] CVE.org fetcher
- [x] EPSS fetcher
- [x] Sync Orchestrator (parallel + SyncResult tracking)
- [x] SyncJob CRUD (PENDING → RUNNING → COMPLETED/FAILED)
- [x] Cron scheduler với đúng schedules
- [x] NATS publisher cho `cve.synced` và `alert.triggered`
- [x] Admin HTTP API (status + trigger)
- [x] NATS subscriber cho `kev.updated` (update is_kev)
- [x] Service wrapper

---

## 1. Domain Entities

### `internal/cvesync/domain/entity/cve.go`

```go
package entity

import "time"

// CVEIngest — entity dùng khi insert/upsert (khác với cvesearch entity)
type CVEIngest struct {
    ID          string
    Description string
    Summary     string
    PublishedAt *time.Time
    ModifiedAt  *time.Time
    Severity    string
    CVSS3Score  *float64
    CVSS3Vector string
    CVSS2Score  *float64
    Source      string
    References  []string
    RawData     map[string]interface{}
}
```

### `internal/cvesync/domain/entity/sync_job.go`

```go
package entity

import "time"

type SyncStatus string

const (
    StatusPending   SyncStatus = "PENDING"
    StatusRunning   SyncStatus = "RUNNING"
    StatusCompleted SyncStatus = "COMPLETED"
    StatusFailed    SyncStatus = "FAILED"
)

// Source Name Constants
const (
    SourceNVD      = "NVD"
    SourceCIRCL    = "CIRCL"
    SourceJVN      = "JVN"
    SourceExploitDB = "EXPLOITDB"
    SourceCVEOrg   = "CVEORG"
    SourceEPSS     = "EPSS"
    SourceNVDCPE   = "NVD_CPE"
    SourceCISAKEV  = "CISA_KEV"
)

type SyncJob struct {
    ID        int64
    Source    string
    Status    SyncStatus
    Fetched   int
    Inserted  int
    Updated   int
    Errors    int
    ErrorMsg  string
    StartedAt *time.Time
    EndedAt   *time.Time
    CreatedAt time.Time
}

// SyncResult — kết quả một lần sync (in-memory, không persist toàn bộ)
type SyncResult struct {
    Source  string
    Fetched int
    Saved   int
    Err     error
}
```

---

## 2. Fetcher Interface (`internal/cvesync/fetcher/fetcher.go`)

```go
package fetcher

import (
    "context"
    "github.com/binhnt/globalcve/internal/cvesync/domain/entity"
)

// FetchOptions — tham số fetch (date range, page, ...)
type FetchOptions struct {
    Since    *time.Time // incremental sync từ ngày này
    MaxItems int
}

// Fetcher — interface mà tất cả fetchers phải implement
type Fetcher interface {
    // Name trả về source name constant (NVD, CIRCL, ...)
    Name() string
    // Fetch lấy CVEs và upsert vào DB, trả về số lượng
    Fetch(ctx context.Context, opts FetchOptions) (fetched int, saved int, err error)
}
```

---

## 3. NVD CVE Fetcher (`internal/cvesync/fetcher/nvd_cve.go`)

Port từ `ingestion-service/fetcher/nvd_cve.go`, thay MongoDB → PostgreSQL.

```go
// NVD API v2.0: https://services.nvd.nist.gov/rest/json/cves/2.0
// Rate limit: 5 req/30s without API key, 50 req/30s with API key
// Pagination: resultsPerPage + startIndex

type NVDFetcher struct {
    apiKey string
    repo   repository.CVERepository
    client *http.Client
}

func (f *NVDFetcher) Name() string { return entity.SourceNVD }

func (f *NVDFetcher) Fetch(ctx context.Context, opts FetchOptions) (int, int, error) {
    // 1. Build URL với lastModStartDate nếu có opts.Since
    // 2. Paginate: startIndex += resultsPerPage cho đến hết
    // 3. Parse NVD CVE JSON response
    // 4. Map → CVEIngest
    // 5. UpsertBatch (bulk insert với ON CONFLICT)
    // 6. Return fetched, saved, err
}
```

**NVD API Response structure:**
```go
type NVDResponse struct {
    ResultsPerPage int       `json:"resultsPerPage"`
    StartIndex     int       `json:"startIndex"`
    TotalResults   int       `json:"totalResults"`
    Vulnerabilities []NVDItem `json:"vulnerabilities"`
}

type NVDItem struct {
    CVE struct {
        ID          string `json:"id"`
        Descriptions []struct {
            Lang  string `json:"lang"`
            Value string `json:"value"`
        } `json:"descriptions"`
        Metrics struct {
            CVSSMetricV31 []struct {
                CVSSData struct {
                    BaseScore    float64 `json:"baseScore"`
                    VectorString string  `json:"vectorString"`
                    BaseSeverity string  `json:"baseSeverity"`
                } `json:"cvssData"`
            } `json:"cvssMetricV31"`
        } `json:"metrics"`
        Published string `json:"published"`
        LastModified string `json:"lastModified"`
        References []struct {
            URL string `json:"url"`
        } `json:"references"`
    } `json:"cve"`
}
```

---

## 4. Các Fetchers Khác

### CIRCL (`fetcher/circl.go`)
- Source: `https://cve.circl.lu/api/query`
- Port từ `src/app/api/cves/route.ts` (TypeScript)
- Response: `{"results": [...CVE objects...]}`

### JVN RSS (`fetcher/jvn.go`)
- Source: `https://jvndb.jvn.jp/en/rss/jvndb.rdf`
- Port từ `src/lib/jvn.ts`
- Parse RSS/RDF XML

### ExploitDB (`fetcher/exploitdb.go`)
- Source: Download CSV từ `https://gitlab.com/exploit-database/exploitdb/-/raw/main/files_exploits.csv`
- Port từ `src/lib/exploitdb.ts`
- Parse CSV, map đến CVE IDs

### CVE.org (`fetcher/cveorg.go`)
- Source: `https://cveawg.mitre.org/api/cve`
- Port từ `src/app/api/cves/route.ts`

### EPSS (`fetcher/epss.go`)
- Source: `https://epss.cyentia.com/epss_scores-current.csv.gz`
- Port từ `ingestion-service/fetcher/epss.go`
- `UpdateEPSS` → PostgreSQL (UPDATE cves SET epss_score=..., epss_percentile=... WHERE id=...)

---

## 5. Sync Orchestrator (`internal/cvesync/usecase/orchestrator.go`)

```go
package usecase

import (
    "context"
    "sync"

    "golang.org/x/sync/errgroup"
    "github.com/binhnt/globalcve/internal/cvesync/domain/entity"
    "github.com/binhnt/globalcve/internal/cvesync/fetcher"
)

type Orchestrator struct {
    fetchers []fetcher.Fetcher
    syncRepo repository.SyncRepository
    nats     *natsInfra.Client // nullable
    logger   zerolog.Logger
}

// SyncAll — chạy tất cả fetchers song song
func (o *Orchestrator) SyncAll(ctx context.Context) ([]*entity.SyncResult, error) {
    var mu sync.Mutex
    var results []*entity.SyncResult

    g, gctx := errgroup.WithContext(ctx)

    for _, f := range o.fetchers {
        f := f
        g.Go(func() error {
            result := o.syncOne(gctx, f, fetcher.FetchOptions{})
            mu.Lock()
            results = append(results, result)
            mu.Unlock()
            return nil // Non-fatal: lỗi được capture trong SyncResult.Err
        })
    }

    g.Wait()
    return results, nil
}

// SyncOne — sync một source cụ thể
func (o *Orchestrator) SyncOne(ctx context.Context, sourceName string) (*entity.SyncResult, error) {
    for _, f := range o.fetchers {
        if f.Name() == sourceName {
            return o.syncOne(ctx, f, fetcher.FetchOptions{}), nil
        }
    }
    return nil, fmt.Errorf("unknown source: %s", sourceName)
}

func (o *Orchestrator) syncOne(ctx context.Context, f fetcher.Fetcher, opts fetcher.FetchOptions) *entity.SyncResult {
    // 1. Create sync_job (PENDING → RUNNING)
    job, _ := o.syncRepo.Create(ctx, f.Name())
    o.syncRepo.UpdateStatus(ctx, job.ID, entity.StatusRunning)

    // 2. Fetch
    fetched, saved, err := f.Fetch(ctx, opts)

    // 3. Update sync_job (COMPLETED/FAILED)
    if err != nil {
        o.syncRepo.MarkFailed(ctx, job.ID, err.Error())
    } else {
        o.syncRepo.MarkCompleted(ctx, job.ID, fetched, saved)
    }

    // 4. Publish NATS event
    if err == nil && o.nats != nil {
        evt := events.CVESyncedEvent{
            Source:   f.Name(),
            Synced:   saved,
            SyncedAt: time.Now().Format(time.RFC3339),
        }
        data, _ := json.Marshal(evt)
        o.nats.JS.Publish(ctx, events.SubjectCVESynced, data)
    }

    return &entity.SyncResult{Source: f.Name(), Fetched: fetched, Saved: saved, Err: err}
}
```

---

## 6. Scheduler (`internal/cvesync/scheduler/scheduler.go`)

Theo §7.1 của architecture-solutions.md:

```go
package scheduler

import (
    "context"
    "github.com/robfig/cron/v3"
    "github.com/binhnt/globalcve/internal/cvesync/usecase"
)

type Scheduler struct {
    cron  *cron.Cron
    orch  *usecase.Orchestrator
}

func New(orch *usecase.Orchestrator) *Scheduler {
    c := cron.New(cron.WithSeconds()) // seconds field enabled
    s := &Scheduler{cron: c, orch: orch}

    // §7.1 Cron schedules
    c.AddFunc("0 0 */2 * * *", s.syncNVD)        // Mỗi 2 giờ
    c.AddFunc("0 0 * * * *",   s.syncJVN)         // Mỗi 1 giờ
    c.AddFunc("0 0 */6 * * *", s.syncCIRCL)       // Mỗi 6 giờ
    c.AddFunc("0 0 2 * * *",   s.syncExploitDB)   // Hàng ngày 2am
    c.AddFunc("0 0 */12 * * *",s.syncCVEOrg)      // Mỗi 12 giờ
    c.AddFunc("0 0 3 * * *",   s.syncEPSS)        // Hàng ngày 3am
    c.AddFunc("0 0 4 * * 0",   s.syncNVDCPE)      // Chủ nhật 4am
    c.AddFunc("0 0 5 * * 0",   s.syncCAPEC)       // Chủ nhật 5am

    return s
}

func (s *Scheduler) Start() { s.cron.Start() }
func (s *Scheduler) Stop()  { <-s.cron.Stop().Done() }

func (s *Scheduler) syncNVD()      { s.orch.SyncOne(context.Background(), entity.SourceNVD) }
func (s *Scheduler) syncJVN()      { s.orch.SyncOne(context.Background(), entity.SourceJVN) }
// ... etc
```

---

## 7. NATS Subscriber — KEV Updated

CVE Sync cần subscribe `kev.updated` để update `is_kev` flag:

```go
// Trong service.go Start()
func (s *Service) subscribeKEVUpdated(ctx context.Context) error {
    consumer, err := s.nats.JS.CreateOrUpdateConsumer(ctx, "KEV_EVENTS", jetstream.ConsumerConfig{
        Durable:       "cve-sync-kev-subscriber",
        FilterSubject: events.SubjectKEVUpdated,
    })
    if err != nil {
        return err
    }

    msgCtx, _ := consumer.Messages()
    go func() {
        defer msgCtx.Stop()
        for {
            select {
            case <-ctx.Done():
                return
            default:
                msg, err := msgCtx.Next()
                if err != nil {
                    continue
                }
                var evt events.KEVUpdatedEvent
                if err := json.Unmarshal(msg.Data(), &evt); err == nil && len(evt.NewKEVIDs) > 0 {
                    // UPDATE cves SET is_kev=TRUE WHERE id = ANY($1)
                    s.cveRepo.MarkAsKEV(ctx, evt.NewKEVIDs)
                }
                msg.Ack()
            }
        }
    }()
    return nil
}
```

---

## 8. Admin HTTP API (`internal/cvesync/service.go`)

```go
// Internal HTTP server (port 8082) expose admin endpoints
r.Get("/health",                  s.handleHealth)
r.Get("/sync/status",             s.handleSyncStatus)   // GET /api/v2/sync/status (via proxy)
r.Post("/sync/trigger",           s.handleTriggerAll)   // POST /api/v2/sync/trigger
r.Post("/sync/trigger/{source}",  s.handleTriggerOne)   // POST /api/v2/sync/trigger/{source}
```

---

## Định Nghĩa Hoàn Thành

- [x] `POST /sync/trigger/NVD` trigger sync thành công, sync_job được tạo
- [x] `GET /sync/status` trả về danh sách sync jobs gần nhất
- [x] NVD fetcher fetch được CVEs từ NVD API (test với API key)
- [x] CIRCL fetcher fetch được CVEs từ CIRCL API
- [x] EPSS fetcher update được epss_score trong DB
- [x] Cron scheduler chạy đúng schedule (verify bằng log)
- [x] NATS event `cve.synced` được publish sau mỗi sync
- [x] `is_kev` được update đúng khi nhận `kev.updated` event

---

*TASK-05 | CVE Sync Service | GlobalCVE v3.0*
