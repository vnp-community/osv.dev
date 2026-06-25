package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	domainerr "github.com/osv/identity-service/internal/domain/error"
	ucapikey "github.com/osv/identity-service/internal/usecase/manage_api_key"
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
		Scopes      []string `json:"scopes"` // alias for permissions — frontend sends scopes
		ExpiresAt   *string  `json:"expires_at"` // RFC3339
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("invalid_request", err.Error()))
		return
	}

	// Accept 'scopes' as alias for 'permissions'
	if len(req.Permissions) == 0 && len(req.Scopes) > 0 {
		req.Permissions = req.Scopes
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

	// Response: CR-012 schema — {key: {...}, raw_key: string}
	// Frontend expects: key (object) + raw_key (the plaintext key shown ONCE)
	writeJSON(w, http.StatusCreated, map[string]any{
		"key": map[string]any{
			"id":          resp.KeyID,
			"name":        resp.Name,
			"prefix":      resp.Prefix,
			"scopes":      req.Permissions,
			"created_at":  time.Now().UTC(),
			"last_used_at": nil,
			"expires_at":  resp.ExpiresAt,
			"status":      "active",
			"created_by":  r.Header.Get("X-User-Email"),
		},
		"raw_key": resp.FullKey, // shown ONCE — copy now
		"warning": "Copy this key now. It will not be shown again.",
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

	// Response: align with frontend APIKey interface
	// frontend expects: { items: APIKey[], total: number }
	type keyItem struct {
		ID         string     `json:"id"`
		Name       string     `json:"name"`
		Prefix     string     `json:"prefix"`
		Scopes     []string   `json:"scopes"`
		CreatedAt  time.Time  `json:"created_at"`
		LastUsedAt *time.Time `json:"last_used_at"`
		ExpiresAt  *time.Time `json:"expires_at"`
		Status     string     `json:"status"`
		CreatedBy  string     `json:"created_by"`
	}
	items := make([]keyItem, 0, len(keys))
	for _, k := range keys {
		status := "active"
		if k.RevokedAt != nil {
			status = "revoked"
		} else if k.ExpiresAt != nil && time.Now().After(*k.ExpiresAt) {
			status = "expired"
		}
		// Use Scopes if set, fall back to Permissions for backward compat
		scopes := k.Scopes
		if len(scopes) == 0 {
			scopes = k.Permissions
		}
		if scopes == nil {
			scopes = []string{}
		}
		items = append(items, keyItem{
			ID:         k.ID.String(),
			Name:       k.Name,
			Prefix:     k.Prefix,
			Scopes:     scopes,
			CreatedAt:  k.CreatedAt,
			LastUsedAt: k.LastUsedAt,
			ExpiresAt:  k.ExpiresAt,
			Status:     status,
			CreatedBy:  "", // not stored in current schema
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"keys":  items,
		"total": len(items),
	})
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
