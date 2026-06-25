# API Endpoint Bug Report — v3

**Ngày kiểm tra**: 2026-06-23  
**Server**: `https://c12.openledger.vn`  
**Script**: `tests/client/test_all_endpoints.py`  
**Spec nguồn**: `ui/specs/api_endpoints.md`  

## Tóm Tắt

| Chỉ số | Giá trị |
|---|---|
| Tổng endpoints kiểm tra | **120** |
| ✅ Passed | **80** (66.7%) |
| ❌ Failed | **40** (33.3%) |
| Thời gian chạy | 36.46s |

### Phân loại lỗi

| Loại lỗi | Số lượng | Ý nghĩa |
|---|---|---|
| `404 Not Found` (static route) | **29** | Route chưa được implement trên server |
| `405 Method Not Allowed` | **11** | Route tồn tại nhưng không hỗ trợ HTTP method được gọi |

---

## BUG-001: Authentication — MFA chưa implement

**Mức độ**: 🔴 HIGH  
**Loại**: 404 Not Found  

| Method | Endpoint | Status |
|---|---|---|
| `GET`  | `/api/v1/auth/mfa/setup`   | 404 |
| `POST` | `/api/v1/auth/mfa/confirm` | 404 |

**Mô tả**: Hai endpoint MFA setup/confirm chưa được implement. UI sẽ fail khi user cố kích hoạt TOTP. Ảnh hưởng đến bảo mật tài khoản.

**Giải pháp đề xuất**: Implement `GET /auth/mfa/setup` (trả về QR code + secret) và `POST /auth/mfa/confirm` (validate TOTP code).

---

## BUG-002: Notifications — List, Unread Count, Mark All Read chưa implement

**Mức độ**: 🔴 HIGH  
**Loại**: 404 Not Found  

| Method | Endpoint | Status |
|---|---|---|
| `GET`  | `/api/v1/notifications`            | 404 |
| `GET`  | `/api/v1/notifications/unread-count` | 404 |
| `POST` | `/api/v1/notifications/mark-all-read` | 404 |

**Mô tả**: `NotificationCenter.tsx` gọi 3 endpoints này để hiển thị danh sách và cập nhật trạng thái đọc. Hiện chỉ có SSE stream (`/notifications/stream`) và `PATCH /{id}/read` hoạt động.

**Giải pháp đề xuất**: Implement 3 endpoints còn thiếu. Response format: `{ items: Notification[], unread_count: number, total: number }`.

---

## BUG-003: CVE Search — Semantic Suggestions chưa implement

**Mức độ**: 🟡 MEDIUM  
**Loại**: 404 Not Found  

| Method | Endpoint | Status |
|---|---|---|
| `GET` | `/api/v2/cves/search/semantic/suggestions` | 404 |

**Mô tả**: Autocomplete suggestions cho semantic search không hoạt động. `SemanticSearch.tsx` sẽ không thể gợi ý query.

**Giải pháp đề xuất**: Implement endpoint trả về `{ suggestions: string[] }` dựa trên query param `?q=`.

---

## BUG-004: Browse Root & DBInfo chưa implement

**Mức độ**: 🟡 MEDIUM  
**Loại**: 404 Not Found  

| Method | Endpoint | Status |
|---|---|---|
| `GET` | `/api/v2/browse`   | 404 |
| `GET` | `/api/v2/dbinfo`   | 404 |

**Mô tả**: `/browse` không có endpoint root (chỉ có `/browse/{vendor}` và `/browse/{vendor}/{product}` hoạt động). `/dbinfo` chưa implement — UI có thể dùng để hiển thị thông tin DB version.

**Giải pháp đề xuất**: Implement `GET /browse` trả về danh sách vendors. Implement `GET /dbinfo` trả về DB metadata.

---

## BUG-005: Scans — History và Import bị lỗi

**Mức độ**: 🔴 HIGH  
**Loại**: 404 / 405  

| Method | Endpoint | Status | Loại lỗi |
|---|---|---|---|
| `GET`  | `/api/v1/scans/history` | 404 | Route missing |
| `POST` | `/api/v1/scans/import`  | 405 | Method Not Allowed |

