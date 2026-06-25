# SOL-OVS-001 — Giải Pháp: Scan Service (Nmap / ZAP / Agent)

| Trường | Giá trị |
|--------|---------|
| **Solution ID** | SOL-OVS-001 |
| **CR tham chiếu** | CR-OVS-001 |
| **Tiêu đề** | Scan Service — Network Vulnerability Scanning (Nmap), Web Application Scanning (OWASP ZAP), Agent-Based Scanning |
| **Ngày tạo** | 2026-06-16 |
| **Ngày implement** | 2026-06-17 |
| **Trạng thái** | ✅ Implemented |

---

## 0. Implementation Status

> **Trạng thái**: ✅ **IMPLEMENTED** — 2026-06-17

| Task | File | Trạng thái |
|------|------|------------|
| T-SCAN-001 | `scan-service/internal/scanner/nmap/scanner.go` | ✅ Done |
| T-SCAN-002 | `scan-service/internal/scanner/zap/scanner.go` | ✅ Done |
| T-SCAN-003 | `scan-service/internal/usecase/execute_scan.go` | ✅ Done |
| T-SCAN-004 | `scan-service/internal/delivery/sse/hub.go` | ✅ Done |
| T-SCAN-005 | `scan-service/agent/agent.py` | ✅ Done |

**Chi tiết implementation**:
- **Nmap Scanner**: subprocess wrapper với XML parser, context-aware cancellation, CVE extraction via regex
- **ZAP Scanner**: Spider → Active Scan → Collect Alerts, HTTP polling với configurable timeout
- **ExecuteScan**: Full lifecycle (pending → queued → running → completed/failed/cancelled), NATS publish, sync.Map for process tracking
- **SSE Hub**: Goroutine-per-client, 15s heartbeat, auto-close khi `progress=100`, channel-based fan-out
- **Python Agent**: dpkg/rpm/pip package collection, OSV.dev API batch query, listening ports detection

---

## 1. Tổng Quan Giải Pháp

### 1.1 Bối Cảnh

OSV.dev là một **CVE database + search engine** thuần túy. Nó không có khả năng **active scanning** — tức là quét thực sự vào mạng/host để phát hiện lỗ hổng. CR-OVS-001 yêu cầu thêm một `scan-service` mới vào OpenVulnScan để bổ sung khả năng này.

### 1.2 Phạm Vi Giải Pháp

Xây dựng **mới hoàn toàn** microservice `scan-service` theo kiến trúc **Clean Architecture** (Domain → UseCase → Adapter → Delivery), viết bằng **Go**, expose:
- **HTTP REST API** (port 8058) cho client UI/CLI
- **gRPC** (port 50058) cho inter-service communication
- **NATS** consumers và publishers cho event-driven flow
- **SSE endpoint** cho real-time progress streaming

---

## 2. Kiến Trúc Tổng Thể

### 2.1 Vị Trí Trong Hệ Thống

