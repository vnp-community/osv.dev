# SOL-002: Identity-Service — MFA & Profile Management

> **Bugs giải quyết**: BUG-001 (MFA), BUG-014 (Profile)  
> **Service**: `services/identity-service`  
> **Port**: 8081  
> **Architecture ref**: §3.4 Identity-Service  
> **Status**: `[x] DONE`

## Kết Quả Thực Thi

**Đã hoàn thành trong identity-service:**

| Fix | File | Trạng thái |
|---|---|---|
| `GET /api/v1/auth/mfa/setup` handler (TOTP) | `adapter/handler/http/totp_handler.go` | ✅ Đã có |
| `GET /api/v1/profile/sessions` handler | `adapter/handler/http/profile_handler.go` | ✅ Đã có |
| `DELETE /api/v1/profile/sessions/{id}` handler | `adapter/handler/http/profile_handler.go` | ✅ Đã có |
| `GET /api/v1/profile/notifications/settings` handler | `adapter/handler/http/profile_handler.go` | ✅ Đã có |
| `PUT /api/v1/profile/notifications/settings` handler | `adapter/handler/http/profile_handler.go` | ✅ Đã có |
| Thêm import `postgres` vào `router.go` (fix `undefined: postgres`) | `adapter/handler/http/router.go` | ✅ Fixed |
| Thêm `type Role = string`, `RoleUser`, `Permissions()` vào entity | `internal/domain/entity/user.go` | ✅ Fixed |
| Thêm `UpdatedAt` + đổi `TokenFamily string→uuid.UUID` trong Session | `internal/domain/entity/session.go` | ✅ Fixed |
| Fix MFATOTPSecret *string↔string trong mongo repo | `adapter/repository/mongo/user_repo.go` | ✅ Fixed |
| Fix `RevokeByFamily(uuid.UUID)` interface + implementations | `internal/domain/repository/repositories.go` | ✅ Fixed |
| Fix `handlers.go` dùng đúng types và domain errors | `internal/delivery/http/handlers.go` | ✅ Fixed |

**Build verify**: `go build ./...` ✅ identity-service


## Phân Tích

Theo `01-architecture.md` §3.4:
- TOTP MFA đã được thiết kế với paths `/api/v1/auth/totp/setup`, `/api/v1/auth/totp/verify`, `DELETE /api/v1/auth/totp`
- Profile management (`/api/v1/profile/*`) được routing đến identity-service (Sprint 1 table)
- Schema `osv_identity` đã có tables: `users`, `api_keys`, `sessions`, `ldap_configs`

**Vấn đề**: Identity-service có thể chưa implement một số HTTP handlers, hoặc gateway chưa route đến đúng paths.

---

## BUG-001: MFA Setup/Confirm

### Phân Tích Sâu

Architecture chỉ rõ:
```
GET  /api/v1/auth/mfa/setup   → /api/v1/auth/totp/setup  (alias, BFF rewrite)
POST /api/v1/auth/mfa/confirm → /api/v1/auth/totp/verify (alias, BFF rewrite)
```

Nghĩa là gateway cần rewrite path `/auth/mfa/*` → `/auth/totp/*` rồi forward đến identity-service. Identity-service đã có TOTP implementation (`internal/provider/` + TOTP RFC 6238).

### Giải Pháp

**Bước 1**: Gateway rewrite (xem SOL-001)

**Bước 2**: Verify identity-service TOTP handlers tồn tại

```bash
# Kiểm tra handlers
find services/identity-service -name "*.go" | xargs grep -l "totp\|mfa" 2>/dev/null
grep -r "totp/setup\|totp/verify" services/identity-service/
```

**Bước 3**: Nếu chưa có, thêm HTTP handler trong identity-service:

