# Seed Error Report — 2026-06-19T17:31:00+07:00

**Server:** `https://c12.openledger.vn`  
**Scripts:** `02_push_seed_data.py` → `03_verify_seed_data.py`  
**Generated at:** 2026-06-19 17:31 (ICT)

---

## 📊 Tổng quan

| Metric | Giá trị |
|--------|---------|
| **Push: created** | 231 |
| **Push: failed** | 13 |
| **Push: skipped** | 16 |
| **Verify: checked** | 180 |
| **Verify: passed** | 9 |
| **Verify: failed** | 171 |
| **Verify: missing on server** | 158 |
| **Verify: field mismatches** | 203 |

---

## 🔴 Push Failures (13 items)

### ERR-P01 · `user` — admin@company.com (HTTP 400 · DUPLICATE KEY)

| Field | Value |
|-------|-------|
| **domain** | `user` |
| **local_id** | `0fd9ca14-8cb3-4325-acb6-98bc6b3ec831` |
| **http_status** | 400 |
| **error** | `VALIDATION_ERROR` |
| **message** | `create direct user: ERROR: duplicate key value violates unique constraint "uq_users_username" (SQLSTATE 23505)` |
| **root_cause** | Admin user tạo qua username khác với email → trùng constraint DB |
| **severity** | ⚠️ WARNING (data đã tồn tại, không mất dữ liệu) |

---

### ERR-P02 · `webhook` — https://ci.company.internal/hooks/securi (HTTP 400)

| Field | Value |
|-------|-------|
| **domain** | `webhook` |
| **local_id** | `b7e29036-4a1e-4f32-bc17-136ce55d1b0b` |
| **http_status** | 400 |
| **error** | `{"error":"webhook URL hostname cannot be resolved"}` |
| **root_cause** | URL `https://ci.company.internal/hooks/securi` là internal hostname không thể resolve từ server production |
| **severity** | ℹ️ EXPECTED — internal hostname không accessible từ staging |
| **action** | Thay URL bằng public/accessible endpoint khi seed cho staging |

---

### ERR-P03 · `asset` — 10 assets (HTTP 405 · METHOD NOT ALLOWED)

| Field | Value |
|-------|-------|
| **domain** | `asset` |
| **count** | 10 items (10.0.0.1 → 10.0.0.10) |
| **http_status** | 405 |
| **endpoints tried** | `POST /api/v1/assets/bulk` → fallback `POST /api/v1/assets` |
| **error** | `405 Method Not Allowed` |
| **root_cause** | **Backend chưa implement POST endpoint cho assets** — cả bulk lẫn individual đều trả về 405 |
| **severity** | 🔴 BACKEND BUG — endpoint thiếu hoàn toàn |
| **action** | Cần implement `POST /api/v1/assets` trong asset-service |

**Danh sách assets bị fail:**

| IP | local_id |
|----|---------|
| 10.0.0.1 | `a806a192-fa25-4b7b-a774-38294b9fd6c9` |
| 10.0.0.2 | `a948d537-ab1a-49ef-91f8-fdfc22264a23` |
| 10.0.0.3 | `ca338110-f2a1-4dd8-aade-f8e2a007238d` |
| 10.0.0.4 | `705955c0-ea6b-4051-83a8-65a3ef8adf44` |
| 10.0.0.5 | `0dd4d094-2710-42e3-b382-5526565d81f0` |
| 10.0.0.6 | `92048ae1-b52f-403c-8028-edd781ceced6` |
| 10.0.0.7 | `447ef620-442d-42d6-8d85-ab8d6a3bf248` |
| 10.0.0.8 | `5432b3dd-992b-4064-84bd-80649b59c237` |
| 10.0.0.9 | `cd4627b5-4a4f-48dd-836f-64645b9dc11f` |
| 10.0.0.10 | `bc2c5624-5ce0-4af9-b365-8ac0907f7279` |

---

### ERR-P04 · `api_key` — 2 items skipped (user not seeded)

| Field | Value |
|-------|-------|
| **domain** | `api_key` |
| **local_id** | `e2aeba63-fd6a-407d-9e74-2a7f1fe84891` (Admin Master Key) |
| **local_id** | `5356caf9-8c5c-4a06-83d3-15156194c016` (CI/CD Pipeline) |
| **reason** | User owner (`0fd9ca14-...`) không được seed thành công (ERR-P01) → api_keys bị skip |
| **severity** | ⚠️ DOWNSTREAM — fix ERR-P01 sẽ resolve |

---

## 🟡 Push Skipped (16 items)

Các items sau bị skip do đã tồn tại trên server (idempotency xử lý đúng):

| Domain | Count | Lý do |
|--------|-------|-------|
| `user` | 9 | Email đã registered (HTTP 409 CONFLICT) — đúng expected behavior |
| `api_key` | 2 | User owner không seeded — downstream của ERR-P01 |
| `product_type` | 5 | Đã tồn tại trên server (409/conflict) |

---

## 🔴 Verify Failures (171 items tổng)

### VER-01 · `users` — 1 failed / 1 missing

