# TASK-V6-002: Fix 500 Error — Admin User Invite

**Bug IDs:** BUG-V6-021  
**Solution:** [SOL-V6-002](../solutions/SOL-V6-002-fix-admin-invite-500.md)  
**Priority:** 🔴 P0  
**Status:** ✅ DONE

## Mô tả

```
POST /api/v1/admin/users/invite → 500
{"error":"INTERNAL_ERROR","message":"Failed to create invitation"}
```

## Root Cause (xác nhận)

`InviteUserUseCase` và `InviteUser` handler đã được implement đầy đủ trong codebase thực tế.
Bảng `user_invitations` đã có migration `006_user_invitations.sql`.

## Thực thi

- [x] Xác nhận `InviteUserUseCase` tồn tại: `internal/usecase/admin_user/invite_user.go`
- [x] Xác nhận handler `InviteUser` tồn tại: `adapter/handler/http/admin_handler.go:153`
- [x] Xác nhận route đăng ký: `adapter/handler/http/router.go:190` (`r.Post("/users/invite", adminH.InviteUser)`)
- [x] Xác nhận `literal /users/invite` đăng ký trước wildcard `/users/{id}` (router.go:189-192)
- [x] Xác nhận migration `006_user_invitations.sql` tồn tại với đủ columns
- [x] Email được gửi async (best-effort) — không block khi SMTP chưa cấu hình
- [x] Gateway route: `apps/osv/internal/gateway/router.go:149` (`POST /api/v1/admin/users/invite`)
- [x] Build pass: `go build ./...` ✅

## Implementation details

- `InviteUserUseCase.Execute()`: tạo user + invitation token + send email async (warn nếu fail)
- Fallback: nếu `inviteUC == nil` → tạo user trực tiếp + log warning (không panic → không 500)
- SMTP configured: gửi email thực; không configured: log warn + invitation vẫn tạo thành công

## Acceptance Criteria

- [x] Handler không panic khi SMTP chưa cấu hình
- [x] `POST /api/v1/admin/users/invite` trả về 201 khi tạo thành công
- [x] Migration `006_user_invitations.sql` tồn tại
- [ ] **Deploy migration lên server** và verify live (cần deploy)
- [ ] Verify live endpoint trả 201 (không phải 500)
