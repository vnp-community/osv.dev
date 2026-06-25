# BUG-BE-005 — GET /auth/me Thiếu Wrapper `{ "user": {...} }`

| Trường | Giá trị |
|---|---|
| **ID** | BUG-BE-005 |
| **Severity** | 🟠 High |
| **Priority** | P1 |
| **Component** | Backend / Identity Service / Auth Handler |
| **Endpoint** | `GET /api/v1/auth/me` |
| **Phát hiện** | 2026-06-19 |
| **Server** | https://c12.openledger.vn |
| **Status** | Open |

---

## Mô tả

`GET /api/v1/auth/me` trả về User object trực tiếp thay vì được bọc trong wrapper `{ "user": {...} }` như OpenAPI spec định nghĩa. Frontend đang đọc `response.user` → sẽ nhận `undefined`.

## Tái hiện

```bash
curl -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/auth/me
```

**Actual response:**
```json
HTTP/1.1 200 OK
{
  "id": "uuid",
  "email": "admin@openvulnscan.io",
  "role": "admin",
  "permissions": ["scan:create", ...],
  "mfa_enabled": false,
  "created_at": "..."
}
```

**Expected response (theo spec):**
```json
HTTP/1.1 200 OK
{
  "user": {
    "id": "uuid",
    "email": "admin@openvulnscan.io",
    "name": "Admin User",
    "role": "admin",
    "permissions": ["scan:create", ...],
    "mfa_enabled": false,
    "created_at": "..."
  }
}
```

## Tác động với Frontend

```typescript
// Frontend hiện tại đang làm:
const { data } = await api.get('/auth/me')
const user = data.user  // ← undefined vì server không wrap
```

## Fix

Trong auth handler (identity service), wrap response:

```go
// Hiện tại
c.JSON(200, user)

// Sửa thành
c.JSON(200, gin.H{"user": user})
```

## Ảnh hưởng

- `useCurrentUser()` hook trả về `undefined`
- Header avatar và user menu không hiển thị
- Permission-based UI bị ảnh hưởng
