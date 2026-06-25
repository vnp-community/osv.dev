# F13 — Active Vulnerability Scanning (Nmap / OWASP ZAP)

**Status:** 🔵 Planned v3.0  
**CR References:** CR-OVS-001, CR-OVS-007  
**Services:** `scan-service-ovs` (HTTP: 8058, gRPC: 50058)  
**UI Routes:** `/scans`, `/scans/new`, `/scans/history`, `/scans/:id`, `/scans/:id/results/nmap`, `/scans/:id/results/zap`  
**UI Components:** `ScanDashboard`, `ScanWizard`, `RunningScan`, `ScanHistory`, `NmapResults`, `ZAPResults`

---

## 1. Mô tả

Module Active Scanning cho phép khởi động scan chủ động trên infrastructure và web applications trực tiếp từ OSV Platform — không cần tool bên ngoài. Hỗ trợ Nmap (network/host scanning), OWASP ZAP (web DAST), và agent-based scanning với real-time progress qua SSE.

> **⚠️ Quan trọng:** Chỉ quét targets được authorized. Không hỗ trợ internet-wide scanning.

---

## 2. Scan Types

### 2.1 Nmap Full Scan

**Command thực thi:**
```bash
nmap -sV -O --script=vulners -oX - --open -T4 {targets}
```

**Flags:**
- `-sV` — Service version detection
- `-O` — OS fingerprinting
- `--script=vulners` — CVE detection via Vulners NSE script
- `-oX -` — XML output to stdout
- `--open` — Chỉ show open ports
- `-T4` — Timing template (fast)

**Output parsed:**
- Open ports + service versions
- OS detection
- CVE IDs extracted từ vulners script output (regex)
- Host uptime

**Targets:** IP address, CIDR range, hostname

### 2.2 OWASP ZAP Active Scan

**Pipeline:**
```
Spider (crawl target URL)
    → Passive scan (analyze responses)
    → Active Scan (inject payloads)
    → Get alerts
```

**ZAP Detections:**
- XSS — Cross-Site Scripting
- SQLi — SQL Injection
- CSRF — Cross-Site Request Forgery
- Path traversal
- Remote File Inclusion
- XML injection
- Server-side includes
- ...40+ vulnerability types

**Alert Risk Levels:**
| Level | Mô tả |
|-------|-------|
| High | Critical web vulnerabilities |
| Medium | Moderate impact |
| Low | Low risk but worth noting |
| Informational | FYI, not a direct risk |

### 2.3 Agent-Based Scanning

**Use case:** Python agent chạy trên remote host (không có network access về OSV)

**Agent report endpoint:**
```
POST /api/v1/agents/report
Authorization: X-API-Key: agent_...  (scoped: agent:report)
```

**Payload (SBOM-style):**
```json
{
  "agent_id": "agent-001",
  "target": "10.0.1.100",
  "packages": [
    {"name": "log4j", "version": "2.14.1", "type": "java"},
    {"name": "openssl", "version": "1.0.2k", "type": "system"}
  ],
  "findings": [...],
  "collected_at": "2026-06-18T08:00:00Z"
}
```

---

## 3. Scan State Machine

```
pending → queued → running → completed
                           → failed
                           → cancelled
```

| State | Mô tả |
|-------|-------|
| `pending` | Scan created, chờ worker |
| `queued` | Đang chờ trong NATS queue |
| `running` | Đang thực thi |
| `completed` | Hoàn thành, findings imported |
| `failed` | Lỗi trong quá trình scan |
| `cancelled` | User cancelled |

---

## 4. SSE Real-Time Progress Stream

**Endpoint:** `GET /api/v1/scans/{id}/stream`  
**Content-Type:** `text/event-stream`

**SSE Events:**
```
event: scan.progress
data: {"scan_id":"s-001","status":"running","progress":45,"message":"Scanning 10.0.1.100"}

event: scan.host_found
data: {"host":"10.0.1.100","open_ports":["22/ssh","80/http","443/https"]}

event: scan.cve_found
data: {"host":"10.0.1.100","cve_id":"CVE-2021-44228","severity":"CRITICAL"}

event: scan.completed
data: {"scan_id":"s-001","findings_count":12,"duration_seconds":180}
```

**Latency target:** < 2 giây từ event đến browser

---

## 5. Scan Scheduling

**Entity:** `ScheduledScan`

```json
{
  "id": "ss-001",
  "name": "Weekly Infrastructure Scan",
  "scan_type": "nmap",
  "targets": ["10.0.0.0/24"],
  "cron_expr": "0 2 * * 1",
  "frequency": "weekly",
  "product_id": "prod-001",
  "enabled": true,
  "last_run": "2026-06-10T02:00:00Z",
  "next_run": "2026-06-17T02:00:00Z"
}
```

**Scheduler:** NATS JetStream-based, checks every 1 minute

---

## 6. Scan APIs

```
POST /api/v1/scans                           → Create new scan
GET /api/v1/scans                            → List scans (history)
GET /api/v1/scans/{id}                       → Scan detail + status
GET /api/v1/scans/{id}/stream                → SSE progress stream
DELETE /api/v1/scans/{id}                    → Cancel running scan
GET /api/v1/scans/{id}/results/nmap          → Nmap parsed results
GET /api/v1/scans/{id}/results/zap           → ZAP alerts results
POST /api/v1/scheduled-scans                 → Create scheduled scan
GET /api/v1/scheduled-scans                  → List scheduled scans
```

---

## 7. Scan Wizard UI

**Route:** `/scans/new`  
**Component:** `ScanWizard`

**Steps:**
1. **Select scan type:** Nmap / ZAP / Agent Report Upload
2. **Configure targets:** IP/CIDR/URL input + validation
3. **Options:** Scan intensity, custom flags, notification settings
4. **Schedule:** Run now / Schedule (cron)
5. **Review & Launch**

---

## 8. Results Views

### NmapResults (`/scans/:id/results/nmap`)
- Host discovery table (IP, OS, uptime)
- Port/service inventory per host
- CVEs detected per service (linked to CVE detail)
- Export to CSV

### ZAPResults (`/scans/:id/results/zap`)
- Alert list grouped by risk level
- Alert detail với request/response evidence
- CWE mapping per alert
- Export to JSON/HTML

---

## 9. Non-Functional Requirements

| NFR | Target |
|-----|--------|
| Nmap /24 subnet | < 5 phút |
| ZAP active scan | < 30 phút (small app) |
| SSE latency | < 2 giây |
| Concurrent scans | 5 per user, 20 system-wide |
| Authorization check | Scan chỉ authorized targets |
