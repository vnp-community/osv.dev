# CR-OVS-001 — Scan Service: Nmap/ZAP Vulnerability Scanning Engine

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-OVS-001 |
| **Tiêu đề** | Scan Service — Network Vulnerability Scanning (Nmap), Web Application Scanning (OWASP ZAP), Agent-Based Scanning |
| **Nguồn tham chiếu** | `OpenVulnScan/specs/services/05-scan-service.md`, `OpenVulnScan/docs/PRD.md §F-001,F-002,F-003,F-004` |
| **Target Service** | **MỚI**: `scan-service` (port gRPC: 50058) |
| **Ưu tiên** | 🔴 High |
| **Loại** | New Service |
| **Ngày tạo** | 2026-06-14 |
| **Ngày implement** | 2026-06-17 |
| **Trạng thái** | ✅ Implemented |

---

## 0. Implementation Status

> **Trạng thái**: ✅ **IMPLEMENTED** — 2026-06-17

| Task | File | Trạng thái |
|------|------|------------|
| Scan domain entity + state machine | `scan-service/internal/infra/scan_infra/domain/entity/scan.go` | ✅ Done |
| CreateScan use case | `scan-service/internal/infra/scan_infra/usecase/create_scan/create_scan.go` | ✅ Done |
| ExecuteScan use case (Nmap/ZAP/Agent) | `scan-service/internal/infra/scan_infra/usecase/execute_scan/execute_scan.go` | ✅ Done |
| ZAP client | `scan-service/internal/adapters/scanner/zap/zap_client.go` | ✅ Done |
| Scan HTTP handler (SSE stream) | `scan-service/internal/adapters/handler/http/scan_handler.go` | ✅ Done |
| Scan gRPC handler | `scan-service/internal/adapters/handler/grpc/scan_grpc_handler.go` | ✅ Done |
| Scan PostgreSQL repository | `scan-service/internal/adapters/repository/postgres/scan_repo.go` | ✅ Done |
| Parallel scan worker pool | `scan-service/internal/adapters/worker/pool.go` | ✅ Done |
| Scheduled scan cron worker | `scan-service/internal/scheduler/cron_worker.go` | ✅ Done |
| Scan scheduler | `scan-service/internal/scheduler/scheduler.go` | ✅ Done |
| NATS agent publisher | `scan-service/internal/infra/messaging/nats/agent_publisher.go` | ✅ Done |
| Schedule NATS publisher | `scan-service/internal/infra/messaging/nats/schedule/publisher.go` | ✅ Done |
| Schedule PostgreSQL repository | `scan-service/internal/infra/persistence/postgres/schedule/schedule_repo.go` | ✅ Done |
| Dedup engine (SHA-256) | `scan-service/internal/infra/dedup/engine.go` | ✅ Done |

**Chi tiết implementation**:
- **State Machine**: `pending → queued → running → completed/failed/cancelled` với `CanTransitionTo()` validation
- **Nmap Engine**: exec `nmap -sV -O --script=vulners` → parse XML output → extract CVE IDs
- **ZAP Engine**: ZAP REST API client, spider + active scan, alert extraction
- **SSE Streaming**: `http.Flusher`-based real-time progress, ping mỗi 30s, `event: done` khi xong
- **Worker Pool**: semaphore-based parallel execution, max concurrent scans configurable
- **Scheduler**: tick 1 min, `FindDue(now)` query, `UpdateLastRunAt + ComputeNextRun` với `robfig/cron`
- **NATS Events**: publish `scan.scan.completed` → consumed bởi finding-service, ai-service

---

## 1. Tổng quan

OSV là hệ thống **CVE database + search**. OpenVulnScan bổ sung khả năng **active scanning** — thực sự quét mạng và host để phát hiện lỗ hổng. Đây là capability hoàn toàn mới với OSV.

**Khả năng scan của OpenVulnScan:**
- **Nmap Full Scan** (`-sV -O --script=vulners`) → phát hiện CVE trên network hosts
- **OWASP ZAP** → quét lỗ hổng web (XSS, SQLi, CSRF...)
- **Discovery Scan** (`nmap -sn`) → khám phá hosts đang hoạt động trong subnet
- **Agent-Based** → agent Python trên remote hosts báo cáo installed packages + CVEs

