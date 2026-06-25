# BUG-008/009/010/012/015/016/017 — Các Bug Còn Lại

Tổng hợp các bugs mức MEDIUM và còn lại chưa có file riêng.

---

## BUG-008: SLA Config — PUT Không Được Support

**Mức độ**: 🟡 MEDIUM | **Loại**: 405  

| Method | Endpoint | Status |
|---|---|---|
| `GET` | `/api/v1/sla/config`  | ✅ 200 |
| `PUT` | `/api/v1/sla/config`  | ❌ **405** |

GET hoạt động nhưng PUT trả 405. Server có thể chỉ implement GET hoặc dùng PATCH thay PUT.

**Fix**: Implement `PUT /sla/config` hoặc thay đổi sang `PATCH` và đồng bộ UI.

---

## BUG-009: Assets & Products — PATCH Không Được Support

**Mức độ**: 🔴 HIGH | **Loại**: 405  

| Method | Endpoint | Status |
|---|---|---|
| `PATCH` | `/api/v1/assets/{id}`   | ❌ **405** |
| `PATCH` | `/api/v1/products/{id}` | ❌ **405** |

Edit asset và edit product sẽ fail. `GET` và `POST` các routes liên quan đều hoạt động tốt.

**Fix**: Server cần expose PATCH handler. Nếu server dùng PUT, UI phải gửi full object.

```
PATCH /api/v1/assets/{id}
Body: { "tags": [...], "criticality": "high", "owner": "..." }
→ 200 OK { updated asset }

PATCH /api/v1/products/{id}
Body: { "name": "...", "type": "...", "team": "..." }
→ 200 OK { updated product }
```

---

## BUG-010: Products Grades — Route Chưa Implement

**Mức độ**: 🟡 MEDIUM | **Loại**: 404  

| Method | Endpoint | Status |
|---|---|---|
| `GET` | `/api/v1/products/grades` | ❌ **404** |
| `GET` | `/api/v1/products/types`  | ✅ 200 |

Products grade listing (`A/B/C`) không load được. Dashboard sẽ thiếu overview sản phẩm.

**Fix**: 
```
GET /api/v1/products/grades
→ 200 OK
{
  "distribution": [
    { "grade": "A", "count": 3, "products": [...] },
    { "grade": "B", "count": 8 },
    { "grade": "C", "count": 2 }
  ]
}
```

> ⚠️ **Routing conflict**: `/products/grades` có thể bị bắt nhầm vào `/products/{id}` (với id="grades") nếu router không register static trước dynamic.

---

## BUG-012: Webhooks Stats — Method Not Allowed

**Mức độ**: 🟡 MEDIUM | **Loại**: 405  

| Method | Endpoint | Status |
|---|---|---|
| `GET` | `/api/v1/webhooks/deliveries` | ✅ 200 |
| `GET` | `/api/v1/webhooks/stats`      | ❌ **405** |

Route tồn tại nhưng không accept GET. Biểu đồ thống kê webhook delivery sẽ không load.

> **Chú ý**: `endpoints.ts` định nghĩa `deliveryStats: '/api/v1/webhooks/stats/hourly'` — có thể path đúng là `/stats/hourly` không phải `/stats`.

**Fix**: Kiểm tra path thực tế và đồng bộ. Server expose GET cho stats route.

---

## BUG-015: Admin User Detail — Method Not Allowed

**Mức độ**: 🟡 MEDIUM | **Loại**: 405  

| Method | Endpoint | Status |
|---|---|---|
| `GET` | `/api/v1/admin/users`       | ✅ 200 |
| `GET` | `/api/v1/admin/users/{id}`  | ❌ **405** |

List users hoạt động nhưng xem detail một user trả 405. Router conflict có thể xảy ra.

**Fix**: Kiểm tra router — tách route `/admin/users/{id}` với handler GET riêng.

---

## BUG-016: Audit Log — Route Chưa Implement

**Mức độ**: 🔴 HIGH | **Loại**: 404  

| Method | Endpoint | Status |
|---|---|---|
| `GET` | `/api/v1/audit-log` | ❌ **404** |

`AuditLogs.tsx` không load được. Compliance và accountability bị ảnh hưởng.

**Fix**:
```
GET /api/v1/audit-log?severity=&action=&user_id=&from=&to=&page=1&limit=50
→ 200 OK
{
  "items": [{
    "id": "...",
    "action": "finding.updated",
    "user": { "name": "..." },
    "resource_type": "finding",
    "resource_id": "...",
    "changes": {},
    "ip": "...",
    "timestamp": "..."
  }],
  "total": 1203
}
```

---

## BUG-017: Global Search — Recent và Suggested Chưa Implement

**Mức độ**: 🟡 MEDIUM | **Loại**: 404  

| Method | Endpoint | Status |
|---|---|---|
| `GET` | `/api/v1/search/recent`    | ❌ **404** |
| `GET` | `/api/v1/search/suggested` | ❌ **404** |

`CommandPalette.tsx` hiển thị màn hình trống khi không có query.

**Fix**:
```
GET /api/v1/search/recent
→ 200 OK { "items": ["CVE-2025-44228", "webserver01 findings"] }

GET /api/v1/search/suggested
→ 200 OK { "items": ["Critical findings due this week", "Assets with KEV vulnerabilities"] }
```

`recent` nên persist per-user (lưu vào DB khi user search). `suggested` có thể là static list hoặc AI-generated.

---

## BUG-003: CVE Semantic Suggestions Chưa Implement

**Mức độ**: 🟡 MEDIUM | **Loại**: 404  

| Method | Endpoint | Status |
|---|---|---|
| `POST` | `/api/v2/cves/search/semantic` | ✅ 400 |
| `GET`  | `/api/v2/cves/search/semantic/suggestions` | ❌ **404** |

Autocomplete cho semantic search không có backend. `SemanticSearch.tsx` sẽ không gợi ý query.

**Fix**:
```
GET /api/v2/cves/search/semantic/suggestions?q=log4j
→ 200 OK
{
  "suggestions": [
    "Apache Log4j vulnerabilities",
    "Log4Shell RCE (CVSS 10)",
    "log4j-core deserialization"
  ]
}
```

---

## BUG-004: Browse Root và DBInfo Chưa Implement

**Mức độ**: 🟡 MEDIUM | **Loại**: 404  

| Method | Endpoint | Status |
|---|---|---|
| `GET` | `/api/v2/browse`              | ❌ **404** |
| `GET` | `/api/v2/browse/{vendor}`     | ✅ pass |
| `GET` | `/api/v2/browse/{vendor}/{product}` | ✅ pass |
| `GET` | `/api/v2/dbinfo`              | ❌ **404** |

Browse root (danh sách tất cả vendors) chưa implement. DBInfo (metadata database) cũng chưa implement.

**Fix**:
```
GET /api/v2/browse
→ 200 OK { "vendors": ["apache", "microsoft", "linux"], "total": 1247 }

GET /api/v2/dbinfo
→ 200 OK { "version": "2026.06", "cve_count": 245000, "last_updated": "..." }
```
