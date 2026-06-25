# F13 — Active Scanning: Data Flow

---

## 1. Scan Lifecycle Flow

```
Client → POST /api/v1/scans {type: "nmap", targets: ["10.0.0.0/24"], product_id}
    │
    ▼
scan-service-ovs:
    1. Validate: user có quyền trong product
    2. INSERT scans {status='pending'}
    3. Return 202 {scan_id}
    │
    ▼ (async worker pickup)
    4. UPDATE scans SET status='queued'
    5. Allocate Nmap process
    6. UPDATE scans SET status='running', started_at=NOW()
    │
    ▼
Execute: nmap -sV -O --script=vulners -oX - --open -T4 {targets}
    │
    ├── Emit SSE events (host_discovered, finding_detected, scan_progress)
    │
    ▼
Parse Nmap XML output:
    Extract CVE IDs per host:port
    │
    ▼
For each finding:
    Enrich với CVE data từ data-service
    Compute SHA-256 hash dedup
    Check existing findings → set is_duplicate
    INSERT scan_findings
    Auto-upsert asset {ip, last_scanned}
    │
    ▼
UPDATE scans SET status='completed', completed_at=NOW(), finding_count=N
Publish NATS: scan.completed {scan_id, product_id, finding_count}
    │
    ▼
notification-service ← scan.completed → alert product team
```

---

## 2. SSE Progress Stream

```
Client → GET /api/v1/scans/{id}/stream
    Accept: text/event-stream
    │
    ▼
scan-service-ovs:
    Hold connection (SSE)
    
    As scan progresses:
    ─────────────────────────────────────
    event: scan_started
    data: {"percent": 0, "status": "running"}
    
    event: host_discovered
    data: {"ip": "10.0.0.1", "ports": [80, 443]}
    
    event: finding_detected
    data: {"cve_id": "CVE-2021-44228", "severity": "Critical"}
    
    event: scan_progress
    data: {"percent": 60}
    
    event: scan_completed
    data: {"total_findings": 12, "duration_s": 95}
    ─────────────────────────────────────
    [Connection closed by server]
```

---

## 3. Agent Report Ingestion Flow

```
External Agent → POST /api/v1/agents/report
    X-Api-Key: {agent API key with scope: agent:report}
    Body: {agent_id, target, packages[], findings[]}
    │
    ▼
gateway: validate API key scope
    │
    ▼
scan-service-ovs:
    1. Lookup scan_agents by agent_id (register nếu chưa có)
    2. UPDATE scan_agents SET last_seen=NOW()
    3. INSERT scans {type='agent', status='completed', source=agent_id}
    4. For each finding:
        enrich CVE data
        hash dedup
        INSERT scan_findings
    5. Auto-upsert asset {ip: target}
    │
    ▼
Client ← 200 {scan_id, findings_created, duplicates}
```

---

## 4. Scheduled Scan Flow

```
[Scheduler — check mỗi 1 phút]
    │
    ▼
SELECT scheduled_scans WHERE enabled=true AND next_run <= NOW()
    │
    ▼
For each due schedule:
    1. INSERT scans {type, targets, product_id, source='scheduled'}
    2. UPDATE next_run = nextCronTime(cron_expr)
    3. Queue scan job → same flow như manual scan
    │
    ▼
Scan executes, findings created, asset updated
```

---

## 5. NATS Events

| Event | Publisher | Trigger | Subscribers |
|-------|-----------|---------|------------|
| `scan.completed` | scan-service-ovs | Scan kết thúc | notification-service, audit-service |
| `scan.failed` | scan-service-ovs | Scan lỗi | notification-service, audit-service |
| `finding.created` | finding-service-ovs | New active finding | sla-service, audit-service |
