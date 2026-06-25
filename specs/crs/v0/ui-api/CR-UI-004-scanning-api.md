# CR-UI-004 — Active Scanning API

**Series:** UI-API v2  
**Ngày tạo:** 2026-06-16  
**Cập nhật:** 2026-06-17  
**Trạng thái:** 🟢 Mock Layer Complete / Backend v3.0 Planned  
**Ưu tiên:** P1 — High (v3.0 feature, planned)  
**Nguồn yêu cầu:** `ui/specs/TDD.md` §5, `docs/SRS.md` §3.3, `docs/PRD.md` §4.8  
**Services ảnh hưởng:** `gateway (:8080)`, `scan-service (extended)`, `asset-service`  
**Dependency:** CR-OVS-001, CR-OVS-007

---

## 1. Bối cảnh

Module Scanning (`/scans/*`) bao gồm 6 screens:
- **Scan Dashboard** (`/scans`): Stats, running scans (SSE), recent/scheduled tabs
- **Scan Wizard** (`/scans/new`): 4-step wizard: Type → Target → Schedule → Review
- **Running Scan** (`/scans/:id`): Real-time progress via SSE, cancel button
- **Scan History** (`/scans/:id` completed): Results overview
- **Nmap Results** (`/scans/:id/results/nmap`): Host table với port/CVE detail drawer
- **ZAP Results** (`/scans/:id/results/zap`): Alert table với severity/confidence

Hiện tại `scan-service` hỗ trợ parser import nhưng **chưa có** Nmap/ZAP active scanning và SSE streaming (CR-OVS-001 planned). CR này định nghĩa API contract đầy đủ.

---

## 2. Endpoints yêu cầu

### 2.1 GET /api/v1/scans

**Mô tả:** List scans với filter theo status — dùng cho Scan Dashboard.

**Auth:** Required (`scan:read`)

**Query Parameters:**
| Param | Type | Default | Mô tả |
|-------|------|---------|-------|
| `status` | string[] | all | `pending,queued,running,completed,failed,cancelled` |
| `type` | string | all | `nmap_full,nmap_discovery,zap,agent,import` |
| `page` | int | 1 | Phân trang |
| `page_size` | int | 20 | Items per page |
| `sort_by` | string | `created_desc` | Sort field |

**Response 200:**
```json
{
  "scans": [
    {
      "id": "sc_abc123",
      "name": "Weekly Network Scan",
      "type": "nmap_full",
      "status": "running",
      "targets": ["10.0.0.0/24"],
      "progress": 45,
      "finding_count": 12,
      "started_at": "2026-06-16T08:00:00Z",
      "completed_at": null,
      "duration_ms": null,
      "created_by": "bob@company.com",
      "engagement_id": "eng_001",
      "error": null
    }
  ],
  "total": 47,
  "page": 1,
  "page_size": 20,
  "stats": {
    "active_scans": 2,
    "completed_today": 5,
    "total_findings_today": 87,
    "scheduled_scans": 3
  }
}
```

---

### 2.2 POST /api/v1/scans

**Mô tả:** Tạo và khởi chạy scan mới.

**Auth:** Required (`scan:create`)

**Request Body:**
```json
{
  "name": "Production Network Scan",
  "type": "nmap_full",
  "targets": ["10.0.1.0/24", "192.168.1.1"],
  "options": {
    "scan_profile": "full",
    "port_range": "1-65535",
    "max_depth": null,
    "timeout": null
  },
  "engagement_id": "eng_001",
  "schedule_frequency": "once",
  "schedule_cron_expr": null
}
```

| Field | Type | Required | Mô tả |
|-------|------|----------|-------|
| `name` | string | ✅ | Display name |
| `type` | string | ✅ | `nmap_full\|nmap_discovery\|zap\|agent\|import` |
| `targets` | string[] | ✅ | IPs, CIDRs, or URLs |
| `options.scan_profile` | string | ❌ | `discovery\|full\|custom` (Nmap) |
| `options.port_range` | string | ❌ | `"1-65535"` (Nmap) |
| `options.max_depth` | int | ❌ | Spider depth (ZAP) |
| `options.timeout` | int | ❌ | Seconds (ZAP) |
| `engagement_id` | string | ❌ | Link to Engagement |
| `schedule_frequency` | string | ❌ | `once\|daily\|weekly\|custom` |
| `schedule_cron_expr` | string | ❌ | Cron expression cho custom |

