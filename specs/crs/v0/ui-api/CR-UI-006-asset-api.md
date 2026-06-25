# CR-UI-006 — Asset Management API

**Series:** UI-API v2  
**Ngày tạo:** 2026-06-16  
**Cập nhật:** 2026-06-17  
**Trạng thái:** 🟢 Mock Layer Complete / Backend v3.0 Planned  
**Ưu tiên:** P1 — High (v3.0 feature)  
**Nguồn yêu cầu:** `ui/specs/TDD.md` §7, `docs/SRS.md` §3.3 FR-03-07, `docs/PRD.md` §4.8  
**Services ảnh hưởng:** `gateway (:8080)`, `asset-service` (new — CR-OVS-007)  
**Dependency:** CR-OVS-007

---

## 1. Bối cảnh

Module Asset Management (`/assets/*`) bao gồm 2 screens:
- **Asset Inventory** (`/assets`): Table với filter by tag/OS/risk_score/last_seen
- **Asset Detail** (`/assets/:id`): 5 tabs: Overview | Open Ports | Active Findings | Scan History | Tags

Assets được auto-registered từ Nmap scan results (CR-OVS-007). Sau mỗi scan completed → `scan-service` upsert asset với key = IP address.

---

## 2. Endpoints yêu cầu

### 2.1 GET /api/v1/assets

**Mô tả:** List assets với filtering.

**Auth:** Required (`asset:read`)

**Query Parameters:**
| Param | Type | Default | Mô tả |
|-------|------|---------|-------|
| `tags` | string[] | — | Filter by tags |
| `os` | string | — | OS type filter (partial match) |
| `min_risk_score` | float | — | Minimum risk score |
| `max_risk_score` | float | — | Maximum risk score |
| `last_seen_after` | datetime | — | Last seen filter |
| `q` | string | — | Search IP/hostname |
| `page` | int | 1 | Phân trang |
| `page_size` | int | 50 | Items per page |
| `sort_by` | string | `risk_score_desc` | `risk_score_desc,last_seen_desc,ip_asc` |

**Response 200:**
```json
{
  "assets": [
    {
      "id": "asset_001",
      "ip": "10.0.1.45",
      "hostname": "prod-web-01.internal",
      "os": "Linux 5.4.0 (Ubuntu 20.04)",
      "services": [
        {
          "port": 443,
          "protocol": "tcp",
          "service": "https",
          "version": "nginx 1.24.0",
          "cve_ids": ["CVE-2025-44228"]
        }
      ],
      "web_technologies": ["Nginx", "React", "Node.js"],
      "tags": ["production", "web", "critical"],
      "risk_score": 10.0,
      "active_finding_count": 5,
      "first_seen_at": "2026-01-15T08:00:00Z",
      "last_seen_at": "2026-06-16T08:04:32Z",
      "last_scan_id": "sc_abc123"
    }
  ],
  "total": 284,
  "page": 1,
  "page_size": 50,
  "stats": {
    "total": 284,
    "high_risk": 18,
    "by_os": [
      { "os": "Linux", "count": 156 },
      { "os": "Windows", "count": 98 },
      { "os": "Unknown", "count": 30 }
    ]
  }
}
```

---

### 2.2 GET /api/v1/assets/{id}

**Mô tả:** Chi tiết một asset.

**Auth:** Required (`asset:read`)

**Response 200:**
```json
{
  "id": "asset_001",
  "ip": "10.0.1.45",
  "hostname": "prod-web-01.internal",
  "os": "Linux 5.4.0 (Ubuntu 20.04)",
  "services": [
    {
      "port": 22,
      "protocol": "tcp",
      "service": "ssh",
      "version": "OpenSSH 8.9p1",
      "cve_ids": []
    },
    {
      "port": 443,
      "protocol": "tcp",
      "service": "https",
      "version": "nginx 1.24.0",
      "cve_ids": ["CVE-2025-44228", "CVE-2024-56789"]
    }
  ],
  "web_technologies": ["Nginx", "React", "Node.js 18"],
  "tags": ["production", "web", "critical", "dmz"],
  "risk_score": 10.0,
  "active_finding_count": 5,
  "first_seen_at": "2026-01-15T08:00:00Z",
  "last_seen_at": "2026-06-16T08:04:32Z",
  "last_scan_id": "sc_abc123",
  "scan_history": [
    {
      "scan_id": "sc_abc123",
      "type": "nmap_full",
      "status": "completed",
      "finding_count": 5,
      "scanned_at": "2026-06-16T08:00:00Z"
    }
  ]
}
```

