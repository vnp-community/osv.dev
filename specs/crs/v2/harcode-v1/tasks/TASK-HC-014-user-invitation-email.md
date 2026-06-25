# TASK-HC-014: User Invitation với Email SMTP thật

**Status:** ✅ DONE  
**Sprint:** 3 | **Ước lượng:** 6 giờ  
**Solution:** [SOL-011](../solutions/SOL-011-identity-invitation-email.md)  
**Service:** `services/identity-service`
**Completed:** 2026-06-24

---

## Implementation Summary

| File | Action | Status |
|------|--------|--------|
| `migrations/006_user_invitations.sql` | NEW — `user_invitations` table với token, expiry, user_id | ✅ |
| `internal/domain/repository/invitation.go` | NEW — `InvitationRepository` + `EmailSender` interfaces | ✅ |
| `internal/infra/postgres/invitation_repo.go` | NEW — `InvitationRepo` PostgreSQL: `Create`, `FindByToken`, `MarkAccepted` | ✅ |
| `internal/infra/smtp/email_sender.go` | NEW — `Sender` (stdlib `net/smtp`), nil-safe khi SMTP không có config | ✅ |
| `internal/usecase/admin_user/invite_user.go` | NEW — `InviteUserUseCase`: tạo user + token + gửi email | ✅ |
| `adapter/handler/http/admin_handler.go` | MODIFY — `WithInviteUC()`, `InviteUser` handler thật | ✅ |
| `adapter/handler/http/auth_handler.go` | MODIFY — `AcceptInvite` endpoint + `invitationRepo` field | ✅ |
| `adapter/handler/http/router.go` | MODIFY — `InviteUC`/`InvitationRepo`/`AppBaseURL` trong `RouterDeps` | ✅ |
| `adapter/repository/postgres/user_repo.go` | MODIFY — `Activate()` method mới | ✅ |
| `internal/domain/repository/repositories.go` | MODIFY — `Activate()` trong `UserRepository` interface | ✅ |
| `embedded.go` | MODIFY — wire SMTP sender + InvitationRepo + InviteUseCase | ✅ |

**Build:** `go build ./...` ✅ PASS  
**Acceptance Criteria Met:**
- ✅ Table `user_invitations` tồn tại (migration 006)
- ✅ `POST /api/v1/admin/users/invite` tạo user + token + gửi email (nếu SMTP config)
- ✅ Token expire sau 48 giờ
- ✅ `GET /api/v1/auth/accept-invite?token=...` activate user account
- ✅ Khi SMTP khưa config → invitation vẫn tạo, log warning về email
- ✅ `go build ./...` pass trong `services/identity-service`

---

## Mô tả

`InviteUser` handler có `// TODO: Send invitation email`. Cần implement đầy đủ: invitation token table, SMTP sender, và InviteUser usecase với email thật.

---

## Acceptance Criteria

- [x] Table `user_invitations` tồn tại
- [x] `POST /api/v1/admin/users/invite` tạo user + invitation token + gửi email
- [x] Token expire sau 48 giờ
- [x] `GET /api/v1/auth/accept-invite?token=...` activate user account
- [x] Khi SMTP chưa config → invitation vẫn tạo, chỉ log warning về email
- [x] `go build ./...` pass trong `services/identity-service`

---

## Files cần sửa/tạo

| Action | File | Thay đổi |
|--------|------|---------|
| NEW | `services/identity-service/migrations/007_user_invitations.sql` | Schema |
| NEW | `services/identity-service/internal/domain/repository/invitation.go` | Interfaces |
| NEW | `services/identity-service/internal/infra/postgres/invitation_repo.go` | PostgreSQL impl |
| NEW | `services/identity-service/internal/infra/smtp/email_sender.go` | SMTP sender |
| NEW | `services/identity-service/internal/usecase/admin_user/invite_user.go` | UseCase |
| MODIFY | `services/identity-service/adapter/handler/http/admin_handler.go` | Wire real usecase |
| MODIFY | `services/identity-service/adapter/handler/http/auth_handler.go` | Accept invite endpoint |
| MODIFY | `services/identity-service/embedded.go` | Wire SMTP + invitationRepo |

---

## Bước thực thi

### 1. Tạo migration

**File:** `services/identity-service/migrations/007_user_invitations.sql`

```sql
CREATE TABLE IF NOT EXISTS user_invitations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email       VARCHAR(255) NOT NULL,
    token       VARCHAR(128) NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '48 hours'),
    accepted_at TIMESTAMPTZ,
    invited_by  UUID,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_invitations_token   ON user_invitations(token) WHERE accepted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_invitations_user    ON user_invitations(user_id);
CREATE INDEX IF NOT EXISTS idx_invitations_expires ON user_invitations(expires_at) WHERE accepted_at IS NULL;
```

```bash
psql $DATABASE_URL -f services/identity-service/migrations/007_user_invitations.sql
psql $DATABASE_URL -c "\d user_invitations"
```

### 2. Tạo domain interfaces

**File:** `services/identity-service/internal/domain/repository/invitation.go`

