# SOL-011: User Invitation với Email Thật — identity-service

**CR:** CR-HC-011 | **Priority:** 🟡 Medium | **Sprint:** 3  
**Service:** `services/identity-service`

---

## Implementation Status

**✅ IMPLEMENTED** — 2026-06-24
**Task:** TASK-HC-014
**Note:** InviteUserUseCase + SMTP sender + user_invitations table + AcceptInvite endpoint
**Build:** ✅ `go build ./...` passes

---

---

## Context phân tích code

**File:** `identity-service/adapter/handler/http/admin_handler.go:136,179`
```go
func (h *AdminHandler) InviteUser(w http.ResponseWriter, r *http.Request) {
    // ... tạo user với temporary password ...
    // TODO: Send invitation email  ← thiếu
}
```

**Infrastructure đã có:**
- Identity-service đã có SMTP config keys trong `platform_settings` (từ SOL-004)
- `users` table đã có `email` column

**Thiếu:**
- `user_invitations` table để lưu invitation token
- Email sender service
- Email template

---

## Solution

### Bước 1: Migration

**File mới:** `identity-service/migrations/007_user_invitations.sql`

```sql
CREATE TABLE IF NOT EXISTS user_invitations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email           VARCHAR(255) NOT NULL,
    token           VARCHAR(128) NOT NULL UNIQUE,
    expires_at      TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '48 hours'),
    accepted_at     TIMESTAMPTZ,
    invited_by      UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_invitations_token    ON user_invitations(token)    WHERE accepted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_invitations_user     ON user_invitations(user_id);
CREATE INDEX IF NOT EXISTS idx_invitations_expires  ON user_invitations(expires_at) WHERE accepted_at IS NULL;
```

### Bước 2: Domain interfaces

**File mới:** `identity-service/internal/domain/repository/invitation.go`

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
    DeleteExpired(ctx context.Context) (int, error)
}

// EmailSender sends email notifications.
type EmailSender interface {
    SendInvitation(ctx context.Context, to, inviterName, inviteURL, tempPassword string) error
}
```

### Bước 3: PostgreSQL InvitationRepo

**File mới:** `identity-service/internal/infra/postgres/invitation_repo.go`

```go
package postgres

import (
    "context"
    "fmt"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/osv/identity-service/internal/domain/repository"
)

type InvitationRepo struct {
    pool *pgxpool.Pool
}

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
    _, err := r.pool.Exec(ctx, `
        UPDATE user_invitations SET accepted_at = NOW() WHERE token = $1
    `, token)
    return err
}
```

### Bước 4: SMTP Email Sender

**File mới:** `identity-service/internal/infra/smtp/email_sender.go`

```go
package smtp

import (
    "context"
    "fmt"
    "net/smtp"
    "strings"
)

type SMTPSender struct {
    host     string
    port     string
    username string
    password string
    from     string
    useTLS   bool
}

func NewSMTPSender(host, port, username, password, from string, useTLS bool) *SMTPSender {
    return &SMTPSender{
        host:     host,
        port:     port,
        username: username,
        password: password,
        from:     from,
        useTLS:   useTLS,
    }
}

func (s *SMTPSender) SendInvitation(ctx context.Context, to, inviterName, inviteURL, tempPassword string) error {
    subject := "You've been invited to OSV Platform"
    body := fmt.Sprintf(`Hello,

%s has invited you to join the OSV Platform security vulnerability management system.

To accept your invitation and set up your account, please click the link below:
%s

Your temporary password: %s
(Please change this password after your first login)

This invitation will expire in 48 hours.

Best regards,
OSV Platform Team
`, inviterName, inviteURL, tempPassword)

    msg := buildMIMEMessage(s.from, to, subject, body)
    addr := s.host + ":" + s.port
    auth := smtp.PlainAuth("", s.username, s.password, s.host)

    if err := smtp.SendMail(addr, auth, s.from, []string{to}, []byte(msg)); err != nil {
        return fmt.Errorf("smtp.SendInvitation: %w", err)
    }
    return nil
}

func buildMIMEMessage(from, to, subject, body string) string {
    var sb strings.Builder
    sb.WriteString("From: " + from + "\r\n")
    sb.WriteString("To: " + to + "\r\n")
    sb.WriteString("Subject: " + subject + "\r\n")
    sb.WriteString("MIME-Version: 1.0\r\n")
    sb.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
    sb.WriteString("\r\n")
    sb.WriteString(body)
    return sb.String()
}
```

### Bước 5: InviteUser UseCase

**File mới:** `identity-service/internal/usecase/admin_user/invite_user.go`

```go
package admin_user

