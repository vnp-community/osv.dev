# SOL-V6-002: Fix 500 — Admin User Invite

**Bugs:** BUG-V6-021  
**Task:** TASK-V6-002  
**Service:** `identity-service` (:8081)  
**Kiến trúc tham chiếu:** `01-architecture.md §3.4`, `02-technical-design.md §10`, `§14.1`

---

## Root Cause Analysis

```
POST /api/v1/admin/users/invite →  500
{"error":"INTERNAL_ERROR","message":"Failed to create invitation"}
```

Theo kiến trúc (`01-architecture.md §3.1`):
```
Sprint 1: /api/v1/admin/users/*, /api/v1/admin/roles → identity-service:8081 | Admin
```

Lỗi "Failed to create invitation" xảy ra ở usecase layer, có thể do:
1. **DB**: Bảng `user_invitations` chưa tồn tại
2. **SMTP**: Email service chưa cấu hình — handler không handle gracefully khi SMTP fails
3. **Conflict**: unique constraint violation không được catch

---

## Solution

### Fix 1: DB Migration — Bảng `user_invitations`

**File:** `services/identity-service/migrations/XXX_add_user_invitations.sql`

```sql
CREATE TABLE IF NOT EXISTS user_invitations (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email      VARCHAR(255) NOT NULL UNIQUE,
    role       VARCHAR(50)  NOT NULL DEFAULT 'readonly',
    name       VARCHAR(255),
    token      CHAR(64)     NOT NULL UNIQUE,  -- SHA-256 của invitation token
    invited_by UUID NOT NULL REFERENCES users(id),
    accepted   BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '7 days',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_user_invitations_email ON user_invitations(email);
CREATE INDEX idx_user_invitations_token ON user_invitations(token);
```

### Fix 2: Domain Entity

**File:** `services/identity-service/internal/domain/entity/invitation.go`

```go
package entity

import (
    "crypto/rand"
    "crypto/sha256"
    "encoding/hex"
    "time"
    "github.com/google/uuid"
)

type Invitation struct {
    ID        uuid.UUID  `db:"id"`
    Email     string     `db:"email"`
    Role      string     `db:"role"`
    Name      string     `db:"name"`
    Token     string     `db:"token"`  // SHA-256 hash stored in DB
    PlainToken string    `db:"-"`      // raw token — chỉ trả về 1 lần khi tạo
    InvitedBy uuid.UUID  `db:"invited_by"`
    Accepted  bool       `db:"accepted"`
    ExpiresAt time.Time  `db:"expires_at"`
    CreatedAt time.Time  `db:"created_at"`
}

func NewInvitation(email, role, name string, invitedBy uuid.UUID) (*Invitation, error) {
    // Generate 32-byte random token
    raw := make([]byte, 32)
    if _, err := rand.Read(raw); err != nil {
        return nil, fmt.Errorf("generate token: %w", err)
    }
    plainToken := hex.EncodeToString(raw)
    
    // Store SHA-256 hash only
    h := sha256.Sum256([]byte(plainToken))
    tokenHash := hex.EncodeToString(h[:])
    
    return &Invitation{
        ID:         uuid.New(),
        Email:      email,
        Role:       role,
        Name:       name,
        Token:      tokenHash,
        PlainToken: plainToken,
        InvitedBy:  invitedBy,
        ExpiresAt:  time.Now().UTC().Add(7 * 24 * time.Hour),
        CreatedAt:  time.Now().UTC(),
    }, nil
}

func (i *Invitation) IsExpired() bool {
    return time.Now().UTC().After(i.ExpiresAt)
}
```

### Fix 3: Use Case — Decouple Email từ Invite Creation

Vấn đề chính: nếu SMTP fails → toàn bộ invitation fails → 500.  
**Giải pháp**: Tách invitation creation (sync) khỏi email sending (async/best-effort).

**File:** `services/identity-service/internal/usecase/admin/invite_user.go`

