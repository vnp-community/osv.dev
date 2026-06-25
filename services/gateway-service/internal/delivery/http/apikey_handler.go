// Package http — API Key CRUD handler for gateway-service.
package http

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/osv/gateway-service/internal/auth"
	"github.com/osv/gateway-service/internal/domain/entity"
	"github.com/osv/gateway-service/internal/domain/repository"
)

// APIKeyHandler handles API key management endpoints.
type APIKeyHandler struct {
	repo repository.APIKeyRepository
}

func NewAPIKeyHandler(repo repository.APIKeyRepository) *APIKeyHandler {
	return &APIKeyHandler{repo: repo}
}

// CreateAPIKeyRequest is the request body for POST /api/v2/api-keys.
type CreateAPIKeyRequest struct {
	Description string   `json:"description"` // required
	Scopes      []string `json:"scopes"`       // e.g. ["cve:read", "webhook:write"]
	ExpiresIn   *int     `json:"expires_in"`  // optional: days until expiry
}

// POST /api/v2/api-keys
// Returns plain key ONCE — not stored in DB.
func (h *APIKeyHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(auth.CtxKeyUserID).(string)
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req CreateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Description == "" {
		respondError(w, http.StatusBadRequest, "description is required")
		return
	}
	if len(req.Scopes) == 0 {
		req.Scopes = []string{entity.ScopeCVERead} // default scope
	}

	// Validate scopes
	for _, s := range req.Scopes {
		if !isValidScope(s) {
			respondError(w, http.StatusBadRequest, "invalid scope: "+s)
			return
		}
	}

	// Generate cryptographically secure key
	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		respondError(w, http.StatusInternalServerError, "key generation failed")
		return
	}
	plainKey := "gcve_" + base64.URLEncoding.EncodeToString(rawBytes)

	keyHash := sha256Hex(plainKey)

	apiKey := &entity.APIKey{
		ID:          uuid.New().String(),
		KeyHash:     keyHash,
		OwnerID:     userID,
		Description: req.Description,
		Scopes:      req.Scopes,
		IsActive:    true,
		CreatedAt:   time.Now().UTC(),
	}

	if req.ExpiresIn != nil && *req.ExpiresIn > 0 {
		exp := time.Now().UTC().AddDate(0, 0, *req.ExpiresIn)
		apiKey.ExpiresAt = &exp
	}

	if err := h.repo.Save(r.Context(), apiKey); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create api key")
		return
	}

	// Return plain key ONCE — this is the only time it's shown
	resp := map[string]interface{}{
		"id":          apiKey.ID,
		"key":         plainKey, // ONLY ONCE!
		"description": apiKey.Description,
		"scopes":      apiKey.Scopes,
		"created_at":  apiKey.CreatedAt.Format(time.RFC3339),
		"expires_at":  apiKey.ExpiresAt,
		"warning":     "Store this key securely — it will not be shown again.",
	}
	respondJSON(w, http.StatusCreated, resp)
}

// GET /api/v2/api-keys
// Lists owner's API keys — no plain keys returned.
func (h *APIKeyHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(auth.CtxKeyUserID).(string)
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	keys, err := h.repo.ListByOwner(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list api keys")
		return
	}

	type keyResponse struct {
		ID          string     `json:"id"`
		Description string     `json:"description"`
		Scopes      []string   `json:"scopes"`
		LastUsedAt  *time.Time `json:"last_used_at"`
		ExpiresAt   *time.Time `json:"expires_at"`
		CreatedAt   time.Time  `json:"created_at"`
	}

	result := make([]keyResponse, len(keys))
	for i, k := range keys {
		result[i] = keyResponse{
			ID:          k.ID,
			Description: k.Description,
			Scopes:      k.Scopes,
			LastUsedAt:  k.LastUsedAt,
			ExpiresAt:   k.ExpiresAt,
			CreatedAt:   k.CreatedAt,
		}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"api_keys": result})
}

// DELETE /api/v2/api-keys/{id}
// Revokes (soft-deletes) an API key.
func (h *APIKeyHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(auth.CtxKeyUserID).(string)
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	keyID := chi.URLParam(r, "id")
	if err := h.repo.Revoke(r.Context(), keyID, userID); err != nil {
		if errors.Is(err, repository.ErrAPIKeyNotFound) {
			respondError(w, http.StatusNotFound, "api key not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to revoke api key")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

var validScopes = map[string]bool{
	entity.ScopeCVERead:   true,
	entity.ScopeKEVRead:   true,
	entity.ScopeWebhook:   true,
	entity.ScopeSyncAdmin: true,
	entity.ScopeReadAll:   true,
}

func isValidScope(s string) bool { return validScopes[s] }

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func respondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message}) //nolint:errcheck
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data) //nolint:errcheck
}