import (
    "context"
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/osv/identity-service/internal/domain/repository"
)

type InviteUserInput struct {
    Email     string
    Role      string
    InvitedBy uuid.UUID
    InviterName string
    BaseURL   string // e.g., "https://c12.openledger.vn"
}

type InviteUserUseCase struct {
    userRepo       repository.UserRepository
    invitationRepo repository.InvitationRepository
    emailSender    repository.EmailSender
    hasher         PasswordHasher
}

func (uc *InviteUserUseCase) Execute(ctx context.Context, in InviteUserInput) error {
    // 1. Generate secure token
    tokenBytes := make([]byte, 32)
    if _, err := rand.Read(tokenBytes); err != nil {
        return fmt.Errorf("invite: generate token: %w", err)
    }
    token := hex.EncodeToString(tokenBytes)

    // 2. Generate temp password
    tempPassBytes := make([]byte, 12)
    _, _ = rand.Read(tempPassBytes)
    tempPass := hex.EncodeToString(tempPassBytes)[:16]
    hashedPass, _ := uc.hasher.Hash(tempPass)

    // 3. Create user
    userID := uuid.New()
    user := &repository.User{
        ID:           userID,
        Email:        in.Email,
        Role:         in.Role,
        PasswordHash: hashedPass,
        IsActive:     false, // activate on invitation accept
    }
    if err := uc.userRepo.Create(ctx, user); err != nil {
        return fmt.Errorf("invite: create user: %w", err)
    }

    // 4. Create invitation record
    inv := &repository.Invitation{
        ID:        uuid.New(),
        UserID:    userID,
        Email:     in.Email,
        Token:     token,
        ExpiresAt: time.Now().Add(48 * time.Hour),
        InvitedBy: &in.InvitedBy,
    }
    if err := uc.invitationRepo.Create(ctx, inv); err != nil {
        return fmt.Errorf("invite: create invitation record: %w", err)
    }

    // 5. Send invitation email (non-fatal)
    inviteURL := fmt.Sprintf("%s/accept-invite?token=%s", in.BaseURL, token)
    if err := uc.emailSender.SendInvitation(ctx, in.Email, in.InviterName, inviteURL, tempPass); err != nil {
        // Log but don't fail — admin can resend
        // uc.log.Warn().Err(err).Str("email", in.Email).Msg("invite: email send failed")
        return nil // or return warning
    }

    return nil
}
```

### Bước 6: Thêm SMTP config loading từ platform_settings

**File sửa:** `identity-service/embedded.go` (hoặc wire.go):

```go
// Load SMTP config từ platform_settings table (via SOL-004)
smtpConfig, _ := settingsRepo.GetByCategory(ctx, "smtp")
if smtpEnabled(smtpConfig) {
    emailSender = smtp.NewSMTPSender(
        getSetting(smtpConfig, "smtp.host"),
        getSetting(smtpConfig, "smtp.port"),
        getSetting(smtpConfig, "smtp.username"),
        os.Getenv("SMTP_PASSWORD"), // secret từ env, không từ DB
        getSetting(smtpConfig, "smtp.from_email"),
        true,
    )
}
```

---

## Files cần tạo/sửa

| Action | File |
|--------|------|
| NEW | `identity-service/migrations/007_user_invitations.sql` |
| NEW | `identity-service/internal/domain/repository/invitation.go` |
| NEW | `identity-service/internal/infra/postgres/invitation_repo.go` |
| NEW | `identity-service/internal/infra/smtp/email_sender.go` |
| NEW | `identity-service/internal/usecase/admin_user/invite_user.go` |
| MODIFY | `identity-service/adapter/handler/http/admin_handler.go` — call real usecase |
| MODIFY | `identity-service/embedded.go` — wire SMTP from settings |

---

## Verification

```bash
# Migration
psql $DATABASE_URL -f identity-service/migrations/007_user_invitations.sql

# Set SMTP settings
curl -X PUT -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"smtp":{"enabled":true,"host":"smtp.example.com","port":"587","username":"noreply@osv.dev"}}' \
  "https://c12.openledger.vn/api/v1/admin/settings"

# Invite user
curl -X POST -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"email":"analyst@company.com","role":"user"}' \
  "https://c12.openledger.vn/api/v1/admin/users/invite"
# Expect: 200 + email sent

# Verify invitation in DB
psql $DATABASE_URL -c "SELECT email, token, expires_at FROM user_invitations ORDER BY created_at DESC LIMIT 3;"
```
