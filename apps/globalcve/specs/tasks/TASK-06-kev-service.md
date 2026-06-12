# TASK-06 — KEV Service

## Mục Tiêu

Implement **KEV Service** — goroutine service quản lý CISA Known Exploited Vulnerabilities (KEV) catalog. Sync dữ liệu từ CISA, expose HTTP API, publish NATS events khi có KEV mới.

## Phụ Thuộc

- TASK-03 (Database Migrations — table `kev_entries` phải tồn tại)
- TASK-02 (NATS Client — để publish `kev.updated`)

## Đầu Ra

- `internal/kevservice/domain/entity/kev.go`
- `internal/kevservice/domain/repository/kev_repository.go`
- `internal/kevservice/adapter/postgres/kev_repo.go`
- `internal/kevservice/adapter/cisa/client.go` — CISA KEV API client
- `internal/kevservice/usecase/usecase.go`
- `internal/kevservice/service.go`

---

## Checklist

- [x] KEV domain entity
- [x] Repository interface
- [x] CISA API client (port từ `src/lib/kev.ts`)
- [x] PostgreSQL adapter (upsert KEV entries)
- [x] Usecase: sync, list, get by ID, bulk check, stats
- [x] HTTP handler: REST API
- [x] NATS publisher cho `kev.updated`
- [x] Cron: mỗi 6 giờ sync KEV (`0 0 */6 * * *`)
- [x] Service wrapper

---

## 1. Domain Entity (`internal/kevservice/domain/entity/kev.go`)

```go
package entity

import "time"

// KEVEntry — CISA Known Exploited Vulnerability entry
type KEVEntry struct {
    CVEID              string     `json:"cve_id"`
    VendorProject      string     `json:"vendor_project"`
    Product            string     `json:"product"`
    VulnerabilityName  string     `json:"vulnerability_name"`
    DateAdded          *time.Time `json:"date_added"`
    ShortDescription   string     `json:"short_description"`
    RequiredAction     string     `json:"required_action"`
    DueDate            *time.Time `json:"due_date"`
    KnownRansomware    string     `json:"known_ransomware"` // "Known" / "Unknown"
}

// KEVStats — thống kê KEV catalog
type KEVStats struct {
    Total     int        `json:"total"`
    LastSync  *time.Time `json:"last_sync"`
    NewToday  int        `json:"new_today"`
}

// KEVBulkCheckResult — kết quả bulk check
type KEVBulkCheckResult struct {
    CVEID string `json:"cve_id"`
    IsKEV bool   `json:"is_kev"`
}
```

---

## 2. Repository Interface

```go
package repository

import (
    "context"
    "github.com/binhnt/globalcve/internal/kevservice/domain/entity"
)

type KEVRepository interface {
    // Upsert nhiều KEV entries cùng lúc
    UpsertBatch(ctx context.Context, entries []*entity.KEVEntry) (inserted int, err error)

    // List với pagination
    List(ctx context.Context, page, limit int) ([]*entity.KEVEntry, int64, error)

    // GetByID trả về một KEV entry
    GetByID(ctx context.Context, cveID string) (*entity.KEVEntry, error)

    // BulkCheck kiểm tra nhiều CVE IDs có trong KEV không
    BulkCheck(ctx context.Context, cveIDs []string) ([]entity.KEVBulkCheckResult, error)

    // GetStats trả về thống kê
    GetStats(ctx context.Context) (*entity.KEVStats, error)

    // GetNewSince trả về KEV IDs mới được thêm từ ngày timestamp
    GetNewSince(ctx context.Context, since time.Time) ([]string, error)
}
```

---

## 3. CISA API Client (`internal/kevservice/adapter/cisa/client.go`)

Port từ `src/lib/kev.ts`.

