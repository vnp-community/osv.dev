# CR-OVS-007 — Asset Management & Scan Scheduling

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-OVS-007 |
| **Tiêu đề** | Asset Management (Asset Registry, Tagging, History), Scheduled Scan (Celery-style Cron via NATS JetStream) |
| **Nguồn tham chiếu** | `OpenVulnScan/docs/PRD.md §F-005,F-006,F-011`, `OpenVulnScan/specs/services/05-scan-service.md §Schedule` |
| **Target Service** | `scan-service` (extend) + `asset-service` (MỚI) |
| **Ưu tiên** | 🟡 Medium |
| **Loại** | New Feature + New Service |
| **Ngày tạo** | 2026-06-14 |
| **Ngày implement** | 2026-06-17 |
| **Trạng thái** | ✅ Implemented |

---

## 0. Implementation Status

> **Trạng thái**: ✅ **IMPLEMENTED** — 2026-06-17

| Task | File | Trạng thái |
|------|------|------------|
| Asset domain entity | `scan-service/internal/domain/asset/entity.go` | ✅ Done |
| Asset risk calculator | `scan-service/internal/domain/asset/risk.go` | ✅ Done |
| Asset upsert use case | `scan-service/internal/usecase/asset/upsert.go` | ✅ Done |
| Asset upsert (extended) | `scan-service/internal/usecase/asset/upsert_asset/upsert_asset.go` | ✅ Done |
| Schedule domain entity | `scan-service/internal/domain/schedule/entity.go` | ✅ Done |
| ScheduledScan entity | `scan-service/internal/domain/schedule/entity/scheduled_scan.go` | ✅ Done |
| CreateSchedule handler | `scan-service/internal/usecase/create_schedule/handler.go` | ✅ Done |
| Schedule HTTP handler | `scan-service/internal/delivery/http/schedule/schedule_handler.go` | ✅ Done |
| Schedule PostgreSQL repo | `scan-service/internal/infra/persistence/postgres/schedule/schedule_repo.go` | ✅ Done |
| Schedule NATS publisher | `scan-service/internal/infra/messaging/nats/schedule/publisher.go` | ✅ Done |
| Cron worker | `scan-service/internal/scheduler/cron_worker.go` | ✅ Done |
| Scheduler | `scan-service/internal/scheduler/scheduler.go` | ✅ Done |
| Asset service HTTP handlers | `asset-service/internal/delivery/http/handlers.go` | ✅ Done |
| Asset tagging + risk use case | `asset-service/internal/usecase/asset/tagging_risk.go` | ✅ Done |

**Chi tiết implementation**:
- **Asset Upsert**: sau khi scan hoàn thành → `UpsertAsset(ip, hostname, os, ports, services)` từ Nmap output
- **Asset Risk Score**: `Critical×25 + High×15 + Medium×5 + Low×1` capped 100, tự động tính sau mỗi finding update
- **Asset Tagging**: 3 modes: `add` (append), `remove` (diff), `set` (replace)
- **Asset History**: mỗi scan ghi lại `scan_id`, `finding_count`, `risk_score` snapshot
- **Scheduled Scan**: entity `ScheduledScan` với `cron_expr`, `frequency`, `enabled`, `next_run_at`
- **Cron Worker**: tick 1 phút, `FindDue(now)` → trigger `CreateScan` → `UpdateLastRunAt + ComputeNextRun`
- **NATS**: publish `scan.schedule.triggered` → scan-service worker pick up
- **API**: CRUD scheduled scans: `GET /schedules`, `POST /schedules`, `PATCH /schedules/{id}`, `POST /schedules/{id}/trigger`

---

## 1. Tổng quan

OpenVulnScan có hai features quan trọng cho enterprise security management:
1. **Asset Management** — theo dõi tất cả IT assets đã được quét (IP, hostname, OS, services, tags)
2. **Scheduled Scanning** — lên lịch quét định kỳ tự động (daily/weekly) qua NATS JetStream

---

## 2. Gap Analysis

