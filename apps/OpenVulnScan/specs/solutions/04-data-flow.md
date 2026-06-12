# Data Flow — Các luồng dữ liệu chính

## 1. Luồng: Tạo và thực thi Scan

```
┌─────────────────────────────────────────────────────────────────┐
│                     SCAN CREATION FLOW                          │
│                                                                 │
│  Client                                                         │
│    │                                                            │
│    │  POST /api/v1/scans                                        │
│    │  {targets: ["192.168.1.0/24"], scan_type: "full"}          │
│    ▼                                                            │
│  [HTTP Handler]                                                 │
│    │                                                            │
│    │  Direct Call                                               │
│    ▼                                                            │
│  [scan-service/usecase/create_scan.Execute()]                   │
│    │                                                            │
│    ├──► PostgreSQL: INSERT INTO scans (status=pending)          │
│    │                                                            │
│    └──► NATS Publish: "scan.created" {scan_id, targets, type}  │
│              │                                                  │
│    ◄─────────┘                                                  │
│    │  Response: {id: "uuid", status: "pending"}                 │
│    ▼                                                            │
│  Client receives response immediately (async scan)             │
│                                                                 │
│  ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─      │
│  BACKGROUND: ScanWorker goroutine                               │
│                                                                 │
│  [NATS Subscribe: "scan.created"]                               │
│    │                                                            │
│    ▼                                                            │
│  [scan-service/usecase/execute_scan.Execute()]                  │
│    │                                                            │
│    ├──► PostgreSQL: UPDATE scans SET status=running            │
│    │                                                            │
│    ├── scan_type == "full" ──► [nmap adapter] → nmap -sV ...   │
│    ├── scan_type == "web"  ──► [zap adapter]  → ZAP active     │
│    └── scan_type == "discovery" ──► [nmap -sn adapter]         │
│              │                                                  │
│    ┌─────────┘                                                  │
│    │  raw results (JSON)                                        │
│    ▼                                                            │
│  [parse results]                                                │
│    │                                                            │
│    ├──► PostgreSQL: UPDATE scans SET status=completed, raw_data │
│    │                                                            │
│    └──► NATS Publish: "scan.completed" {scan_id, finding_count} │
└─────────────────────────────────────────────────────────────────┘
```

---

## 2. Luồng: Xử lý Finding sau khi Scan hoàn thành

```
┌─────────────────────────────────────────────────────────────────┐
│                    FINDING PROCESSING FLOW                      │
│                                                                 │
│  NATS: "scan.completed" event                                   │
│    │                                                            │
│    ▼                                                            │
│  [FindingWorker goroutine]                                      │
│    │                                                            │
│    ├──► Fetch raw_data from PostgreSQL (scans table)            │
│    │                                                            │
│    ├──► For each host found:                                    │
│    │      [finding-service/domain/finding state machine]        │
│    │      Create Finding entity (state: OPEN)                   │
│    │                                                            │
│    ├──► For each CVE in finding:                                │
│    │      [vulnerability-service/usecase] Lookup CVE detail     │
│    │      Enrich: severity, CVSS, remediation                   │
│    │      [shared/pkg/severity] Calculate severity              │
│    │                                                            │
│    ├──► PostgreSQL: INSERT INTO findings                        │
│    │                                                            │
│    ├──► Upsert Asset (from finding IP/hostname)                 │
│    │      [product-service/usecase] CreateOrUpdateAsset()       │
│    │      Associate vulnerabilities with asset                  │
│    │                                                            │
│    └──► NATS Publish: "notification.send"                       │
│              {type: "scan_complete", scan_id, severity_summary} │
└─────────────────────────────────────────────────────────────────┘
```

---

## 3. Luồng: Agent Report

```
┌─────────────────────────────────────────────────────────────────┐
│                     AGENT REPORT FLOW                           │
│                                                                 │
│  Agent (Python script on remote host)                           │
│    │                                                            │
│    │  POST /agent/report                                        │
│    │  {hostname, os_info, packages: [{name, version},...]}      │
│    ▼                                                            │
│  [HTTP Handler]                                                 │
│    │                                                            │
│    ├──► PostgreSQL: INSERT INTO agent_reports                   │
│    │                                                            │
│    └──► NATS Publish: "agent.report.in" {report_id}            │
│              │                                                  │
│    ◄─────────┘                                                  │
│    │  Response: 200 OK                                          │
│                                                                 │
│  ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─      │
│  BACKGROUND: AgentWorker goroutine                              │
│                                                                 │
│  [NATS Subscribe: "agent.report.in"]                            │
│    │                                                            │
│    ▼                                                            │
│  [ingestion-service usecase]                                    │
│    │                                                            │
│    ├──► For each package:                                       │
│    │      Call OSV API: POST https://api.osv.dev/v1/query       │
│    │      {package: {name, version}}                            │
│    │      → Get list of CVEs affecting this version             │
│    │                                                            │
│    ├──► Enrich CVEs with severity from CVSS                     │
│    │      [shared/pkg/severity]                                 │
│    │                                                            │
│    ├──► PostgreSQL: INSERT INTO packages, package_cves          │
│    │                                                            │
│    └──► Upsert Asset with vulnerability data                    │
│              [product-service/usecase]                          │
└─────────────────────────────────────────────────────────────────┘
```