```go
package cisa

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "github.com/binhnt/globalcve/internal/kevservice/domain/entity"
)

const KEVCatalogURL = "https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json"

type Client struct {
    httpClient *http.Client
}

func NewClient() *Client {
    return &Client{
        httpClient: &http.Client{Timeout: 30 * time.Second},
    }
}

// cisaResponse — raw CISA JSON response
type cisaResponse struct {
    Title           string     `json:"title"`
    CatalogVersion  string     `json:"catalogVersion"`
    DateReleased    string     `json:"dateReleased"`
    Count           int        `json:"count"`
    Vulnerabilities []cisaVuln `json:"vulnerabilities"`
}

type cisaVuln struct {
    CVEID             string `json:"cveID"`
    VendorProject     string `json:"vendorProject"`
    Product           string `json:"product"`
    VulnerabilityName string `json:"vulnerabilityName"`
    DateAdded         string `json:"dateAdded"`      // "2021-11-03"
    ShortDescription  string `json:"shortDescription"`
    RequiredAction    string `json:"requiredAction"`
    DueDate           string `json:"dueDate"`         // "2021-11-17"
    KnownRansomware   string `json:"knownRansomwareCampaignUse"`
}

// FetchAll downloads và parse toàn bộ CISA KEV catalog
func (c *Client) FetchAll(ctx context.Context) ([]*entity.KEVEntry, error) {
    req, _ := http.NewRequestWithContext(ctx, http.MethodGet, KEVCatalogURL, nil)
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("cisa: fetch KEV catalog: %w", err)
    }
    defer resp.Body.Close()

    var raw cisaResponse
    if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
        return nil, fmt.Errorf("cisa: decode response: %w", err)
    }

    entries := make([]*entity.KEVEntry, 0, len(raw.Vulnerabilities))
    for _, v := range raw.Vulnerabilities {
        entry := &entity.KEVEntry{
            CVEID:             v.CVEID,
            VendorProject:     v.VendorProject,
            Product:           v.Product,
            VulnerabilityName: v.VulnerabilityName,
            ShortDescription:  v.ShortDescription,
            RequiredAction:    v.RequiredAction,
            KnownRansomware:   v.KnownRansomware,
        }

        if d, err := time.Parse("2006-01-02", v.DateAdded); err == nil {
            entry.DateAdded = &d
        }
        if d, err := time.Parse("2006-01-02", v.DueDate); err == nil {
            entry.DueDate = &d
        }

        entries = append(entries, entry)
    }

    return entries, nil
}
```

---

## 4. PostgreSQL Adapter (`internal/kevservice/adapter/postgres/kev_repo.go`)

```go
// UpsertBatch — bulk upsert KEV entries
func (r *KEVRepo) UpsertBatch(ctx context.Context, entries []*entity.KEVEntry) (int, error) {
    // Dùng pgx CopyFrom hoặc batch INSERT ... ON CONFLICT DO UPDATE
    // Return số rows inserted mới (xmax = 0)
    batch := &pgx.Batch{}
    for _, e := range entries {
        batch.Queue(`
            INSERT INTO kev_entries (cve_id, vendor_project, product, vulnerability_name,
                date_added, short_description, required_action, due_date, known_ransomware)
            VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
            ON CONFLICT (cve_id) DO UPDATE SET
                vendor_project     = EXCLUDED.vendor_project,
                product            = EXCLUDED.product,
                vulnerability_name = EXCLUDED.vulnerability_name,
                short_description  = EXCLUDED.short_description,
                required_action    = EXCLUDED.required_action,
                due_date           = EXCLUDED.due_date,
                known_ransomware   = EXCLUDED.known_ransomware,
                updated_at         = NOW()
            RETURNING (xmax = 0) AS is_new
        `, e.CVEID, e.VendorProject, e.Product, e.VulnerabilityName,
           e.DateAdded, e.ShortDescription, e.RequiredAction, e.DueDate, e.KnownRansomware)
    }

    br := r.pool.SendBatch(ctx, batch)
    defer br.Close()

    inserted := 0
    for range entries {
        var isNew bool
        if err := br.QueryRow().Scan(&isNew); err == nil && isNew {
            inserted++
        }
    }
    return inserted, nil
}

// BulkCheck
func (r *KEVRepo) BulkCheck(ctx context.Context, cveIDs []string) ([]entity.KEVBulkCheckResult, error) {
    rows, err := r.pool.Query(ctx,
        `SELECT id AS cve_id, id = ANY($1) AS is_kev FROM unnest($1::text[]) AS id`,
        cveIDs)
    // ...
}

// GetStats
func (r *KEVRepo) GetStats(ctx context.Context) (*entity.KEVStats, error) {
    var stats entity.KEVStats
    r.pool.QueryRow(ctx, `
        SELECT
            COUNT(*) AS total,
            COUNT(*) FILTER (WHERE date_added = CURRENT_DATE) AS new_today
        FROM kev_entries
    `).Scan(&stats.Total, &stats.NewToday)
    return &stats, nil
}
```

---

## 5. Usecase (`internal/kevservice/usecase/usecase.go`)