---

## 2. Gap Analysis

| Feature | OSV | OpenVulnScan |
|---------|-----|-------------|
| Nmap network scan | ❌ | ✅ `-sV -O --script=vulners` |
| OWASP ZAP web scan | ❌ | ✅ Spider + Active Scan |
| Host discovery scan | ❌ | ✅ `-sn -PE` |
| Agent-based scan | ❌ | ✅ Python agent |
| Scan job lifecycle | ❌ | ✅ pending→queued→running→completed |
| Scan scheduling | ❌ | ✅ NATS-triggered |
| Scan progress (SSE) | ❌ | ✅ Server-Sent Events |
| Scan cancellation | ❌ | ✅ |
| CVE matching per host | ❌ | ✅ via vulnerability-service |
| Web alerts (ZAP) | ❌ | ✅ XSS/SQLi/CSRF detection |

---

## 3. Domain Model

### 3.1 Core Entities

```go
// scan-service/internal/domain/entity/scan.go

type ScanType string
const (
    ScanTypeFull      ScanType = "full"      // nmap -sV -O --script=vulners
    ScanTypeDiscovery ScanType = "discovery" // nmap -sn (host discovery)
    ScanTypeWeb       ScanType = "web"       // OWASP ZAP active scan
    ScanTypeAgent     ScanType = "agent"     // triggered by agent report
)

type ScanStatus string
const (
    ScanStatusPending   ScanStatus = "pending"
    ScanStatusQueued    ScanStatus = "queued"
    ScanStatusRunning   ScanStatus = "running"
    ScanStatusCompleted ScanStatus = "completed"
    ScanStatusFailed    ScanStatus = "failed"
    ScanStatusCancelled ScanStatus = "cancelled"
)

// State Machine: pending → queued → running → completed|failed
//                ← cancelled (from queued or running)
func (s ScanStatus) CanTransitionTo(next ScanStatus) bool {
    transitions := map[ScanStatus][]ScanStatus{
        ScanStatusPending:   {ScanStatusQueued, ScanStatusCancelled},
        ScanStatusQueued:    {ScanStatusRunning, ScanStatusCancelled},
        ScanStatusRunning:   {ScanStatusCompleted, ScanStatusFailed, ScanStatusCancelled},
    }
    for _, allowed := range transitions[s] {
        if allowed == next { return true }
    }
    return false
}

type ScanOptions struct {
    Ports     string `json:"ports,omitempty"`     // "1-1024,8080,8443"
    Timeout   int    `json:"timeout,omitempty"`   // seconds (default: 300)
    Intensity int    `json:"intensity,omitempty"` // nmap -T1..-T5 (default: 4)
    MaxDepth  int    `json:"max_depth,omitempty"` // web crawl depth (default: 5)
    ZAPConfig struct {
        SpiderTimeout     int `json:"spider_timeout,omitempty"`    // seconds
        ActiveScanTimeout int `json:"active_scan_timeout,omitempty"` // default 600s
    } `json:"zap_config,omitempty"`
}

type Scan struct {
    ID           uuid.UUID
    UserID       uuid.UUID
    Targets      []string    // IPs, CIDRs, hostnames, URLs
    ScanType     ScanType
    Status       ScanStatus
    Priority     int         // 1-10, higher = more urgent
    Options      ScanOptions
    ScheduledFor *time.Time
    StartedAt    *time.Time
    CompletedAt  *time.Time
    FailedAt     *time.Time
    ErrorMsg     string
    Progress     int        // 0-100 percentage
    FindingCount int
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

func (s *Scan) IsTerminal() bool {
    return s.Status == ScanStatusCompleted ||
        s.Status == ScanStatusFailed ||
        s.Status == ScanStatusCancelled
}
```

### 3.2 Scan Finding (Network)

