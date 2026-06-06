package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	domainerr "github.com/osv/auth-service/internal/domain/error"
	ucapikey "github.com/osv/auth-service/internal/usecase/manage_api_key"
	"github.com/rs/zerolog"
	"time"
)

// APIKeyHTTPHandler handles API key CRUD endpoints.
type APIKeyHTTPHandler struct {
	uc  *ucapikey.UseCase
	log zerolog.Logger
}

// NewAPIKeyHTTPHandler creates an APIKeyHTTPHandler.
func NewAPIKeyHTTPHandler(uc *ucapikey.UseCase, log zerolog.Logger) *APIKeyHTTPHandler {
	return &APIKeyHTTPHandler{uc: uc, log: log}
}

// CreateAPIKey handles POST /api/v1/auth/api-keys
func (h *APIKeyHTTPHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	uid, err := uuid.Parse(userID)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp("unauthorized", "missing user context"))
		return
	}

	var req struct {
		Name        string   `json:"name"`
		Permissions []string `json:"permissions"`
		ExpiresAt   *string  `json:"expires_at"` // RFC3339
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("invalid_request", err.Error()))
		return
	}

	var exp *time.Time
	if req.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errResp("invalid_date", "expires_at must be RFC3339"))
			return
		}
		exp = &t
	}

	resp, err := h.uc.CreateAPIKey(r.Context(), ucapikey.CreateRequest{
		UserID:      uid,
		Name:        req.Name,
		Permissions: req.Permissions,
		ExpiresAt:   exp,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("create_failed", err.Error()))
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"key_id":     resp.KeyID,
		"full_key":   resp.FullKey, // ⚠ shown ONCE
		"prefix":     resp.Prefix,
		"name":       resp.Name,
		"expires_at": resp.ExpiresAt,
		"warning":    "Copy this key now. It will not be shown again.",
	})
}

// ListAPIKeys handles GET /api/v1/auth/api-keys
func (h *APIKeyHTTPHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	uid, err := uuid.Parse(userID)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp("unauthorized", "missing user context"))
		return
	}

	keys, err := h.uc.ListAPIKeys(r.Context(), uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("list_failed", err.Error()))
		return
	}

	type keyItem struct {
		KeyID      string     `json:"key_id"`
		Name       string     `json:"name"`
		Prefix     string     `json:"prefix"`
		Perms      []string   `json:"permissions"`
		CreatedAt  time.Time  `json:"created_at"`
		LastUsedAt *time.Time `json:"last_used_at"`
		ExpiresAt  *time.Time `json:"expires_at"`
		IsActive   bool       `json:"is_active"`
	}
	items := make([]keyItem, 0, len(keys))
	for _, k := range keys {
		items = append(items, keyItem{
			KeyID:      k.ID.String(),
			Name:       k.Name,
			Prefix:     k.Prefix,
			Perms:      k.Permissions,
			CreatedAt:  k.CreatedAt,
			LastUsedAt: k.LastUsedAt,
			ExpiresAt:  k.ExpiresAt,
			IsActive:   k.IsActive(),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"api_keys": items})
}

// RevokeAPIKey handles DELETE /api/v1/auth/api-keys/{key_id}
func (h *APIKeyHTTPHandler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.Header.Get("X-User-ID")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp("unauthorized", "missing user context"))
		return
	}

	keyIDStr := chi.URLParam(r, "key_id")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("invalid_key_id", "key_id must be a UUID"))
		return
	}

	if err := h.uc.RevokeAPIKey(r.Context(), keyID, userID); err != nil {
		if strings.Contains(err.Error(), domainerr.ErrAPIKeyNotFound.Error()) {
			writeJSON(w, http.StatusNotFound, errResp("not_found", "API key not found"))
			return
		}
		writeJSON(w, http.StatusInternalServerError, errResp("revoke_failed", err.Error()))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "API key revoked"})
}