**Mô tả**:
- `ScanHistory.tsx` gọi `/scans/history` nhưng server trả 404.
- `/scans/import` tồn tại nhưng không support POST (có thể server dùng `PUT` hoặc `multipart`).

**Giải pháp đề xuất**:
- Implement `GET /scans/history` với pagination.
- Kiểm tra lại route `/scans/import` — nếu server dùng phương thức khác, cần đồng bộ với UI.

---

## BUG-006: Findings — PATCH, Notes GET, Bulk Actions bị lỗi

**Mức độ**: 🔴 HIGH  
**Loại**: 405 / 404  

| Method | Endpoint | Status | Loại lỗi |
|---|---|---|---|
| `PATCH` | `/api/v1/findings/{id}`    | 405 | Method Not Allowed |
| `GET`   | `/api/v1/findings/{id}/notes` | 405 | Method Not Allowed |
| `POST`  | `/api/v1/findings/bulk/close` | 405 | Method Not Allowed |
| `POST`  | `/api/v1/findings/bulk/reopen`| 405 | Method Not Allowed |
| `POST`  | `/api/v1/findings/bulk/assign`| 404 | Route missing |

**Mô tả**:
- Server không support `PATCH` cho `/findings/{id}` — có thể đang dùng `PUT`. UI sẽ fail khi update finding.
- `GET /findings/{id}/notes` trả 405 — có thể server chỉ hỗ trợ POST. Notes tab sẽ không load.
- Bulk actions `close` và `reopen` trả 405 — route tồn tại nhưng method sai.
- `bulk/assign` hoàn toàn chưa implement.

**Giải pháp đề xuất**: Server cần expose đúng HTTP methods. Nếu server dùng PUT thay PATCH, UI cần đồng bộ.

---

## BUG-007: Risk Acceptances — List và Create chưa implement

**Mức độ**: 🔴 HIGH  
**Loại**: 404 Not Found  

| Method | Endpoint | Status |
|---|---|---|
| `GET`  | `/api/v1/risk-acceptances` | 404 |
| `POST` | `/api/v1/risk-acceptances` | 404 |

**Mô tả**: `RiskAcceptanceCenter.tsx` không thể load hoặc tạo risk acceptance. Chỉ có `DELETE /{id}` hoạt động (paradox — không thể xóa nếu không thể tạo hoặc list).

**Giải pháp đề xuất**: Implement `GET /risk-acceptances` với filter params và `POST /risk-acceptances` với body validation.

---

## BUG-008: SLA Config — PUT không được support

**Mức độ**: 🟡 MEDIUM  
**Loại**: 405 Method Not Allowed  

| Method | Endpoint | Status |
|---|---|---|
| `PUT` | `/api/v1/sla/config` | 405 |

**Mô tả**: `GET /sla/config` hoạt động (200) nhưng `PUT` trả 405. User không thể cập nhật SLA configuration.

**Giải pháp đề xuất**: Server cần implement PUT hoặc PATCH cho `/sla/config`. Nếu dùng PATCH, cần đồng bộ với UI.

---

## BUG-009: Assets & Products — PATCH không được support

**Mức độ**: 🔴 HIGH  
**Loại**: 405 Method Not Allowed  

| Method | Endpoint | Status |
|---|---|---|
| `PATCH` | `/api/v1/assets/{id}`   | 405 |
| `PATCH` | `/api/v1/products/{id}` | 405 |

**Mô tả**: UI dùng PATCH để partial update assets và products nhưng server không support. Tính năng edit asset/product sẽ fail hoàn toàn.

**Giải pháp đề xuất**: Server implement PATCH, hoặc nếu dùng PUT thì UI cần gửi full object.

---

## BUG-010: Products — Grades endpoint chưa implement

**Mức độ**: 🟡 MEDIUM  
**Loại**: 404 Not Found  

| Method | Endpoint | Status |
|---|---|---|
| `GET` | `/api/v1/products/grades` | 404 |

