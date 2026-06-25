# scan-service — Upgrade Specification (Chỉ Thêm, Không Xóa)

> **Audit tại**: `services/scan-service/`
> **Trạng thái hiện tại**: ~65% complete
> **Ưu tiên**: P2
> **Nguyên tắc**: Mọi thay đổi chỉ THÊM file/package mới. Code hiện có GIỮ NGUYÊN.

---

## ✅ Implementation Status — 2026-06-13

> **Trạng thái cũ**: ~65% | **Trạng thái mới**: ~85% ✅
> **Build**: `go build ./...` PASSED

### Đã implement (Sprint 3):
- ✅ `adapters/scanner/trivy/trivy_client.go` — Trivy CLI + Server mode adapter
  - `ScanImage()` — container image scanning → CycloneDX SBOM
  - `ScanDirectory()` / `ScanFilesystem()` — filesystem scanning
  - Configurable timeout, cache dir management
- ✅ `infra/messaging/nats/agent_publisher.go` — agent event publisher
- ✅ `infra/messaging/nats/schedule/publisher.go` — schedule publisher
- ✅ `adapters/messaging/publisher/scan_publisher.go`
- ✅ `delivery/http/schedule/schedule_handler.go`
- ✅ `scheduler/cron_worker.go`
- ✅ Existing: nmap + zap scanners giữ nguyên ✅

### Còn lại (Backlog P3):
- ⏳ `adapters/scanner/semgrep/semgrep_client.go` — binary dep
- ⏳ `usecase/register_agent/` + `usecase/agent_heartbeat/`
- ⏳ `usecase/process_scan_result/`
- ⏳ `delivery/http/sbom_handler.go`

---


## 1. Những gì đã có — GIỮ NGUYÊN ✅

### Domain Layer — GIỮ TẤT CẢ
- `domain/scan/`: Scan entity + repository ✅
- `domain/agent/`: Agent entity ✅
- `domain/asset/`: Asset entity ✅
- `domain/schedule/`: Schedule entity ✅
- `domain/entity/` (generic): ScanType, ScanStatus, ScanOptions ✅
- `domain/repository/repositories.go` ✅

### Use Cases — GIỮ NGUYÊN
- `usecase/create_scan/create_scan.go`: Validate targets → persist → publish NATS ✅
- `usecase/execute_scan/execute_scan.go` ✅

### SBOM/VEX Parsers — GIỮ NGUYÊN (toàn bộ)
- `sbom/cyclonedx/json.go` ✅
- `sbom/spdx/tag_value.go` ✅
- `sbom/swid/xml.go` ✅
- `sbom/vex/cdx/cdx_vex.go` ✅
- `sbom/vex/openvex/openvex.go` ✅
- `sbom/vex/csaf/csaf_vex.go` ✅
- `sbom/parser.go` ✅
- `sbom/entity/sbom.go` + `vex.go` ✅

### Dependency Parsers — GIỮ NGUYÊN (toàn bộ)
- `parsers/golang.go`, `python.go`, `nodejs.go`, `java.go`, `rust.go` ✅
- `parsers/checkers/` ✅

### Infrastructure — GIỮ TẤT CẢ
- `infra/leader/redis_lock.go` ✅
- `infra/persistence/postgres/schedule/schedule_repo.go` ✅
- `infra/messaging/nats/agent_publisher.go` + `schedule/publisher.go` ✅
- `infra/validator/xml_schema.go` ✅
- `infra/scan_infra/` ✅ **GIỮ NGUYÊN** (legacy domain/usecase trong infra)
- `infrastructure/` ✅ **GIỮ NGUYÊN** (legacy)

### Adapters — GIỮ NGUYÊN
- `adapters/handler/grpc/scan_grpc_handler.go` ✅
- `adapters/handler/http/scan_handler.go` ✅
- `adapters/repository/postgres/scan_repo.go` ✅
- `adapters/worker/pool.go` ✅
- `adapters/scanner/zap/zap_client.go` ✅ OWASP ZAP
- `adapters/scanner/nmap/nmap_scanner.go` ✅ Nmap
- `adapters/messaging/publisher/scan_publisher.go` ✅
- `scheduler/cron_worker.go` ✅
- `delivery/http/schedule/schedule_handler.go` ✅

---

## 2. Những gì cần THÊM (Gaps)

### 🟡 P1 — Thêm: Agent Management Use Cases

`domain/agent/` đã có Agent entity, nhưng thiếu use cases để quản lý agent lifecycle.

**Thêm mới** (không sửa domain/agent/ cũ):
```
usecase/register_agent/
└── usecase.go    ← NEW

usecase/agent_heartbeat/
└── usecase.go    ← NEW

usecase/get_agent_status/
└── usecase.go    ← NEW
```

