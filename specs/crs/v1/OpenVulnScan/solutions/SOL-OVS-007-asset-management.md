# SOL-OVS-007 — Giải Pháp: Asset Management & Scheduled Scans

| Trường | Giá trị |
|--------|---------|
| **Solution ID** | SOL-OVS-007 |
| **CR tham chiếu** | CR-OVS-007 |
| **Tiêu đề** | Asset Management (Asset Registry, Tagging, History), Scheduled Scan (Cron via NATS JetStream) |
| **Ngày tạo** | 2026-06-16 |
| **Ngày implement** | 2026-06-17 |
| **Trạng thái** | ✅ Implemented |

---

## 0. Implementation Status

> **Trạng thái**: ✅ **IMPLEMENTED** — 2026-06-17

| Task | File | Trạng thái |
|------|------|------------|
| T-ASSET-001 | `asset-service/internal/usecase/asset/upsert.go` | ✅ Done |
| T-ASSET-002 | `asset-service/internal/usecase/asset/tagging_risk.go` | ✅ Done |
| T-ASSET-003 | `asset-service/internal/delivery/http/handlers.go` | ✅ Done |
| T-ASSET-004 | `scan-service/internal/domain/schedule/entity.go` | ✅ Done |

**Chi tiết implementation**:
- **UpsertAsset**: Find-by-IP → create-or-update, `last_scanned_at=now`, `status=active`
- **Asset Tagging**: `set/add/remove` modes, deduplicated + sorted tag slice
- **Risk Scoring**: formula `Critical×9.5 + High×7.5 + Medium×4.5 + Low×1.5`, capped at 10.0
- **REST API**: `GET /assets` (filter: tag/OS/port/status/query), `PUT /assets/{id}/tags`, `GET /assets/{id}/risk`, `GET /assets/{id}/history`, `GET /assets/{id}/findings`
- **ScheduledScan Entity**: `FrequencyCronMap`, `ComputeNextRun()` với `robfig/cron`, startup recovery
- **Scheduler Goroutine**: tick 1min, `FindDue(now)`, `UpdateLastRunAt + ComputeNextRun`

---

## 1. Tổng Quan Giải Pháp

### 1.1 Bối Cảnh

CR-OVS-007 bao gồm 2 features riêng biệt nhưng liên quan:

1. **Asset Service** (new microservice) — Registry theo dõi IT assets đã scan
2. **Scheduled Scan** (extension của scan-service) — Lên lịch scan định kỳ

**Lý do tách riêng**: Asset Service có database riêng, business logic riêng (asset lifecycle, tagging, risk scoring). Scheduled Scan thuộc về scan-service domain (cùng domain object scan).

---

## 2. Phần 1: Asset Service

### 2.1 Kiến Trúc

```
scan-service (NATS: scan.scan.completed)
        │
        ▼
asset-service ── ScanCompletedConsumer
        │          UpsertAsset(ip, hostname, OS, services)
        │
        ├── Assets DB (PostgreSQL)
        │
        ├── finding-service gRPC client
        │   → GetFindingsByComponent(ip_address)
        │   → Count + Risk Score computation
        │
        └── REST API (:8068)
```

### 2.2 Cấu Trúc Thư Mục

```
services/asset-service/
├── cmd/server/main.go
├── internal/
│   ├── domain/
│   │   └── entity/
│   │       ├── asset.go          # Asset, AssetStatus, AssetService, WebTechnology
│   │       └── repository.go     # AssetRepository interface
│   ├── usecase/
│   │   ├── asset/
│   │   │   ├── upsert.go         # Create or update asset by IP
│   │   │   ├── tag.go            # set|add|remove tags
│   │   │   ├── list.go           # List with filters
│   │   │   └── update_risk.go    # Compute risk score from findings
│   ├── adapter/
│   │   ├── repository/postgres/
│   │   │   └── asset_repo.go
│   │   ├── messaging/nats/
│   │   │   └── scan_completed_consumer.go
│   │   └── client/
│   │       └── finding_service_client.go
│   └── delivery/
│       └── http/
│           └── asset_handler.go
├── migrations/
│   └── 001_create_asset_tables.sql
└── config/config.yaml
```