```go
package admin

type InviteUserInput struct {
    Email string `json:"email" validate:"required,email"`
    Role  string `json:"role"  validate:"required,oneof=admin user readonly"`
    Name  string `json:"name"`
}

type InviteUserOutput struct {
    InvitationID string    `json:"invitation_id"`
    Email        string    `json:"email"`
    Role         string    `json:"role"`
    ExpiresAt    time.Time `json:"expires_at"`
    // InviteLink chỉ trả về trong response (không lưu DB)
    InviteLink   string    `json:"invite_link,omitempty"`
}

type InviteUserUseCase struct {
    invitationRepo InvitationRepository
    userRepo       UserRepository
    emailSender    EmailSender  // Optional — nil nếu SMTP chưa cấu hình
    appBaseURL     string
    log            zerolog.Logger
}

func (uc *InviteUserUseCase) Execute(ctx context.Context, in InviteUserInput, inviterID uuid.UUID) (*InviteUserOutput, error) {
    // 1. Validate role
    if !isValidRole(in.Role) {
        return nil, ErrInvalidInput
    }

    // 2. Check nếu email đã tồn tại (user hoặc pending invitation)
    if existing, _ := uc.userRepo.FindByEmail(ctx, in.Email); existing != nil {
        return nil, fmt.Errorf("%w: user with email already exists", ErrConflict)
    }

    // 3. Tạo invitation entity
    invitation, err := entity.NewInvitation(in.Email, in.Role, in.Name, inviterID)
    if err != nil {
        return nil, fmt.Errorf("create invitation: %w", err)
    }

    // 4. Persist — tạo DB record trước
    if err := uc.invitationRepo.Create(ctx, invitation); err != nil {
        // Handle unique violation gracefully
        if isUniqueViolation(err) {
            return nil, fmt.Errorf("%w: invitation for this email already pending", ErrConflict)
        }
        return nil, fmt.Errorf("save invitation: %w", err)
    }

    // 5. Build invite link
    inviteLink := fmt.Sprintf("%s/accept-invite?token=%s", uc.appBaseURL, invitation.PlainToken)

    // 6. Send email — ASYNC + best-effort (không làm fail request)
    if uc.emailSender != nil {
        go func() {
            emailCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
            defer cancel()
            if err := uc.emailSender.SendInvite(emailCtx, SendInviteEmail{
                To:         invitation.Email,
                Name:       invitation.Name,
                InviteLink: inviteLink,
                InviterID:  inviterID,
                ExpiresAt:  invitation.ExpiresAt,
            }); err != nil {
                // Log warning — không fail request
                uc.log.Warn().Err(err).
                    Str("email", invitation.Email).
                    Msg("invitation email failed — invitation still created")
            }
        }()
    } else {
        uc.log.Warn().
            Str("email", invitation.Email).
            Msg("SMTP not configured — invitation created without email")
    }

    return &InviteUserOutput{
        InvitationID: invitation.ID.String(),
        Email:        invitation.Email,
        Role:         invitation.Role,
        ExpiresAt:    invitation.ExpiresAt,
        // Trả về invite link trong response khi SMTP chưa cấu hình
        InviteLink: func() string {
            if uc.emailSender == nil {
                return inviteLink  // FE có thể copy/share manually
            }
            return ""
        }(),
    }, nil
}
```

### Fix 4: HTTP Handler

**File:** `services/identity-service/internal/delivery/http/admin_handler.go`

```go
// POST /admin/users/invite
func (h *AdminHandler) InviteUser(w http.ResponseWriter, r *http.Request) {
    inviterID := extractUserID(r)

    var input admin.InviteUserInput
    if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    // Validate
    if err := h.validator.Struct(input); err != nil {
        writeValidationError(w, err)
        return
    }

    result, err := h.inviteUserUC.Execute(r.Context(), input, inviterID)
    if err != nil {
        switch {
        case errors.Is(err, domain.ErrConflict):
            writeError(w, http.StatusConflict, err.Error())
        case errors.Is(err, domain.ErrInvalidInput):
            writeError(w, http.StatusBadRequest, err.Error())
        default:
            h.log.Error().Err(err).Msg("invite user failed")
            writeError(w, http.StatusInternalServerError, "failed to create invitation")
        }
        return
    }

    writeJSON(w, http.StatusCreated, result)
}
```

### Fix 5: Config — SMTP optional

**File:** `services/identity-service/internal/config/config.go`

```go
type Config struct {
    // ... existing fields ...
    
    // SMTP — optional. Nếu không cấu hình, invitation email bị skip
    SMTP SMTPConfig `env:",prefix=SMTP_"`
}

type SMTPConfig struct {
    Host     string `env:"HOST"`     // e.g. smtp.gmail.com
    Port     int    `env:"PORT"`     // e.g. 587
    Username string `env:"USERNAME"` // e.g. noreply@company.com
    Password string `env:"PASSWORD"`
    From     string `env:"FROM"`     // Display name + email
    Enabled  bool   `env:"ENABLED"`  // Explicitly enable/disable
}

func (c SMTPConfig) IsConfigured() bool {
    return c.Enabled && c.Host != "" && c.Username != "" && c.Password != ""
}
```

**File:** `deploy/dev/configs/identity-service.yaml` — thêm optional SMTP config

```yaml
smtp:
  enabled: false  # Set true khi có SMTP server
  host: ""
  port: 587
  username: ""
  password: ""
  from: "OSV Platform <noreply@osv.local>"
```

---

## Expected Response

### POST /api/v1/admin/users/invite → 201

```json
{
  "invitation_id": "uuid",
  "email": "newuser@company.com",
  "role": "readonly",
  "expires_at": "2026-07-01T00:00:00Z",
  "invite_link": "https://app/accept-invite?token=abc123"
}
```

> **Note:** `invite_link` chỉ có trong response khi SMTP không được cấu hình.

### POST → 409 (email đã tồn tại)
```json
{"error": "CONFLICT", "message": "invitation for this email already pending"}
```

---

## Verification

```bash
# Test invite (SMTP not configured → returns invite_link)
curl -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","role":"readonly","name":"Test User"}' \
  https://c12.openledger.vn/api/v1/admin/users/invite
# Expected: 201 {"invitation_id": "...", "email": "...", "invite_link": "..."}

# Test duplicate (409)
# Run same command again → 409 Conflict
```
