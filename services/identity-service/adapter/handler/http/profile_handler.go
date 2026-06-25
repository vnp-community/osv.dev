package http

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/osv/identity-service/adapter/repository/postgres"
	"github.com/osv/identity-service/internal/domain/entity"
	"github.com/osv/identity-service/internal/domain/repository"
	"github.com/osv/identity-service/internal/infrastructure/crypto"
	"github.com/rs/zerolog"
)

// SessionRepository is required by ProfileHandler to manage user sessions.
type SessionRepository interface {
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]*entity.Session, error)
	RevokeSession(ctx context.Context, userID uuid.UUID, sessionID uuid.UUID) error
}

// ProfileHandler handles user self-service profile endpoints
type ProfileHandler struct {
	userRepo    repository.UserRepository
	sessionRepo SessionRepository
	notifRepo   postgres.NotifPrefRepository
	log         zerolog.Logger
}

func NewProfileHandler(userRepo repository.UserRepository, sessionRepo SessionRepository, notifRepo postgres.NotifPrefRepository, log zerolog.Logger) *ProfileHandler {
	return &ProfileHandler{userRepo: userRepo, sessionRepo: sessionRepo, notifRepo: notifRepo, log: log}
}

// GET /profile
func (h *ProfileHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(r.Header.Get("X-User-ID"))
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp("UNAUTHORIZED", "Invalid user ID"))
		return
	}

	user, err := h.userRepo.FindByID(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errResp("NOT_FOUND", "User not found"))
		return
	}

	writeJSON(w, http.StatusOK, toUserDTO(user))
}

// PATCH /profile
func (h *ProfileHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(r.Header.Get("X-User-ID"))
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp("UNAUTHORIZED", "Invalid user ID"))
		return
	}

	var req struct {
		Name *string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("VALIDATION_ERROR", "Invalid request body"))
		return
	}

	user, err := h.userRepo.FindByID(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errResp("NOT_FOUND", "User not found"))
		return
	}

	if req.Name != nil {
		user.Username = *req.Name
	}

	if err := h.userRepo.Update(r.Context(), user); err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Failed to update profile"))
		return
	}

	writeJSON(w, http.StatusOK, toUserDTO(user))
}

// POST /profile/change-password
func (h *ProfileHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(r.Header.Get("X-User-ID"))
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp("UNAUTHORIZED", "Invalid user ID"))
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("VALIDATION_ERROR", "Invalid request body"))
		return
	}
	if len(req.NewPassword) < 8 {
		writeJSON(w, http.StatusBadRequest, errResp("VALIDATION_ERROR", "New password must be at least 8 characters"))
		return
	}

	user, err := h.userRepo.FindByID(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errResp("NOT_FOUND", "User not found"))
		return
	}

	// Verify current password (using the crypto package)
	match, err := crypto.VerifyPassword(req.CurrentPassword, user.HashedPassword)
	if err != nil || !match {
		writeJSON(w, http.StatusUnauthorized, errResp("INVALID_CREDENTIALS", "Current password is incorrect"))
		return
	}

	// Hash new password
	newHash, err := crypto.HashPassword(req.NewPassword)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Failed to hash password"))
		return
	}

	user.HashedPassword = newHash
	if err := h.userRepo.Update(r.Context(), user); err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Failed to update password"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

// GET /api/v1/auth/profile/sessions — list active sessions for current user
func (h *ProfileHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
    userIDStr := r.Header.Get("X-User-ID")
    if userIDStr == "" {
        writeJSON(w, http.StatusUnauthorized, errResp("UNAUTHORIZED", "missing user identity"))
        return
    }
    userID, err := uuid.Parse(userIDStr)
    if err != nil {
        writeJSON(w, http.StatusUnauthorized, errResp("UNAUTHORIZED", "Invalid user ID"))
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
            ID:         s.ID.String(),
            IPAddress:  s.IPAddress,
            UserAgent:  s.UserAgent,
            LastActive: s.CreatedAt.Format(time.RFC3339), // since we don't have last_active_at we use created_at or we can skip
            CreatedAt:  s.CreatedAt.Format(time.RFC3339),
            IsCurrent:  (s.ID.String() == currentJTI),
        })
    }

    writeJSON(w, http.StatusOK, map[string]interface{}{
        "sessions": items,
        "total":    len(items),
    })
}

// DELETE /api/v1/auth/profile/sessions/{sessionId} — revoke a session
func (h *ProfileHandler) RevokeSession(w http.ResponseWriter, r *http.Request) {
    userIDStr := r.Header.Get("X-User-ID")
    sessionIDStr := chi.URLParam(r, "sessionId")

    userID, err := uuid.Parse(userIDStr)
    if err != nil {
        writeJSON(w, http.StatusUnauthorized, errResp("UNAUTHORIZED", "Invalid user ID"))
        return
    }
    sessionID, err := uuid.Parse(sessionIDStr)
    if err != nil {
        writeJSON(w, http.StatusBadRequest, errResp("BAD_REQUEST", "Invalid session ID"))
        return
    }

    if err := h.sessionRepo.RevokeSession(r.Context(), userID, sessionID); err != nil {
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
        // Return empty array if not found or err
        prefs = []postgres.NotifPreference{}
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
