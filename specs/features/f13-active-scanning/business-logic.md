# F13 — Active Scanning: Business Logic

> 🔵 Tính năng Planned (v3.0 — OpenVulnScan). Mô tả theo thiết kế đã có.

---

## 1. Scan State Machine

### 1.1 States

- **pending:** Scan được tạo, chưa có worker nhận
- **queued:** Worker đã nhận, đang chờ resource
- **running:** Scan đang thực thi (Nmap/ZAP đang chạy)
- **completed:** Scan kết thúc thành công, findings đã được ghi
- **failed:** Lỗi trong quá trình scan (timeout, connection error, ...)
- **cancelled:** User cancel hoặc tự động cancel khi reschedule

### 1.2 Transitions

```
Hợp lệ:
    pending → queued    (worker pickup)
    queued  → running   (scan started)
    running → completed (scan done)
    running → failed    (error)
    pending → cancelled (user cancel)
    queued  → cancelled (user cancel)
    running → cancelled (user cancel, graceful stop)

Không hợp lệ:
    completed → * (terminal state)
    failed    → * (terminal state, tạo scan mới để retry)
```

---

## 2. Nmap Network Scan

### 2.1 Cách thực thi

```
Target input: IP, IP range (CIDR), hostname
Command: nmap -sV -O --script=vulners -oX - --open -T4 {targets}

Params:
    -sV: service/version detection
    -O:  OS fingerprint
    --script=vulners: CVE lookup via Vulners DB
    -oX -: XML output to stdout
    --open: chỉ show open ports
    -T4: timing template 4 (aggressive)
```

### 2.2 Parse Nmap Output

```
parseNmapXML(xml_output):
    for each host in xml:
        ip = host.address
        for each port in host.ports:
            service = {port_id, protocol, service_name, version}
            for each script_output in port.scripts["vulners"]:
                cve_ids = extractCVEIDs(script_output.text)
                // regex: CVE-\d{4}-\d{4,7}
                for cve_id in cve_ids:
                    findings.append({
                        type: "network_vulnerability",
                        cve_id: cve_id,
                        target: ip + ":" + port_id,
                        service: service
                    })
    return findings
```

---

## 3. OWASP ZAP Web Scan

### 3.1 Flow

```
Input: target_url (ví dụ: "https://app.company.com")

Step 1: Spider
    ZAP API: action/spider/scan {url=target_url}
    Poll until progress=100%

Step 2: Active Scan
    ZAP API: action/ascan/scan {url=target_url}
    Poll until progress=100%
    (Active scan thực sự gửi payloads để test injection, XSS, ...)

Step 3: Get Alerts
    ZAP API: view/core/alerts {baseurl=target_url}
    Parse alerts: [{name, risk, confidence, description, solution, url}]

Step 4: Map Risk → Severity
    risk mapping:
        High        → "High"
        Medium      → "Medium"
        Low         → "Low"
        Informational → "Info"
```

---

## 4. Agent Report Ingestion

### 4.1 Endpoint

```
POST /api/v1/agents/report
Auth: X-Api-Key (scope: agent:report)
Body: {
    agent_id: "agent-prod-server-01",
    target: "192.168.1.50",
    scan_type: "package",
    packages: [
        {name: "log4j-core", version: "2.14.1", ecosystem: "maven"}
    ],
    findings: [
        {cve_id: "CVE-2021-44228", severity: "Critical", package: "log4j-core:2.14.1"}
    ]
}
```

### 4.2 Processing

```
handleAgentReport(payload):
    1. Validate API key với scope "agent:report"
    2. Register/update scan_agents {agent_id, last_seen=NOW()}
    3. Create scan record {type: "agent", source: agent_id, status: "completed"}
    4. For each finding in payload:
        enrich với CVE data (CVSS, EPSS, KEV)
        compute SHA-256 hash dedup
        create scan_finding (6-state starting at Active)
    5. Auto-upsert asset: {ip: payload.target, last_scanned: NOW()}
```

---

## 5. Scan Scheduling

### 5.1 Scheduled Scan Config

```
ScheduledScan {
    cron_expr: "0 2 * * 1"  // every Monday 2am
    scan_type: "nmap"
    targets:   ["10.0.0.0/24"]
    product_id: "prod-123"
    enabled:    true
}
```

### 5.2 Scheduler Logic

```
[Scheduler — check mỗi 1 phút]

scheduled_scans due = SELECT * FROM scheduled_scans
    WHERE enabled = true
      AND next_run <= NOW()

for each scheduled_scan:
    1. Create scan record {type, targets, product_id, source: "scheduled"}
    2. UPDATE scheduled_scans SET last_run=NOW(), next_run=nextCronTime(cron_expr)
    3. Queue scan job
```

---

## 6. Finding Dedup (SHA-256)

Khác với scan-service (import) dùng SHA-256 của title+component, active scanning dùng:

```
sha256_hash = SHA-256(cve_id + "|" + target_ip + "|" + port_or_url)

Khi tạo scan_finding:
    Check: existing finding với cùng hash trong product, state != Mitigated
    → Có → mark as duplicate, link to original
    → Không → create new Active finding
```

---

## 7. SSE Progress Stream

```
GET /api/v1/scans/{id}/stream
Content-Type: text/event-stream

Events emitted trong quá trình scan:
    event: scan_started
    data: {"scan_id": "...", "target_count": 5, "timestamp": "..."}

    event: host_discovered
    data: {"ip": "10.0.0.1", "open_ports": [80, 443, 22]}

    event: finding_detected
    data: {"cve_id": "CVE-...", "severity": "High", "target": "10.0.0.1:80"}

    event: scan_progress
    data: {"percent": 45, "hosts_done": 2, "hosts_total": 5}

    event: scan_completed
    data: {"scan_id": "...", "total_findings": 12, "duration_seconds": 120}
```
