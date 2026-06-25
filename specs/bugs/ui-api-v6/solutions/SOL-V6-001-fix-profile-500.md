# SOL-V6-001: Fix 500 — Profile Sessions & Notification Settings

**Bugs:** BUG-V6-018, BUG-V6-019, BUG-V6-020  
**Task:** TASK-V6-001  
**Service:** `identity-service` (:8081) → proxied qua `apps/osv` gateway  
**Kiến trúc tham chiếu:** `01-architecture.md §3.4`, `02-technical-design.md §10`, `§14.1`

---

## Root Cause Analysis

Theo kiến trúc, `identity-service` (:8081) quản lý toàn bộ `/api/v1/profile/*` và `/api/v1/auth/*`.
Route group (từ `01-architecture.md §3.1`):
```
Sprint 1: /api/v1/profile, /api/v1/api-keys → identity-service:8081 | JWT/APIKey
```

3 endpoints bị 500 với empty body:
- `GET /api/v1/profile/sessions` 
- `GET /api/v1/profile/notifications/settings`
- `PUT /api/v1/profile/notifications/settings`

Empty body (không có JSON error) nghĩa là **handler panic hoặc nil pointer dereference** không được recover. Đây là dấu hiệu của thiếu bảng DB hoặc handler chưa hoàn thiện.

---

## Solution

### Fix 1: DB Migration — Tạo bảng `user_sessions` và `user_notification_settings`

**Service:** `services/identity-service`  
**File:** `services/identity-service/migrations/XXX_add_sessions_notif_settings.sql`

```sql
-- Bảng lưu active sessions (cho GET /profile/sessions)
CREATE TABLE IF NOT EXISTS user_sessions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_hash CHAR(64) NOT NULL,  -- SHA-256 của refresh token
    token_family  UUID NOT NULL,           -- Detect reuse attacks
    user_agent    TEXT,
    ip_address    INET,
    last_active_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at    TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_user_sessions_user_id ON user_sessions(user_id);
CREATE INDEX idx_user_sessions_token_family ON user_sessions(token_family);

-- Bảng lưu notification preferences (cho GET/PUT /profile/notifications/settings)
CREATE TABLE IF NOT EXISTS user_notification_settings (
    id      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    -- Email preferences (JSONB cho flexibility)
    email   JSONB NOT NULL DEFAULT '{
        "finding_created": true,
        "scan_completed": true,
        "sla_breached": true,
        "kev_new": false
    }',
    -- In-app preferences
    in_app  JSONB NOT NULL DEFAULT '{
        "finding_created": true,
        "scan_completed": true,
        "sla_breached": true
    }',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### Fix 2: Repository Interface

**File:** `services/identity-service/internal/domain/interfaces.go`

```go
// SessionRepository — quản lý active sessions
type SessionRepository interface {
    // ListByUser trả về tất cả sessions đang active của user
    ListByUser(ctx context.Context, userID uuid.UUID) ([]Session, error)
    // RevokeByID thu hồi một session cụ thể
    RevokeByID(ctx context.Context, sessionID uuid.UUID, userID uuid.UUID) error
    // Save lưu session mới (được gọi sau login thành công)
    Save(ctx context.Context, s *Session) error
    // CleanExpired xóa sessions hết hạn (chạy bởi cron)
    CleanExpired(ctx context.Context) error
}

// NotificationSettingsRepository
type NotificationSettingsRepository interface {
    // GetByUser trả về settings của user, tạo default nếu chưa có
    GetByUser(ctx context.Context, userID uuid.UUID) (*NotificationSettings, error)
    // Update cập nhật settings
    Update(ctx context.Context, s *NotificationSettings) error
}
```

### Fix 3: Domain Entities

**File:** `services/identity-service/internal/domain/entity/session.go`

```go
package entity

import (
    "time"
    "github.com/google/uuid"
)

type Session struct {
    ID               uuid.UUID  `db:"id"`
    UserID           uuid.UUID  `db:"user_id"`
    RefreshTokenHash string     `db:"refresh_token_hash"`
    TokenFamily      uuid.UUID  `db:"token_family"`
    UserAgent        string     `db:"user_agent"`
    IPAddress        string     `db:"ip_address"`
    LastActiveAt     time.Time  `db:"last_active_at"`
    CreatedAt        time.Time  `db:"created_at"`
    ExpiresAt        time.Time  `db:"expires_at"`
}