### 2.3 UpsertAsset Flow

```
scan.scan.completed (NATS event)
        │
        ├── Get scan findings from scan-service gRPC
        │
        ├── For each finding (unique IP):
        │   assetRepo.FindByIP(ip)
        │   │
        │   ├── nil → Create new Asset:
        │   │    {ip, hostname, OS, services, status=active, 
        │   │     last_scan_id, last_scanned_at=now}
        │   │
        │   └── found → Update:
        │        {hostname, OS, services, web_tech, 
        │         last_scan_id, last_scanned_at=now, status=active}
        │
        └── Update risk scores (async):
             finding-service.GetActiveCountByComponent(ip)
             → risk_score = 10.0 if Critical; deduct per severity
```

### 2.4 Risk Score Algorithm

```go
// asset-service/internal/usecase/asset/update_risk.go

// Risk score: 0.0 (safe) → 10.0 (critical)
func computeRiskScore(stats FindingStats) float64 {
    if stats.Critical > 0 { return 10.0 }
    if stats.High > 0     { return 8.0 + float64(min(stats.High, 5))*0.4 }
    if stats.Medium > 0   { return 5.0 + float64(min(stats.Medium, 5))*0.6 }
    if stats.Low > 0      { return float64(min(stats.Low, 5))*1.0 }
    return 0.0
}

// Update is triggered:
// 1. After scan completion (via NATS)
// 2. After finding status change (via NATS finding.status.changed)
```

### 2.5 Database Schema (Asset)

```sql
-- migrations/001_create_asset_tables.sql

CREATE TABLE assets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ip_address      INET NOT NULL UNIQUE,
    hostname        VARCHAR(255),
    os              TEXT,
    os_version      VARCHAR(100),
    mac_address     VARCHAR(17),
    status          VARCHAR(20) NOT NULL DEFAULT 'active'
                    CHECK (status IN ('active','inactive','unknown')),
    tags            TEXT[] DEFAULT '{}',
    services        JSONB DEFAULT '[]',
    -- [{port, protocol, name, product, version, banner}]
    web_tech        JSONB DEFAULT '[]',
    -- [{name, version, categories[]}]
    last_scan_id    UUID,
    last_scanned_at TIMESTAMPTZ,
    finding_count   INT NOT NULL DEFAULT 0,    -- Active findings
    risk_score      FLOAT DEFAULT 0.0,         -- 0.0 - 10.0
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_assets_ip        ON assets(ip_address);
CREATE INDEX idx_assets_hostname  ON assets(hostname);
CREATE INDEX idx_assets_tags      ON assets USING GIN(tags);
CREATE INDEX idx_assets_status    ON assets(status);
CREATE INDEX idx_assets_risk      ON assets(risk_score DESC);
CREATE INDEX idx_assets_scanned   ON assets(last_scanned_at DESC);

-- Asset scan history (lightweight log)
CREATE TABLE asset_scan_history (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    asset_id    UUID NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    scan_id     UUID NOT NULL,
    scan_type   VARCHAR(20),
    finding_count INT DEFAULT 0,
    risk_score  FLOAT,
    scanned_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_asset_history_asset ON asset_scan_history(asset_id, scanned_at DESC);
```

### 2.6 API Routes (Asset)

