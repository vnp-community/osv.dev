# UI API v6 — Bug Report

**Ngày phát hiện:** 2026-06-24  
**Phát hiện từ:** Client Test Suite (`tests/client/test_all_endpoints.py` + `run_all.py`)  
**Server:** `https://c12.openledger.vn`  
**Kết quả tổng hợp:** 23 bugs độc lập (sau khi deduplicate)

---

## Tóm tắt kết quả

| Lần chạy | Command | Total | Passed | Failed | Skipped |
|----------|---------|-------|--------|--------|---------|
| 1 | `python3 test_all_endpoints.py` | 120 | 101 | **17** | 2 |
| 2 | `python3 run_all.py missing_endpoints` | 41 | 12 | **10** | 19 |
| 3 | `python3 run_all.py` (auth module only) | 18 | 13 | **3** | 2 |

---

## Phân loại Bugs

### Nhóm A — Routes chưa được đăng ký (404 Not Found trên static path)

Các endpoint được định nghĩa trong `api_endpoints.md` nhưng server trả 404, nghĩa là route chưa được implement hoặc chưa được đăng ký trong router.

| Bug ID | Endpoint | Method | HTTP | Ghi chú |
|--------|----------|--------|------|---------|
| BUG-V6-001 | `/api/v1/auth/mfa/setup` | GET | 404 | MFA setup route missing |
| BUG-V6-002 | `/api/v1/auth/mfa/confirm` | POST | 404 | MFA confirm route missing |
| BUG-V6-003 | `/api/v2/browse` | GET | 404 | Vendor browse root route missing |
| BUG-V6-004 | `/api/v2/dbinfo` | GET | 404 | DB info route missing |
| BUG-V6-005 | `/api/v1/products/grades` | GET | 404 | Product grades route missing |
| BUG-V6-006 | `/api/v1/ai/insights` | GET | 404 | AI insights route missing |
| BUG-V6-007 | `/api/v1/jira/config` | GET | 404 | JIRA config route missing |
| BUG-V6-008 | `/api/v1/jira/config/test` | POST | 404 | JIRA config test route missing |
| BUG-V6-009 | `/api/v1/integrations/jira` | GET | 404 | JIRA integration GET route missing |
| BUG-V6-010 | `/api/v1/integrations/jira` | PUT | 404 | JIRA integration PUT route missing |
| BUG-V6-011 | `/api/v1/audit-log` | GET | 404 | Audit log route missing |
| BUG-V6-012 | `/api/v1/search/recent` | GET | 404 | Global search recent route missing |
| BUG-V6-013 | `/api/v1/search/suggested` | GET | 404 | Global search suggested route missing |

### Nhóm B — Method Not Allowed (405 — Route đăng ký sai HTTP method)

Route tồn tại nhưng không chấp nhận method được định nghĩa trong spec.

| Bug ID | Endpoint | Method | HTTP | Ghi chú |
|--------|----------|--------|------|---------|
| BUG-V6-014 | `/api/v1/findings/{id}/notes` | GET | 405 | Route tồn tại nhưng không accept GET |
| BUG-V6-015 | `/api/v1/sla/config` | PUT | 405 | SLA config update không accept PUT |
| BUG-V6-016 | `/api/v1/webhooks/stats` | GET | 405 | Webhook stats không accept GET |
| BUG-V6-017 | `/api/v1/admin/users/{id}` | GET | 405 | Admin user detail không accept GET |

### Nhóm C — Internal Server Error (500 — Handler lỗi runtime)

Route tồn tại nhưng handler gặp exception khi xử lý.

| Bug ID | Endpoint | Method | HTTP | Error message |
|--------|----------|--------|------|--------------|
| BUG-V6-018 | `/api/v1/profile/sessions` | GET | 500 | Empty response body |
| BUG-V6-019 | `/api/v1/profile/notifications/settings` | GET | 500 | Empty response body |
| BUG-V6-020 | `/api/v1/profile/notifications/settings` | PUT | 500 | Empty response body |
| BUG-V6-021 | `/api/v1/admin/users/invite` | POST | 500 | `"Failed to create invitation"` |

### Nhóm D — Response Schema Sai (2xx nhưng body không đúng spec)

Server trả status code thành công nhưng response body thiếu fields theo spec.

| Bug ID | Endpoint | Method | HTTP | Lỗi |
|--------|----------|--------|------|-----|
| BUG-V6-022 | `/api/v1/scans` | POST | 201 | Response thiếu field `id` |
| BUG-V6-023 | `/api/v1/webhooks` | POST | 201 | Response thiếu field `id` |

### Nhóm E — OAuth Configuration Error (400 — OAuth chưa được cấu hình)

OAuth endpoints tồn tại nhưng trả 400 thay vì redirect, do chưa cấu hình OAuth provider.

| Bug ID | Endpoint | Method | HTTP | Ghi chú |
|--------|----------|--------|------|---------|
| BUG-V6-024 | `/api/v1/auth/oauth/google` | GET | 400 | Google OAuth chưa cấu hình (không redirect) |
| BUG-V6-025 | `/api/v1/auth/oauth/github` | GET | 400 | GitHub OAuth chưa cấu hình (không redirect) |
| BUG-V6-026 | `/api/v1/auth/callback` | GET | 401 | OAuth callback trả 401 thay vì 400/302 |

### Nhóm F — Feature Not Implemented (501/503 — Planned features)

Endpoints được định nghĩa trong spec nhưng feature chưa được implement.

| Bug ID | Endpoint | Method | HTTP | Error message |
|--------|----------|--------|------|--------------|
| BUG-V6-027 | `/api/v1/scans/import` | POST | 501 | `"Scan import feature is planned for a future release"` |
| BUG-V6-028 | `/api/v1/reports/{id}/download` | GET | 503 | `"report download not available: storage backend not configured"` |

---

## Độ ưu tiên

| Priority | Bug IDs | Số lượng | Mô tả |
|----------|---------|----------|-------|
| 🔴 P0 | BUG-V6-018, 019, 020, 021 | 4 | 500 errors — crashes trong production |
| 🟠 P1 | BUG-V6-014, 015, 016, 017 | 4 | 405 errors — method mismatch với spec |
| 🟠 P1 | BUG-V6-022, 023 | 2 | Schema thiếu `id` — FE không thể navigate |
| 🟡 P2 | BUG-V6-001..013 | 13 | 404 routes — features chưa implement |
| 🔵 P3 | BUG-V6-024..026 | 3 | OAuth chưa cấu hình |
| ⚪ P4 | BUG-V6-027, 028 | 2 | Planned features (501/503) |

---

## Tasks thực thi

Xem chi tiết tại thư mục [tasks/](tasks/).