```
┌─────────────────────────────────────────────────────────────────────┐
│                        OpenVulnScan Platform                        │
│                                                                     │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────────┐  │
│  │  unified-    │    │  scan-       │    │  finding-service     │  │
│  │  gateway     │───▶│  service     │───▶│  (CR-OVS-002)        │  │
│  │              │    │  :8058/:50058│    │                      │  │
│  └──────────────┘    └──────┬───────┘    └──────────────────────┘  │
│                             │ NATS                                   │
│                    ┌────────▼────────┐   ┌──────────────────────┐  │
│                    │  NATS JetStream │   │  asset-service       │  │
│                    │                 │   │  (CR-OVS-007)        │  │
│                    └─────────────────┘   └──────────────────────┘  │
│                                                                     │
│  External tools:                                                    │
│  ┌──────────┐  ┌──────────┐  ┌──────────────────────────────────┐  │
│  │  Nmap    │  │  OWASP   │  │  Python Agent (on remote hosts)  │  │
│  │  binary  │  │  ZAP API │  │                                  │  │
│  └──────────┘  └──────────┘  └──────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

### 2.2 Cấu Trúc Thư Mục

```
services/scan-service/
├── cmd/
│   └── server/
│       └── main.go                      # Entry point
├── internal/
│   ├── domain/
│   │   ├── entity/
│   │   │   ├── scan.go                  # Scan, ScanType, ScanStatus, ScanOptions
│   │   │   ├── finding.go               # Finding, Port, Service, WebAlert, DiscoveryHost
│   │   │   └── schedule.go              # ScheduledScan entity (CR-OVS-007)
│   │   └── port/
│   │       ├── repository.go            # ScanRepository, FindingRepository interfaces
│   │       └── event_bus.go             # EventBus interface
│   ├── usecase/
│   │   ├── create_scan/
│   │   │   └── usecase.go
│   │   ├── execute_scan/
│   │   │   └── usecase.go
│   │   ├── cancel_scan/
│   │   │   └── usecase.go
│   │   └── agent/
│   │       └── process_report.go
│   ├── scanner/
│   │   ├── nmap/
│   │   │   └── scanner.go               # Nmap subprocess wrapper
│   │   └── zap/
│   │       └── scanner.go               # OWASP ZAP API client
│   ├── adapter/
│   │   ├── repository/
│   │   │   └── postgres/                # PostgreSQL implementations
│   │   └── messaging/
│   │       └── nats/                    # NATS publisher/consumer
│   └── delivery/
│       ├── http/
│       │   ├── scan_handler.go          # REST handlers
│       │   ├── agent_handler.go         # Agent report endpoint
│       │   └── sse_handler.go           # SSE stream handler
│       └── grpc/
│           └── scan_server.go
├── migrations/
│   └── 001_create_scans.sql
├── proto/
│   └── scan/v1/scan.proto
└── config/
    └── config.yaml
```

---

## 3. Domain Model Chi Tiết

### 3.1 State Machine — ScanStatus

```
              ┌──────────┐
     create   │          │  cancel
    ─────────▶│ pending  │──────────────────────────────┐
              │          │                              │
              └────┬─────┘                              │
                   │ worker picks up                    │
                   ▼                                    ▼
              ┌──────────┐  cancel             ┌─────────────┐
              │          │──────────────────▶  │             │
              │  queued  │                     │  cancelled  │
              │          │◀──(invalid)─────── │             │
              └────┬─────┘                     └─────────────┘
                   │                                    ▲
                   │ nmap/zap starts                    │ cancel
                   ▼                                    │
              ┌──────────┐  cancel             ┌────────┴────┐
              │          │──────────────────▶  │             │
              │  running │                     │  cancelled  │
              │          │                     └─────────────┘
              └────┬─────┘
           ┌───────┴───────┐
           ▼               ▼
    ┌──────────┐     ┌──────────┐
    │completed │     │  failed  │
    └──────────┘     └──────────┘
```

**Quy tắc kinh doanh:**
- Chỉ `pending/queued/running` có thể bị cancel
- Completed/failed/cancelled là **terminal states** (không thể chuyển đổi tiếp)
- Transition validation xảy ra trong `ScanStatus.CanTransitionTo()`

### 3.2 Target Validation Logic

```go
// validateTarget — phân biệt 4 loại targets
func validateTarget(target string) error {
    // 1. IP address: 192.168.1.1
    if net.ParseIP(target) != nil { return nil }
    
    // 2. CIDR range: 192.168.1.0/24
    if _, _, err := net.ParseCIDR(target); err == nil { return nil }
    
    // 3. URL (for ZAP): https://example.com
    if u, err := url.ParseRequestURI(target); err == nil && u.Host != "" { return nil }
    
    // 4. Hostname: example.com
    if isValidHostname(target) { return nil }
    
    return fmt.Errorf("must be a valid IP, CIDR, hostname, or URL")
}

// Hostname regex: RFC 1123 compliant
var hostnameRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`)
```

---

## 4. Scan Execution Flow

### 4.1 Nmap Full Scan Flow

