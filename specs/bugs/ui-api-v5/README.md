# UI API v5 — Bug Report

**Ngày phát hiện:** 2026-06-23  
**Phát hiện từ:** Client Test Suite (`tests/client/run_all.py --no-stop-on-fail`)  
**Server:** `https://c12.openledger.vn`  
**Kết quả tổng hợp:** 12 failures còn tồn đọng sau khi V4 đã xử lý

---

## Phân loại Bugs

### Nhóm A — Database Schema Missing (Root Cause: `scan` schema chưa migrate)
Scan-service sử dụng schema `scan.scans` và `scan.findings` nhưng chưa được migrate vào DB server.
DB chỉ có schemas: `auth`, `osv_asset`, `public`.

| Bug ID | Endpoint | HTTP Status | Lỗi |
|--------|----------|-------------|-----|
| BUG-V5-001 | `GET /api/v1/scans` | 500 | `scan.scans` table không tồn tại |
| BUG-V5-002 | `GET /api/v1/scans/stats/weekly` | 500 | `scan.scans` table không tồn tại |
| BUG-V5-003 | `GET /api/v1/scans/stats` (via `/scans`) | 500 | `scan.scans` table không tồn tại |

### Nhóm B — Schema Mismatch (Response format khác spec)

| Bug ID | Endpoint | Lỗi |
|--------|----------|-----|
| BUG-V5-004 | `GET /api/v2/kev/stats` | Thiếu fields `by_vendor` và `recent_additions` trong response JSON |
| BUG-V5-005 | `GET /api/v1/scans/scheduled` | Response `{ scheduled: [] }` thay vì `{ scheduled_scans: [] }` |
| BUG-V5-006 | `GET /api/v1/risk-acceptances` | Response thiếu field `risk_acceptances` (wrapper) |
| BUG-V5-007 | `GET /api/v1/reports` | 500 — `ListByProduct` query SQL thất bại khi `product_id` là empty string |
| BUG-V5-008 | `POST /api/v1/reports` | Response thiếu `name`, `type`, `created_by` fields |
| BUG-V5-009 | `GET /api/v1/api-keys` | Response là `{ keys: [], total: 0 }` thay vì `[]` (array) |

### Nhóm C — Permission Enum Error

| Bug ID | Endpoint | Lỗi |
|--------|----------|-----|
| BUG-V5-010 | `GET /api/v1/profile` | Permission `scan:delete` không thuộc enum hợp lệ |

### Nhóm D — AI Semantic Search (External dependency)

| Bug ID | Endpoint | Lỗi |
|--------|----------|-----|
| BUG-V5-011 | `POST /api/v2/cves/search/semantic` | 500 — Ollama chưa load model `nomic-embed-text` |

---

## Solutions

Xem chi tiết tại thư mục [solutions/](solutions/).

## Tasks thực thi

Xem chi tiết tại thư mục [tasks/](tasks/).
