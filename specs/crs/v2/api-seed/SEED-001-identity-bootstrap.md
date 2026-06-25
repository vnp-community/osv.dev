# Change Request SEED-001: Seed Users, API Keys & RBAC Roles qua Gateway

**Cập nhật:** 2026-06-18  
**Status:** Proposed  
**Domain:** identity-service  
**Priority:** 🔴 CRITICAL — Không thể seed bất kỳ domain nào nếu chưa có users và RBAC

---

## 1. Bối cảnh

Để client có thể seed dữ liệu (từ scripts, CI/CD, developer tools), hệ thống cần trước tiên hỗ trợ tạo tài khoản người dùng và API keys thông qua gateway. Phân tích hiện trạng:

| Use-case | Endpoint hiện tại | Trạng thái |
|---------|-------------------|-----------|
| Tạo user mới (admin invite) | `POST /api/v1/admin/users/invite` | ✅ Có, nhưng chỉ qua invite flow, không có bulk |
| Tạo user trực tiếp (bootstrap) | **THIẾU** | ❌ `POST /api/v1/admin/users` chưa tồn tại |
| Set password cho user mới | **THIẾU** | ❌ Admin không thể set password; chỉ có `reset-password` |
| Gán role cho user trong product | **THIẾU** | ❌ `POST /api/v1/admin/users/{id}/roles` chưa có |
| Tạo API key cho user khác (admin) | **THIẾU** | ❌ Chỉ có `POST /api/v1/api-keys` cho chính user |
| Bulk create users | **THIẾU** | ❌ Không hỗ trợ |
| Lấy danh sách roles khả dụng | `GET /api/v1/admin/roles` | ✅ Có (RBAC matrix) |
| Lấy user theo ID | **THIẾU** | ❌ Chỉ có list; không có `GET /api/v1/admin/users/{id}` |

---

## 2. Thay đổi Đề Xuất

### 2.1 [CRITICAL] `POST /api/v1/admin/users` — Tạo user trực tiếp

Cho phép admin tạo user với password đã biết (thay vì qua invite link).

**Gateway**: Thêm route trong `apps/osv/internal/gateway/router.go`:
```
POST /api/v1/admin/users  →  identity-service:8081  (adminOnly)
```

**Request body**:
```json
{
  "email": "alice@company.com",
  "username": "alice",
  "password": "InitialPass123!",
  "role": "user",
  "is_active": true,
  "is_verified": true
}
```

**Response** `201 Created`:
```json
{
  "id": "uuid",
  "email": "alice@company.com",
  "username": "alice",
  "role": "user",
  "is_active": true,
  "is_verified": true,
  "created_at": "2026-06-18T00:00:00Z"
}
```

**Validation**:
- `email` unique, format hợp lệ
- `password` tối thiểu 8 ký tự
- `role` phải là một trong: `admin`, `user`, `readonly`

---

### 2.2 [CRITICAL] `POST /api/v1/admin/users/bulk` — Tạo nhiều users

Hỗ trợ bulk insert để seed script tạo hàng chục users trong một request.

**Gateway**:
```
POST /api/v1/admin/users/bulk  →  identity-service:8081  (adminOnly)
```

**Request body**:
```json
{
  "users": [
    { "email": "alice@corp.com", "username": "alice", "password": "Pass123!", "role": "user" },
    { "email": "bob@corp.com",   "username": "bob",   "password": "Pass456!", "role": "readonly" }
  ]
}
```

**Response** `207 Multi-Status`:
```json
{
  "results": [
    { "email": "alice@corp.com", "status": "created", "id": "uuid-1" },
    { "email": "bob@corp.com",   "status": "created", "id": "uuid-2" }
  ],
  "created_count": 2,
  "failed_count": 0,
  "errors": []
}
```

---

### 2.3 [CRITICAL] `GET /api/v1/admin/users/{id}` — Lấy user theo ID

Đã được identify ở CR-001. Cần thiết để seed script verify user đã tạo.

**Gateway**:
```
GET /api/v1/admin/users/{id}  →  identity-service:8081  (adminOnly)
```

**Response**: `User` object đầy đủ.

---

### 2.4 [HIGH] `POST /api/v1/admin/users/{id}/api-keys` — Tạo API key cho user khác

Admin tạo API key dài hạn cho service accounts, CI/CD pipelines.

**Gateway**:
```
POST /api/v1/admin/users/{id}/api-keys  →  identity-service:8081  (adminOnly)
```

**Request body**:
```json
{
  "name": "CI/CD Pipeline",
  "scopes": ["cve:read", "finding:write", "scan:execute"],
  "expires_at": null
}
```

**Response** `201 Created` (key chỉ trả về 1 lần):
```json
{
  "id": "uuid",
  "name": "CI/CD Pipeline",
  "key": "ovs_Ab3xYz9qPlain...",
  "prefix": "ovs_Ab3xYz",
  "scopes": ["cve:read", "finding:write", "scan:execute"],
  "expires_at": null,
  "created_at": "2026-06-18T00:00:00Z"
}
```

---

### 2.5 [HIGH] `POST /api/v1/admin/users/{id}/roles` — Gán role cho user

Admin gán product-scoped role cho user.

**Gateway**:
```
POST /api/v1/admin/users/{id}/roles  →  identity-service:8081  (adminOnly)
```

**Request body**:
```json
{
  "role_id": 3,
  "scope": "product",
  "resource_id": "product-uuid"
}
```

**Response** `200 OK`:
```json
{
  "user_id": "uuid",
  "role_id": 3,
  "scope": "product",
  "resource_id": "product-uuid",
  "assigned_at": "2026-06-18T00:00:00Z"
}
```

---

## 3. Tiêu chí nghiệm thu (Acceptance Criteria)

1. `POST /api/v1/admin/users` với valid body → `201` với user object; với email trùng → `409 Conflict`.
2. `POST /api/v1/admin/users/bulk` với 10 users hợp lệ → `207` với `created_count: 10`.
3. `POST /api/v1/admin/users/bulk` với 1 user email trùng → `207` với partial success, entry đó `"status": "error"`.
4. `GET /api/v1/admin/users/{id}` trả về user object; với ID không tồn tại → `404`.
5. `POST /api/v1/admin/users/{id}/api-keys` → `201` với `key` field (plaintext, chỉ 1 lần).
6. `POST /api/v1/admin/users/{id}/roles` với product scope → user có thể xem product đó trong finding-service.
7. Tất cả endpoints yêu cầu `role: admin` — caller là `user` → `403 Forbidden`.