```go
// scan-service/internal/domain/entity/finding.go

type Finding struct {
    ID        uuid.UUID
    ScanID    uuid.UUID
    IPAddress string
    Hostname  string
    OS        string
    OpenPorts []Port
    Services  []Service
    WebTech   []WebTechnology
    CVEIDs    []string         // CVE IDs detected from nmap vulners script
    Severity  Severity
    RawData   json.RawMessage  // nmap XML or ZAP JSON raw output
    CreatedAt time.Time
}

type Port struct {
    Port     int    `json:"port"`
    Protocol string `json:"protocol"` // tcp|udp
    State    string `json:"state"`    // open|closed|filtered
}

type Service struct {
    Port    int    `json:"port"`
    Name    string `json:"name"`      // ssh, http, https, mysql...
    Product string `json:"product"`   // OpenSSH, Apache httpd...
    Version string `json:"version"`   // 8.2p1, 2.4.41...
}

type WebTechnology struct {
    Name       string   `json:"name"`
    Version    string   `json:"version,omitempty"`
    Categories []string `json:"categories,omitempty"` // CMS, JavaScript, Web Framework...
}

// WebAlert — OWASP ZAP finding
type WebAlert struct {
    ID          uuid.UUID
    ScanID      uuid.UUID
    TargetURL   string
    AlertName   string
    Risk        string    // High|Medium|Low|Informational
    Confidence  string    // High|Medium|Low|False Positive
    Description string
    Solution    string
    Reference   string
    Evidence    string
    CreatedAt   time.Time
}

// DiscoveryHost — from host discovery scan
type DiscoveryHost struct {
    ID        uuid.UUID
    ScanID    uuid.UUID
    IPAddress string
    Hostname  string
    Status    string // up|down
    CreatedAt time.Time
}
```

---

## 4. Use Cases

### 4.1 CreateScan

```go
// scan-service/internal/usecase/create_scan/usecase.go

type CreateScanInput struct {
    UserID       uuid.UUID
    Targets      []string    // validate: non-empty, valid IPs/CIDRs/URLs
    Type         ScanType
    Options      ScanOptions
    Priority     int        // 1-10
    ScheduledFor *time.Time
}

func (uc *CreateScanUseCase) Execute(ctx context.Context, in CreateScanInput) (*entity.Scan, error) {
    // 1. Validate targets
    for _, target := range in.Targets {
        if err := validateTarget(target); err != nil {
            return nil, fmt.Errorf("invalid target %q: %w", target, err)
        }
    }

    // 2. Create scan entity
    scan := &entity.Scan{
        ID:           uuid.New(),
        UserID:       in.UserID,
        Targets:      in.Targets,
        ScanType:     in.Type,
        Status:       entity.ScanStatusPending,
        Priority:     in.Priority,
        Options:      in.Options,
        ScheduledFor: in.ScheduledFor,
        CreatedAt:    time.Now().UTC(),
    }
    if scan.Priority < 1 { scan.Priority = 5 }

    // 3. Persist to DB
    if err := uc.scanRepo.Save(ctx, scan); err != nil {
        return nil, err
    }

    // 4. Publish NATS (triggers worker)
    uc.eventBus.Publish(ctx, "scan.scan.created", &ScanCreatedEvent{
        ScanID:   scan.ID,
        UserID:   scan.UserID,
        Type:     string(scan.ScanType),
        Targets:  scan.Targets,
        Priority: scan.Priority,
    })

    return scan, nil
}

// Target validation
func validateTarget(target string) error {
    // Accept: IP (192.168.1.1), CIDR (192.168.1.0/24), hostname (example.com), URL (https://example.com)
    if net.ParseIP(target) != nil { return nil }
    if _, _, err := net.ParseCIDR(target); err == nil { return nil }
    if _, err := url.ParseRequestURI(target); err == nil { return nil }
    if isValidHostname(target) { return nil }
    return fmt.Errorf("must be a valid IP, CIDR, hostname, or URL")
}
```

### 4.2 ExecuteScan (Worker)