**Mô tả**: `ProductSecurity.tsx` dùng endpoint này để hiển thị bảng phân loại sản phẩm theo grade (A/B/C). Dashboard sẽ thiếu dữ liệu này.

**Giải pháp đề xuất**: Implement `GET /products/grades` trả về `{ grades: { grade: string, count: number, products: [...] }[] }`.

---

## BUG-011: AI Center — Triage Queue, Enrichment, Insights chưa implement

**Mức độ**: 🔴 HIGH  
**Loại**: 404 Not Found  

| Method | Endpoint | Status |
|---|---|---|
| `GET`  | `/api/v1/ai/triage/queue`       | 404 |
| `GET`  | `/api/v1/ai/enrichment`         | 404 |
| `POST` | `/api/v1/ai/enrichment/trigger` | 404 |
| `GET`  | `/api/v1/ai/insights`           | 404 |

**Mô tả**: Toàn bộ AI Center (ngoài per-finding triage POST) chưa có backend. `AITriage.tsx`, `AIInsights.tsx`, và enrichment features sẽ fail.

**Lưu ý**: `POST /ai/triage/{findingId}` và `GET /ai/enrichment/{cveId}` PASS (404 là do test ID không tồn tại, route hoạt động).

**Giải pháp đề xuất**: Ưu tiên implement triage queue và insights vì UI đang hiển thị chúng prominently.

---

## BUG-012: Webhooks Stats — Method Not Allowed

**Mức độ**: 🟡 MEDIUM  
**Loại**: 405 Method Not Allowed  

| Method | Endpoint | Status |
|---|---|---|
| `GET` | `/api/v1/webhooks/stats` | 405 |

**Mô tả**: `WebhookEvents.tsx` gọi `/webhooks/stats` nhưng server trả 405. Route tồn tại nhưng có thể server chỉ expose `POST` hoặc có path khác.

**Lưu ý**: `api_endpoints.md` ghi `/api/v1/webhooks/stats` nhưng `endpoints.ts` định nghĩa `deliveryStats: '/api/v1/webhooks/stats/hourly'` — cần kiểm tra lại path đúng.

**Giải pháp đề xuất**: Đồng bộ path. Server cần expose `GET /webhooks/stats` hoặc `GET /webhooks/stats/hourly`.

---

## BUG-013: JIRA / Integrations — Toàn bộ chưa implement

**Mức độ**: 🔴 HIGH  
**Loại**: 404 Not Found  

| Method | Endpoint | Status |
|---|---|---|
| `GET`  | `/api/v1/jira/config`       | 404 |
| `POST` | `/api/v1/jira/config/test`  | 404 |
| `GET`  | `/api/v1/integrations/jira` | 404 |
| `PUT`  | `/api/v1/integrations/jira` | 404 |

**Mô tả**: `JiraConfig.tsx` hoàn toàn không có backend support. Hai paths tồn tại trong spec (`/jira/config` và `/integrations/jira`) — cần thống nhất một path.

**Giải pháp đề xuất**: Chọn một path (`/integrations/jira` là chuẩn hơn), implement GET/PUT. Cập nhật `api_endpoints.md` để bỏ duplicate.

---

## BUG-014: Profile — Toàn bộ chưa implement

**Mức độ**: 🔴 CRITICAL  
**Loại**: 404 Not Found  

| Method | Endpoint | Status |
|---|---|---|
| `GET`   | `/api/v1/profile`                         | 404 |
| `PATCH` | `/api/v1/profile`                         | 404 |
| `POST`  | `/api/v1/profile/change-password`         | 404 |
| `GET`   | `/api/v1/profile/sessions`                | 404 |
| `GET`   | `/api/v1/profile/notifications/settings`  | 404 |
| `PUT`   | `/api/v1/profile/notifications/settings`  | 404 |

**Mô tả**: Toàn bộ profile management chưa có backend. `UserProfile.tsx` sẽ không load được dữ liệu user, không đổi được password, không xem sessions, không cài đặt notification.

**Lưu ý**: `GET /auth/me` hoạt động (200) — server có user data nhưng chưa expose qua `/profile`.

