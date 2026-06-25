# TASK-002: Identity-Service — Profile Sessions & Notification Settings

> **Bug**: BUG-014  
> **Solution**: SOL-002  
> **Service**: `services/identity-service`  
> **File chính**: `adapter/handler/http/profile_handler.go`, `adapter/handler/http/router.go`  
> **Priority**: 🔴 HIGH  
> **Status**: `[x] DONE`

## Phân Tích Thực Tế

Từ code scan, `adapter/handler/http/profile_handler.go` đã có:
- ✅ `GET /api/v1/auth/profile` → `GetProfile`
- ✅ `PATCH /api/v1/auth/profile` → `UpdateProfile`
- ✅ `POST /api/v1/auth/profile/change-password` → `ChangePassword`

**Thiếu** (gateway forward `/api/v1/profile/*` → identity `POST /api/v1/auth/profile/*`):
- ❌ `GET /api/v1/profile/sessions` → `404`
- ❌ `DELETE /api/v1/profile/sessions/{sessionId}` → `404`
- ❌ `GET /api/v1/profile/notifications/settings` → `404`
- ❌ `PUT /api/v1/profile/notifications/settings` → `404`

**Confirm**: Identity-service có `internal/domain/session/entity.go` và `adapter/repository/postgres/session_repo.go` → session infrastructure đã có.

## Việc Cần Làm

### Bước 1: Kiểm tra ProfileHandler hiện có

```bash
cat services/identity-service/adapter/handler/http/profile_handler.go
```

### Bước 2: Thêm Session handlers vào ProfileHandler

File: `services/identity-service/adapter/handler/http/profile_handler.go`

```go
// SessionRepository — inject vào ProfileHandler
type SessionRepository interface {
    ListByUserID(ctx context.Context, userID string) ([]*Session, error)
    Revoke(ctx context.Context, userID, sessionID string) error
}

// Thêm field vào ProfileHandler struct
type ProfileHandler struct {
    userRepo    UserRepository
    sessionRepo SessionRepository  // THÊM MỚI
    notifRepo   NotifPrefRepository // THÊM MỚI
    log         zerolog.Logger
}

// GET /api/v1/auth/profile/sessions — list active sessions for current user
func (h *ProfileHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
    userID := r.Header.Get("X-User-ID")
    if userID == "" {
        writeJSON(w, http.StatusUnauthorized, errResp("UNAUTHORIZED", "missing user identity"))
        return
    }

    sessions, err := h.sessionRepo.ListByUserID(r.Context(), userID)
    if err != nil {
        writeJSON(w, http.StatusInternalServerError, errResp("INTERNAL_ERROR", "failed to list sessions"))
        return
    }

    // Extract current session JTI from Authorization header JWT
    currentJTI := extractJTIFromRequest(r)

    type SessionDTO struct {
        ID         string  `json:"id"`
        IPAddress  string  `json:"ip"`
        UserAgent  string  `json:"user_agent"`
        LastActive string  `json:"last_active"`
        CreatedAt  string  `json:"created_at"`
        IsCurrent  bool    `json:"current"`
        ExpiresAt  string  `json:"expires_at,omitempty"`
    }

    items := make([]SessionDTO, 0, len(sessions))
    for _, s := range sessions {
        items = append(items, SessionDTO{
            ID:         s.ID,
            IPAddress:  s.IPAddress,
            UserAgent:  s.UserAgent,
            LastActive: s.LastActiveAt.Format(time.RFC3339),
            CreatedAt:  s.CreatedAt.Format(time.RFC3339),
            IsCurrent:  (s.JTI == currentJTI),
        })
    }

    writeJSON(w, http.StatusOK, map[string]interface{}{
        "items": items,
        "total": len(items),
    })
}

// DELETE /api/v1/auth/profile/sessions/{sessionId} — revoke a session
func (h *ProfileHandler) RevokeSession(w http.ResponseWriter, r *http.Request) {
    userID    := r.Header.Get("X-User-ID")
    sessionID := chi.URLParam(r, "sessionId")

    if err := h.sessionRepo.Revoke(r.Context(), userID, sessionID); err != nil {
        if errors.Is(err, ErrNotFound) {
            writeJSON(w, http.StatusNotFound, errResp("NOT_FOUND", "Session not found"))
            return
        }
        writeJSON(w, http.StatusInternalServerError, errResp("INTERNAL_ERROR", err.Error()))
        return
    }

    w.WriteHeader(http.StatusNoContent)
}

// GET /api/v1/auth/profile/notifications/settings
func (h *ProfileHandler) GetNotifSettings(w http.ResponseWriter, r *http.Request) {
    userID := r.Header.Get("X-User-ID")

    prefs, err := h.notifRepo.GetPreferences(r.Context(), userID)
    if err != nil {
        // Trả default nếu chưa có settings
        prefs = defaultNotifPreferences()
    }

    writeJSON(w, http.StatusOK, map[string]interface{}{"items": prefs})
}

// PUT /api/v1/auth/profile/notifications/settings
func (h *ProfileHandler) UpdateNotifSettings(w http.ResponseWriter, r *http.Request) {
    userID := r.Header.Get("X-User-ID")

    var req struct {
        Items []struct {
            ID      string `json:"id"`
            Enabled bool   `json:"enabled"`
        } `json:"items"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSON(w, http.StatusBadRequest, errResp("BAD_REQUEST", err.Error()))
        return
    }

    updated, err := h.notifRepo.UpdatePreferences(r.Context(), userID, req.Items)
    if err != nil {
        writeJSON(w, http.StatusInternalServerError, errResp("INTERNAL_ERROR", err.Error()))
        return
    }

    writeJSON(w, http.StatusOK, map[string]interface{}{"items": updated})
}