```
CreateScan (HTTP POST)
      │
      │ 1. Validate targets
      │ 2. Persist scan entity (status=pending)
      │ 3. Publish NATS: scan.scan.created
      ▼
NATS Consumer (ExecuteScanUseCase)
      │
      │ 1. Load scan
      │ 2. Transition → queued
      │ 3. Transition → running
      │ 4. Run NmapScanner.FullScan():
      │    - Build nmap args: [-sV, -O, --script=vulners, -oX, -, -T4]
      │    - exec.CommandContext(ctx, "nmap", args...)
      │    - Stream stdout/stderr
      │    - Parse XML output → Finding entities
      │    - Extract CVE IDs via regex
      │ 5. Call vulnerability-service gRPC for CVE enrichment
      │ 6. Save findings to PostgreSQL
      │ 7. Transition → completed
      │ 8. Publish NATS: scan.scan.completed
      ▼
finding-service (NATS consumer)
asset-service (NATS consumer)
```

### 4.2 ZAP Scan Flow

```
CreateScan (type=web)
      │
      ▼
ExecuteScanUseCase.runZAPScan()
      │
      │ 1. POST /JSON/spider/action/scan/ → spiderID
      │ 2. Poll GET /JSON/spider/view/status/ every 5s
      │    until status=100% or timeout
      │ 3. POST /JSON/ascan/action/scan/ → scanID  
      │ 4. Poll GET /JSON/ascan/view/status/ every 5s
      │    until status=100% or timeout (600s default)
      │ 5. GET /JSON/core/view/alerts/ → alerts[]
      │ 6. Map ZAP alerts → WebAlert entities
      │ 7. Classify risk level
      ▼
Store WebAlert[] in web_alerts table
```

### 4.3 Nmap Process Cancel

```go
// scan-service/internal/usecase/execute_scan/usecase.go

type ExecuteScanUseCase struct {
    // ...
    activeCmds sync.Map  // scanID → *exec.Cmd (for cancellation)
}

func (uc *ExecuteScanUseCase) runNmapWithContext(ctx context.Context, 
    scan *entity.Scan) ([]*entity.Finding, error) {
    
    cmd := exec.CommandContext(ctx, nmapPath, args...)
    // Store cmd reference for cancel
    uc.activeCmds.Store(scan.ID, cmd)
    defer uc.activeCmds.Delete(scan.ID)
    
    // ctx cancellation terminates nmap process automatically
    return parse(cmd.Output())
}

// Called by CancelScan use case
func (uc *ExecuteScanUseCase) Cancel(scanID uuid.UUID) {
    if cmd, ok := uc.activeCmds.Load(scanID); ok {
        cmd.(*exec.Cmd).Process.Kill()
    }
}
```

---

## 5. SSE (Server-Sent Events) Implementation

### 5.1 Stream Protocol

```
GET /api/v1/scans/{id}/stream
→ Content-Type: text/event-stream

data: {"scan_id":"uuid","status":"running","progress":35}

data: {"scan_id":"uuid","status":"running","progress":70}

data: {"scan_id":"uuid","status":"completed","progress":100,"finding_count":5}

event: done
data: {}
```

### 5.2 Handler Design

```go
func (h *Handler) StreamScanProgress(w http.ResponseWriter, r *http.Request) {
    // Validate auth (JWT) - same middleware as other endpoints
    
    scanID := chi.URLParam(r, "id")
    
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("X-Accel-Buffering", "no")  // Disable nginx buffering
    
    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "SSE not supported", http.StatusInternalServerError)
        return
    }
    
    ticker := time.NewTicker(2 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-r.Context().Done():
            return
        case <-ticker.C:
            scan, err := h.scanRepo.FindByID(r.Context(), scanID)
            if err != nil { return }
            
            data, _ := json.Marshal(ScanProgressEvent{
                ScanID:       scan.ID,
                Status:       scan.Status,
                Progress:     scan.Progress,
                FindingCount: scan.FindingCount,
            })
            
            fmt.Fprintf(w, "data: %s\n\n", data)
            flusher.Flush()
            
            if scan.IsTerminal() {
                fmt.Fprintf(w, "event: done\ndata: {}\n\n")
                flusher.Flush()
                return
            }
        }
    }
}
```

---

## 6. Agent Python Script

### 6.1 Agent Design