| Feature | OSV | OpenVulnScan |
|---------|-----|-------------|
| Asset registry | ❌ | ✅ |
| Asset tagging | ❌ | ✅ |
| Asset history | ❌ | ✅ Last scan, finding counts |
| OS fingerprinting | ❌ | ✅ via Nmap |
| Services inventory | ❌ | ✅ open ports + versions |
| Scheduled scan | ❌ | ✅ cron via NATS |
| Daily/weekly schedule | ❌ | ✅ |
| Asset search/filter | ❌ | ✅ |
| Asset risk score | ❌ | ✅ derived from findings |
| WhatWeb fingerprinting | ❌ | ✅ |

---

## 3. Domain Model — Asset

```go
// asset-service/internal/domain/entity/asset.go

// Asset — an IT asset discovered via scanning
type Asset struct {
    ID            uuid.UUID
    IPAddress     string
    Hostname      string
    OS            string           // Detected OS (from nmap -O)
    OSVersion     string
    MACAddress    string
    Status        AssetStatus      // active|inactive|unknown
    Tags          []string         // user-assigned tags
    Services      []AssetService   // open ports + services
    WebTech       []WebTechnology  // detected web technologies
    LastScanID    *uuid.UUID
    LastScannedAt *time.Time
    FindingCount  int              // active findings
    RiskScore     float64          // 0.0-10.0 derived from findings
    CreatedAt     time.Time
    UpdatedAt     time.Time
}

type AssetStatus string
const (
    AssetStatusActive   AssetStatus = "active"
    AssetStatusInactive AssetStatus = "inactive"
    AssetStatusUnknown  AssetStatus = "unknown"
)

type AssetService struct {
    Port     int    `json:"port"`
    Protocol string `json:"protocol"` // tcp|udp
    Name     string `json:"name"`     // http, ssh, mysql...
    Product  string `json:"product"`  // Apache httpd, OpenSSH...
    Version  string `json:"version"`
    Banner   string `json:"banner,omitempty"`
}

type WebTechnology struct {
    Name       string   `json:"name"`
    Version    string   `json:"version,omitempty"`
    Categories []string `json:"categories,omitempty"`
}

// AssetFilter — for list/search
type AssetFilter struct {
    Tag       string
    OS        string
    Status    AssetStatus
    HasPort   *int
    Query     string       // text search on IP/hostname
    Page      int
    Limit     int
}
```

### 3.1 Scheduled Scan Config

```go
// scan-service/internal/domain/schedule/entity.go

type ScheduleFrequency string
const (
    FreqDaily   ScheduleFrequency = "daily"
    FreqWeekly  ScheduleFrequency = "weekly"
    FreqHourly  ScheduleFrequency = "hourly"
    FreqCustom  ScheduleFrequency = "custom"  // cron expression
)

type ScheduledScan struct {
    ID            uuid.UUID
    UserID        uuid.UUID
    Name          string
    Targets       []string     // IPs, CIDRs, URLs
    ScanType      ScanType     // full|discovery|web|agent
    Frequency     ScheduleFrequency
    CronExpr      string       // "0 2 * * *" for daily at 2am
    Enabled       bool
    LastRunAt     *time.Time
    NextRunAt     *time.Time
    Options       ScanOptions
    CreatedAt     time.Time
    UpdatedAt     time.Time
}

func (s *ScheduledScan) ComputeNextRun() time.Time {
    schedule, _ := cron.ParseStandard(s.CronExpr)
    return schedule.Next(time.Now())
}

// Default cron expressions by frequency
var FrequencyCronMap = map[ScheduleFrequency]string{
    FreqHourly: "0 * * * *",       // every hour at :00
    FreqDaily:  "0 2 * * *",       // daily at 2:00 AM
    FreqWeekly: "0 2 * * 0",       // weekly at Sunday 2:00 AM
}
```

---

## 4. Use Cases

### 4.1 UpsertAsset (called after scan completion)