```go
// usecase/register_agent/usecase.go
type RegisterAgentRequest struct {
    AgentID      uuid.UUID  `json:"agent_id"`
    Hostname     string     `json:"hostname"`
    IPAddress    string     `json:"ip_address"`
    Capabilities []string   `json:"capabilities"`  // ["nmap", "zap", "sbom"]
    Version      string     `json:"version"`
}

type UseCase struct {
    agentRepo repository.AgentRepository  // existing interface
}

func (uc *UseCase) Execute(ctx context.Context, req RegisterAgentRequest) error
```

```go
// usecase/agent_heartbeat/usecase.go
type HeartbeatRequest struct {
    AgentID    uuid.UUID `json:"agent_id"`
    RunningJobs int      `json:"running_jobs"`
    SystemLoad  float64  `json:"system_load"`
}

// Update agent.last_heartbeat + is_online = true
// Nếu running_jobs > 0 → mark agent.status = busy
```

### 🟡 P1 — Thêm: Agent NATS Consumer

Hiện tại có `agent_publisher` nhưng không có consumer để nhận heartbeats từ agents.

**Thêm mới**:
```
infra/messaging/nats/agent_consumer.go   ← NEW
```

```go
// infra/messaging/nats/agent_consumer.go
// Subscribe: scan.agent.heartbeat  → agent_heartbeat UC
// Subscribe: scan.agent.registered → register_agent UC
// Subscribe: scan.agent.result     → process_scan_result UC (see below)
```

### 🟡 P1 — Thêm: Agent Offline Detection Cron

**Thêm vào** `scheduler/cron_worker.go` (không sửa các jobs cũ, thêm job mới):

```go
// scheduler/cron_worker.go — CHỈ THÊM job mới:
c.AddFunc("*/5 * * * *", func() {
    // Find agents where last_heartbeat < NOW() - 5 minutes
    // Set is_online = false
    // Publish: scan.agent.offline event
    agentOfflineChecker.Run(ctx)
})
```

Hoặc tạo file riêng:
```
scheduler/agent_offline_checker.go   ← NEW
```

### 🟡 P1 — Thêm: Migration cho Agent Heartbeat Fields

**Thêm migration mới** (không sửa 001-003):
```sql
-- migrations/004_agent_heartbeat.up.sql  ← NEW
-- Thêm columns vào bảng agents đã có
ALTER TABLE agents ADD COLUMN IF NOT EXISTS last_heartbeat TIMESTAMPTZ;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS is_online BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS capabilities TEXT[];
ALTER TABLE agents ADD COLUMN IF NOT EXISTS system_load FLOAT;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS running_jobs INT NOT NULL DEFAULT 0;

-- Index cho offline detection query
CREATE INDEX IF NOT EXISTS idx_agents_heartbeat ON agents(last_heartbeat) WHERE is_online = TRUE;
```

### 🟡 P1 — Thêm: Scan Result Processing Pipeline

**Thêm mới**:
```
usecase/process_scan_result/
├── usecase.go    ← NEW: Receive agent result → parse → forward to finding-service
└── dto.go        ← NEW: ScanResultInput, ProcessedResult
```

```go
// usecase/process_scan_result/usecase.go
// Flow:
// 1. Receive raw scan output từ agent (via NATS)
// 2. Detect format (Nmap XML? ZAP JSON? SBOM?)
// 3. Parse bằng existing parsers/
// 4. Call finding-service.ImportScanResult (gRPC)
// 5. Update scan status → COMPLETED
// 6. Publish scan.job.completed event

type UseCase struct {
    scanRepo      repository.ScanRepository
    findingClient *grpcclient.FindingServiceClient  // NEW grpc client
    publisher     nats.ScanEventPublisher
    parsers       map[string]parser.ScanOutputParser
}
```

### 🟡 P1 — Thêm: gRPC Client to Finding-Service

**Thêm mới**:
```
adapters/grpcclient/
└── finding_client.go   ← NEW
```

```go
// adapters/grpcclient/finding_client.go
package grpcclient

type FindingServiceClient struct {
    conn   *grpc.ClientConn
    client findingpb.FindingServiceClient
}

func NewFindingServiceClient(addr string) (*FindingServiceClient, error)

func (c *FindingServiceClient) ImportScanResult(ctx context.Context, req *ImportScanResultRequest) error
// ImportScanResultRequest chứa: scan_id, findings [], tool_type, ...
```

### 🟡 P1 — Thêm: SBOM Ingest HTTP Endpoint

SBOM parsers đã có, nhưng thiếu HTTP endpoint để upload SBOM file.

**Thêm mới** (không sửa scan_handler.go cũ):
```
delivery/http/sbom_handler.go   ← NEW
```

```go
// POST /api/v1/scan/sbom/upload
// Content-Type: multipart/form-data
// Fields: file (SBOM file), format (cyclonedx/spdx/swid), product_id

func (h *SBOMHandler) Upload(w http.ResponseWriter, r *http.Request) {
    // 1. Parse multipart form
    // 2. Detect/validate format
    // 3. Call existing sbom.Parse() 
    // 4. Extract components
    // 5. Create scan job for each component
    // 6. Return scan_id
}
```