```python
#!/usr/bin/env python3
# /api/v1/agents/download → trả về script này

"""
OpenVulnScan Agent
- Thu thập installed packages (apt/rpm/pip/npm)
- Query OSV.dev API để tìm CVEs
- Report findings về scan-service
"""

import sys
import json
import platform
import subprocess
import requests

API_URL = "https://your-openvulnscan.example.com"
AGENT_ID = "agent-" + platform.node()

def get_installed_packages() -> list[dict]:
    packages = []
    
    # Debian/Ubuntu: dpkg
    try:
        out = subprocess.check_output(["dpkg", "-l"], text=True)
        for line in out.splitlines()[5:]:
            parts = line.split()
            if len(parts) >= 3 and parts[0] == 'ii':
                packages.append({
                    "name": parts[1], 
                    "version": parts[2], 
                    "ecosystem": "Debian"
                })
    except FileNotFoundError:
        pass
    
    # RPM-based: rpm -qa
    try:
        out = subprocess.check_output(["rpm", "-qa", "--qf", "%{NAME} %{VERSION}\n"], text=True)
        for line in out.splitlines():
            parts = line.split()
            if len(parts) == 2:
                packages.append({
                    "name": parts[0], 
                    "version": parts[1], 
                    "ecosystem": "AlmaLinux"
                })
    except FileNotFoundError:
        pass
    
    # Python: pip list
    try:
        out = subprocess.check_output([sys.executable, "-m", "pip", "list", "--format=json"], text=True)
        for pkg in json.loads(out):
            packages.append({
                "name": pkg["name"], 
                "version": pkg["version"], 
                "ecosystem": "PyPI"
            })
    except:
        pass
    
    return packages

def query_osv(package: str, version: str, ecosystem: str) -> list[dict]:
    resp = requests.post("https://api.osv.dev/v1/query", json={
        "package": {"name": package, "ecosystem": ecosystem},
        "version": version
    }, timeout=10)
    if resp.status_code == 200:
        return resp.json().get("vulns", [])
    return []

def main(api_key: str):
    packages = get_installed_packages()
    findings = []
    
    for pkg in packages:
        vulns = query_osv(pkg["name"], pkg["version"], pkg["ecosystem"])
        for vuln in vulns:
            severity = "unknown"
            if vuln.get("severity"):
                severity = vuln["severity"][0].get("score", "unknown")
            findings.append({
                "package": pkg["name"],
                "version": pkg["version"],
                "cve_id": vuln.get("id", ""),
                "severity": severity,
                "summary": vuln.get("summary", "")
            })
    
    # Report to scan-service
    resp = requests.post(
        f"{API_URL}/api/v1/agents/report",
        headers={"Authorization": f"Bearer {api_key}"},
        json={
            "agent_id": AGENT_ID,
            "target": platform.node(),
            "packages": packages,
            "findings": findings
        },
        timeout=30
    )
    print(f"Reported {len(findings)} findings. Status: {resp.status_code}")

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: agent.py <api_key>")
        sys.exit(1)
    main(sys.argv[1])
```

---

## 7. Database Schema Chi Tiết

### 7.1 Migrations Strategy

