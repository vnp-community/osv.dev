# Change Request SEED-005: Seed Assets, Scan Results & Agent Data qua Gateway

**Cập nhật:** 2026-06-18  
**Status:** Proposed  
**Domain:** scan-service / asset-service  
**Priority:** 🟠 HIGH — Cần có assets/scans để demo vulnerability management workflow  
**Depends on:** SEED-001 (users)

---

## 1. Bối cảnh

`scan-service` và `asset-service` xử lý dữ liệu về network assets và scan results. Để seed demo data, client cần khả năng:

1. **Import assets** (hosts, IPs) mà không cần thực sự scan — ví dụ: nhập từ CMDB, network inventory.
2. **Inject scan results** từ external tools (Nessus, Qualys) mà không qua nmap/ZAP pipeline.
3. **Register agents** từ client (thay vì agent tự register).
4. **Seed agent reports** để có package vulnerability data mà không cần deploy agent thực.

Phân tích hiện trạng:

| Use-case | Endpoint hiện tại | Trạng thái |
|---------|------------------|-----------|
| Tạo asset thủ công | **THIẾU** | ❌ Assets chỉ được tạo bởi scan results |
| Bulk create assets | **THIẾU** | ❌ Không có API tạo asset từ client |
| Import assets từ file | **THIẾU** | ❌ |
| Update asset metadata | `PUT /assets/{id}/tags` | ⚠️ Chỉ update tags, không có full PATCH |
| Register agent | **THIẾU** | ❌ Agents tự register qua agent binary |
| Submit agent report (seed) | **THIẾU** | ❌ Chỉ agent binary mới submit report |
| Import scan results (custom format) | `POST /api/v1/scans/import` | ⚠️ Chỉ cho file formats cụ thể (nmap XML, ZAP JSON) |
| Inject pre-computed findings vào scan | **THIẾU** | ❌ |
| Bulk seed scan findings | **THIẾU** | ❌ |
| Create scheduled scan | `POST /schedules` (scan-service) | ⚠️ Có nhưng **gateway chưa route** |

---

## 2. Thay đổi Đề Xuất

### 2.1 [CRITICAL] `POST /api/v1/assets` — Tạo asset thủ công

Cho phép client tạo asset records trực tiếp (từ CMDB, network discovery tool, manual entry).

**Gateway**:
```
POST /api/v1/assets     →  asset-service:8091  (authenticated, Writer+)
DELETE /api/v1/assets/{id}  →  asset-service:8091  (authenticated, Maintainer+)
```

**Request body**:
```json
{
  "ip_address": "192.168.1.50",
  "hostname": "web-server-01.internal",
  "os": "Ubuntu 22.04 LTS",
  "mac_address": "00:1A:2B:3C:4D:5E",
  "services": [
    { "port": 80,  "protocol": "tcp", "name": "http",  "product": "nginx", "version": "1.24.0" },
    { "port": 443, "protocol": "tcp", "name": "https", "product": "nginx", "version": "1.24.0" },
    { "port": 22,  "protocol": "tcp", "name": "ssh",   "product": "openssh", "version": "8.9" }
  ],
  "tags": ["production", "web-tier", "dmz"],
  "labels": {
    "environment": "production",
    "owner": "web-team",
    "criticality": "high"
  }
}
```

**Response** `201 Created`: Asset object đầy đủ.

---

### 2.2 [CRITICAL] `POST /api/v1/assets/bulk` — Bulk create assets

Tạo nhiều assets trong một request từ network inventory export.

**Gateway**:
```
POST /api/v1/assets/bulk  →  asset-service:8091  (authenticated, Writer+)
```

**Request body**:
```json
{
  "assets": [
    {
      "ip_address": "10.0.1.1",
      "hostname": "gateway.internal",
      "os": "Cisco IOS 15.2",
      "tags": ["network", "core"]
    },
    {
      "ip_address": "10.0.1.10",
      "hostname": "db-primary.internal",
      "os": "CentOS 8",
      "services": [
        { "port": 5432, "protocol": "tcp", "name": "postgresql", "version": "14.5" }
      ],
      "tags": ["database", "critical"]
    }
  ],
  "update_existing": true
}
```

**Options**:
| Field | Mô tả |
|-------|-------|
| `update_existing` | true = upsert theo ip_address; false = skip existing |

**Response** `207 Multi-Status`:
```json
{
  "created_count": 1,
  "updated_count": 1,
  "failed_count": 0,
  "results": [
    { "ip_address": "10.0.1.1",  "status": "created", "id": "asset-uuid-1" },
    { "ip_address": "10.0.1.10", "status": "created", "id": "asset-uuid-2" }
  ]
}
```

---

### 2.3 [CRITICAL] `POST /api/v1/assets/import` — Import assets từ file

**Gateway**:
```
POST /api/v1/assets/import  →  asset-service:8091  (authenticated, Writer+)
Content-Type: multipart/form-data
```

**Form fields**: `file` (JSON hoặc CSV), `format` (`json`|`csv`), `update_existing` (`true`|`false`).

**CSV format**:
```
ip_address,hostname,os,tags
192.168.1.50,web-01.internal,Ubuntu 22.04,"production,web-tier"
192.168.1.51,web-02.internal,Ubuntu 22.04,"production,web-tier"
```

---

### 2.4 [HIGH] `POST /api/v1/assets/{id}/vulnerabilities` — Inject vulnerabilities vào asset