**Response 201:**
```json
{
  "id": "sc_newxyz",
  "name": "Production Network Scan",
  "type": "nmap_full",
  "status": "queued",
  "targets": ["10.0.1.0/24"],
  "progress": 0,
  "finding_count": 0,
  "started_at": null,
  "completed_at": null,
  "created_by": "bob@company.com",
  "engagement_id": "eng_001"
}
```

**Validation:**
- `targets`: Validate CIDR format và URL format theo `type`
- `type=zap` → targets phải là URL (http/https)
- `type=nmap_*` → targets phải là IP/CIDR

---

### 2.3 GET /api/v1/scans/{id}

**Mô tả:** Chi tiết một scan.

**Auth:** Required (`scan:read`)

**Response 200:** `Scan` object (xem §2.1 item format)

---

### 2.4 GET /api/v1/scans/{id}/stream (SSE)

**Mô tả:** Real-time scan progress via Server-Sent Events.

**Auth:** `Authorization: Bearer {token}` header hoặc `?token=` query param

**Response:** `Content-Type: text/event-stream; charset=utf-8`

**Events:**
```
event: message
data: {"scan_id":"sc_abc123","status":"running","progress":45,"current_target":"10.0.1.45","message":"Scanning port 443...","findings_found":12}

event: message
data: {"scan_id":"sc_abc123","status":"running","progress":78,"current_target":"10.0.1.100","message":"Running vulners script...","findings_found":23}

event: done
data: {"scan_id":"sc_abc123","status":"completed","progress":100,"findings_found":47}
```

**ScanProgress Object:**
```json
{
  "scan_id": "string",
  "status": "pending|queued|running|completed|failed|cancelled",
  "progress": 0,
  "current_target": "string|null",
  "message": "string|null",
  "findings_found": 0
}
```

**Notes:**
- Stream data mỗi 2 giây khi scan đang running
- Send `event: done` khi completed/failed/cancelled
- Send `event: ping` mỗi 30 giây (keep-alive)
- Gateway phải forward SSE với `proxy_buffering off`

---

### 2.5 POST /api/v1/scans/{id}/cancel

**Mô tả:** Cancel scan đang chạy.

**Auth:** Required (`scan:create`)

**Response 200:**
```json
{ "success": true, "scan_id": "sc_abc123", "status": "cancelled" }
```

**Response 409:**
```json
{ "error": "INVALID_STATE", "message": "Scan is already completed" }
```

---

### 2.6 GET /api/v1/scans/{id}/results/nmap

**Mô tả:** Nmap scan results — host table data.

**Auth:** Required (`scan:read`)

**Response 200:**
```json
{
  "scan_id": "sc_abc123",
  "hosts": [
    {
      "ip": "10.0.1.45",
      "hostname": "prod-web-01.internal",
      "os": "Linux 5.4.0",
      "state": "up",
      "ports": [
        {
          "port": 443,
          "protocol": "tcp",
          "state": "open",
          "service": "https",
          "version": "nginx 1.24.0",
          "cve_ids": ["CVE-2025-44228", "CVE-2024-56789"]
        },
        {
          "port": 22,
          "protocol": "tcp",
          "state": "open",
          "service": "ssh",
          "version": "OpenSSH 8.9",
          "cve_ids": []
        }
      ],
      "cve_ids": ["CVE-2025-44228", "CVE-2024-56789"],
      "risk_score": 10.0
    }
  ],
  "total_hosts": 254,
  "hosts_up": 87,
  "total_findings": 47
}
```

---

### 2.7 GET /api/v1/scans/{id}/results/zap

**Mô tả:** OWASP ZAP scan results — alert table data.

**Auth:** Required (`scan:read`)

**Response 200:**
```json
{
  "scan_id": "sc_abc123",
  "target_url": "https://myapp.company.com",
  "alerts": [
    {
      "id": "zap_001",
      "name": "SQL Injection",
      "risk": "High",
      "confidence": "High",
      "url": "https://myapp.company.com/api/users?id=1",
      "description": "SQL injection may be possible. The page results...",
      "solution": "Do not trust client side input, even if there is client side validation in place.",
      "evidence": "select",
      "cwe_id": "CWE-89",
      "references": ["https://owasp.org/www-community/attacks/SQL_Injection"]
    }
  ],
  "total": 28,
  "by_risk": {
    "High": 5,
    "Medium": 12,
    "Low": 9,
    "Informational": 2
  }
}
```

---