```go
// scan-service/internal/usecase/execute_scan/usecase.go
// Triggered by NATS consumer: scan.scan.created

func (uc *ExecuteScanUseCase) Execute(ctx context.Context, scanID uuid.UUID) error {
    // 1. Load scan
    scan, err := uc.scanRepo.FindByID(ctx, scanID)
    if err != nil { return err }

    // 2. Transition: pending → queued → running
    if err := uc.transition(ctx, scan, entity.ScanStatusQueued); err != nil { return err }
    if err := uc.transition(ctx, scan, entity.ScanStatusRunning); err != nil { return err }

    // 3. Execute based on type
    var findings []*entity.Finding
    var webAlerts []*entity.WebAlert
    var discoveryHosts []*entity.DiscoveryHost

    switch scan.ScanType {
    case entity.ScanTypeFull:
        findings, err = uc.runNmapFullScan(ctx, scan)
    case entity.ScanTypeDiscovery:
        discoveryHosts, err = uc.runNmapDiscovery(ctx, scan)
    case entity.ScanTypeWeb:
        webAlerts, err = uc.runZAPScan(ctx, scan)
    case entity.ScanTypeAgent:
        // Results already stored by agent report ingestion
    }

    if err != nil {
        uc.transition(ctx, scan, entity.ScanStatusFailed)
        scan.ErrorMsg = err.Error()
        uc.scanRepo.Update(ctx, scan)
        uc.eventBus.Publish(ctx, "scan.scan.failed", &ScanFailedEvent{
            ScanID: scanID, Error: err.Error(),
        })
        return err
    }

    // 4. Store results
    uc.findingRepo.SaveBatch(ctx, findings)
    uc.webAlertRepo.SaveBatch(ctx, webAlerts)
    uc.discoveryRepo.SaveBatch(ctx, discoveryHosts)

    // 5. Complete
    scan.FindingCount = len(findings) + len(webAlerts)
    scan.Progress = 100
    uc.transition(ctx, scan, entity.ScanStatusCompleted)

    // 6. Publish completed event → notification-service, finding-service
    uc.eventBus.Publish(ctx, "scan.scan.completed", &ScanCompletedEvent{
        ScanID:       scanID,
        FindingCount: scan.FindingCount,
    })

    return nil
}
```

### 4.3 Nmap Scanner