```go
package repository

import (
    "context"
    "time"
    "github.com/google/uuid"
)

type Invitation struct {
    ID         uuid.UUID  `db:"id"`
    UserID     uuid.UUID  `db:"user_id"`
    Email      string     `db:"email"`
    Token      string     `db:"token"`
    ExpiresAt  time.Time  `db:"expires_at"`
    AcceptedAt *time.Time `db:"accepted_at"`
    InvitedBy  *uuid.UUID `db:"invited_by"`
    CreatedAt  time.Time  `db:"created_at"`
}

type InvitationRepository interface {
    Create(ctx context.Context, inv *Invitation) error
    FindByToken(ctx context.Context, token string) (*Invitation, error)
    MarkAccepted(ctx context.Context, token string) error
}

// EmailSender sends notification emails.
type EmailSender interface {
    SendInvitation(ctx context.Context, to, inviterName, inviteURL, tempPassword string) error
}
```

### 3. Tạo InvitationRepo

**File:** `services/identity-service/internal/infra/postgres/invitation_repo.go`

```go
package postgres

import (
    "context"
    "fmt"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/osv/identity-service/internal/domain/repository"
)

type InvitationRepo struct{ pool *pgxpool.Pool }

func NewInvitationRepo(pool *pgxpool.Pool) *InvitationRepo {
    return &InvitationRepo{pool: pool}
}

func (r *InvitationRepo) Create(ctx context.Context, inv *repository.Invitation) error {
    _, err := r.pool.Exec(ctx, `
        INSERT INTO user_invitations (id, user_id, email, token, expires_at, invited_by)
        VALUES ($1, $2, $3, $4, $5, $6)
    `, inv.ID, inv.UserID, inv.Email, inv.Token, inv.ExpiresAt, inv.InvitedBy)
    if err != nil {
        return fmt.Errorf("invitation_repo.Create: %w", err)
    }
    return nil
}

func (r *InvitationRepo) FindByToken(ctx context.Context, token string) (*repository.Invitation, error) {
    inv := &repository.Invitation{}
    err := r.pool.QueryRow(ctx, `
        SELECT id, user_id, email, token, expires_at, accepted_at, invited_by, created_at
        FROM user_invitations
        WHERE token = $1 AND accepted_at IS NULL AND expires_at > NOW()
    `, token).Scan(&inv.ID, &inv.UserID, &inv.Email, &inv.Token,
        &inv.ExpiresAt, &inv.AcceptedAt, &inv.InvitedBy, &inv.CreatedAt)
    if err != nil {
        return nil, fmt.Errorf("invitation_repo.FindByToken: %w", err)
    }
    return inv, nil
}

func (r *InvitationRepo) MarkAccepted(ctx context.Context, token string) error {
    _, err := r.pool.Exec(ctx,
        `UPDATE user_invitations SET accepted_at=NOW() WHERE token=$1`, token)
    return err
}
```

### 4. Tạo SMTP Sender

**File:** `services/identity-service/internal/infra/smtp/email_sender.go`

```go
package smtp

import (
    "context"
    "fmt"
    "net/smtp"
    "strings"
)

type Sender struct {
    host     string
    port     string
    username string
    password string
    from     string
}

func New(host, port, username, password, from string) *Sender {
    return &Sender{host: host, port: port, username: username, password: password, from: from}
}

func (s *Sender) SendInvitation(_ context.Context, to, inviterName, inviteURL, tempPassword string) error {
    body := fmt.Sprintf(
        "From: %s\r\nTo: %s\r\nSubject: Invited to OSV Platform\r\n\r\n"+
        "Hello,\n\n%s has invited you to join OSV Platform.\n\n"+
        "Accept invitation: %s\n\nTemporary password: %s\n\nExpires in 48 hours.\n",
        s.from, to, inviterName, inviteURL, tempPassword,
    )
    auth := smtp.PlainAuth("", s.username, s.password, s.host)
    return smtp.SendMail(s.host+":"+s.port, auth, s.from, []string{to}, []byte(body))
}
```

### 5. Tạo InviteUser UseCase

**File:** `services/identity-service/internal/usecase/admin_user/invite_user.go`

