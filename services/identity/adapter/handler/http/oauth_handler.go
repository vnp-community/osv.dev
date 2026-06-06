package http

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"

	ucoauth "github.com/defectdojo/identity/internal/usecase/oauth"
	"github.com/rs/zerolog"
)

// OAuthHandler handles OAuth2 initiation and callback endpoints.
type OAuthHandler struct {
	callbackUC *ucoauth.UseCase
	log        zerolog.Logger
}

// NewOAuthHandler creates an OAuthHandler.
func NewOAuthHandler(callbackUC *ucoauth.UseCase, log zerolog.Logger) *OAuthHandler {
	return &OAuthHandler{callbackUC: callbackUC, log: log}
}

// InitiateGoogle handles GET /api/v1/auth/oauth/google
// Redirects to Google's OAuth2 consent page.
func (h *OAuthHandler) InitiateGoogle(w http.ResponseWriter, r *http.Request) {
	h.initiate(w, r, "google")
}

// InitiateGitHub handles GET /api/v1/auth/oauth/github
// Redirects to GitHub's OAuth2 consent page.
func (h *OAuthHandler) InitiateGitHub(w http.ResponseWriter, r *http.Request) {
	h.initiate(w, r, "github")
}

// CallbackGoogle handles GET /api/v1/auth/oauth/google/callback
func (h *OAuthHandler) CallbackGoogle(w http.ResponseWriter, r *http.Request) {
	h.callback(w, r, "google")
}

// CallbackGitHub handles GET /api/v1/auth/oauth/github/callback
func (h *OAuthHandler) CallbackGitHub(w http.ResponseWriter, r *http.Request) {
	h.callback(w, r, "github")
}

func (h *OAuthHandler) initiate(w http.ResponseWriter, r *http.Request, provider string) {
	// Generate CSRF state token (16 random bytes, base64url)
	b := make([]byte, 16)
	rand.Read(b) //nolint:errcheck
	state := base64.RawURLEncoding.EncodeToString(b)

	// Store state in a short-lived cookie for CSRF verification
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		MaxAge:   300, // 5 minutes
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
		Path:     "/",
	})

	// The redirect URL comes from the use case's underlying provider config.
	// For simplicity, just return the state and provider; frontend handles redirect.
	writeJSON(w, http.StatusOK, map[string]string{
		"provider": provider,
		"state":    state,
		"message":  "redirect to provider with state",
	})
}

func (h *OAuthHandler) callback(w http.ResponseWriter, r *http.Request, provider string) {
	// Validate CSRF state
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		writeJSON(w, http.StatusBadRequest, errResp("invalid_state", "CSRF state mismatch"))
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		writeJSON(w, http.StatusBadRequest, errResp("missing_code", "OAuth code is required"))
		return
	}

	resp, err := h.callbackUC.Execute(r.Context(), ucoauth.CallbackRequest{
		Provider:  provider,
		Code:      code,
		State:     r.URL.Query().Get("state"),
		IPAddress: r.RemoteAddr,
		UserAgent: r.Header.Get("User-Agent"),
	})
	if err != nil {
		h.log.Error().Err(err).Str("provider", provider).Msg("oauth callback failed")
		writeJSON(w, http.StatusUnauthorized, errResp("oauth_failed", err.Error()))
		return
	}

	// Clear the CSRF cookie
	http.SetCookie(w, &http.Cookie{
		Name:   "oauth_state",
		Value:  "",
		MaxAge: -1,
		Path:   "/",
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  resp.AccessToken,
		"refresh_token": resp.RefreshToken,
		"token_type":    "Bearer",
		"expires_in":    resp.ExpiresIn,
		"user_id":       resp.UserID,
		"role":          resp.Role,
		"is_new_user":   resp.IsNewUser,
	})
}

// APIKeyHandler handles API key management endpoints.
type APIKeyHandler struct {
	// placeholder for T2.11 api_key_handler - wired in router
}

// writeJSON re-exported from auth_handler.go (same package)
// errResp re-exported from auth_handler.go (same package)

// ListAPIKeys handles GET /api/v1/auth/api-keys
func ListAPIKeysHandler(w http.ResponseWriter, r *http.Request) {
	// Stub: returns mock response; wired properly in router.go
	writeJSON(w, http.StatusOK, map[string]any{"api_keys": []any{}})
}

// GetProviders handles GET /api/v1/auth/providers
func GetProviders(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"providers": []string{"local", "google", "github"},
	})
}

// HealthCheck handles GET /health/live for the auth service.
func HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