```go
// scan-service/internal/scanner/nmap/scanner.go
// Orchestrates nmap subprocess + parses XML output

type NmapScanner struct {
    nmapPath string  // "/usr/bin/nmap" or from PATH
    logger   zerolog.Logger
}

// FullScan — Nmap with vulners script (CVE detection)
func (s *NmapScanner) FullScan(ctx context.Context, targets []string, opts entity.ScanOptions) ([]*entity.Finding, error) {
    args := []string{
        "-sV",          // service version detection
        "-O",           // OS detection
        "--script=vulners", // CVE detection via vulners NSE
        "-oX", "-",     // XML output to stdout
        "--open",       // only show open ports
        "-T" + strconv.Itoa(opts.Intensity), // timing template (default -T4)
    }

    if opts.Ports != "" {
        args = append(args, "-p", opts.Ports)
    }
    args = append(args, targets...)

    // Execute nmap subprocess
    cmd := exec.CommandContext(ctx, s.nmapPath, args...)
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    if err := cmd.Run(); err != nil {
        // nmap exits 1 if some hosts down — not a fatal error
        if cmd.ProcessState.ExitCode() > 1 {
            return nil, fmt.Errorf("nmap failed: %s", stderr.String())
        }
    }

    return s.parseXMLOutput(ctx, stdout.Bytes())
}

// parseXMLOutput — parses nmap -oX output
// Extracts: hosts, OS, ports, services, CVE IDs (from vulners script output)
func (s *NmapScanner) parseXMLOutput(ctx context.Context, xmlData []byte) ([]*entity.Finding, error) {
    var nmapRun struct {
        XMLName xml.Name `xml:"nmaprun"`
        Hosts   []struct {
            Addresses []struct {
                Addr     string `xml:"addr,attr"`
                AddrType string `xml:"addrtype,attr"` // ipv4|mac|ipv6
            } `xml:"address"`
            Hostnames []struct {
                Name string `xml:"name,attr"`
            } `xml:"hostnames>hostname"`
            OS struct {
                Matches []struct {
                    Name     string `xml:"name,attr"`
                    Accuracy int    `xml:"accuracy,attr"`
                } `xml:"osmatch"`
            } `xml:"os"`
            Ports struct {
                Ports []struct {
                    Protocol string `xml:"protocol,attr"`
                    PortID    int    `xml:"portid,attr"`
                    State     struct {
                        State string `xml:"state,attr"`
                    } `xml:"state"`
                    Service struct {
                        Name    string `xml:"name,attr"`
                        Product string `xml:"product,attr"`
                        Version string `xml:"version,attr"`
                    } `xml:"service"`
                    Script struct {
                        ID     string `xml:"id,attr"`
                        Output string `xml:"output,attr"` // Contains CVE IDs from vulners
                    } `xml:"script"`
                } `xml:"port"`
            } `xml:"ports"`
        } `xml:"host"`
    }

    if err := xml.Unmarshal(xmlData, &nmapRun); err != nil {
        return nil, fmt.Errorf("parse nmap XML: %w", err)
    }

    var findings []*entity.Finding
    for _, host := range nmapRun.Hosts {
        finding := &entity.Finding{
            ID:        uuid.New(),
            RawData:   xmlData,
            CreatedAt: time.Now().UTC(),
        }

        // Extract primary IP
        for _, addr := range host.Addresses {
            if addr.AddrType == "ipv4" {
                finding.IPAddress = addr.Addr
            }
        }

        // Hostname
        if len(host.Hostnames) > 0 {
            finding.Hostname = host.Hostnames[0].Name
        }

        // OS
        if len(host.OS.Matches) > 0 {
            finding.OS = host.OS.Matches[0].Name
        }

        // Ports and CVEs
        cveSet := make(map[string]bool)
        for _, port := range host.Ports.Ports {
            if port.State.State != "open" { continue }

            finding.OpenPorts = append(finding.OpenPorts, entity.Port{
                Port:     port.PortID,
                Protocol: port.Protocol,
                State:    port.State.State,
            })

            finding.Services = append(finding.Services, entity.Service{
                Port:    port.PortID,
                Name:    port.Service.Name,
                Product: port.Service.Product,
                Version: port.Service.Version,
            })

            // Extract CVE IDs from vulners script output
            // Format: "CVE-2021-44228\t9.8\thttps://nvd.nist.gov/..."
            if port.Script.ID == "vulners" {
                for _, cveID := range extractCVEIDs(port.Script.Output) {
                    cveSet[cveID] = true
                }
            }
        }

        for cveID := range cveSet {
            finding.CVEIDs = append(finding.CVEIDs, cveID)
        }

        // Derive severity from CVE data (via vulnerability-service)
        finding.Severity = entity.SeverityFromCVSS(getMaxCVSSScore(finding.CVEIDs))

        findings = append(findings, finding)
    }
    return findings, nil
}

// extractCVEIDs — extract CVE IDs from nmap vulners script output
var cveRegex = regexp.MustCompile(`CVE-\d{4}-\d{4,}`)
func extractCVEIDs(output string) []string {
    matches := cveRegex.FindAllString(output, -1)
    seen := make(map[string]bool)
    var unique []string
    for _, m := range matches {
        if !seen[m] { unique = append(unique, m); seen[m] = true }
    }
    return unique
}
```

### 4.4 OWASP ZAP Scanner

```go
// scan-service/internal/scanner/zap/scanner.go
// Interfaces with ZAP API (http://zap:8090)

type ZAPScanner struct {
    zapAPIURL string  // "http://zap:8090"
    client    *http.Client
    logger    zerolog.Logger
}

// ActiveScan — spider + active scan via ZAP API
func (s *ZAPScanner) ActiveScan(ctx context.Context, targetURL string, opts entity.ScanOptions) ([]*entity.WebAlert, error) {
    // 1. Start spider
    spiderID, err := s.startSpider(ctx, targetURL, opts.MaxDepth)
    if err != nil { return nil, fmt.Errorf("zap spider start: %w", err) }

    // 2. Wait for spider to complete (poll every 5s)
    if err := s.waitForProgress(ctx, "spider", spiderID,
        opts.ZAPConfig.SpiderTimeout); err != nil {
        return nil, fmt.Errorf("zap spider timeout: %w", err)
    }

    // 3. Start active scan
    scanID, err := s.startActiveScan(ctx, targetURL)
    if err != nil { return nil, fmt.Errorf("zap active scan start: %w", err) }

    // 4. Wait for active scan
    timeout := opts.ZAPConfig.ActiveScanTimeout
    if timeout <= 0 { timeout = 600 }
    if err := s.waitForProgress(ctx, "ascan", scanID, timeout); err != nil {
        return nil, fmt.Errorf("zap scan timeout: %w", err)
    }

    // 5. Get alerts
    alerts, err := s.getAlerts(ctx, targetURL)
    if err != nil { return nil, err }

    return alerts, nil
}

func (s *ZAPScanner) startSpider(ctx context.Context, url string, maxDepth int) (string, error) {
    resp, err := s.client.Get(fmt.Sprintf(
        "%s/JSON/spider/action/scan/?url=%s&maxDepth=%d",
        s.zapAPIURL, url, maxDepth))
    // ...parse scan ID from response
    return scanID, err
}

func (s *ZAPScanner) getAlerts(ctx context.Context, baseURL string) ([]*entity.WebAlert, error) {
    resp, err := s.client.Get(fmt.Sprintf(
        "%s/JSON/core/view/alerts/?baseurl=%s", s.zapAPIURL, baseURL))
    // Parse ZAP JSON alerts → entity.WebAlert
    return alerts, err
}
```