**Route** (thêm vào router):
```go
r.Post("/api/v1/scan/sbom/upload", sbomH.Upload)          // NEW
r.Get("/api/v1/scan/sbom/{scan_id}/status", sbomH.Status) // NEW
```

### 🟡 P1 — Thêm: Scan Event Publisher Enhancement

**Thêm vào** `adapters/messaging/publisher/scan_publisher.go` (thêm methods mới):

```go
// Thêm methods mới vào ScanPublisher (giữ methods cũ):
func (p *ScanPublisher) PublishJobCompleted(ctx context.Context, scanID uuid.UUID, findingCount int) error
func (p *ScanPublisher) PublishJobFailed(ctx context.Context, scanID uuid.UUID, reason string) error
func (p *ScanPublisher) PublishAgentOffline(ctx context.Context, agentID uuid.UUID) error

// Events:
// scan.job.completed  → finding-service, notification-service
// scan.job.failed     → notification-service
// scan.agent.offline  → notification-service
```

### 🟢 P2 — Thêm: Trivy Scanner Adapter

Bên cạnh ZAP và Nmap, thêm Trivy cho container scanning:

```
adapters/scanner/trivy/
└── trivy_client.go   ← NEW
```

```go
// Gọi Trivy CLI hoặc Trivy Server API
// Output: SBOM (cyclonedx) → reuse existing sbom parsers
```

### 🟢 P2 — Thêm: Semgrep Adapter (SAST)

```
adapters/scanner/semgrep/
└── semgrep_client.go   ← NEW
```

### 🟢 P2 — Thêm: Asset Discovery Use Case

```
usecase/discover_assets/
└── usecase.go   ← NEW
// Scan network range → discover hosts, services, versions
// Create/update asset records
```

---

## 3. Migration Plan — Chỉ Thêm File Mới

```
migrations/
├── 001_initial_schema.sql             ← GIỮ NGUYÊN
├── 002_agent_001_initial_schema.sql   ← GIỮ NGUYÊN
├── 003_asset_001_initial_schema.sql   ← GIỮ NGUYÊN
└── 004_agent_heartbeat.up.sql         ← NEW (ALTER TABLE, không drop)
```

---

## 4. File Changes Summary

### Files cần THÊM MỚI:
```
usecase/register_agent/usecase.go
usecase/agent_heartbeat/usecase.go
usecase/get_agent_status/usecase.go
usecase/process_scan_result/usecase.go
usecase/process_scan_result/dto.go
usecase/discover_assets/usecase.go    (P2)
scheduler/agent_offline_checker.go
infra/messaging/nats/agent_consumer.go
adapters/grpcclient/finding_client.go
adapters/scanner/trivy/trivy_client.go    (P2)
adapters/scanner/semgrep/semgrep_client.go (P2)
delivery/http/sbom_handler.go
migrations/004_agent_heartbeat.up.sql
```

### Files cần EXTEND (chỉ thêm vào):
```
scheduler/cron_worker.go                           ← Thêm agent_offline_checker job
adapters/messaging/publisher/scan_publisher.go     ← Thêm 3 publish methods mới
cmd/server/main.go                                 ← Thêm wire cho new UCs + consumers
```

### Files KHÔNG ĐƯỢC CHẠM:
```
infra/scan_infra/          ← GIỮ NGUYÊN (legacy)
infrastructure/             ← GIỮ NGUYÊN (legacy)
usecase/create_scan/        ← GIỮ NGUYÊN
usecase/execute_scan/       ← GIỮ NGUYÊN
migrations/001-003          ← KHÔNG BAO GIỜ SỬA
```

---

## 5. Checklist

### Phase A — P1 (Sprint 2)
- [ ] Thêm `usecase/register_agent/usecase.go`
- [ ] Thêm `usecase/agent_heartbeat/usecase.go`
- [ ] Thêm `infra/messaging/nats/agent_consumer.go`
- [ ] Thêm `migrations/004_agent_heartbeat.up.sql` (ALTER TABLE only)
- [ ] Thêm `scheduler/agent_offline_checker.go`
- [x] Thêm agent offline job vào `scheduler/cron_worker.go`

### Phase B — P1 (Sprint 2)
- [ ] Thêm `adapters/grpcclient/finding_client.go`
- [ ] Thêm `usecase/process_scan_result/usecase.go`
- [ ] Wire scan result consumer trong `infra/messaging/nats/agent_consumer.go`
- [ ] Thêm 3 publish methods vào `scan_publisher.go`

### Phase C — P1 (Sprint 2-3)
- [ ] Thêm `delivery/http/sbom_handler.go`
- [ ] Thêm SBOM upload + status routes

### Phase D — P2 (Sprint 3+)
- [ ] Thêm Trivy scanner adapter
- [ ] Thêm Semgrep adapter
- [ ] Thêm `usecase/discover_assets/`