```go
// services/identity-service/internal/delivery/http/auth_handler.go

// GET /api/v1/auth/totp/setup
func (h *AuthHandler) TOTPSetup(w http.ResponseWriter, r *http.Request) {
    userID := r.Header.Get("X-User-ID")  // Injected by gateway
    
    // Generate TOTP secret
    secret, qrURL, err := h.totpUC.GenerateSecret(r.Context(), userID)
    if err != nil {
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "secret":       secret,
        "qr_url":       qrURL,
        "backup_codes": h.totpUC.GenerateBackupCodes(r.Context(), userID),
    })
}

// POST /api/v1/auth/totp/verify
func (h *AuthHandler) TOTPVerify(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Code string `json:"code"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, http.StatusBadRequest, err)
        return
    }
    
    userID := r.Header.Get("X-User-ID")
    
    if err := h.totpUC.Verify(r.Context(), userID, req.Code); err != nil {
        respondError(w, http.StatusBadRequest, map[string]string{
            "error": "Invalid TOTP code",
        })
        return
    }
    
    // Enable MFA for user
    h.totpUC.Enable(r.Context(), userID)
    
    respondJSON(w, http.StatusOK, map[string]bool{"enabled": true})
}
```

**Bước 4**: Route trong identity-service router:

```go
// services/identity-service/internal/delivery/http/router.go
r.GET("/api/v1/auth/totp/setup",   authMiddleware(h.TOTPSetup))
r.POST("/api/v1/auth/totp/verify", authMiddleware(h.TOTPVerify))
r.DELETE("/api/v1/auth/totp",      authMiddleware(h.TOTPDisable))
```

---

## BUG-014: Profile Management (6 endpoints)

### Phân Tích

Identity-service quản lý `osv_identity` schema với tables:
- `users` — thông tin user cơ bản
- `sessions` — active sessions (JWT store)
- `notification_preferences` — có thể chưa có table này

Cần implement 6 HTTP handlers trong identity-service.

### Schema Database Bổ Sung

```sql
-- Thêm vào osv_identity schema nếu chưa có

-- Profile extended fields
ALTER TABLE users ADD COLUMN IF NOT EXISTS department    VARCHAR(100);
ALTER TABLE users ADD COLUMN IF NOT EXISTS job_title     VARCHAR(100);
ALTER TABLE users ADD COLUMN IF NOT EXISTS phone         VARCHAR(30);
ALTER TABLE users ADD COLUMN IF NOT EXISTS timezone      VARCHAR(50) DEFAULT 'UTC';
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url    TEXT;

-- Notification preferences (per-user, per-event-type)
CREATE TABLE IF NOT EXISTS notification_preferences (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_type  VARCHAR(100) NOT NULL,  -- e.g., "critical_findings", "kev_new"
    label       VARCHAR(200) NOT NULL,
    description TEXT,
    enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at  TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, event_type)
);

-- Seed default notification preferences
INSERT INTO notification_preferences (user_id, event_type, label, enabled)
SELECT id, 'critical_findings', 'Critical Finding Alerts', true FROM users
ON CONFLICT DO NOTHING;
```

### HTTP Handlers

```go
// services/identity-service/internal/delivery/http/profile_handler.go

// GET /api/v1/profile
func (h *ProfileHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
    userID := r.Header.Get("X-User-ID")
    
    user, err := h.userUC.GetByID(r.Context(), userID)
    if err != nil {
        respondError(w, http.StatusNotFound, err)
        return
    }
    
    respondJSON(w, http.StatusOK, ProfileResponse{
        ID:         user.ID,
        Name:       user.Name,
        Email:      user.Email,
        Role:       user.Role,
        Department: user.Department,
        JobTitle:   user.JobTitle,
        Phone:      user.Phone,
        Timezone:   user.Timezone,
        MFAEnabled: user.MFAEnabled,
        AvatarURL:  user.AvatarURL,
        CreatedAt:  user.CreatedAt,
    })
}

// PATCH /api/v1/profile
func (h *ProfileHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
    userID := r.Header.Get("X-User-ID")
    
    var req UpdateProfileRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, http.StatusBadRequest, err)
        return
    }
    
    updated, err := h.userUC.UpdateProfile(r.Context(), userID, req)
    if err != nil {
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    
    // Publish audit event
    h.nats.PublishJSON("user.profile.updated", map[string]string{
        "user_id": userID,
        "action":  "profile.updated",
    })
    
    respondJSON(w, http.StatusOK, updated)
}