```go
// asset-service/internal/usecase/asset/upsert.go

type UpsertAssetInput struct {
    IPAddress  string
    Hostname   string
    OS         string
    MACAddress string
    Services   []AssetService
    WebTech    []WebTechnology
    ScanID     uuid.UUID
}

func (uc *UpsertAssetUseCase) Execute(ctx context.Context, in UpsertAssetInput) (*entity.Asset, error) {
    // 1. Find existing asset by IP
    existing, err := uc.assetRepo.FindByIP(ctx, in.IPAddress)

    if err != nil || existing == nil {
        // 2a. Create new asset
        asset := &entity.Asset{
            ID:           uuid.New(),
            IPAddress:    in.IPAddress,
            Hostname:     in.Hostname,
            OS:           in.OS,
            Services:     in.Services,
            WebTech:      in.WebTech,
            Status:       entity.AssetStatusActive,
            Tags:         []string{},
            LastScanID:   &in.ScanID,
            LastScannedAt: pointTime(time.Now()),
            CreatedAt:    time.Now().UTC(),
        }
        uc.assetRepo.Save(ctx, asset)
        return asset, nil
    }

    // 2b. Update existing asset
    existing.Hostname = in.Hostname
    existing.OS = in.OS
    existing.Services = in.Services
    existing.WebTech = in.WebTech
    existing.Status = entity.AssetStatusActive
    existing.LastScanID = &in.ScanID
    now := time.Now().UTC()
    existing.LastScannedAt = &now
    existing.UpdatedAt = now

    uc.assetRepo.Update(ctx, existing)
    return existing, nil
}
```

### 4.2 Asset Tagging

```go
// asset-service/internal/usecase/asset/tag.go

type TagAssetInput struct {
    AssetID uuid.UUID
    Tags    []string
    Mode    string  // "set"|"add"|"remove"
}

func (uc *TagAssetUseCase) Execute(ctx context.Context, in TagAssetInput) error {
    asset, err := uc.assetRepo.FindByID(ctx, in.AssetID)
    if err != nil { return err }

    switch in.Mode {
    case "set":
        asset.Tags = in.Tags
    case "add":
        asset.Tags = addUniqueStrings(asset.Tags, in.Tags)
    case "remove":
        asset.Tags = removeStrings(asset.Tags, in.Tags)
    }

    asset.UpdatedAt = time.Now().UTC()
    return uc.assetRepo.Update(ctx, asset)
}
```

### 4.3 Scheduled Scan Trigger

```go
// scan-service/internal/scheduler/scheduler.go
// Checks for due schedules every minute via NATS JetStream timer

type Scheduler struct {
    scheduledScanRepo ScheduledScanRepository
    createScanUC      *CreateScanUseCase
    eventBus          EventBus
    logger            zerolog.Logger
}

// CheckAndTrigger — run every minute
func (s *Scheduler) CheckAndTrigger(ctx context.Context) error {
    now := time.Now().UTC()

    // Find all enabled schedules due for execution
    dueSched, err := s.scheduledScanRepo.FindDue(ctx, now)
    if err != nil { return err }

    for _, sched := range dueSched {
        s.logger.Info().
            Str("schedule_id", sched.ID.String()).
            Str("name", sched.Name).
            Strs("targets", sched.Targets).
            Msg("triggering scheduled scan")

        // Create scan
        scan, err := s.createScanUC.Execute(ctx, CreateScanInput{
            UserID:   sched.UserID,
            Targets:  sched.Targets,
            Type:     sched.ScanType,
            Options:  sched.Options,
            Priority: 5,
        })
        if err != nil {
            s.logger.Error().Err(err).Msg("scheduled scan create failed")
            continue
        }

        // Update next run time
        sched.LastRunAt = &now
        nextRun := sched.ComputeNextRun()
        sched.NextRunAt = &nextRun
        s.scheduledScanRepo.Update(ctx, sched)

        s.logger.Info().
            Str("scan_id", scan.ID.String()).
            Time("next_run", nextRun).
            Msg("scheduled scan created")
    }

    return nil
}

// Cron job: every minute
func (s *Scheduler) Start(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Minute)
    go func() {
        for {
            select {
            case <-ticker.C:
                s.CheckAndTrigger(ctx)
            case <-ctx.Done():
                ticker.Stop()
                return
            }
        }
    }()
}
```

---

## 5. Database Schema

