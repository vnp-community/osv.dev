# BUG-BE-010 — Các Endpoints Chưa Implement (404 Not Found)

| Trường | Giá trị |
|---|---|
| **ID** | BUG-BE-010 |
| **Severity** | 🟠 High |
| **Priority** | P1 |
| **Component** | Backend / Data Service & API Gateway |
| **Phát hiện** | 2026-06-19 |
| **Server** | https://c12.openledger.vn |
| **Status** | Open |

---

## Mô tả

16 endpoints được định nghĩa trong OpenAPI spec nhưng trả về **404 Not Found** — chưa được implement hoặc chưa được mount trong service/gateway.

## Danh Sách Endpoints Bị Thiếu

### Data Service (`/api/v2`) — thiếu trong data-service (:18082)

| Endpoint | Method | Trang UI bị ảnh hưởng |
|---|---|---|
| `/api/v2/vendors` | GET | `/cve/vendors` — Vendor Browse |
| `/api/v2/browse` | GET | Vendor Browse paginated |
| `/api/v2/browse/{vendor}` | GET | Vendor products |
| `/api/v2/browse/{vendor}/{product}` | GET | Product CVEs |
| `/api/v2/cwe` | GET | `/cve/cwe` — CWE Library |
| `/api/v2/cwe/{id}` | GET | CWE Detail |
| `/api/v2/capec/{id}` | GET | CAPEC Detail (CAPEC Library không click được) |
| `/api/v2/epss/top` | GET | `/cve/epss` — EPSS Analytics |
| `/api/v2/epss/distribution` | GET | EPSS Distribution chart |
| `/api/v2/epss/{cveId}` | GET | EPSS detail per CVE |
| `/api/v2/dbinfo` | GET | Database info |
| `/api/v2/cves/export` | GET | CVE Export |

### API Gateway (`/api/v1`) — thiếu route

| Endpoint | Method | Trang UI bị ảnh hưởng |
|---|---|---|
| `/api/v1/sla/overview` | GET | Dashboard SLA |
| `/api/v1/risk-acceptances` | GET | Risk Acceptances |
| `/api/v1/reports` | GET, POST | Reports |
| `/api/v1/profile` | GET | Profile page |
| `/api/v1/admin/settings` | GET, PATCH | Admin Settings |
| `/api/v1/api-keys` | GET, POST | API Keys |
| `/api/v1/jira/config` | GET, PUT | Jira Integration |
| `/api/v1/assets/tags` | GET | Asset tag filter |
| `/api/v1/findings/stats` | GET | Findings stats widget |

## Xác nhận Route Config (nginx)

Từ `c12.openledger.vn.conf`:
- `/api/v2/cwe` → proxy về `172.20.2.48:18082` ✅ nginx OK
- `/api/v2/capec` → proxy về `172.20.2.48:18082` ✅ nginx OK
- Nhưng **data-service chưa có handler** cho các routes này

## Fix Plan

### Phase 1 — Data Service handlers (P1)
Implement trong data-service (`osv-backend-data-service`):
- `GET /api/v2/vendors?q=&limit=` — query từ CVE dataset
- `GET /api/v2/cwe` và `GET /api/v2/cwe/{id}` — từ MITRE CWE data
- `GET /api/v2/capec/{id}` — từ MITRE CAPEC data
- `GET /api/v2/epss/top?limit=` — top EPSS scores
- `GET /api/v2/epss/distribution` — histogram EPSS
- `GET /api/v2/epss/{cveId}` — EPSS history per CVE

### Phase 2 — Gateway routes (P1)
Thêm route handlers vào API gateway:
- `/api/v1/sla/overview` — forward to SLA service
- `/api/v1/profile` — alias cho `/auth/me`
- `/api/v1/risk-acceptances` — CRUD
- `/api/v1/reports` — Reports service

### Phase 3 — Lower priority (P2)
- `/api/v1/admin/settings`
- `/api/v1/api-keys`
- `/api/v1/jira/config`
- `/api/v2/cves/export`

## Ảnh hưởng UI

Theo `crash.md`:
- Trang `/cve/vendors` → 404
- Trang `/cve/cwe` → 404
- CAPEC Library **không click được**
- Trang `/cve/epss` → data trống
- EPSS Analytics không hiển thị top list