---

## 4. Luồng: Scheduled Scan

```
┌─────────────────────────────────────────────────────────────────┐
│                   SCHEDULED SCAN FLOW                           │
│                                                                 │
│  Client                                                         │
│    │  POST /api/v1/scans/schedule                               │
│    │  {targets, scan_type, cron: "0 2 * * *"}                   │
│    ▼                                                            │
│  [HTTP Handler]                                                 │
│    │                                                            │
│    └──► [scan-service/usecase/schedule]                         │
│              Register cron job in PostgreSQL                    │
│                                                                 │
│  ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─      │
│  BACKGROUND: SchedulerWorker (cron goroutine)                   │
│                                                                 │
│  [scan-service/internal/scheduler]                              │
│    │  On cron trigger:                                          │
│    ▼                                                            │
│  Create new Scan record                                         │
│    └──► NATS Publish: "scan.created"                            │
│         → Same flow as manual scan                              │
└─────────────────────────────────────────────────────────────────┘
```

---

## 5. Luồng: Authentication

```
┌─────────────────────────────────────────────────────────────────┐
│                    AUTH FLOW                                    │
│                                                                 │
│  Local Login:                                                   │
│    POST /api/v1/auth/login {email, password}                    │
│    → [auth-service/usecase] bcrypt verify                       │
│    → Generate JWT (access_token + refresh_token)                │
│    → Set HttpOnly cookie                                        │
│    → Response: {access_token, expires_in}                       │
│                                                                 │
│  Google OAuth:                                                  │
│    GET /api/v1/auth/google                                      │
│    → Redirect to Google OAuth2 consent                          │
│    GET /api/v1/auth/google/callback?code=...                    │
│    → [auth-service/provider/google] exchange code for token     │
│    → Lookup/create user in PostgreSQL                           │
│    → Generate JWT, set cookie                                   │
│                                                                 │
│  Request authentication (middleware):                           │
│    → Extract JWT from Authorization header or cookie            │
│    → [auth-service/domain] Validate JWT                         │
│    → [Redis] Check token not revoked                            │
│    → Inject user into request context                           │
└─────────────────────────────────────────────────────────────────┘
```

---

## 6. Luồng: PDF Report

```
┌─────────────────────────────────────────────────────────────────┐
│                    PDF REPORT FLOW                              │
│                                                                 │
│  Client                                                         │
│    │  GET /api/v1/scans/{id}/pdf                                │
│    ▼                                                            │
│  [HTTP Handler]                                                 │
│    │                                                            │
│    ├──► Fetch Scan from PostgreSQL                              │
│    ├──► Fetch Findings (with CVE details)                       │
│    ├──► Fetch Asset info                                        │
│    │                                                            │
│    └──► [report-service/internal] GeneratePDF()                 │
│              Render HTML template → wkhtmltopdf / go-pdf        │
│              Return PDF bytes                                   │
│                                                                 │
│    ◄──── FileResponse (Content-Type: application/pdf)           │
└─────────────────────────────────────────────────────────────────┘
```

---

## 7. NATS Event Schema

### `scan.created`
```json
{
  "scan_id": "uuid",
  "user_id": "uuid",
  "targets": ["192.168.1.1", "example.com"],
  "scan_type": "full|discovery|web|agent",
  "options": {
    "ports": "1-1024",
    "intensity": 3,
    "timeout": 300
  },
  "priority": 5,
  "created_at": "2026-01-01T00:00:00Z"
}
```

### `scan.completed`
```json
{
  "scan_id": "uuid",
  "status": "completed|failed",
  "finding_count": 42,
  "duration_seconds": 120,
  "error_msg": "",
  "completed_at": "2026-01-01T00:02:00Z"
}
```

### `agent.report.in`
```json
{
  "report_id": "int",
  "hostname": "server-01",
  "os_info": "Ubuntu 22.04",
  "packages": [
    {"name": "openssl", "version": "1.1.1"},
    {"name": "curl", "version": "7.68.0"}
  ],
  "reported_at": "2026-01-01T00:00:00Z"
}
```

### `notification.send`
```json
{
  "type": "scan_complete|new_critical_cve|scan_failed",
  "user_id": "uuid",
  "scan_id": "uuid",
  "severity": "critical|high|medium|low",
  "message": "Scan completed with 3 critical findings",
  "channels": ["email", "webhook"]
}
```

### `syslog.event`
```json
{
  "facility": 1,
  "severity": 3,
  "hostname": "openvulnscan-server",
  "app_name": "openvulnscan",
  "message": "{\"scan_id\":\"uuid\",\"targets\":[\"192.168.1.1\"],\"status\":\"completed\"}"
}
```
