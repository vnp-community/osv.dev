# scan-service

**Bounded Context**: Vulnerability Scanning Orchestration
**Go Module**: `github.com/osv/scan-service`

---

## Merge từ

| Source | Trạng thái |
|--------|-----------|
| `services/scan-service` | ✅ Active — base chính |
| `services/schedule-service` | ✅ Active — merged |
| `archive/scan-service-old` | 📦 Archive — merged |
| `archive/scan-orchestrator` | 📦 Archive — merged |
| `archive/scanner` | 📦 Archive — merged |
| `archive/agent-service` | 📦 Archive — merged |
| `archive/asset-service` | 📦 Archive — merged |
| `archive/sbomvex` | 📦 Archive — merged |
| `archive/schedule-service` | 📦 Archive — merged |

---

## Chức năng

| # | Chức năng | Mô tả |
|---|-----------|-------|
| 1 | **Asset Registry** | Đăng ký và quản lý software assets (apps, containers, hosts) |
| 2 | **Scan Orchestration** | Tạo, phân phối và theo dõi scan jobs |
| 3 | **Agent Management** | Đăng ký scanner agents, heartbeat, health tracking |
| 4 | **Task Assignment** | Phân công scan tasks tới available agents |
| 5 | **SBOM Processing** | Parse và phân tích SBOM (CycloneDX, SPDX) |
| 6 | **VEX Processing** | Parse VEX statements cho vulnerability triage |
| 7 | **Schedule Management** | CRUD cron-based recurring scan schedules |
| 8 | **Schedule Triggering** | Tự động trigger scan khi đến giờ theo schedule |
| 9 | **Scan Results** | Nhận và lưu kết quả scan từ agents |
| 10 | **Event Publishing** | Phát events khi scan complete để finding-service xử lý |

---

## Clean Architecture Layout

```
scan-service/
├── cmd/
│   └── server/
│       └── main.go
│
├── internal/
│   ├── domain/                         # ← Business rules
│   │   ├── asset/
│   │   │   ├── entity.go               # Asset aggregate root
│   │   │   ├── repository.go           # AssetRepository interface
│   │   │   └── events.go               # AssetRegistered, AssetUpdated
│   │   ├── scan/
│   │   │   ├── entity.go               # ScanJob aggregate root
│   │   │   ├── repository.go           # ScanRepository interface
│   │   │   ├── events.go               # ScanStarted, ScanCompleted, ScanFailed
│   │   │   └── service.go              # ScanDomainService
│   │   ├── agent/
│   │   │   ├── entity.go               # ScannerAgent entity
│   │   │   ├── repository.go
│   │   │   └── heartbeat.go            # Heartbeat value object
│   │   ├── schedule/
│   │   │   ├── entity.go               # Schedule aggregate
│   │   │   ├── repository.go
│   │   │   └── cron.go                 # CronExpression value object
│   │   ├── sbom/
│   │   │   ├── entity.go               # SBOM document entity
│   │   │   └── component.go            # Component value object
│   │   └── errors/
│   │       └── errors.go
│   │
│   ├── usecase/                        # ← Application use cases
│   │   ├── register_asset/
│   │   │   ├── usecase.go
│   │   │   └── dto.go
│   │   ├── update_asset/
│   │   │   └── usecase.go
│   │   ├── initiate_scan/
│   │   │   ├── usecase.go
│   │   │   └── dto.go
│   │   ├── assign_to_agent/
│   │   │   └── usecase.go              # Pick best available agent
│   │   ├── update_scan_status/
│   │   │   └── usecase.go              # Agent reports progress
│   │   ├── complete_scan/
│   │   │   └── usecase.go              # Finalize scan, emit event
│   │   ├── register_agent/
│   │   │   └── usecase.go
│   │   ├── agent_heartbeat/
│   │   │   └── usecase.go
│   │   ├── process_sbom/
│   │   │   ├── usecase.go
│   │   │   └── parsers.go              # CycloneDX + SPDX parsers
│   │   ├── create_schedule/
│   │   │   └── usecase.go
│   │   ├── update_schedule/
│   │   │   └── usecase.go
│   │   └── trigger_scheduled_scan/
│   │       └── usecase.go              # Called by cron runner
│   │
│   ├── delivery/                       # ← Transport layer
│   │   ├── grpc/
│   │   │   ├── server.go
│   │   │   ├── scan_handler.go         # ScanService RPC impl
│   │   │   └── agent_handler.go        # ScannerAgentService RPC impl
│   │   └── http/
│   │       ├── router.go
│   │       ├── asset_handler.go
│   │       ├── scan_handler.go
│   │       ├── agent_handler.go
│   │       └── schedule_handler.go
│   │
│   ├── infra/                          # ← External systems
│   │   ├── postgres/
│   │   │   ├── asset_repo.go
│   │   │   ├── scan_repo.go
│   │   │   ├── agent_repo.go
│   │   │   └── schedule_repo.go
│   │   ├── redis/
│   │   │   ├── job_queue.go            # Scan job queue (FIFO)
│   │   │   └── agent_state.go          # Agent online/offline state
│   │   └── nats/
│   │       └── publisher.go            # Publish scan events
│   │
│   └── scheduler/
│       └── cron.go                     # Cron runner (robfig/cron)
│
├── migrations/
│   ├── 001_create_assets.sql
│   ├── 002_create_scan_jobs.sql
│   ├── 003_create_agents.sql
│   └── 004_create_schedules.sql
│
├── go.mod
└── Dockerfile
```