```sql
-- migrations/001_create_scan_tables.sql

-- Extensions
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Scans table
CREATE TABLE scans (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL,
    targets       TEXT[] NOT NULL,
    scan_type     VARCHAR(20) NOT NULL 
                  CHECK (scan_type IN ('full','discovery','web','agent')),
    status        VARCHAR(20) NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending','queued','running','completed','failed','cancelled')),
    priority      INT NOT NULL DEFAULT 5 CHECK (priority BETWEEN 1 AND 10),
    options       JSONB NOT NULL DEFAULT '{}',
    scheduled_for TIMESTAMPTZ,
    started_at    TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ,
    failed_at     TIMESTAMPTZ,
    error_msg     TEXT,
    progress      INT NOT NULL DEFAULT 0 CHECK (progress BETWEEN 0 AND 100),
    finding_count INT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_scans_user_id   ON scans(user_id);
CREATE INDEX idx_scans_status    ON scans(status) WHERE status NOT IN ('completed','failed','cancelled');
CREATE INDEX idx_scans_created   ON scans(created_at DESC);
CREATE INDEX idx_scans_scheduled ON scans(scheduled_for) WHERE scheduled_for IS NOT NULL;

-- Network scan findings
CREATE TABLE scan_findings (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scan_id    UUID NOT NULL REFERENCES scans(id) ON DELETE CASCADE,
    ip_address INET,
    hostname   VARCHAR(255),
    os         TEXT,
    open_ports JSONB DEFAULT '[]',   -- [{port, protocol, state}]
    services   JSONB DEFAULT '[]',   -- [{port, name, product, version}]
    web_tech   JSONB DEFAULT '[]',   -- [{name, version, categories}]
    cve_ids    TEXT[] DEFAULT '{}',
    severity   VARCHAR(10),
    raw_data   JSONB,                -- nmap XML or ZAP JSON
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_scan_findings_scan   ON scan_findings(scan_id);
CREATE INDEX idx_scan_findings_cves   ON scan_findings USING GIN(cve_ids);
CREATE INDEX idx_scan_findings_ip     ON scan_findings(ip_address);

-- ZAP web alerts
CREATE TABLE web_alerts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scan_id     UUID NOT NULL REFERENCES scans(id) ON DELETE CASCADE,
    target_url  TEXT,
    alert_name  VARCHAR(255),
    risk        VARCHAR(20) CHECK (risk IN ('High','Medium','Low','Informational')),
    confidence  VARCHAR(20) CHECK (confidence IN ('High','Medium','Low','False Positive')),
    description TEXT,
    solution    TEXT,
    reference   TEXT,
    evidence    TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_web_alerts_scan ON web_alerts(scan_id);
CREATE INDEX idx_web_alerts_risk ON web_alerts(risk);

-- Discovery hosts (from -sn scan)
CREATE TABLE discovery_hosts (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scan_id    UUID NOT NULL REFERENCES scans(id) ON DELETE CASCADE,
    ip_address INET,
    hostname   VARCHAR(255),
    status     VARCHAR(10) CHECK (status IN ('up','down')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_discovery_scan ON discovery_hosts(scan_id);

-- Agent findings
CREATE TABLE agent_findings (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scan_id     UUID NOT NULL REFERENCES scans(id) ON DELETE CASCADE,
    package     VARCHAR(255),
    version     VARCHAR(100),
    ecosystem   VARCHAR(50),
    cve_id      VARCHAR(30),
    severity    VARCHAR(10),
    summary     TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_agent_findings_scan ON agent_findings(scan_id);
CREATE INDEX idx_agent_findings_cve  ON agent_findings(cve_id);
```

---

## 8. NATS Event Design

### 8.1 Topic Convention

Tất cả NATS events của scan-service đều dùng prefix `scan.`:

| Topic | Direction | Payload | Consumer |
|-------|-----------|---------|----------|
| `scan.scan.created` | Publish | `{scan_id, user_id, type, targets, priority}` | scan-service worker |
| `scan.scan.started` | Publish | `{scan_id, started_at}` | notification-service |
| `scan.scan.progress` | Publish | `{scan_id, progress, finding_count}` | (opt) dashboard |
| `scan.scan.completed` | Publish | `{scan_id, finding_count, duration_ms, scan_type}` | finding-service, asset-service, ai-service |
| `scan.scan.failed` | Publish | `{scan_id, error, scan_type}` | notification-service |
| `schedule.trigger.fired` | Subscribe | `{schedule_id}` | scan-service scheduler |

### 8.2 NATS JetStream Config

```go
// JetStream stream config for reliability
js.AddStream(&nats.StreamConfig{
    Name:      "SCAN_EVENTS",
    Subjects:  []string{"scan.>"},
    Storage:   nats.FileStorage,
    Replicas:  1,
    Retention: nats.LimitsPolicy,
    MaxAge:    7 * 24 * time.Hour,  // 7 days retention
})

// Consumer with retry
js.Subscribe("scan.scan.created", handler,
    nats.ManualAck(),
    nats.MaxDeliver(3),  // retry up to 3 times
    nats.AckWait(5 * time.Minute),
)
```

---

## 9. Configuration

```yaml
# config/config.yaml
server:
  grpc_port: 50058
  http_port: 8058

database:
  host: "${DB_HOST}"
  port: 5432
  name: "scan_service"
  user: "${DB_USER}"
  password: "${DB_PASSWORD}"
  max_open_conns: 25
  max_idle_conns: 5

nats:
  url: "${NATS_URL}"  # nats://nats:4222
  stream: "SCAN_EVENTS"

nmap:
  path: "/usr/bin/nmap"
  default_intensity: 4   # -T4
  default_timeout: 300   # seconds
  
zap:
  api_url: "http://zap:8090"
  default_spider_timeout: 120    # seconds
  default_active_scan_timeout: 600  # seconds

scan:
  max_concurrent_scans: 5       # semaphore limit
  max_targets_per_scan: 256     # CIDR /24 max

auth:
  jwt_public_key_path: "/secrets/jwt_public.pem"
  
metrics:
  enabled: true
  port: 9090

logging:
  level: "info"  # debug|info|warn|error
  format: "json"
```