```go
package usecase

type KEVUsecase struct {
    repo   repository.KEVRepository
    cisa   *cisa.Client
    nats   *natsInfra.Client // nullable
    logger zerolog.Logger
}

// Sync — download và upsert toàn bộ CISA KEV catalog
func (u *KEVUsecase) Sync(ctx context.Context) error {
    // 1. Lấy danh sách KEV IDs hiện tại trước khi sync
    // 2. Fetch từ CISA
    entries, err := u.cisa.FetchAll(ctx)
    if err != nil {
        return err
    }

    // 3. Upsert vào DB
    inserted, err := u.repo.UpsertBatch(ctx, entries)
    if err != nil {
        return err
    }

    // 4. Lấy danh sách KEV IDs mới (inserted)
    newKEVIDs, _ := u.repo.GetNewSince(ctx, time.Now().Add(-1*time.Minute))

    // 5. Publish NATS event kev.updated
    if u.nats != nil && len(newKEVIDs) > 0 {
        evt := events.KEVUpdatedEvent{
            Total:     len(entries),
            Inserted:  inserted,
            NewKEVIDs: newKEVIDs,
        }
        data, _ := json.Marshal(evt)
        u.nats.JS.Publish(ctx, events.SubjectKEVUpdated, data)
    }

    return nil
}

func (u *KEVUsecase) List(ctx context.Context, page, limit int) ([]*entity.KEVEntry, int64, error) {
    return u.repo.List(ctx, page, limit)
}

func (u *KEVUsecase) GetByID(ctx context.Context, id string) (*entity.KEVEntry, error) {
    return u.repo.GetByID(ctx, id)
}

func (u *KEVUsecase) BulkCheck(ctx context.Context, ids []string) ([]entity.KEVBulkCheckResult, error) {
    return u.repo.BulkCheck(ctx, ids)
}

func (u *KEVUsecase) GetStats(ctx context.Context) (*entity.KEVStats, error) {
    return u.repo.GetStats(ctx)
}
```

---

## 6. HTTP API (port 8083)

Theo §6.1:

```
GET  /api/v2/kev              → List KEV entries (paginated)
GET  /api/v2/kev/{id}         → Get KEV entry by CVE ID
GET  /api/v2/kev/check?ids=   → Bulk check (comma-separated CVE IDs)
GET  /api/v2/kev/stats        → KEV statistics
POST /kev/sync                → Manual trigger sync (admin)
GET  /health                  → Health check
```

### Query Parameters cho List:
| Param | Type | Default |
|-------|------|---------|
| `page` | int | 0 |
| `limit` | int | 50 |

### Bulk Check Request:
```
GET /api/v2/kev/check?ids=CVE-2021-44228,CVE-2021-45046
```

### Bulk Check Response:
```json
[
  {"cve_id": "CVE-2021-44228", "is_kev": true},
  {"cve_id": "CVE-2021-45046", "is_kev": true}
]
```

---

## 7. Service Wrapper (`internal/kevservice/service.go`)

```go
package kevservice

type Service struct {
    cfg    config.ServicesConfig
    uc     *usecase.KEVUsecase
    cron   *cron.Cron
    server *http.Server
}

func New(cfg config.ServicesConfig, pool *pgxpool.Pool, nats *natsInfra.Client) *Service {
    // Wire dependencies
    repo := postgresadapter.NewKEVRepo(pool)
    cisaClient := cisa.NewClient()
    uc := usecase.New(repo, cisaClient, nats)

    // Cron: mỗi 6 giờ
    c := cron.New(cron.WithSeconds())
    c.AddFunc("0 0 */6 * * *", func() {
        uc.Sync(context.Background())
    })

    return &Service{cfg: cfg, uc: uc, cron: c}
}

func (s *Service) Start(ctx context.Context) error {
    s.cron.Start()

    // Trigger initial sync khi start
    go s.uc.Sync(ctx)

    r := chi.NewRouter()
    // Register routes...

    s.server = &http.Server{
        Addr:    fmt.Sprintf(":%d", s.cfg.KEVService.Port),
        Handler: r,
    }

    go func() {
        <-ctx.Done()
        shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
        defer cancel()
        s.cron.Stop()
        s.server.Shutdown(shutCtx)
    }()

    return s.server.ListenAndServe()
}
```

---

## Định Nghĩa Hoàn Thành

- [x] `GET /api/v2/kev` trả về danh sách KEV entries
- [x] `GET /api/v2/kev/CVE-2021-44228` trả về entry đúng
- [x] `GET /api/v2/kev/check?ids=CVE-2021-44228` trả về `is_kev: true`
- [x] `GET /api/v2/kev/stats` trả về total count
- [x] Sync từ CISA API thành công
- [x] `kev.updated` event được publish sau sync
- [x] Cron trigger đúng mỗi 6 giờ
- [x] `is_kev` flag trong `cves` table được update (qua NATS → CVE Sync)

---

*TASK-06 | KEV Service | GlobalCVE v3.0*