---

## Domain Model

### Asset Aggregate
```go
type Asset struct {
    ID          uuid.UUID
    ProductID   uuid.UUID           // Link to finding-service product
    Name        string
    Type        AssetType           // APPLICATION | CONTAINER | HOST | REPO
    Identifier  AssetIdentifier     // URL, image name, hostname, etc.
    Tags        []string
    Language    string              // go, python, java, etc.
    Ecosystem   string              // npm, maven, pypi, etc.
    Status      AssetStatus         // ACTIVE | ARCHIVED
    LastScanned *time.Time
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type AssetIdentifier struct {
    PURL        string   // pkg:npm/lodash@4.17.21
    RepoURL     string   // https://github.com/org/repo
    ImageRef    string   // docker.io/library/nginx:1.21
    Hostname    string
}
```

### ScanJob Aggregate
```go
type ScanJob struct {
    ID          uuid.UUID
    AssetID     uuid.UUID
    ScheduleID  *uuid.UUID          // nil if manual scan
    AgentID     *uuid.UUID          // nil until assigned
    Type        ScanType            // SBOM | DEPENDENCY | IMAGE | CODE | FULL
    Status      ScanStatus          // QUEUED | ASSIGNED | RUNNING | COMPLETED | FAILED
    Priority    int                 // 0-10 (higher = more urgent)
    Config      ScanConfig
    StartedAt   *time.Time
    CompletedAt *time.Time
    ResultRef   string              // Path to result in storage
    CreatedAt   time.Time
}

type ScanType  string // SBOM | DEPENDENCY | IMAGE | CODE | FULL
type ScanStatus string // QUEUED | ASSIGNED | RUNNING | COMPLETED | FAILED
```

### ScannerAgent
```go
type ScannerAgent struct {
    ID           uuid.UUID
    Name         string
    Version      string
    Capabilities []ScanType      // What this agent can scan
    Status       AgentStatus     // ONLINE | OFFLINE | BUSY
    LastSeen     time.Time
    CurrentLoad  int             // Active scan count
    MaxLoad      int             // Max concurrent scans
    Endpoint     string          // gRPC endpoint for task delivery
}
```

### Schedule
```go
type Schedule struct {
    ID          uuid.UUID
    Name        string
    AssetIDs    []uuid.UUID     // Targets
    CronExpr    string          // "0 2 * * *"
    ScanType    ScanType
    Status      ScheduleStatus  // ACTIVE | PAUSED | DISABLED
    LastRunAt   *time.Time
    NextRunAt   *time.Time
    CreatedAt   time.Time
}
```

---

## API Specification

### HTTP REST Endpoints

| Method | Path | Auth | Mô tả |
|--------|------|------|-------|
| `GET`  | `/assets` | JWT | Danh sách assets |
| `POST` | `/assets` | JWT | Đăng ký asset mới |
| `GET`  | `/assets/{id}` | JWT | Chi tiết asset |
| `PUT`  | `/assets/{id}` | JWT | Cập nhật asset |
| `DELETE` | `/assets/{id}` | JWT | Archive asset |
| `POST` | `/assets/{id}/scan` | JWT | Trigger scan ngay |
| `POST` | `/assets/sbom` | JWT | Upload SBOM để scan |
| `GET`  | `/scans` | JWT | Danh sách scan jobs |
| `POST` | `/scans` | JWT | Tạo scan job thủ công |
| `GET`  | `/scans/{id}` | JWT | Chi tiết + status |
| `POST` | `/scans/{id}/cancel` | JWT | Huỷ scan |
| `GET`  | `/schedules` | JWT | Danh sách schedules |
| `POST` | `/schedules` | JWT | Tạo recurring schedule |
| `PUT`  | `/schedules/{id}` | JWT | Cập nhật schedule |
| `POST` | `/schedules/{id}/pause` | JWT | Tạm dừng |
| `POST` | `/schedules/{id}/resume` | JWT | Tiếp tục |
| `GET`  | `/agents` | Admin | Danh sách agents |