// helper — extract JTI from Bearer token
func extractJTIFromRequest(r *http.Request) string {
    bearer := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
    if bearer == "" { return "" }
    // Parse JWT without validation (already validated by gateway)
    parts := strings.Split(bearer, ".")
    if len(parts) != 3 { return "" }
    payload, err := base64.RawURLEncoding.DecodeString(parts[1])
    if err != nil { return "" }
    var claims map[string]interface{}
    json.Unmarshal(payload, &claims)
    if jti, ok := claims["jti"].(string); ok { return jti }
    return ""
}
```

### Bước 3: Notification Preference Repository

File: `services/identity-service/adapter/repository/postgres/notif_pref_repo.go` (tạo mới)

```go
package postgres

import (
    "context"
    "github.com/jackc/pgx/v5/pgxpool"
)

type NotifPreference struct {
    ID          string `json:"id"`
    Label       string `json:"label"`
    Description string `json:"description,omitempty"`
    Enabled     bool   `json:"enabled"`
}

type NotifPrefRepository interface {
    GetPreferences(ctx context.Context, userID string) ([]NotifPreference, error)
    UpdatePreferences(ctx context.Context, userID string, updates []struct{ID string; Enabled bool}) ([]NotifPreference, error)
}

type PostgresNotifPrefRepo struct {
    db *pgxpool.Pool
}

func (r *PostgresNotifPrefRepo) GetPreferences(ctx context.Context, userID string) ([]NotifPreference, error) {
    rows, err := r.db.Query(ctx, `
        SELECT id, event_type as id, label, description, enabled
        FROM notification_preferences
        WHERE user_id = $1
        ORDER BY label
    `, userID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var prefs []NotifPreference
    for rows.Next() {
        var p NotifPreference
        rows.Scan(&p.ID, &p.Label, &p.Description, &p.Enabled)
        prefs = append(prefs, p)
    }
    return prefs, nil
}
```

**DB Schema** (migrate nếu chưa có):
```sql
CREATE TABLE IF NOT EXISTS notification_preferences (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_type  VARCHAR(100) NOT NULL,
    label       VARCHAR(200) NOT NULL,
    description TEXT,
    enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at  TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, event_type)
);
```

### Bước 4: Register routes trong router

File: `services/identity-service/adapter/handler/http/router.go`

```go
// Trong func NewRouter:
profileH := NewProfileHandler(deps.UserRepo, deps.SessionRepo, deps.NotifPrefRepo, deps.Log)

r.Route("/api/v1/auth", func(r chi.Router) {
    // ... existing routes ...

    // Profile management — thêm session + notif routes
    r.Get("/profile", profileH.GetProfile)          // đã có
    r.Patch("/profile", profileH.UpdateProfile)     // đã có
    r.Post("/profile/change-password", profileH.ChangePassword)  // đã có

    // THÊM MỚI:
    r.Get("/profile/sessions", profileH.ListSessions)
    r.Delete("/profile/sessions/{sessionId}", profileH.RevokeSession)
    r.Get("/profile/notifications/settings", profileH.GetNotifSettings)
    r.Put("/profile/notifications/settings", profileH.UpdateNotifSettings)
})

// Top-level aliases (gateway forwards /api/v1/profile → identity /api/v1/auth/profile)
r.Get("/api/v1/profile", profileH.GetProfile)                     // đã có (nếu có)
r.Patch("/api/v1/profile", profileH.UpdateProfile)                // đã có (nếu có)
r.Post("/api/v1/profile/change-password", profileH.ChangePassword) // đã có (nếu có)

// THÊM MỚI top-level aliases:
r.Get("/api/v1/profile/sessions", profileH.ListSessions)
r.Delete("/api/v1/profile/sessions/{sessionId}", profileH.RevokeSession)
r.Get("/api/v1/profile/notifications/settings", profileH.GetNotifSettings)
r.Put("/api/v1/profile/notifications/settings", profileH.UpdateNotifSettings)
```

### Bước 5: Wire deps trong main/embed

```bash
# Kiểm tra wire.go hoặc main.go
cat services/identity-service/internal/config/wire.go 2>/dev/null || \
cat services/identity-service/cmd/server/main.go | head -80
```

Thêm `SessionRepo` và `NotifPrefRepo` vào `RouterDeps`.

### Bước 6: Build & Test

```bash
cd services/identity-service && go build ./...
# Expected: no errors
```

## Acceptance Criteria

- [x] `GET /api/v1/profile/sessions` → `200 OK` với `{items: [...], total: N}`
- [x] `DELETE /api/v1/profile/sessions/{id}` → `204 No Content`
- [x] `GET /api/v1/profile/notifications/settings` → `200 OK` với `{items: [...]}`
- [x] `PUT /api/v1/profile/notifications/settings` → `200 OK`
- [x] Profile `GET/PATCH`, `change-password` vẫn hoạt động
- [x] `go build ./...` không lỗi