// POST /api/v1/profile/change-password
func (h *ProfileHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
    userID := r.Header.Get("X-User-ID")
    
    var req struct {
        CurrentPassword string `json:"current_password"`
        NewPassword     string `json:"new_password"`
        ConfirmPassword string `json:"confirm_password"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    
    if req.NewPassword != req.ConfirmPassword {
        respondError(w, http.StatusBadRequest, "passwords do not match")
        return
    }
    
    if err := h.userUC.ChangePassword(r.Context(), userID, req.CurrentPassword, req.NewPassword); err != nil {
        if errors.Is(err, ErrInvalidCredentials) {
            respondError(w, http.StatusBadRequest, "Current password incorrect")
            return
        }
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    
    respondJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// GET /api/v1/profile/sessions
func (h *ProfileHandler) GetSessions(w http.ResponseWriter, r *http.Request) {
    userID := r.Header.Get("X-User-ID")
    currentJTI := extractJTI(r)  // from JWT claims
    
    sessions, err := h.sessionUC.ListByUser(r.Context(), userID)
    if err != nil {
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    
    // Mark current session
    for i := range sessions {
        sessions[i].IsCurrent = (sessions[i].JTI == currentJTI)
    }
    
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "items": sessions,
        "total": len(sessions),
    })
}

// DELETE /api/v1/profile/sessions/{sessionId}
func (h *ProfileHandler) RevokeSession(w http.ResponseWriter, r *http.Request) {
    userID    := r.Header.Get("X-User-ID")
    sessionID := r.PathValue("sessionId")
    
    if err := h.sessionUC.Revoke(r.Context(), userID, sessionID); err != nil {
        respondError(w, http.StatusNotFound, err)
        return
    }
    
    w.WriteHeader(http.StatusNoContent)
}

// GET /api/v1/profile/notifications/settings
func (h *ProfileHandler) GetNotifSettings(w http.ResponseWriter, r *http.Request) {
    userID := r.Header.Get("X-User-ID")
    
    prefs, err := h.notifUC.GetPreferences(r.Context(), userID)
    if err != nil {
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    
    respondJSON(w, http.StatusOK, map[string]interface{}{"items": prefs})
}

// PUT /api/v1/profile/notifications/settings
func (h *ProfileHandler) UpdateNotifSettings(w http.ResponseWriter, r *http.Request) {
    userID := r.Header.Get("X-User-ID")
    
    var req struct {
        Items []struct {
            ID      string `json:"id"`
            Enabled bool   `json:"enabled"`
        } `json:"items"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    
    updated, err := h.notifUC.UpdatePreferences(r.Context(), userID, req.Items)
    if err != nil {
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    
    respondJSON(w, http.StatusOK, map[string]interface{}{"items": updated})
}
```

### Router Registration

```go
// services/identity-service/internal/delivery/http/router.go

profileHandler := NewProfileHandler(...)

r.GET("/api/v1/profile",                         authMiddleware(profileHandler.GetProfile))
r.PATCH("/api/v1/profile",                        authMiddleware(profileHandler.UpdateProfile))
r.POST("/api/v1/profile/change-password",         authMiddleware(profileHandler.ChangePassword))
r.GET("/api/v1/profile/sessions",                 authMiddleware(profileHandler.GetSessions))
r.DELETE("/api/v1/profile/sessions/{sessionId}",  authMiddleware(profileHandler.RevokeSession))
r.GET("/api/v1/profile/notifications/settings",   authMiddleware(profileHandler.GetNotifSettings))
r.PUT("/api/v1/profile/notifications/settings",   authMiddleware(profileHandler.UpdateNotifSettings))
```

## Response Schemas

```go
type ProfileResponse struct {
    ID         string  `json:"id"`
    Name       string  `json:"name"`
    Email      string  `json:"email"`
    Role       string  `json:"role"`
    Department string  `json:"department,omitempty"`
    JobTitle   string  `json:"job_title,omitempty"`
    Phone      string  `json:"phone,omitempty"`
    Timezone   string  `json:"timezone"`
    MFAEnabled bool    `json:"mfa_enabled"`
    AvatarURL  *string `json:"avatar_url"`
    CreatedAt  string  `json:"created_at"`
}

type SessionResponse struct {
    ID         string `json:"id"`
    JTI        string `json:"jti"`
    Device     string `json:"device"`
    IPAddress  string `json:"ip"`
    Location   string `json:"location,omitempty"`
    LastActive string `json:"last_active"`
    IsCurrent  bool   `json:"current"`
}

type NotifPreferenceResponse struct {
    ID          string `json:"id"`
    Label       string `json:"label"`
    Description string `json:"desc,omitempty"`
    Enabled     bool   `json:"enabled"`
}
```