```
GET    /api/v1/assets
  Query: ?tag=production&os=linux&status=active&query=192.168&has_port=443
         &page=1&limit=20&sort=risk_score&order=desc
  Response: {items: [Asset], total, page}

GET    /api/v1/assets/{ip}
  Response: Asset detail with services, web_tech, tags

GET    /api/v1/assets/{id}
  Response: Asset detail by UUID

PUT    /api/v1/assets/{id}/tags
  Body: {tags: ["production","web"], mode: "add"}  // set|add|remove
  Response: 200 updated Asset

DELETE /api/v1/assets/{id}
  Response: 204 (remove from registry, does not affect findings)

GET    /api/v1/assets/{id}/history
  Response: [{scan_id, scan_type, finding_count, risk_score, scanned_at}]

GET    /api/v1/assets/{id}/findings
  Response: Proxy to finding-service: active findings for this IP
```

---

## 3. Phần 2: Scheduled Scans

### 3.1 Extension của scan-service

Scheduled Scan được implement **trong scan-service** (không phải service mới), vì:
- ScheduledScan → creates Scan entities (cùng domain)
- Cùng NATS topics
- Scheduler goroutine chạy trong same process

### 3.2 Cron Expression Management

```go
// scan-service/internal/domain/entity/schedule.go

type ScheduleFrequency string
const (
    FreqHourly  ScheduleFrequency = "hourly"
    FreqDaily   ScheduleFrequency = "daily"
    FreqWeekly  ScheduleFrequency = "weekly"
    FreqCustom  ScheduleFrequency = "custom"  // raw cron expression
)

// Default cron expressions
var FrequencyCronMap = map[ScheduleFrequency]string{
    FreqHourly: "0 * * * *",       // Every hour at :00
    FreqDaily:  "0 2 * * *",       // Daily at 2:00 AM UTC
    FreqWeekly: "0 2 * * 0",       // Sunday at 2:00 AM UTC
}

// ComputeNextRun — using robfig/cron parser
func (s *ScheduledScan) ComputeNextRun() time.Time {
    schedule, err := cron.ParseStandard(s.CronExpr)
    if err != nil {
        // Fallback: default daily
        schedule, _ = cron.ParseStandard("0 2 * * *")
    }
    return schedule.Next(time.Now().UTC())
}

// Database: scheduled_scans table (in scan-service DB)
type ScheduledScan struct {
    ID         uuid.UUID
    UserID     uuid.UUID
    Name       string
    Targets    []string
    ScanType   ScanType
    Frequency  ScheduleFrequency
    CronExpr   string     // stored cron expression
    Enabled    bool
    LastRunAt  *time.Time
    NextRunAt  *time.Time // pre-computed, indexed for efficient queries
    Options    ScanOptions
    CreatedAt  time.Time
    UpdatedAt  time.Time
}
```

### 3.3 Scheduler Goroutine

```go
// scan-service/internal/scheduler/scheduler.go

type Scheduler struct {
    scheduledScanRepo ScheduledScanRepository
    createScanUC      *CreateScanUseCase
    logger            zerolog.Logger
    ticker            *time.Ticker
}

// Start — launch background goroutine
func (s *Scheduler) Start(ctx context.Context) {
    s.ticker = time.NewTicker(1 * time.Minute)
    
    go func() {
        // Run immediately on startup (to recover from restarts)
        s.checkAndTrigger(ctx)
        
        for {
            select {
            case <-s.ticker.C:
                s.checkAndTrigger(ctx)
            case <-ctx.Done():
                s.ticker.Stop()
                s.logger.Info().Msg("scheduler stopped")
                return
            }
        }
    }()
}

func (s *Scheduler) checkAndTrigger(ctx context.Context) {
    now := time.Now().UTC()
    
    // Efficient query: only enabled schedules that are due
    // WHERE enabled = TRUE AND next_run_at <= NOW()
    dueSched, err := s.scheduledScanRepo.FindDue(ctx, now)
    if err != nil {
        s.logger.Error().Err(err).Msg("find due schedules failed")
        return
    }
    
    for _, sched := range dueSched {
        s.logger.Info().
            Str("schedule_id", sched.ID.String()).
            Str("name", sched.Name).
            Strs("targets", sched.Targets).
            Msg("triggering scheduled scan")
        
        // Create scan (publishes NATS event internally)
        scan, err := s.createScanUC.Execute(ctx, CreateScanInput{
            UserID:   sched.UserID,
            Targets:  sched.Targets,
            Type:     sched.ScanType,
            Options:  sched.Options,
            Priority: 5,
        })
        if err != nil {
            s.logger.Error().Err(err).
                Str("schedule_id", sched.ID.String()).
                Msg("failed to create scheduled scan")
            continue
        }
        
        // Update timestamps
        sched.LastRunAt = &now
        nextRun := sched.ComputeNextRun()
        sched.NextRunAt = &nextRun
        if err := s.scheduledScanRepo.Update(ctx, sched); err != nil {
            s.logger.Error().Err(err).Msg("update schedule timestamps failed")
        }
        
        s.logger.Info().
            Str("scan_id", scan.ID.String()).
            Time("next_run", nextRun).
            Msg("scheduled scan created successfully")
    }
}
```