```sql
-- asset-service migrations/

CREATE TABLE assets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ip_address      INET NOT NULL UNIQUE,
    hostname        VARCHAR(255),
    os              TEXT,
    os_version      VARCHAR(100),
    mac_address     VARCHAR(17),
    status          VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active','inactive','unknown')),
    tags            TEXT[] DEFAULT '{}',
    services        JSONB DEFAULT '[]',
    web_tech        JSONB DEFAULT '[]',
    last_scan_id    UUID,
    last_scanned_at TIMESTAMPTZ,
    finding_count   INT NOT NULL DEFAULT 0,
    risk_score      FLOAT DEFAULT 0.0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_assets_ip       ON assets(ip_address);
CREATE INDEX idx_assets_hostname ON assets(hostname);
CREATE INDEX idx_assets_tags     ON assets USING GIN(tags);
CREATE INDEX idx_assets_status   ON assets(status);

-- Scheduled scans (in scan-service schema)
CREATE TABLE scheduled_scans (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL,
    name         VARCHAR(255) NOT NULL,
    targets      TEXT[] NOT NULL,
    scan_type    VARCHAR(20) NOT NULL DEFAULT 'full',
    frequency    VARCHAR(20) NOT NULL DEFAULT 'daily',
    cron_expr    VARCHAR(100) NOT NULL DEFAULT '0 2 * * *',
    enabled      BOOLEAN NOT NULL DEFAULT TRUE,
    last_run_at  TIMESTAMPTZ,
    next_run_at  TIMESTAMPTZ,
    options      JSONB NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_sched_user    ON scheduled_scans(user_id);
CREATE INDEX idx_sched_next    ON scheduled_scans(next_run_at) WHERE enabled = TRUE;
```

---

## 6. API Routes

```
# Asset Management
GET    /api/v1/assets              → List assets (filters: tag, os, status, query)
GET    /api/v1/assets/{ip}         → Get asset details by IP
GET    /api/v1/assets/{id}         → Get asset details by UUID
PUT    /api/v1/assets/{id}/tags    → Tag asset
DELETE /api/v1/assets/{id}         → Remove asset from registry
GET    /api/v1/assets/{id}/history → Scan history for this asset
GET    /api/v1/assets/{id}/findings → Active findings for this asset

# Scheduled Scans
GET    /api/v1/schedules           → List scheduled scans
POST   /api/v1/schedules           → Create scheduled scan
GET    /api/v1/schedules/{id}      → Get schedule details
PUT    /api/v1/schedules/{id}      → Update schedule (targets, frequency)
DELETE /api/v1/schedules/{id}      → Delete schedule
POST   /api/v1/schedules/{id}/enable  → Enable schedule
POST   /api/v1/schedules/{id}/disable → Disable schedule
POST   /api/v1/schedules/{id}/trigger → Manual trigger now
```

---

## 7. Acceptance Criteria

**Asset Management:**
- [ ] Sau khi scan hoàn thành → assets tự động được upsert từ scan findings
- [ ] Asset với same IP → update (không tạo mới)
- [ ] `PUT /api/v1/assets/{id}/tags` với `mode=add` → tags được thêm (không replace)
- [ ] `GET /api/v1/assets?tag=production` → chỉ trả về assets có tag "production"
- [ ] `GET /api/v1/assets?query=192.168.1` → search theo IP prefix
- [ ] `GET /api/v1/assets/{ip}/findings` → active findings từ finding-service cho IP này
- [ ] Asset `finding_count` được update khi findings thay đổi
- [ ] `risk_score` = 10.0 nếu có Critical finding; giảm dần theo severity

**Scheduled Scanning:**
- [ ] `POST /api/v1/schedules` với `frequency=daily` → `cron_expr="0 2 * * *"`, `next_run_at` = tomorrow 2am
- [ ] Scheduler check every 1 minute, trigger scans khi `next_run_at <= now`
- [ ] Sau khi trigger → `last_run_at=now`, `next_run_at` được tính lại
- [ ] `POST /api/v1/schedules/{id}/trigger` → tạo scan ngay lập tức (bất kể lịch)
- [ ] `POST /api/v1/schedules/{id}/disable` → không trigger cho đến khi re-enable
- [ ] Custom cron: `"0 */6 * * *"` → every 6 hours
- [ ] Scheduler phục hồi đúng sau service restart (load next_run_at từ DB)
