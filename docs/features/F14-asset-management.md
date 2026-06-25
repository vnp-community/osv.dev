# F14 — Asset Inventory & Management

**Status:** 🔵 Planned v3.0  
**CR References:** CR-OVS-007  
**Services:** `asset-service` (HTTP: 8068, gRPC: 50068)  
**UI Routes:** `/assets`, `/assets/:id`  
**UI Components:** `AssetInventory`, `AssetDetail`

---

## 1. Mô tả

Asset Management tự động xây dựng và duy trì inventory của hạ tầng dựa trên kết quả scan. Sau mỗi scan Nmap hoàn thành, các hosts được upsert vào asset registry với thông tin port/service/CVE. Cho phép tagging, risk scoring, và scheduled scans per asset.

---

## 2. Asset Auto-Discovery

### 2.1 Auto-Registration sau Nmap Scan
Khi Nmap scan completed:
```
scan.completed event
    → asset-service receives event
    → For each discovered host:
        → Upsert asset (key: IP address)
        → Update: open_ports, services, os_fingerprint
        → Link CVEs detected per service
        → Recalculate risk score
        → Update last_seen timestamp
```

### 2.2 Manual Registration
```
POST /api/v1/assets
{
  "ip_address": "10.0.1.100",
  "hostname": "web-server-01",
  "tags": ["web", "production"],
  "product_id": "prod-001"
}
```

---

## 3. Asset Data Model

```json
{
  "id": "asset-001",
  "ip_address": "10.0.1.100",
  "hostname": "web-server-01.company.com",
  "mac_address": "aa:bb:cc:dd:ee:ff",
  "os": {
    "name": "Ubuntu",
    "version": "22.04",
    "confidence": 95
  },
  "open_ports": [
    {
      "port": 22,
      "protocol": "tcp",
      "service": "ssh",
      "version": "OpenSSH 8.9"
    },
    {
      "port": 443,
      "protocol": "tcp",
      "service": "https",
      "version": "nginx 1.24.0"
    }
  ],
  "tags": ["web", "production", "pci-scope"],
  "risk_score": 8.5,
  "risk_level": "HIGH",
  "product_id": "prod-001",
  "last_seen": "2026-06-18T08:00:00Z",
  "first_seen": "2026-01-01T00:00:00Z",
  "active_findings": 3,
  "critical_findings": 1
}
```

---

## 4. Risk Scoring

### 4.1 Risk Score Calculation (0–10)
```
base_score = max(CVSS scores of active CVEs)
epss_multiplier = 1 + avg(EPSS scores)
kev_bonus = 2.0 if any_kev else 0
exposure_factor = 1.5 if internet_facing else 1.0

risk_score = min(10, base_score * epss_multiplier + kev_bonus) * exposure_factor
```

### 4.2 Risk Levels
| Score | Level |
|-------|-------|
| 9.0–10.0 | CRITICAL |
| 7.0–8.9 | HIGH |
| 4.0–6.9 | MEDIUM |
| 0.1–3.9 | LOW |
| 0 | NONE |

### 4.3 Risk Recalculation
- Khi finding status thay đổi (mitigated → giảm score)
- Khi CVE EPSS score update
- Khi CVE vào KEV (tăng score)
- Daily background job

---

## 5. Asset Tagging

**Tag Types:**
- Environment: `production`, `staging`, `development`
- Compliance: `pci-scope`, `hipaa-scope`
- Type: `web`, `database`, `network`, `iot`
- Custom: User-defined tags

**APIs:**
```
PATCH /api/v1/assets/{id}/tags    → Add/remove tags
GET /api/v1/assets?tag=pci-scope  → Filter by tag
```

---

## 6. Scan History per Asset

```
GET /api/v1/assets/{id}/scans     → All scans for this asset
GET /api/v1/assets/{id}/findings  → All findings for this asset
```

**Timeline View:**
- Khi nào first seen
- Scan history (date, type, findings count)
- Port/service changes over time
- Risk score trend

---

## 7. Scheduled Scans per Asset

```json
{
  "asset_id": "asset-001",
  "scan_type": "nmap",
  "schedule": "0 2 * * *",
  "enabled": true
}
```

**APIs:**
```
POST /api/v1/assets/{id}/scheduled-scans → Schedule auto-scan
GET /api/v1/assets/{id}/scheduled-scans  → List schedules
```

---

## 8. Asset Inventory UI

**Route:** `/assets`  
**Component:** `AssetInventory`

**Features:**
- Grid/table view của tất cả assets
- Filter by: tag, risk level, product, last_seen
- Sort by: risk_score, active_findings, last_seen
- Quick stats: total assets, by risk level
- Bulk tagging
- Export to CSV

### 8.2 Asset Detail UI

**Route:** `/assets/:id`  
**Component:** `AssetDetail`

**Sections:**
- Asset overview (IP, hostname, OS, risk)
- Open ports/services table
- Active findings with severity
- Scan history timeline
- Tags management

---

## 9. APIs

```
GET /api/v1/assets                       → List assets (paginated, filterable)
POST /api/v1/assets                      → Create asset manually
GET /api/v1/assets/{id}                  → Asset detail
PATCH /api/v1/assets/{id}                → Update tags, metadata
GET /api/v1/assets/by-ip/{ip}            → Find by IP address
GET /api/v1/assets/{id}/findings         → Findings per asset
GET /api/v1/assets/{id}/scans            → Scan history
GET /api/v1/assets/{id}/risk-trend       → Risk score over time
```

---

## 10. Database Schema (`osv_asset`)

| Table | Mô tả |
|-------|-------|
| `assets` | Core asset data + risk score |
| `asset_ports` | Open ports per asset (updated per scan) |
| `asset_tags` | Tag assignments |
| `asset_cves` | CVEs linked to asset services |
| `asset_scan_history` | Scan results per asset per time |

---

## 11. Non-Functional Requirements

| NFR | Target |
|-----|--------|
| Asset upsert after scan | < 5 giây |
| Risk score recalc | < 1 giây per asset |
| Asset list query | < 200ms |
| Auto-discovery | Automatic after every scan |