---

## 10. Security Considerations

### 10.1 Scan Target Authorization

```go
// Chống scan bừa bãi: validate targets trước khi chạy
func validateScanPermission(userRole string, targets []string) error {
    for _, target := range targets {
        // Chỉ admin có thể scan external networks
        if isExternalNetwork(target) && userRole != "admin" {
            return ErrScanExternalNetworkForbidden
        }
        
        // Block private ranges nếu không phải internal scan
        if isLoopback(target) {
            return ErrScanLoopbackForbidden
        }
    }
    return nil
}
```

### 10.2 Rate Limiting cho Scans

```go
// Semaphore pattern: max concurrent scans
var scanSemaphore = make(chan struct{}, maxConcurrentScans)

func (uc *ExecuteScanUseCase) Execute(ctx context.Context, scanID uuid.UUID) error {
    select {
    case scanSemaphore <- struct{}{}:
        defer func() { <-scanSemaphore }()
    case <-time.After(30 * time.Second):
        return ErrScanQueueFull
    }
    // ... execute scan
}
```

### 10.3 Agent API Key Auth

```go
// Agent endpoints sử dụng API key (ovs_ prefix) thay vì JWT
func AgentAuthMiddleware(authSvcClient AuthServiceClient) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            key := extractBearerToken(r)
            if !strings.HasPrefix(key, "ovs_") {
                http.Error(w, "agent:report permission required", 401)
                return
            }
            
            resp, err := authSvcClient.ValidateAPIKey(r.Context(), 
                &authpb.ValidateAPIKeyRequest{Key: key})
            if err != nil || !resp.Valid {
                http.Error(w, "invalid API key", 401)
                return
            }
            
            if !hasPermission(resp.Permissions, "agent:report") {
                http.Error(w, "forbidden", 403)
                return
            }
            
            next.ServeHTTP(w, r)
        })
    }
}
```

---

## 11. Prometheus Metrics

```go
// scan-service/internal/metrics/metrics.go

var (
    ScansCreated = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "scan_created_total"},
        []string{"type"},
    )
    ScansCompleted = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "scan_completed_total"},
        []string{"type"},
    )
    ScansFailed = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "scan_failed_total"},
        []string{"type"},
    )
    ScanDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "scan_duration_seconds",
            Buckets: []float64{30, 60, 120, 300, 600, 1800},
        },
        []string{"type"},
    )
    ScansActive = prometheus.NewGauge(
        prometheus.GaugeOpts{Name: "scan_active_gauge"},
    )
    NmapDuration = prometheus.NewHistogram(
        prometheus.HistogramOpts{
            Name:    "nmap_subprocess_duration_seconds",
            Buckets: []float64{10, 30, 60, 120, 300},
        },
    )
    ZAPDuration = prometheus.NewHistogram(
        prometheus.HistogramOpts{
            Name:    "zap_scan_duration_seconds",
            Buckets: []float64{60, 120, 300, 600, 1200},
        },
    )
    AgentReports = prometheus.NewCounter(
        prometheus.CounterOpts{Name: "agent_reports_received_total"},
    )
)
```

---

## 12. Docker / Deployment

### 12.1 Dockerfile

```dockerfile
# services/scan-service/Dockerfile
FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o scan-service ./cmd/server/

FROM debian:bookworm-slim

# Install nmap và wkhtmltopdf
RUN apt-get update && apt-get install -y \
    nmap \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /app/scan-service .
COPY --from=builder /app/config/config.yaml ./config/

EXPOSE 8058 50058 9090

ENTRYPOINT ["./scan-service"]
```

### 12.2 Docker Compose Service