### 4.5 Agent Report Ingestion

```go
// scan-service/internal/usecase/agent/process_report.go
// POST /api/v1/agents/report (API key auth: ovs_xxx)

type AgentReportInput struct {
    AgentID  string
    Target   string         // hostname of the reporting machine
    Packages []AgentPackage // installed packages
    Findings []AgentFinding // CVE findings from OSV.dev API
}

type AgentPackage struct {
    Name    string
    Version string
    Ecosystem string // debian, rpm, npm, pip...
}

type AgentFinding struct {
    Package string
    Version string
    CVEID   string
    Severity string
    Summary  string
}

func (uc *ProcessAgentReport) Execute(ctx context.Context, in AgentReportInput) error {
    // 1. Create agent scan (type=agent, target=in.Target)
    scan, _ := uc.createScan.Execute(ctx, CreateScanInput{
        Targets:  []string{in.Target},
        Type:     entity.ScanTypeAgent,
        Priority: 5,
    })

    // 2. Store agent findings
    findings := make([]*entity.AgentFinding, 0, len(in.Findings))
    for _, f := range in.Findings {
        findings = append(findings, &entity.AgentFinding{
            ScanID:   scan.ID,
            Package:  f.Package,
            Version:  f.Version,
            CVEID:    f.CVEID,
            Severity: f.Severity,
            Summary:  f.Summary,
        })
    }
    uc.agentFindingRepo.SaveBatch(ctx, findings)

    // 3. Complete the scan
    scan.FindingCount = len(findings)
    scan.Status = entity.ScanStatusCompleted
    uc.scanRepo.Update(ctx, scan)

    // 4. Trigger CVE enrichment via NATS
    uc.eventBus.Publish(ctx, "scan.scan.completed", &ScanCompletedEvent{
        ScanID:       scan.ID,
        FindingCount: len(findings),
        ScanType:     "agent",
    })

    return nil
}
```

---

## 5. Scan Progress (Server-Sent Events)

```go
// GET /api/v1/scans/{id}/stream
// Streams real-time scan progress as SSE

func (h *Handler) StreamScanProgress(w http.ResponseWriter, r *http.Request) {
    scanID := chi.URLParam(r, "id")

    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")

    flusher := w.(http.Flusher)
    ticker := time.NewTicker(2 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-r.Context().Done():
            return
        case <-ticker.C:
            scan, err := h.scanRepo.FindByID(r.Context(), scanID)
            if err != nil { return }

            data, _ := json.Marshal(map[string]interface{}{
                "scan_id":  scan.ID,
                "status":   scan.Status,
                "progress": scan.Progress,
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

## 6. Database Schema

```sql
-- scan-service migrations/