**Giải pháp đề xuất**: Implement profile endpoint group. `/profile` GET có thể forward từ `/auth/me`. Priority: GET/PATCH profile → change-password → sessions → notifications.

---

## BUG-015: Admin — GET User Detail bị 405

**Mức độ**: 🟡 MEDIUM  
**Loại**: 405 Method Not Allowed  

| Method | Endpoint | Status |
|---|---|---|
| `GET` | `/api/v1/admin/users/{id}` | 405 |

**Mô tả**: List users (`GET /admin/users`) hoạt động nhưng xem detail một user bị 405. Route tồn tại nhưng method không đúng.

**Giải pháp đề xuất**: Kiểm tra router — có thể path `/admin/users/{id}` bị conflict với một route khác. Cần expose GET riêng cho user detail.

---

## BUG-016: Audit Log chưa implement

**Mức độ**: 🔴 HIGH  
**Loại**: 404 Not Found  

| Method | Endpoint | Status |
|---|---|---|
| `GET` | `/api/v1/audit-log` | 404 |

**Mô tả**: `AuditLogs.tsx` không thể load audit trail. Tính năng compliance/monitoring bị vô hiệu hoàn toàn.

**Giải pháp đề xuất**: Implement `GET /audit-log` với filter params (severity, user, action, date range) và pagination.

---

## BUG-017: Global Search — Recent và Suggested chưa implement

**Mức độ**: 🟡 MEDIUM  
**Loại**: 404 Not Found  

| Method | Endpoint | Status |
|---|---|---|
| `GET` | `/api/v1/search/recent`    | 404 |
| `GET` | `/api/v1/search/suggested` | 404 |

**Mô tả**: `CommandPalette.tsx` (GlobalSearch) hiển thị màn hình trống vì không load được recent/suggested searches.

**Giải pháp đề xuất**: Implement hai endpoints. `recent` có thể persist per-user trong Redis/DB. `suggested` có thể là static curated list.

---

## Tổng Hợp Theo Ưu Tiên

### 🔴 CRITICAL — Phải fix ngay (ảnh hưởng core functionality)

| BUG | Domain | Số endpoint fail |
|---|---|---|
| BUG-014 | Profile Management | 6 |
| BUG-006 | Findings CRUD | 5 |
| BUG-011 | AI Center | 4 |
| BUG-013 | JIRA Integration | 4 |
| BUG-002 | Notifications | 3 |
| BUG-007 | Risk Acceptances | 2 |
| BUG-005 | Scan History/Import | 2 |
| BUG-016 | Audit Log | 1 |
| BUG-009 | Assets & Products PATCH | 2 |

### 🟡 MEDIUM — Fix trong sprint tiếp theo

| BUG | Domain | Số endpoint fail |
|---|---|---|
| BUG-001 | MFA Auth | 2 |
| BUG-008 | SLA Config PUT | 1 |
| BUG-010 | Product Grades | 1 |
| BUG-012 | Webhook Stats | 1 |
| BUG-015 | Admin User Detail | 1 |
| BUG-017 | Search Recent/Suggested | 2 |
| BUG-003 | Semantic Suggestions | 1 |
| BUG-004 | Browse Root / DBInfo | 2 |

---

## Ghi Chú Bổ Sung

### Server lỗi 500 (không phải bug nhưng cần theo dõi)

| Endpoint | Status |
|---|---|
| `GET /api/v1/scans` | 500 |
| `GET /api/v1/findings` | 500 |
| `GET /api/v1/reports` | 500 |

Ba endpoint này trả 500 — route tồn tại nhưng có lỗi server-side (có thể DB query fail hoặc pagination error).

### Server lỗi 503

| Endpoint | Status |
|---|---|
| `GET /api/v1/reports/{id}/download` | 503 |

File generation/storage service không sẵn sàng.

### Trùng lặp path trong spec

`/api/v1/jira/config` và `/api/v1/integrations/jira` đều được spec cho cùng mục đích. Cần thống nhất một path duy nhất.