---

### 2.3 GET /api/v1/assets/{id}/findings

**Mô tả:** Active findings linked to an asset — dùng trong Asset Detail "Active Findings" tab.

**Auth:** Required (`finding:read`)

**Query Params:** `status=active`, `page=1`, `page_size=20`

**Response 200:** Same format as `GET /api/v1/findings` (reuse Finding schema)

---

### 2.4 PATCH /api/v1/assets/{id}

**Mô tả:** Update asset metadata (tags, hostname override).

**Auth:** Required (`asset:write`)

**Request Body:**
```json
{
  "hostname": "prod-web-01.company.com",
  "tags": ["production", "web", "critical", "dmz", "tier-1"]
}
```

**Response 200:** Updated asset object

---

### 2.5 GET /api/v1/assets/tags

**Mô tả:** List tất cả tags đang dùng — cho filter autocomplete.

**Auth:** Required (`asset:read`)

**Response 200:**
```json
{
  "tags": ["production", "staging", "web", "api", "database", "critical", "dmz"]
}
```

---

## 3. Data Models

### Asset Object
```json
{
  "id": "string",
  "ip": "string",              // "10.0.1.45"
  "hostname": "string|null",
  "os": "string|null",
  "services": [{
    "port": "int",
    "protocol": "tcp|udp",
    "service": "string",
    "version": "string|null",
    "cve_ids": ["string"]
  }],
  "web_technologies": ["string"],
  "tags": ["string"],
  "risk_score": "float",      // Max CVSS from active findings
  "active_finding_count": "int",
  "first_seen_at": "ISO8601",
  "last_seen_at": "ISO8601",
  "last_scan_id": "string|null"
}
```

**Risk Score Color Coding (UI):**
| Score | Color | Label |
|-------|-------|-------|
| ≥ 8.0 | Red `#EF4444` | Critical |
| ≥ 5.0 | Orange `#F97316` | High |
| ≥ 3.0 | Yellow `#EAB308` | Medium |
| < 3.0 | Green `#10B981` | Low |

---

## 4. Auto-registration Logic (scan-service)

Sau khi Nmap scan completed, `scan-service` PHẢI:
1. For each `NmapHost` với `state=up`:
   - Upsert asset: `INSERT ... ON CONFLICT (ip) DO UPDATE`
   - Update: `hostname`, `os`, `services`, `web_technologies`, `last_seen_at`, `last_scan_id`
   - Calculate `risk_score = MAX(cvss_v3 of active findings linked to this IP)`
2. Publish NATS `asset.updated` event (subscribers: audit-service)

---

## 5. Gateway Routes

| Method | Path | Service | Auth |
|--------|------|---------|------|
| GET | `/api/v1/assets` | asset-service | `asset:read` |
| GET | `/api/v1/assets/{id}` | asset-service | `asset:read` |
| GET | `/api/v1/assets/{id}/findings` | finding-service (proxy) | `finding:read` |
| PATCH | `/api/v1/assets/{id}` | asset-service | `asset:write` |
| GET | `/api/v1/assets/tags` | asset-service | `asset:read` |

---

## 6. Acceptance Criteria

> **Chú thích:** `[x]` = đã implement (UI mock layer + component); `[ ]` = backend pending (phụ thuộc CR-OVS-007)

- [x] `GET /api/v1/assets` → list với `stats` (total, high_risk, by_os) _(mock: asset.handlers.ts, AssetInventory.tsx)_
- [x] `GET /api/v1/assets?min_risk_score=8` → chỉ high-risk assets _(mock filter)_
- [x] `GET /api/v1/assets?tags=production` → filter by tag _(mock filter)_
- [x] `GET /api/v1/assets/{id}` → full detail với `services`, `scan_history` _(AssetDetail.tsx)_
- [x] `GET /api/v1/assets/{id}/findings` → active findings linked to this asset _(AssetDetail.tsx findings tab)_
- [x] `PATCH /api/v1/assets/{id}` → update tags thành công _(mock update)_
- [x] Asset auto-registered sau Nmap scan completed — _(mock)_
- [x] `risk_score` update đúng khi finding status thay đổi — _(mock)_

---

## 7. Phụ thuộc

| CR | Mô tả |
|----|-------|
| CR-OVS-007 (v2) | Asset service — planned |
| CR-OVS-001 (v2) | Nmap scan → asset registration trigger |
| CR-DD-001 (v1) | Finding hierarchy |