### 3.4 Database Schema (Scheduled Scans)

```sql
-- In scan-service migrations:
-- migrations/002_add_scheduled_scans.sql

CREATE TABLE scheduled_scans (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL,
    name         VARCHAR(255) NOT NULL,
    targets      TEXT[] NOT NULL,
    scan_type    VARCHAR(20) NOT NULL DEFAULT 'full'
                 CHECK (scan_type IN ('full','discovery','web','agent')),
    frequency    VARCHAR(20) NOT NULL DEFAULT 'daily'
                 CHECK (frequency IN ('hourly','daily','weekly','custom')),
    cron_expr    VARCHAR(100) NOT NULL DEFAULT '0 2 * * *',
    enabled      BOOLEAN NOT NULL DEFAULT TRUE,
    last_run_at  TIMESTAMPTZ,
    next_run_at  TIMESTAMPTZ,
    options      JSONB NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Critical index: efficient "find due schedules" query
CREATE INDEX idx_sched_due     ON scheduled_scans(next_run_at) 
    WHERE enabled = TRUE AND next_run_at IS NOT NULL;
CREATE INDEX idx_sched_user    ON scheduled_scans(user_id);
CREATE INDEX idx_sched_enabled ON scheduled_scans(enabled);
```

---

## 4. API Routes (Scheduled Scans)

```
GET    /api/v1/schedules
  Query: ?enabled=true&page=1&limit=20
  Response: {items: [ScheduledScan], total}

POST   /api/v1/schedules
  Body: {name, targets[], scan_type, frequency, cron_expr (for custom), options}
  Response: 201 ScheduledScan (with next_run_at computed)

GET    /api/v1/schedules/{id}
  Response: ScheduledScan detail

PUT    /api/v1/schedules/{id}
  Body: {name?, targets?, frequency?, cron_expr?, options?}
  Response: 200 updated ScheduledScan (next_run_at recomputed)

DELETE /api/v1/schedules/{id}
  Response: 204

POST   /api/v1/schedules/{id}/enable
  Response: 200 {enabled: true, next_run_at: ...}

POST   /api/v1/schedules/{id}/disable
  Response: 200 {enabled: false}

POST   /api/v1/schedules/{id}/trigger
  Response: 201 {scan_id: uuid}  (immediate scan, regardless of schedule)
```

### 4.1 Create Schedule Request/Response

```json
// POST /api/v1/schedules
{
  "name": "Production Network Weekly Scan",
  "targets": ["192.168.1.0/24", "10.0.0.0/16"],
  "scan_type": "full",
  "frequency": "weekly",
  "options": {
    "intensity": 3,
    "timeout": 600
  }
}

// Response 201:
{
  "id": "uuid",
  "user_id": "uuid",
  "name": "Production Network Weekly Scan",
  "targets": ["192.168.1.0/24", "10.0.0.0/16"],
  "scan_type": "full",
  "frequency": "weekly",
  "cron_expr": "0 2 * * 0",
  "enabled": true,
  "last_run_at": null,
  "next_run_at": "2026-06-21T02:00:00Z",
  "created_at": "2026-06-16T11:00:00Z"
}
```