```go
package admin_user

import (
    "context"
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/rs/zerolog"
    "github.com/osv/identity-service/internal/domain/repository"
)

type InviteUserInput struct {
    Email       string
    Role        string
    InvitedByID uuid.UUID
    InviterName string
    BaseURL     string
}

type InviteUserUseCase struct {
    userRepo       repository.UserRepository
    invitationRepo repository.InvitationRepository
    emailSender    repository.EmailSender  // nil-safe
    log            zerolog.Logger
}

func NewInviteUserUseCase(
    userRepo repository.UserRepository,
    invitationRepo repository.InvitationRepository,
    emailSender repository.EmailSender,
    log zerolog.Logger,
) *InviteUserUseCase {
    return &InviteUserUseCase{
        userRepo: userRepo, invitationRepo: invitationRepo,
        emailSender: emailSender, log: log,
    }
}

func (uc *InviteUserUseCase) Execute(ctx context.Context, in InviteUserInput) error {
    // 1. Generate secure token
    tokenBytes := make([]byte, 32)
    if _, err := rand.Read(tokenBytes); err != nil {
        return fmt.Errorf("invite: generate token: %w", err)
    }
    token := hex.EncodeToString(tokenBytes)

    // 2. Temp password
    passBytes := make([]byte, 8)
    rand.Read(passBytes)
    tempPass := hex.EncodeToString(passBytes)

    // 3. Create user (inactive)
    userID := uuid.New()
    // Adjust method name/signature to match existing UserRepository interface
    if err := uc.userRepo.Create(ctx, &repository.User{
        ID:       userID,
        Email:    in.Email,
        Role:     in.Role,
        IsActive: false,
    }); err != nil {
        return fmt.Errorf("invite: create user: %w", err)
    }

    // 4. Create invitation
    inv := &repository.Invitation{
        ID:        uuid.New(),
        UserID:    userID,
        Email:     in.Email,
        Token:     token,
        ExpiresAt: time.Now().Add(48 * time.Hour),
        InvitedBy: &in.InvitedByID,
    }
    if err := uc.invitationRepo.Create(ctx, inv); err != nil {
        return fmt.Errorf("invite: create invitation: %w", err)
    }

    // 5. Send email (non-fatal)
    inviteURL := fmt.Sprintf("%s/accept-invite?token=%s", in.BaseURL, token)
    if uc.emailSender != nil {
        if err := uc.emailSender.SendInvitation(ctx, in.Email, in.InviterName, inviteURL, tempPass); err != nil {
            uc.log.Warn().Err(err).Str("email", in.Email).Msg("invite: email send failed")
        }
    } else {
        uc.log.Warn().Str("email", in.Email).Msg("invite: email sender not configured")
    }
    return nil
}
```

### 6. Sửa InviteUser handler

```bash
grep -n "func.*InviteUser\|TODO.*email\|InviteUser" \
  services/identity-service/adapter/handler/http/admin_handler.go | head -10
```

Thay phần `// TODO: Send invitation email`:
```go
if err := h.inviteUC.Execute(r.Context(), admin_user.InviteUserInput{
    Email:       req.Email,
    Role:        req.Role,
    InvitedByID: userID,
    InviterName: r.Header.Get("X-User-Name"),
    BaseURL:     h.cfg.AppBaseURL,
}); err != nil {
    writeError(w, http.StatusInternalServerError, "failed to send invitation: "+err.Error())
    return
}
writeJSON(w, http.StatusOK, map[string]string{"status": "invitation_sent", "email": req.Email})
```

### 7. Thêm Accept Invite endpoint

```bash
grep -n "accept.*invite\|AcceptInvite" services/identity-service/adapter/handler/http/auth_handler.go | head -5
```

Thêm nếu chưa có:
```go
func (h *AuthHandler) AcceptInvite(w http.ResponseWriter, r *http.Request) {
    token := r.URL.Query().Get("token")
    inv, err := h.invitationRepo.FindByToken(r.Context(), token)
    if err != nil || inv == nil {
        writeError(w, http.StatusBadRequest, "invalid or expired invitation token")
        return
    }
    // Activate user
    if err := h.userRepo.Activate(r.Context(), inv.UserID); err != nil {
        writeError(w, http.StatusInternalServerError, "failed to activate account")
        return
    }
    _ = h.invitationRepo.MarkAccepted(r.Context(), token)
    writeJSON(w, http.StatusOK, map[string]string{"status": "account_activated"})
}
```

### 8. Wire trong embedded.go
```go
// SMTP sender (từ platform_settings hoặc env)
var emailSender repository.EmailSender
if smtpHost := os.Getenv("SMTP_HOST"); smtpHost != "" {
    emailSender = smtpinfra.New(smtpHost, os.Getenv("SMTP_PORT"),
        os.Getenv("SMTP_USER"), os.Getenv("SMTP_PASSWORD"), os.Getenv("SMTP_FROM"))
}
invitationRepo := pginfra.NewInvitationRepo(pool)
inviteUC := adminusecase.NewInviteUserUseCase(userRepo, invitationRepo, emailSender, logger)
adminHandler := handler.NewAdminHandler(..., inviteUC)
```

### 9. Build check
```bash
cd services/identity-service && go build ./...
```

---

## Verification

```bash
# Invite user
curl -s -X POST -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","role":"user"}' \
  "https://c12.openledger.vn/api/v1/admin/users/invite" | jq '.status'
# PASS nếu = "invitation_sent"

# Verify invitation in DB
psql $DATABASE_URL -c "SELECT email, token, expires_at FROM user_invitations ORDER BY created_at DESC LIMIT 1;"

# Accept invite
TOKEN=$(psql $DATABASE_URL -t -c "SELECT token FROM user_invitations ORDER BY created_at DESC LIMIT 1;" | xargs)
curl -s "https://c12.openledger.vn/api/v1/auth/accept-invite?token=$TOKEN" | jq '.status'
# PASS nếu = "account_activated"
```