### gRPC Services (internal)

```protobuf
service ScanService {
    rpc CreateScan(CreateScanRequest) returns (ScanJobResponse);
    rpc GetScan(GetScanRequest) returns (ScanJobResponse);
    rpc UpdateScanStatus(UpdateScanStatusRequest) returns (ScanJobResponse);
    rpc ListAssets(ListAssetsRequest) returns (ListAssetsResponse);
}

// Called by scanner agents
service ScannerAgentService {
    rpc Register(RegisterAgentRequest) returns (RegisterAgentResponse);
    rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse);
    rpc PollTask(PollTaskRequest) returns (ScanTask);
    rpc SubmitResult(SubmitResultRequest) returns (SubmitResultResponse);
}
```

---

## Event Publishing (NATS)

| Event | Subject | Payload | Consumer |
|-------|---------|---------|----------|
| `ScanStarted` | `scan.job.started` | job_id, asset_id | notification-service |
| `ScanCompleted` | `scan.job.completed` | job_id, asset_id, result_ref | finding-service |
| `ScanFailed` | `scan.job.failed` | job_id, error | notification-service |
| `AgentOffline` | `scan.agent.offline` | agent_id | notification-service |

---

## Dependencies

```
github.com/jackc/pgx/v5        # PostgreSQL
github.com/redis/go-redis/v9   # Job queue, agent state
github.com/nats-io/nats.go     # NATS events
github.com/go-chi/chi/v5       # HTTP router
github.com/robfig/cron/v3      # Schedule cron runner
google.golang.org/grpc         # gRPC (server + agent client)
github.com/osv/shared/pkg      # Shared utilities
github.com/osv/shared/proto    # gRPC contracts
```

---

## Database Schema (PostgreSQL)

```sql
-- Assets
CREATE TABLE assets (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id   UUID,
    name         VARCHAR(255) NOT NULL,
    type         VARCHAR(30) NOT NULL,
    purl         TEXT,
    repo_url     TEXT,
    image_ref    TEXT,
    hostname     TEXT,
    tags         TEXT[],
    status       VARCHAR(20) DEFAULT 'active',
    last_scanned TIMESTAMPTZ,
    created_at   TIMESTAMPTZ DEFAULT NOW(),
    updated_at   TIMESTAMPTZ DEFAULT NOW()
);

-- Scan Jobs
CREATE TABLE scan_jobs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    asset_id     UUID REFERENCES assets(id),
    schedule_id  UUID,
    agent_id     UUID,
    type         VARCHAR(30) NOT NULL,
    status       VARCHAR(20) DEFAULT 'queued',
    priority     INT DEFAULT 5,
    config       JSONB,
    started_at   TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    result_ref   TEXT,
    created_at   TIMESTAMPTZ DEFAULT NOW()
);

-- Agents
CREATE TABLE scanner_agents (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name         VARCHAR(255),
    version      VARCHAR(50),
    capabilities TEXT[],
    status       VARCHAR(20) DEFAULT 'offline',
    endpoint     TEXT,
    max_load     INT DEFAULT 5,
    last_seen    TIMESTAMPTZ,
    created_at   TIMESTAMPTZ DEFAULT NOW()
);

-- Schedules
CREATE TABLE schedules (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(255),
    asset_ids  UUID[],
    cron_expr  VARCHAR(100),
    scan_type  VARCHAR(30),
    status     VARCHAR(20) DEFAULT 'active',
    last_run   TIMESTAMPTZ,
    next_run   TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

---

## Configuration

```yaml
server:
  http_port: 8084
  grpc_port: 50054

postgres:
  dsn: "${POSTGRES_DSN}"

redis:
  addr: "${REDIS_ADDR}"
  db: 2
  job_queue: "scan:jobs"
  agent_state_prefix: "scan:agent:"

nats:
  url: "${NATS_URL}"
  stream: "SCAN_EVENTS"

scheduler:
  check_interval: "1m"    # How often to check for due schedules
  max_concurrent: 50      # Max parallel scan jobs

agent:
  heartbeat_timeout: "30s"
  offline_threshold: "2m"
```