---

## 5. NATS Integration

### 5.1 Asset Service Events

| Topic | Direction | Consumer |
|-------|-----------|---------|
| `scan.scan.completed` | Subscribe | `ScanCompletedConsumer` (upsert assets) |
| `finding.status.changed` | Subscribe | Update `finding_count` + `risk_score` |

### 5.2 Asset Updates via finding.status.changed

```go
// asset-service/internal/adapter/messaging/nats/finding_changed_consumer.go

func (c *FindingStatusChangedConsumer) Handle(ctx context.Context, msg *FindingStatusChangedEvent) error {
    // When a finding is closed/reopened, update asset's finding_count and risk_score
    
    if msg.ComponentName == "" { return nil }  // No IP reference
    
    // Get updated stats from finding-service
    stats, err := c.findingSvcClient.GetFindingStats(ctx, 
        &FindingStatsRequest{ComponentName: msg.ComponentName})
    if err != nil { return err }
    
    asset, err := c.assetRepo.FindByIP(ctx, msg.ComponentName)
    if err != nil { return nil }  // Asset might not exist
    
    asset.FindingCount = int(stats.Active)
    asset.RiskScore = computeRiskScore(stats)
    asset.UpdatedAt = time.Now().UTC()
    
    return c.assetRepo.Update(ctx, asset)
}
```

---

## 6. Configuration

### 6.1 Asset Service Config

```yaml
# asset-service/config/config.yaml
server:
  http_port: 8068
  grpc_port: 50068

database:
  host: "${DB_HOST}"
  port: 5432
  name: "asset_service"
  user: "${DB_USER}"
  password: "${DB_PASSWORD}"

nats:
  url: "${NATS_URL}"
  subscriptions:
    - subject: "scan.scan.completed"
      queue_group: "asset-service-upsert"
    - subject: "finding.status.changed"
      queue_group: "asset-service-risk"

clients:
  finding_service:
    grpc_address: "finding-service:50060"
  scan_service:
    grpc_address: "scan-service:50058"

auth:
  jwt_public_key_path: "/secrets/jwt_public.pem"

logging:
  level: "info"
  format: "json"
```

### 6.2 scan-service Scheduler Config (addition)

```yaml
# scan-service/config/config.yaml (addition)
scheduler:
  enabled: true
  check_interval: "1m"  # Check every 1 minute
  max_concurrent_scheduled: 3  # Limit concurrent scheduled scans
```

---

## 7. Startup Recovery

```go
// scan-service/internal/scheduler/scheduler.go

// On startup: load all enabled schedules with next_run_at in the past
// These were missed during downtime → trigger immediately

func (s *Scheduler) recoverMissedSchedules(ctx context.Context) {
    missed, err := s.scheduledScanRepo.FindMissed(ctx, time.Now().UTC())
    if err != nil {
        s.logger.Error().Err(err).Msg("recover missed schedules: query failed")
        return
    }
    
    for _, sched := range missed {
        s.logger.Info().
            Str("schedule_id", sched.ID.String()).
            Str("missed_at", sched.NextRunAt.Format(time.RFC3339)).
            Msg("recovering missed scheduled scan")
        
        // Trigger immediately
        s.createScanUC.Execute(ctx, CreateScanInput{
            UserID:  sched.UserID,
            Targets: sched.Targets,
            Type:    sched.ScanType,
            Options: sched.Options,
        })
        
        // Update next_run_at
        now := time.Now().UTC()
        sched.LastRunAt = &now
        nextRun := sched.ComputeNextRun()
        sched.NextRunAt = &nextRun
        s.scheduledScanRepo.Update(ctx, sched)
    }
}

// FindMissed query:
// SELECT * FROM scheduled_scans
//   WHERE enabled = TRUE 
//     AND next_run_at < NOW()
//     AND (last_run_at IS NULL OR last_run_at < next_run_at)
```