func (s *Session) IsExpired() bool {
    return time.Now().UTC().After(s.ExpiresAt)
}
```

**File:** `services/identity-service/internal/domain/entity/notification_settings.go`

```go
package entity

import (
    "time"
    "github.com/google/uuid"
)

type EmailNotifPrefs struct {
    FindingCreated bool `json:"finding_created"`
    ScanCompleted  bool `json:"scan_completed"`
    SLABreached    bool `json:"sla_breached"`
    KEVNew         bool `json:"kev_new"`
}

type InAppNotifPrefs struct {
    FindingCreated bool `json:"finding_created"`
    ScanCompleted  bool `json:"scan_completed"`
    SLABreached    bool `json:"sla_breached"`
}

type NotificationSettings struct {
    ID        uuid.UUID        `db:"id"`
    UserID    uuid.UUID        `db:"user_id"`
    Email     EmailNotifPrefs  `db:"email"`
    InApp     InAppNotifPrefs  `db:"in_app"`
    UpdatedAt time.Time        `db:"updated_at"`
}

// DefaultSettings tạo settings mặc định cho user mới
func DefaultSettings(userID uuid.UUID) *NotificationSettings {
    return &NotificationSettings{
        ID:     uuid.New(),
        UserID: userID,
        Email:  EmailNotifPrefs{FindingCreated: true, ScanCompleted: true, SLABreached: true},
        InApp:  InAppNotifPrefs{FindingCreated: true, ScanCompleted: true, SLABreached: true},
    }
}
```

### Fix 4: Use Case Implementations

**File:** `services/identity-service/internal/usecase/profile/get_sessions.go`

```go
package profile

import (
    "context"
    "github.com/google/uuid"
)

type GetSessionsUseCase struct {
    sessionRepo domain.SessionRepository
}

type SessionDTO struct {
    ID           string    `json:"id"`
    UserAgent    string    `json:"user_agent"`
    IPAddress    string    `json:"ip"`
    LastActiveAt time.Time `json:"last_active_at"`
    CreatedAt    time.Time `json:"created_at"`
    ExpiresAt    time.Time `json:"expires_at"`
    IsCurrent    bool      `json:"is_current"`
}

func (uc *GetSessionsUseCase) Execute(ctx context.Context, userID uuid.UUID, currentTokenFamily uuid.UUID) ([]SessionDTO, error) {
    sessions, err := uc.sessionRepo.ListByUser(ctx, userID)
    if err != nil {
        return nil, fmt.Errorf("list sessions: %w", err)
    }

    dtos := make([]SessionDTO, 0, len(sessions))
    for _, s := range sessions {
        if s.IsExpired() {
            continue // skip expired
        }
        dtos = append(dtos, SessionDTO{
            ID:           s.ID.String(),
            UserAgent:    s.UserAgent,
            IPAddress:    s.IPAddress,
            LastActiveAt: s.LastActiveAt,
            CreatedAt:    s.CreatedAt,
            ExpiresAt:    s.ExpiresAt,
            IsCurrent:    s.TokenFamily == currentTokenFamily,
        })
    }
    return dtos, nil
}
```

**File:** `services/identity-service/internal/usecase/profile/notif_settings.go`

```go
package profile

type GetNotifSettingsUseCase struct {
    repo domain.NotificationSettingsRepository
}

func (uc *GetNotifSettingsUseCase) Execute(ctx context.Context, userID uuid.UUID) (*entity.NotificationSettings, error) {
    settings, err := uc.repo.GetByUser(ctx, userID)
    if errors.Is(err, domain.ErrNotFound) {
        // Auto-create defaults — get-or-create pattern
        defaults := entity.DefaultSettings(userID)
        if createErr := uc.repo.Create(ctx, defaults); createErr != nil {
            return nil, fmt.Errorf("create default settings: %w", createErr)
        }
        return defaults, nil
    }
    if err != nil {
        return nil, fmt.Errorf("get notif settings: %w", err)
    }
    return settings, nil
}

type UpdateNotifSettingsUseCase struct {
    repo domain.NotificationSettingsRepository
}