CREATE TABLE scans (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL,
    targets       TEXT[] NOT NULL,
    scan_type     VARCHAR(20) NOT NULL CHECK (scan_type IN ('full','discovery','web','agent')),
    status        VARCHAR(20) NOT NULL DEFAULT 'pending',
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
CREATE INDEX idx_scans_status    ON scans(status);
CREATE INDEX idx_scans_created   ON scans(created_at DESC);

CREATE TABLE scan_findings (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scan_id    UUID NOT NULL REFERENCES scans(id) ON DELETE CASCADE,
    ip_address INET,
    hostname   VARCHAR(255),
    os         TEXT,
    open_ports JSONB DEFAULT '[]',
    services   JSONB DEFAULT '[]',
    web_tech   JSONB DEFAULT '[]',
    cve_ids    TEXT[] DEFAULT '{}',
    severity   VARCHAR(10),
    raw_data   JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_scan_findings_scan   ON scan_findings(scan_id);
CREATE INDEX idx_scan_findings_cves   ON scan_findings USING GIN(cve_ids);

CREATE TABLE web_alerts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scan_id     UUID NOT NULL REFERENCES scans(id) ON DELETE CASCADE,
    target_url  TEXT,
    alert_name  VARCHAR(255),
    risk        VARCHAR(20) CHECK (risk IN ('High','Medium','Low','Informational')),
    confidence  VARCHAR(20),
    description TEXT,
    solution    TEXT,
    reference   TEXT,
    evidence    TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE discovery_hosts (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scan_id    UUID NOT NULL REFERENCES scans(id) ON DELETE CASCADE,
    ip_address INET,
    hostname   VARCHAR(255),
    status     VARCHAR(10) CHECK (status IN ('up','down')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE agent_findings (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scan_id     UUID NOT NULL REFERENCES scans(id) ON DELETE CASCADE,
    package     VARCHAR(255),
    version     VARCHAR(100),
    cve_id      VARCHAR(30),
    severity    VARCHAR(10),
    summary     TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_agent_findings_cve ON agent_findings(cve_id);
```

---

## 7. API Routes

```
POST   /api/v1/scans                  → Create new scan (auth required)
GET    /api/v1/scans                  → List scans (paginated, with filters)
GET    /api/v1/scans/{id}             → Get scan details + findings summary
DELETE /api/v1/scans/{id}             → Cancel scan
GET    /api/v1/scans/{id}/findings    → Get scan findings (network)
GET    /api/v1/scans/{id}/alerts      → Get web alerts (ZAP)
GET    /api/v1/scans/{id}/hosts       → Get discovery results
GET    /api/v1/scans/{id}/stream      → SSE progress stream
POST   /api/v1/agents/report          → Agent report ingestion (API key)
GET    /api/v1/agents/reports         → List agent reports (auth)
GET    /api/v1/agents/download        → Download agent script
```

---

## 8. NATS Events

**Published:**
```
scan.scan.created     → {scan_id, user_id, type, targets, priority}
scan.scan.completed   → {scan_id, finding_count, duration_ms, scan_type}
scan.scan.failed      → {scan_id, error}
```

**Subscribed:**
```
schedule.trigger.fired → create scheduled scan
```

---

## 9. Metrics

```
# scan-service Prometheus metrics
scan_created_total{type}
scan_completed_total{type}
scan_failed_total{type}
scan_duration_seconds{type}       // Histogram
scan_active_gauge                  // Currently running scans
scan_findings_per_scan{severity}  // Histogram
nmap_subprocess_duration_seconds  // Histogram
zap_scan_duration_seconds         // Histogram
agent_reports_received_total
```

---

## 10. Acceptance Criteria

- [ ] `POST /api/v1/scans` với `type=full` → tạo scan, publish NATS `scan.scan.created`
- [ ] Nmap full scan: targets được resolve, nmap subprocess thực thi với `-sV -O --script=vulners`
- [ ] CVE IDs được extract từ nmap vulners script output (regex `CVE-\d{4}-\d{4,}`)
- [ ] `GET /api/v1/scans/{id}/stream` → SSE stream status mỗi 2 giây
- [ ] Scan status transitions hợp lệ: pending → queued → running → completed
- [ ] Invalid transition (completed → running) → error
- [ ] `DELETE /api/v1/scans/{id}` → cancel running scan (terminate nmap process)
- [ ] ZAP scan: spider first, then active scan, timeout 600s mặc định
- [ ] ZAP alerts được phân loại theo risk: High/Medium/Low/Informational
- [ ] Agent report `POST /api/v1/agents/report` → API key `ovs_xxx` required
- [ ] Discovery scan: parse nmap `-sn` output → list of `{ip, hostname, status}`
- [ ] Khi scan completed → NATS `scan.scan.completed` → finding-service nhận được
- [ ] `finding_count` được update đúng sau scan complete
- [ ] `GET /api/v1/agents/download` → trả về agent Python script
