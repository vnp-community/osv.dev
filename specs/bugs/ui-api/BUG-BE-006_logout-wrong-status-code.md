# BUG-BE-006 — POST /auth/logout Trả 200 Thay Vì 204

| Trường | Giá trị |
|---|---|
| **ID** | BUG-BE-006 |
| **Severity** | 🟡 Medium |
| **Priority** | P1 |
| **Component** | Backend / Identity Service / Auth Handler |
| **Endpoint** | `POST /api/v1/auth/logout` |
| **Phát hiện** | 2026-06-19 |
| **Server** | https://c12.openledger.vn |
| **Status** | Open |

---

## Mô tả

`POST /api/v1/auth/logout` trả về HTTP **200 OK** thay vì **204 No Content** như spec yêu cầu. Một số frontend clients kiểm tra status code chính xác — nếu check `status === 204` sẽ không nhận ra logout thành công.

## Tái hiện

```bash
curl -v -X POST \
  -H "Authorization: Bearer <token>" \
  https://c12.openledger.vn/api/v1/auth/logout

# Actual:
# < HTTP/1.1 200 OK
# {}  (hoặc body rỗng)

# Expected:
# < HTTP/1.1 204 No Content
# (no body)
```

## Fix

```go
// Hiện tại
c.JSON(200, gin.H{})

// Sửa thành
c.Status(204)
```

## Ảnh hưởng

- Frontend logout flow có thể không redirect đúng nếu check `res.status === 204`
- Minor UX issue — logout vẫn hoạt động về mặt chức năng (token bị invalidate)