```yaml
# docker-compose.yml (relevant section)
services:
  scan-service:
    build: ./services/scan-service
    ports:
      - "8058:8058"    # REST API
      - "50058:50058"  # gRPC
      - "9090:9090"    # Metrics
    environment:
      DB_HOST: postgres
      DB_USER: scan_service
      DB_PASSWORD: ${SCAN_SERVICE_DB_PASSWORD}
      NATS_URL: nats://nats:4222
    volumes:
      - /secrets:/secrets:ro  # JWT public key
    depends_on:
      - postgres
      - nats
      - zap              # OWASP ZAP container
    cap_add:
      - NET_ADMIN        # Required for nmap -O (OS detection)
      - NET_RAW
    networks:
      - backend

  zap:
    image: ghcr.io/zaproxy/zaproxy:stable
    command: zap-webswing.sh
    ports:
      - "8090:8090"
    networks:
      - backend
```

---

## 13. Dependency Graph (Inter-service)

```
scan-service
├── DEPENDS ON:
│   ├── auth-service (gRPC: ValidateToken, ValidateAPIKey)    [CR-OVS-003]
│   ├── vulnerability-service (gRPC: GetCVE, GetCVEBatch)    [OSV core]
│   └── NATS JetStream
│
├── PUBLISHES TO:
│   ├── finding-service    (via NATS: scan.scan.completed)    [CR-OVS-002]
│   ├── asset-service      (via NATS: scan.scan.completed)    [CR-OVS-007]
│   ├── ai-service         (via NATS: scan.scan.completed)    [CR-OVS-005]
│   └── notification-service (via NATS: scan.scan.*)
│
└── SUBSCRIBED BY:
    └── product-service (scan_id reference)                   [CR-OVS-004]
```

---

## 14. Implementation Roadmap

### Phase 1 — Core (Sprint 1-2)
- [ ] Domain entities (Scan, Finding, WebAlert, DiscoveryHost)
- [ ] Database migrations + repository implementations
- [ ] CreateScan use case + HTTP handler
- [ ] Basic Nmap scanner (FullScan + parse XML)
- [ ] NATS publisher/consumer setup
- [ ] ExecuteScan use case (Nmap only)

### Phase 2 — Web + Agent (Sprint 3)
- [ ] ZAP scanner integration
- [ ] Agent report ingestion endpoint
- [ ] Python agent script
- [ ] SSE progress streaming
- [ ] CancelScan use case

### Phase 3 — Polish (Sprint 4)
- [ ] Scan authorization (external network check)
- [ ] Rate limiting (semaphore)
- [ ] Prometheus metrics
- [ ] gRPC server
- [ ] Discovery scan (nmap -sn)
- [ ] Integration tests

### Phase 4 — Scheduling (Sprint 5) [CR-OVS-007]
- [ ] ScheduledScan entity + migrations
- [ ] Scheduler goroutine (every 1 minute)
- [ ] Schedule CRUD API endpoints

---

## 15. Acceptance Criteria Mapping

| Criterion | Implementation |
|-----------|---------------|
| `POST /api/v1/scans` type=full → NATS published | `CreateScanUseCase.Execute()` |
| Nmap with `-sV -O --script=vulners` | `NmapScanner.FullScan()` args |
| CVE IDs extracted via regex `CVE-\d{4}-\d{4,}` | `extractCVEIDs()` |
| SSE stream every 2 seconds | `StreamScanProgress()` ticker |
| Status transitions pending→queued→running→completed | `CanTransitionTo()` state machine |
| Invalid transition → error | `transition()` returns `ErrInvalidTransition` |
| `DELETE /api/v1/scans/{id}` → cancel nmap | `activeCmds.Store()` + ctx cancel |
| ZAP: spider + active scan + 600s timeout | `ZAPScanner.ActiveScan()` |
| ZAP alerts by risk: High/Medium/Low/Informational | `getAlerts()` mapping |
| Agent `POST` requires `ovs_xxx` API key | `AgentAuthMiddleware` |
| Discovery scan: list `{ip, hostname, status}` | `runNmapDiscovery()` parse |
| scan.scan.completed → finding-service receives | NATS publish + consumer |
| `finding_count` updated after complete | `scan.FindingCount = len(findings)` |
| `GET /api/v1/agents/download` → Python script | Static handler serving agent.py |