| resource_id | field | expected | actual | note |
|-------------|-------|----------|--------|------|
| `0fd9ca14-8cb3-4325-acb6-98bc6b3ec831` | `<resource>` | `exists` | `404` | User không tồn tại trên server |

**Root cause:** Đây là user bị lỗi DUPLICATE KEY khi push (ERR-P01). Server ID không được ghi vào id_map.json nên verify dùng sai ID.

---

### VER-02 · `product_types` — 0/5 passed · 5 missing (HTTP 404)

Server không trả về được các product_types theo server ID đã map. Khả năng:
- GET endpoint `/api/v1/product-types/{id}` không hoạt động
- Server IDs trong `id_map.json` từ lần seed trước không còn hợp lệ

| resource_id (server_id) |
|------------------------|
| `eba6d0ef-8eda-5b0c-ac60-594f53f7c32a` |
| `a1fa54d0-ad64-5956-aa4e-2120f4c21adf` |
| `10a4476f-69fb-5a22-8031-77dc690eebc4` |
| `dcd60bd2-7be0-5b59-8827-d8ff7702d3b5` |
| `98275ba8-5455-5334-9776-2b95a73fb6b0` |

**Severity:** 🔴 Có thể là BUG backend — GET by ID endpoint trả về 404

---

### VER-03 · `products` — 0/10 passed · 10 missing (HTTP 404)

Tất cả 10 products bị 404 khi verify.

| resource_id |
|-------------|
| `e5ae4546-b8a9-4953-ac39-88c1b59b3cb0` |
| `47a69a03-7839-400a-bc2c-b258bd4cda88` |
| `114de041-fe5c-4cf5-b597-167308dff490` |
| `caaaca50-2b12-4e1d-8feb-78c2f62dbf91` |
| `e9e04a72-9462-4f44-bfda-ceaa1130f0ba` |
| `1897a54d-53fb-4656-bad6-ceaf4cfe24df` |
| `4bedccb8-1767-4790-8304-2128135b7790` |
| `194f0bac-2363-4288-85d4-141183d1635e` |
| `8fae9b34-4318-4aa1-953f-f9ca090f82c9` |
| `7fb0d94f-acb3-4e3a-9c68-bc13c910839c` |

**Root cause:** GET `/api/v1/products/{id}` trả về 404 — có thể verify script dùng sai endpoint path hoặc server ID format khác

---

### VER-04 · `engagements` — 0/20 passed · 20 missing (HTTP 404)

Tất cả 20 engagements bị 404 khi verify. Top 5:

| resource_id |
|-------------|
| `6d96b4a3-ced5-4534-befd-b586bb4b7bb8` |
| `8d8749d8-9d88-4bc5-8c72-ebf6a10dcc3b` |
| `73214926-3b77-4261-91ad-f2866c38dab0` |
| `f07cd0ee-e609-4605-a662-71fcf0d7549d` |
| `4c1b6de2-91f9-4a92-9116-d26f735f1a85` |
| *(+15 more — xem verify_report.json)* |

---

### VER-05 · `findings` — 0/120 passed · 120 missing (HTTP 404)

120 findings bị 404 khi verify. **Push thành công nhưng verify thất bại** → khả năng cao là verify script dùng server_id sai hoặc GET endpoint sai path.

Top 5:

| resource_id |
|-------------|
| `cd125159-4f7b-49a8-b6d3-792ec3b5d2c0` |
| `e5b9c77c-1733-4e24-8e04-29b5c96a9db6` |
| `ab2f16ad-0aae-4043-ac0d-272b06e57b6b` |
| `33158426-6e9d-4a1b-bc49-7eb709275dea` |
| `65ce0d97-ee80-4d9d-bd79-abab39649f34` |
| *(+115 more — xem verify_report.json)* |

> **Note:** Findings được push thành công (231 created bao gồm findings). Lỗi 404 ở verify có thể do:
> 1. GET `/api/v1/findings/{id}` không có hoặc sai route
> 2. `id_map.json` lưu local_id thay vì server_id cho findings (khi bulk create)

---

### VER-06 · `sla_configurations` — 0/3 passed · 15 field mismatches

SLA configs được **push thành công** nhưng GET trả về `null` cho tất cả fields.

| resource_id | field | expected | actual |
|-------------|-------|----------|--------|
| *(Standard SLA)* | `name` | `"Standard SLA"` | `null` |
| *(Standard SLA)* | `critical` | `7` | `null` |
| *(Standard SLA)* | `high` | `30` | `null` |
| *(Standard SLA)* | `medium` | `90` | `null` |
| *(Standard SLA)* | `low` | `365` | `null` |
| *(Critical Assets SLA)* | `name` | `"Critical Assets SLA"` | `null` |
| *(Critical Assets SLA)* | `critical` | `3` | `null` |
| *(Critical Assets SLA)* | `high` | `14` | `null` |
| *(Critical Assets SLA)* | `medium` | `60` | `null` |
| *(Critical Assets SLA)* | `low` | `180` | `null` |
| *(Relaxed SLA)* | `name` | `"Relaxed SLA"` | `null` |
| *(Relaxed SLA)* | `critical` | `14` | `null` |
| *(Relaxed SLA)* | `high` | `60` | `null` |
| *(Relaxed SLA)* | `medium` | `180` | `null` |
| *(Relaxed SLA)* | `low` | `365` | `null` |

