# CR-HC-011: identity-service — InviteUser không gửi Email

## Trạng thái: 🟡 Medium

## Vấn đề
File: `services/identity-service/adapter/handler/http/admin_handler.go:179`

```go
func (h *AdminHandler) InviteUser(w http.ResponseWriter, r *http.Request) {
    // ... create user in DB ...
    
    // TODO: Send invitation email
    // → email never sent → user không biết mình được mời
    
    writeJSON(w, http.StatusCreated, createdUser)
}
```

Khi admin invite user, user không nhận được email thông báo.
Đây là enterprise feature thiết yếu — user cần biết tài khoản và cần set password.

## Giải pháp

### 1. Email notification infrastructure
Cần `notification-service` hoặc email sender:

```go
type EmailSender interface {
    SendInvitation(ctx context.Context, to, name, inviteToken string) error
}
```

### 2. Invitation token
Tạo one-time token để user đặt password lần đầu:
```go
type InvitationToken struct {
    ID        uuid.UUID `db:"id"`
    UserID    uuid.UUID `db:"user_id"`
    Token     string    `db:"token"`      // hashed
    ExpiresAt time.Time `db:"expires_at"` // 72 hours
    UsedAt    *time.Time `db:"used_at"`
}
```

### 3. Migration
```sql
CREATE TABLE IF NOT EXISTS invitation_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    token      VARCHAR(255) NOT NULL UNIQUE,  -- hashed
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### 4. InviteUser UseCase
```go
func (uc *InviteUserUseCase) Execute(ctx context.Context, in InviteInput) error {
    // 1. Create user with temporary random password
    user, err := uc.userRepo.Create(ctx, &User{
        Email:  in.Email,
        Name:   in.Name,
        Role:   in.Role,
        Status: "pending_activation",
    })
    
    // 2. Generate invitation token
    rawToken := generateSecureToken(32)
    hashedToken := sha256Hex(rawToken)
    if err := uc.inviteRepo.Create(ctx, &InvitationToken{
        UserID:    user.ID,
        Token:     hashedToken,
        ExpiresAt: time.Now().Add(72 * time.Hour),
    }); err != nil {
        return err
    }
    
    // 3. Send email
    inviteURL := fmt.Sprintf("%s/accept-invite?token=%s", uc.frontendURL, rawToken)
    return uc.emailSender.SendInvitation(ctx, in.Email, in.Name, inviteURL)
}
```

### 5. Accept Invitation endpoint
```
POST /api/v1/auth/accept-invite
Body: { token, password, confirm_password }
→ Validate token → Set password → Mark token used → Return JWT
```

## Files cần thay đổi
- `services/identity-service/internal/usecase/invite_user/usecase.go` [NEW]
- `services/identity-service/internal/infra/postgres/invitation_repo.go` [NEW]
- `services/identity-service/internal/infra/email/smtp_sender.go` [NEW]
- `services/identity-service/adapter/handler/http/admin_handler.go` — wire InviteUserUseCase
- `services/identity-service/adapter/handler/http/router.go` — thêm accept-invite route
- `services/identity-service/migrations/004_invitation_tokens.sql` [NEW]

## Config required
```env
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_FROM=noreply@openvulnscan.io
SMTP_USERNAME=...
SMTP_PASSWORD=...
FRONTEND_URL=https://c12.openledger.vn
```

## Acceptance Criteria
- [ ] `POST /admin/users/invite` → user được tạo trong DB + email được gửi
- [ ] `POST /auth/accept-invite?token=...` → user đặt được password
- [ ] Token expire sau 72 giờ
- [ ] Token một lần dùng — sau khi used không thể dùng lại