func (uc *UpdateNotifSettingsUseCase) Execute(ctx context.Context, userID uuid.UUID, input UpdateNotifInput) (*entity.NotificationSettings, error) {
    settings, err := uc.repo.GetByUser(ctx, userID)
    if errors.Is(err, domain.ErrNotFound) {
        settings = entity.DefaultSettings(userID)
    } else if err != nil {
        return nil, fmt.Errorf("get settings: %w", err)
    }

    // Apply updates
    if input.Email != nil {
        settings.Email = *input.Email
    }
    if input.InApp != nil {
        settings.InApp = *input.InApp
    }
    settings.UpdatedAt = time.Now().UTC()

    if err := uc.repo.Upsert(ctx, settings); err != nil {
        return nil, fmt.Errorf("upsert settings: %w", err)
    }
    return settings, nil
}
```

### Fix 5: HTTP Handlers

**File:** `services/identity-service/internal/delivery/http/profile_handler.go`

```go
// GET /profile/sessions
func (h *ProfileHandler) GetSessions(w http.ResponseWriter, r *http.Request) {
    userID := extractUserID(r)  // từ X-User-ID header (injected by gateway)
    // Extract current token family từ JWT claims (qua context)
    currentFamily := extractTokenFamily(r)

    sessions, err := h.getSessionsUC.Execute(r.Context(), userID, currentFamily)
    if err != nil {
        h.log.Error().Err(err).Msg("get sessions failed")
        writeError(w, http.StatusInternalServerError, "internal error")
        return
    }

    writeJSON(w, http.StatusOK, map[string]interface{}{
        "sessions": sessions,
        "total":    len(sessions),
    })
}

// GET /profile/notifications/settings
func (h *ProfileHandler) GetNotifSettings(w http.ResponseWriter, r *http.Request) {
    userID := extractUserID(r)
    settings, err := h.getNotifSettingsUC.Execute(r.Context(), userID)
    if err != nil {
        h.log.Error().Err(err).Msg("get notif settings failed")
        writeError(w, http.StatusInternalServerError, "internal error")
        return
    }
    writeJSON(w, http.StatusOK, settings)
}

// PUT /profile/notifications/settings
func (h *ProfileHandler) UpdateNotifSettings(w http.ResponseWriter, r *http.Request) {
    userID := extractUserID(r)

    var input UpdateNotifInput
    if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    settings, err := h.updateNotifSettingsUC.Execute(r.Context(), userID, input)
    if err != nil {
        h.log.Error().Err(err).Msg("update notif settings failed")
        writeError(w, http.StatusInternalServerError, "internal error")
        return
    }
    writeJSON(w, http.StatusOK, settings)
}
```

### Fix 6: Router Registration

**File:** `services/identity-service/internal/delivery/http/router.go`

```go
// Thêm các routes sau vào router của identity-service
mux.Handle("GET /profile/sessions",                    authMiddleware(h.GetSessions))
mux.Handle("GET /profile/notifications/settings",      authMiddleware(h.GetNotifSettings))
mux.Handle("PUT /profile/notifications/settings",      authMiddleware(h.UpdateNotifSettings))
```

---

## Expected Response Schemas

### GET /profile/sessions → 200
```json
{
  "sessions": [
    {
      "id": "uuid",
      "user_agent": "Mozilla/5.0...",
      "ip": "1.2.3.4",
      "last_active_at": "2026-06-24T09:00:00Z",
      "created_at": "2026-06-23T10:00:00Z",
      "expires_at": "2026-07-23T10:00:00Z",
      "is_current": true
    }
  ],
  "total": 1
}
```

### GET /profile/notifications/settings → 200
```json
{
  "email": {
    "finding_created": true,
    "scan_completed": true,
    "sla_breached": true,
    "kev_new": false
  },
  "in_app": {
    "finding_created": true,
    "scan_completed": true,
    "sla_breached": true
  }
}
```

---

## Verification

```bash
# Sau khi apply fix
curl -H "Authorization: Bearer $TOKEN" \
  https://c12.openledger.vn/api/v1/profile/sessions
# Expected: 200 {"sessions": [...], "total": N}

curl -H "Authorization: Bearer $TOKEN" \
  https://c12.openledger.vn/api/v1/profile/notifications/settings
# Expected: 200 {"email": {...}, "in_app": {...}}

curl -X PUT -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"email": {"finding_created": false}}' \
  https://c12.openledger.vn/api/v1/profile/notifications/settings
# Expected: 200
```