Cho phép client inject vulnerability records vào asset mà không qua scan pipeline.

**Gateway**:
```
POST /api/v1/assets/{id}/vulnerabilities  →  asset-service:8091  (authenticated, Writer+)
```

**Request body**:
```json
{
  "vulnerabilities": [
    {
      "cve_id": "CVE-2021-44228",
      "severity": "critical",
      "cvss": 10.0,
      "detected_at": "2026-06-01T00:00:00Z"
    },
    {
      "cve_id": "CVE-2022-22965",
      "severity": "high",
      "cvss": 9.8,
      "detected_at": "2026-06-02T00:00:00Z"
    }
  ]
}
```

**Response** `201 Created`:
```json
{
  "asset_id": "asset-uuid",
  "added_count": 2,
  "vulnerabilities": [ ... ]
}
```

---

### 2.5 [HIGH] `POST /api/v1/agents` — Register agent từ client (Admin)

Cho phép admin tạo agent record và lấy API key trước khi deploy agent binary.

**Gateway**:
```
POST /api/v1/agents  →  scan-service:8084  (adminOnly)
GET  /api/v1/agents  →  scan-service:8084  (authenticated)
GET  /api/v1/agents/{id}  →  scan-service:8084  (authenticated)
```

**Request body**:
```json
{
  "name": "Agent-Prod-WebServer-01",
  "hostname": "web-server-01.internal",
  "ip_address": "192.168.1.50",
  "os": "Ubuntu 22.04 LTS",
  "tags": ["production", "web-tier"]
}
```

**Response** `201 Created`:
```json
{
  "id": "agent-uuid",
  "name": "Agent-Prod-WebServer-01",
  "hostname": "web-server-01.internal",
  "ip_address": "192.168.1.50",
  "os": "Ubuntu 22.04 LTS",
  "api_key": "ovs-agent-PLAINTEXT_KEY_ONE_TIME_ONLY",
  "status": "inactive",
  "created_at": "2026-06-18T00:00:00Z"
}
```

> `api_key` chỉ trả về 1 lần khi tạo, sau đó không thể retrieve lại.

---

### 2.6 [HIGH] `POST /api/v1/agents/{id}/reports` — Submit agent report (seed)

Cho phép seed script submit agent report để có package vulnerability data.

**Gateway**:
```
POST /api/v1/agents/{id}/reports  →  scan-service:8084  (authenticated, cần `scan:execute` scope hoặc API key của agent)
```

**Request body**:
```json
{
  "hostname": "web-server-01.internal",
  "ip_address": "192.168.1.50",
  "os_info": "Ubuntu 22.04.3 LTS",
  "kernel_version": "5.15.0-91-generic",
  "reported_at": "2026-06-18T00:00:00Z",
  "packages": [
    {
      "name": "openssl",
      "version": "3.0.2-0ubuntu1",
      "ecosystem": "debian",
      "architecture": "amd64"
    },
    {
      "name": "log4j",
      "version": "2.14.1",
      "ecosystem": "maven",
      "architecture": "noarch"
    }
  ]
}
```

**Response** `202 Accepted`:
```json
{
  "report_id": "report-uuid",
  "agent_id": "agent-uuid",
  "package_count": 2,
  "status": "queued_for_processing"
}
```

---

### 2.7 [HIGH] Expose Scheduled Scan routes qua Gateway

Scan-service có `/schedules` CRUD nhưng gateway không route.

**Gateway — Thêm routes**:
```
POST   /api/v1/scans/scheduled      →  scan-service:8084  (authenticated)
GET    /api/v1/scans/scheduled      →  scan-service:8084  (authenticated)
GET    /api/v1/scans/scheduled/{id} →  scan-service:8084  (authenticated)
PUT    /api/v1/scans/scheduled/{id} →  scan-service:8084  (authenticated)
DELETE /api/v1/scans/scheduled/{id} →  scan-service:8084  (authenticated)
```

**Request body** (`POST /api/v1/scans/scheduled`):
```json
{
  "targets": ["192.168.1.0/24"],
  "scan_type": "full",
  "cron_expr": "0 2 * * *",
  "options": {
    "ports": "1-1024",
    "timeout": 3600,
    "intensity": 3
  }
}
```

---

## 3. Tiêu chí nghiệm thu (Acceptance Criteria)

1. `POST /api/v1/assets` với valid IP và hostname → `201`; `GET /api/v1/assets/{id}` trả về asset đó.
2. `POST /api/v1/assets/bulk` với 10 assets, 1 IP trùng và `update_existing: true` → `207` với `created_count: 9, updated_count: 1`.
3. `POST /api/v1/assets/import` với CSV 20 rows → `200` với `imported_count: 20`.
4. `POST /api/v1/assets/{id}/vulnerabilities` với 3 CVEs → `201`; `GET /api/v1/assets/{id}` response có `vuln_summary: {critical: 1, high: 2}`.
5. `POST /api/v1/agents` → `201` với `api_key` field (plaintext, chỉ 1 lần).
6. `POST /api/v1/agents/{id}/reports` với 100 packages → `202`; sau khi processed, report có `cve_count > 0` nếu packages có known CVEs.
7. `GET /api/v1/scans/scheduled` không trả về `404` qua gateway.
8. `POST /api/v1/scans/scheduled` với `cron_expr: "0 2 * * *"` → `201`; `next_run_at` được tính đúng.