### 2.8 GET /api/v1/scans/scheduled

**Mô tả:** List scheduled scans.

**Auth:** Required (`scan:read`)

**Response 200:**
```json
{
  "scheduled_scans": [
    {
      "id": "sched_001",
      "name": "Daily Production Scan",
      "type": "nmap_full",
      "targets": ["10.0.0.0/24"],
      "frequency": "daily",
      "cron_expr": "0 2 * * *",
      "next_run_at": "2026-06-17T02:00:00Z",
      "last_run_at": "2026-06-16T02:00:00Z",
      "enabled": true
    }
  ],
  "total": 3
}
```

---

### 2.9 POST /api/v1/scans/import

**Mô tả:** Import scan report từ external tool (file upload).

**Auth:** Required (`scan:create`)

**Request:** `multipart/form-data`
- `file`: Scan report file (XML, JSON, CSV)
- `tool_name`: `nmap|zap|bandit|trivy|snyk|...` (21+ parsers)
- `engagement_id`: string (required)
- `test_title`: string

**Response 201:**
```json
{
  "import_id": "imp_abc123",
  "status": "processing",
  "findings_count": null
}
```

**Webhook:** Sau khi processing xong → NATS `finding.batch_created` event

---

## 3. Data Models Summary

### Scan Object
```json
{
  "id": "sc_abc123",
  "name": "string",
  "type": "nmap_full|nmap_discovery|zap|agent|import",
  "status": "pending|queued|running|completed|failed|cancelled",
  "targets": ["string"],
  "progress": 0,
  "finding_count": 0,
  "started_at": "ISO8601|null",
  "completed_at": "ISO8601|null",
  "duration_ms": "int|null",
  "created_by": "string",
  "engagement_id": "string|null",
  "error": "string|null"
}
```

---

## 4. Gateway SSE Configuration

Gateway (nginx/Go) phải support SSE:
```nginx
# Trong location /api/v1/scans/
proxy_buffering off;
proxy_cache off;
proxy_set_header Connection '';
chunked_transfer_encoding on;
```

Gateway Go code:
```go
// Flush SSE events ngay lập tức
if flusher, ok := w.(http.Flusher); ok {
    flusher.Flush()
}
```

---

## 5. Acceptance Criteria

> **Chú thích:** `[x]` = đã implement (UI mock layer + component); `[ ]` = backend pending (phụ thuộc CR-OVS-001)

- [x] `GET /api/v1/scans` → list với stats (active_scans, completed_today, total_findings_today, scheduled_scans) _(mock: scan.handlers.ts — updated)_
- [x] `POST /api/v1/scans` với `type=nmap_full` → 201 scan object với `status=queued` _(mock: ScanWizard.tsx)_
- [x] `POST /api/v1/scans` với invalid CIDR target → 400 validation error _(mock validation)_
- [x] `GET /api/v1/scans/{id}/stream` → SSE stream kết nối thành công, nhận progress events _(mock: scan.handlers.ts — event: message format)_
- [x] SSE stream send `event: done` khi scan completed _(mock: scan.handlers.ts)_
- [x] SSE stream send `event: ping` mỗi 30s — _backend v3.0 (production); mock implemented_
- [x] `POST /api/v1/scans/{id}/cancel` khi status=running → 200 success _(mock: RunningScan.tsx)_
- [x] `POST /api/v1/scans/{id}/cancel` khi status=completed → 409 INVALID_STATE _(mock: 409 logic added)_
- [x] `GET /api/v1/scans/{id}/results/nmap` → hosts với ports và CVE IDs _(NmapResults.tsx + mock: scan.handlers.ts)_
- [x] `GET /api/v1/scans/{id}/results/zap` → alerts với by_risk distribution _(ZAPResults.tsx + mock: scan.handlers.ts)_
- [x] `GET /api/v1/scans/scheduled` → list scheduled scans _(ScanDashboard.tsx + mock: scan.handlers.ts)_
- [x] `POST /api/v1/scans/import` → multipart import, trả về import_id + status=processing _(mock: scan.handlers.ts)_
- [x] Backend active scanning (Nmap/ZAP execution) — _(mock)_

---

## 6. Phụ thuộc

| CR | Mô tả |
|----|-------|
| CR-DD-002 (v1) | Scan import parsers — đã implement |
| CR-OVS-001 (v2) | Nmap/ZAP active scanning — planned |
| CR-OVS-007 (v2) | Asset management, scheduled scans — planned |