**Root cause:** Verify script gọi GET `/api/v1/sla-configurations/{id}` nhưng push trả về `server_id=""` (empty string) → không có ID để GET. Hoặc response schema khác tên field.

**Severity:** 🔴 Backend issue — POST `/api/v1/sla-configurations` không trả về ID trong response body

---

### VER-07 · `assets` — 0/10 passed · 30 field mismatches

Toàn bộ 10 assets failed push (ERR-P03 — HTTP 405), nên verify cũng fail hoàn toàn.

| resource_id | field | expected | actual |
|-------------|-------|----------|--------|
| `9b8e0234-6419-...` | `ip_address` | `10.0.0.1` | `null` |
| `9b8e0234-6419-...` | `hostname` | `web-01.internal` | `null` |
| `9b8e0234-6419-...` | `os` | `Alpine Linux 3.18` | `null` |
| *(+27 more fields for remaining 9 assets)* | | | |

**Root cause:** Downstream của ERR-P03. Asset data không tồn tại trên server.

---

### VER-08 · `webhooks` — 0/2 passed · 2 missing

| resource_id | field | expected | actual |
|-------------|-------|----------|--------|
| `a18b16a7-3124-4fd6-bef3-ad6ff0ebc5d1` | `<resource>` | `exists` | `not_found` |
| `b7e29036-4a1e-4f32-bc17-136ce55d1b0b` | `<resource>` | `exists` | `not_found` |

**Root cause:** Webhook thứ nhất (`a18b16a7`) — push báo thành công (✓ `Slack alerts for new KEV`) nhưng verify 404. Có thể server_id không được lưu đúng. Webhook thứ hai (`b7e29036`) — fail push vì unresolvable hostname (ERR-P02).

---

## 🏷️ Phân loại lỗi theo nguyên nhân

| # | Nguyên nhân | Domains ảnh hưởng | Severity |
|---|------------|-------------------|---------|
| **BC-01** | `POST /api/v1/assets` chưa implement (405) | assets | 🔴 BACKEND BUG |
| **BC-02** | GET by ID endpoint trả về 404 cho seeded data | product_types, products, engagements, findings | 🔴 CẦN KIỂM TRA |
| **BC-03** | POST SLA config không trả về server_id trong response | sla_configurations | 🔴 BACKEND BUG |
| **BC-04** | Webhook push thành công nhưng GET 404 | webhooks | 🟡 KHẢO SÁT |
| **DATA-01** | Webhook URL hostname không resolvable từ server | webhooks | ℹ️ EXPECTED (test data) |
| **DATA-02** | User admin duplicate key (username constraint) | users, api_keys | ⚠️ WARNING |
| **SCRIPT-01** | Verify script có thể dùng sai ID format cho findings | findings | 🟡 KHẢO SÁT |

---

## 🔧 Khuyến nghị action items

### Ưu tiên cao (Backend cần fix)

1. **[BC-01]** Implement `POST /api/v1/assets` và `POST /api/v1/assets/bulk` trong asset-service
2. **[BC-03]** Fix response của `POST /api/v1/sla-configurations` để trả về `id` field trong body
3. **[BC-02]** Kiểm tra GET endpoints `/api/v1/product-types/{id}`, `/api/v1/products/{id}`, `/api/v1/engagements/{id}`, `/api/v1/findings/{id}` — test bằng tay với một server_id cụ thể từ `id_map.json`

### Ưu tiên trung bình

4. **[BC-04]** Debug tại sao webhook `a18b16a7` push thành công nhưng GET trả về `not_found`
5. **[SCRIPT-01]** Kiểm tra lại logic lấy `server_id` cho findings trong `03_verify_seed_data.py` — có thể bulk create trả về array IDs theo thứ tự khác

### Ưu tiên thấp (test data fix)

6. **[DATA-01]** Thay webhook URL `https://ci.company.internal/hooks/securi` bằng public URL trong `data/notifications/webhooks.json` để test staging
7. **[DATA-02]** Xử lý admin user conflict — có thể bỏ qua hoặc detect username vs email mismatch

---

## 📁 Files tham khảo

| File | Mô tả |
|------|-------|
| [`data/output/push_results.json`](../data/output/push_results.json) | Chi tiết 231 success + 13 failed + 16 skipped |
| [`data/output/verify_report.json`](../data/output/verify_report.json) | Chi tiết 203 field mismatches + 158 missing |
| [`data/output/id_map.json`](../data/output/id_map.json) | Mapping local_id → server_id |

---

*Report tự động tổng hợp từ output của `02_push_seed_data.py` và `03_verify_seed_data.py` chạy ngày 2026-06-19 lúc 17:28–17:29 ICT.*