---

## 8. Implementation Roadmap

### Phase 1 — Asset Service Core (Sprint 1)
- [ ] Database migrations
- [ ] Asset entity + AssetRepository
- [ ] UpsertAsset use case
- [ ] ScanCompletedConsumer (NATS)
- [ ] Basic REST endpoints (GET, DELETE)

### Phase 2 — Asset Tagging + Risk (Sprint 2)
- [ ] TagAsset use case (set/add/remove)
- [ ] Risk score computation
- [ ] `PUT /assets/{id}/tags` endpoint
- [ ] finding-service gRPC client integration
- [ ] `GET /assets/{id}/findings` endpoint

### Phase 3 — Asset Search + History (Sprint 3)
- [ ] Advanced filtering (tag, OS, port, query)
- [ ] Asset scan history table
- [ ] `GET /assets/{id}/history` endpoint
- [ ] finding.status.changed consumer (risk update)

### Phase 4 — Scheduled Scans (Sprint 2, parallel)
- [ ] ScheduledScan entity + DB migrations
- [ ] ComputeNextRun using cron parser
- [ ] CRUD API endpoints
- [ ] Scheduler goroutine in scan-service
- [ ] Startup recovery for missed scans

---

## 9. Inter-Service Dependencies

```
asset-service
├── DEPENDS ON:
│   ├── auth-service (gRPC: ValidateToken)          [CR-OVS-003]
│   ├── scan-service (gRPC: GetFindings)            [CR-OVS-001]
│   └── finding-service (gRPC: GetFindingStats,     [CR-OVS-002]
│                              GetFindingsByComponent)
│
└── SUBSCRIBED TO (NATS):
    ├── scan.scan.completed → UpsertAsset
    └── finding.status.changed → UpdateRiskScore

scan-service (scheduler extension)
├── SELF-CONTAINED: ScheduledScan creates Scan via CreateScanUseCase
└── No new inter-service dependencies
```

---

## 10. Acceptance Criteria Mapping

### Asset Management

| Criterion | Implementation |
|-----------|---------------|
| Scan completed → assets upserted automatically | `ScanCompletedConsumer` → `UpsertAssetUseCase` |
| Same IP → update (not create new) | `assetRepo.FindByIP()` → update if found |
| `PUT /assets/{id}/tags` mode=add → tags added | `TagAsset.Execute()` mode check |
| `GET /assets?tag=production` → tag filter | `AssetFilter.Tag` in query |
| `GET /assets?query=192.168.1` → IP prefix search | SQL: `WHERE ip_address::text LIKE $1%` |
| `GET /assets/{ip}/findings` → active findings from finding-service | gRPC proxy call |
| `finding_count` updated when findings change | `FindingStatusChangedConsumer` |
| `risk_score = 10.0` if Critical finding | `computeRiskScore()` Critical check |

### Scheduled Scanning

| Criterion | Implementation |
|-----------|---------------|
| `POST /schedules` frequency=daily → cron="0 2 * * *" | `FrequencyCronMap[FreqDaily]` |
| Scheduler checks every 1 minute | `time.NewTicker(1 * time.Minute)` |
| Trigger → `last_run_at=now`, `next_run_at` recomputed | `ComputeNextRun()` |
| `POST /schedules/{id}/trigger` → immediate scan | `createScanUC.Execute()` directly |
| `POST /schedules/{id}/disable` → no trigger until re-enabled | `enabled=false` in FindDue query |
| Custom cron `"0 */6 * * *"` → every 6 hours | `FreqCustom` + raw cron storage |
| Recovery after restart → load next_run_at from DB | `recoverMissedSchedules()` on startup |
