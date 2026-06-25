// Package http — CR-UI-001: Authentication & User API handlers.
// Proxies /api/v1/auth/* to identity-service and adds /me + profile endpoints.
package http

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// authUserResponse is the canonical User object returned by all auth endpoints.
type authUserResponse struct {
	ID          string   `json:"id"`
	Email       string   `json:"email"`
	Name        string   `json:"name"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
	MFAEnabled  bool     `json:"mfa_enabled"`
	AvatarURL   *string  `json:"avatar_url"`
	CreatedAt   string   `json:"created_at"`
}

// rolePermissions maps role → permission set (server-side authoritative).
var rolePermissions = map[string][]string{
	"admin": {
		"scan:create", "scan:read",
		"asset:write", "asset:read",
		"finding:write", "finding:read",
		"report:download",
		"user:manage", "system:configure",
	},
	"user": {
		"scan:create", "scan:read",
		"asset:write", "asset:read",
		"finding:write", "finding:read",
		"report:download",
	},
	"readonly": {
		"scan:read", "asset:read",
		"finding:read", "report:download",
	},
	"agent": {
		"scan:read", "agent:report",
	},
}

// RegisterAuthRoutes mounts CR-UI-001 auth routes on the given router.
// The identityServiceURL is the base URL of identity-service (e.g. "http://identity-service:8081").
func RegisterAuthRoutes(r chi.Router, identityServiceURL string) {
	h := &authHandler{identityURL: strings.TrimRight(identityServiceURL, "/")}

	// Public (no JWT required)
	r.Post("/api/v1/auth/login", h.Login)
	r.Post("/api/v1/auth/refresh", h.Refresh)
	r.Get("/api/v1/auth/oauth/{provider}", h.OAuthRedirect)
	r.Get("/api/v1/auth/callback", h.OAuthCallback)

	// Protected (JWT required — middleware applied upstream by NewCVERouter)
	r.Get("/api/v1/auth/me", h.Me)
	r.Post("/api/v1/auth/logout", h.Logout)
	r.Get("/api/v1/auth/mfa/setup", h.MFASetup)
	r.Post("/api/v1/auth/mfa/confirm", h.MFAConfirm)

	// Profile (CR-UI-010 §4)
	r.Get("/api/v1/profile", h.GetProfile)
	r.Patch("/api/v1/profile", h.UpdateProfile)
	r.Post("/api/v1/profile/change-password", h.ChangePassword)

	// API Keys (CR-UI-010 §5)
	r.Get("/api/v1/api-keys", h.ListAPIKeys)
	r.Post("/api/v1/api-keys", h.CreateAPIKey)
	r.Delete("/api/v1/api-keys/{id}", h.DeleteAPIKey)
}

// PublicAuthHandler is an exported wrapper exposing ONLY the public (no-JWT) auth endpoints.
// Used by embedded.go to register login/refresh/oauth routes without accidentally registering
// protected routes (me, logout, profile) that should go through auth middleware.
type PublicAuthHandler struct {
	IdentityURL string
	inner       *authHandler
}

func (p *PublicAuthHandler) handler() *authHandler {
	if p.inner == nil {
		p.inner = &authHandler{identityURL: p.IdentityURL}
	}
	return p.inner
}

func (p *PublicAuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	p.handler().Login(w, r)
}

func (p *PublicAuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	p.handler().Refresh(w, r)
}

func (p *PublicAuthHandler) OAuthRedirect(w http.ResponseWriter, r *http.Request) {
	p.handler().OAuthRedirect(w, r)
}

func (p *PublicAuthHandler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	p.handler().OAuthCallback(w, r)
}

type authHandler struct {
	identityURL string
	httpClient  *http.Client
}

func (h *authHandler) client() *http.Client {
	if h.httpClient != nil {
		return h.httpClient
	}
	return &http.Client{Timeout: 10 * time.Second}
}


// Login handles POST /api/v1/auth/login (CR-UI-001 §2.1).
// Proxies to identity-service and enriches response with permissions array.
func (h *authHandler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email      string `json:"email"`
		Password   string `json:"password"`
		MFACode    string `json:"mfa_code,omitempty"`
		RememberMe bool   `json:"remember_me,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body", nil)
		return
	}
	if body.Email == "" || body.Password == "" {
		writeAPIError(w, http.StatusBadRequest, "VALIDATION_ERROR", "email and password are required", nil)
		return
	}

	// Proxy to identity-service POST /auth/login
	resp, err := proxyJSON(h.client(), h.identityURL+"/api/v1/auth/login", r.Method, map[string]interface{}{
		"email":       body.Email,
		"password":    body.Password,
		"mfa_code":    body.MFACode,
		"remember_me": body.RememberMe,
	})
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Identity service unavailable", nil)
		return
	}
	defer resp.Body.Close()

	var upstream map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&upstream); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to parse upstream response", nil)
		return
	}

	// Map upstream errors to CR-UI-001 error codes
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		errMsg, _ := upstream["error"].(string)
		code := "INVALID_CREDENTIALS"
		if strings.Contains(errMsg, "mfa") || strings.Contains(errMsg, "totp") {
			code = "MFA_REQUIRED"
		}
		writeAPIError(w, http.StatusUnauthorized, code, "Authentication failed", nil)
		return
	case http.StatusForbidden:
		writeAPIError(w, http.StatusLocked, "ACCOUNT_LOCKED", "Account locked due to too many failed attempts", nil)
		return
	}

	// MFA required flow
	if mfaReq, ok := upstream["mfa_required"].(bool); ok && mfaReq {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"mfa_required": true,
			"access_token": nil,
			"user":         nil,
		})
		return
	}

	// Enrich with permissions
	accessToken, _ := upstream["access_token"].(string)
	role, _ := upstream["role"].(string)
	if role == "" {
		role = "user"
	}

	// Set httpOnly refresh_token cookie
	if rt, ok := upstream["refresh_token"].(string); ok && rt != "" {
		maxAge := 0 // Default to session cookie
		if refreshExp, ok := upstream["refresh_expires_in"].(float64); ok && refreshExp > 0 {
			maxAge = int(refreshExp)
		} else if body.RememberMe {
			maxAge = 2592000 // Fallback to 30 days
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "refresh_token",
			Value:    rt,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
			Path:     "/api/v1/auth/refresh",
			MaxAge:   maxAge,
		})
	}

	user := buildUserResponse(upstream, role)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"access_token": accessToken,
		"expires_in":   900,
		"user":         user,
	})
}

// Refresh handles POST /api/v1/auth/refresh (CR-UI-001 §2.2).
func (h *authHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	// Try to read refresh token from httpOnly cookie first, then body
	refreshToken := ""
	if cookie, err := r.Cookie("refresh_token"); err == nil {
		refreshToken = cookie.Value
	} else {
		var body struct {
			RefreshToken string `json:"refresh_token"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		refreshToken = body.RefreshToken
	}

	if refreshToken == "" {
		writeAPIError(w, http.StatusUnauthorized, "REFRESH_TOKEN_INVALID", "No refresh token provided", nil)
		return
	}

	resp, err := proxyJSON(h.client(), h.identityURL+"/api/v1/auth/refresh", "POST", map[string]string{
		"refresh_token": refreshToken,
	})
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Identity service unavailable", nil)
		return
	}
	defer resp.Body.Close()

	var upstream map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&upstream); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to parse upstream response", nil)
		return
	}

	if resp.StatusCode == http.StatusUnauthorized {
		errMsg, _ := upstream["error"].(string)
		code := "REFRESH_TOKEN_INVALID"
		if strings.Contains(errMsg, "reuse") || strings.Contains(errMsg, "replay") {
			code = "REFRESH_TOKEN_REUSED"
		}
		writeAPIError(w, http.StatusUnauthorized, code, "Refresh token is invalid or expired", nil)
		return
	}

	accessToken, _ := upstream["access_token"].(string)
	if newRT, ok := upstream["refresh_token"].(string); ok && newRT != "" {
		maxAge := 0
		if refreshExp, ok := upstream["refresh_expires_in"].(float64); ok && refreshExp > 0 {
			maxAge = int(refreshExp)
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "refresh_token",
			Value:    newRT,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
			Path:     "/api/v1/auth/refresh",
			MaxAge:   maxAge,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"access_token": accessToken,
		"expires_in":   900,
	})
}

// Me handles GET /api/v1/auth/me (CR-UI-001 §2.3).
func (h *authHandler) Me(w http.ResponseWriter, r *http.Request) {
	// Forward Authorization header to identity-service
	req, _ := http.NewRequestWithContext(r.Context(), "GET", h.identityURL+"/api/v1/auth/me", nil)
	req.Header.Set("Authorization", r.Header.Get("Authorization"))

	resp, err := h.client().Do(req)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Identity service unavailable", nil)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		writeAPIError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Token is invalid or expired", nil)
		return
	}

	var upstream map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&upstream); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to parse upstream response", nil)
		return
	}

	role, _ := upstream["role"].(string)
	if role == "" {
		role = "user"
	}
	user := buildUserResponse(upstream, role)
	respondJSON(w, http.StatusOK, map[string]interface{}{"user": user})
}

// Logout handles POST /api/v1/auth/logout (CR-UI-001 §2.4).
func (h *authHandler) Logout(w http.ResponseWriter, r *http.Request) {
	req, _ := http.NewRequestWithContext(r.Context(), "POST", h.identityURL+"/api/v1/auth/logout", nil)
	req.Header.Set("Authorization", r.Header.Get("Authorization"))
	h.client().Do(req) //nolint:errcheck — best-effort

	// Clear refresh_token cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Path:     "/api/v1/auth/refresh",
		MaxAge:   -1,
	})
	respondJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// MFASetup handles GET /api/v1/auth/mfa/setup (CR-UI-001 §2.5).
func (h *authHandler) MFASetup(w http.ResponseWriter, r *http.Request) {
	req, _ := http.NewRequestWithContext(r.Context(), "POST", h.identityURL+"/api/v1/auth/mfa/setup", nil)
	req.Header.Set("Authorization", r.Header.Get("Authorization"))
	resp, err := h.client().Do(req)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Identity service unavailable", nil)
		return
	}
	defer resp.Body.Close()

	var upstream map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&upstream) //nolint:errcheck
	respondJSON(w, resp.StatusCode, upstream)
}

// MFAConfirm handles POST /api/v1/auth/mfa/confirm (CR-UI-001 §2.6).
func (h *authHandler) MFAConfirm(w http.ResponseWriter, r *http.Request) {
	var body map[string]interface{}
	json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck
	resp, err := proxyJSON(h.client(), h.identityURL+"/api/v1/auth/mfa/confirm", "POST", body)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Identity service unavailable", nil)
		return
	}
	defer resp.Body.Close()
	var upstream map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&upstream) //nolint:errcheck
	respondJSON(w, resp.StatusCode, upstream)
}

// OAuthRedirect handles GET /api/v1/auth/oauth/{provider} (CR-UI-001 §2.7–2.8).
func (h *authHandler) OAuthRedirect(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	req, _ := http.NewRequestWithContext(r.Context(), "GET", h.identityURL+"/api/v1/auth/oauth/"+provider, nil)
	resp, err := h.client().Do(req)
	if err != nil || resp.StatusCode != http.StatusFound {
		writeAPIError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Unsupported OAuth provider: "+provider, nil)
		return
	}
	http.Redirect(w, r, resp.Header.Get("Location"), http.StatusFound)
}

// OAuthCallback handles GET /api/v1/auth/callback (CR-UI-001 §2.9).
func (h *authHandler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	// Forward query params to identity-service
	req, _ := http.NewRequestWithContext(r.Context(), "GET",
		h.identityURL+"/api/v1/auth/oauth/callback?"+r.URL.RawQuery, nil)
	resp, err := h.client().Do(req)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Identity service unavailable", nil)
		return
	}
	defer resp.Body.Close()

	var upstream map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&upstream) //nolint:errcheck

	if at, ok := upstream["access_token"].(string); ok {
		if rt, ok := upstream["refresh_token"].(string); ok && rt != "" {
			http.SetCookie(w, &http.Cookie{
				Name: "refresh_token", Value: rt,
				HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode,
				Path: "/api/v1/auth/refresh", MaxAge: 604800,
			})
		}
		http.Redirect(w, r, "/auth/callback?access_token="+at+"&expires_in=900", http.StatusFound)
		return
	}
	writeAPIError(w, http.StatusUnauthorized, "UNAUTHORIZED", "OAuth2 authentication failed", nil)
}

// GetProfile handles GET /api/v1/profile.
func (h *authHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	req, _ := http.NewRequestWithContext(r.Context(), "GET", h.identityURL+"/api/v1/auth/profile", nil)
	req.Header.Set("Authorization", r.Header.Get("Authorization"))
	resp, err := h.client().Do(req)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Identity service unavailable", nil)
		return
	}
	defer resp.Body.Close()
	var upstream map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&upstream) //nolint:errcheck
	respondJSON(w, resp.StatusCode, upstream)
}

// UpdateProfile handles PATCH /api/v1/profile.
func (h *authHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	var body map[string]interface{}
	json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck
	resp, err := proxyJSON(h.client(), h.identityURL+"/api/v1/auth/profile", "PATCH", body)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Identity service unavailable", nil)
		return
	}
	defer resp.Body.Close()
	var upstream map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&upstream) //nolint:errcheck
	respondJSON(w, resp.StatusCode, upstream)
}

// ChangePassword handles POST /api/v1/profile/change-password.
func (h *authHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var body map[string]interface{}
	json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck
	resp, err := proxyJSON(h.client(), h.identityURL+"/api/v1/auth/profile/change-password", "POST", body)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Identity service unavailable", nil)
		return
	}
	defer resp.Body.Close()
	var upstream map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&upstream) //nolint:errcheck
	respondJSON(w, resp.StatusCode, upstream)
}

// ListAPIKeys handles GET /api/v1/api-keys.
func (h *authHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	req, _ := http.NewRequestWithContext(r.Context(), "GET", h.identityURL+"/api/v1/auth/api-keys", nil)
	req.Header.Set("Authorization", r.Header.Get("Authorization"))
	resp, err := h.client().Do(req)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Identity service unavailable", nil)
		return
	}
	defer resp.Body.Close()
	var upstream interface{}
	json.NewDecoder(resp.Body).Decode(&upstream) //nolint:errcheck
	respondJSON(w, resp.StatusCode, upstream)
}

// CreateAPIKey handles POST /api/v1/api-keys.
func (h *authHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var body map[string]interface{}
	json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck
	resp, err := proxyJSON(h.client(), h.identityURL+"/api/v1/auth/api-keys", "POST", body)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Identity service unavailable", nil)
		return
	}
	defer resp.Body.Close()
	var upstream map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&upstream) //nolint:errcheck
	respondJSON(w, resp.StatusCode, upstream)
}

// DeleteAPIKey handles DELETE /api/v1/api-keys/{id}.
func (h *authHandler) DeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	req, _ := http.NewRequestWithContext(r.Context(), "DELETE", h.identityURL+"/api/v1/auth/api-keys/"+id, nil)
	req.Header.Set("Authorization", r.Header.Get("Authorization"))
	resp, err := h.client().Do(req)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Identity service unavailable", nil)
		return
	}
	resp.Body.Close()
	w.WriteHeader(resp.StatusCode)
}

// buildUserResponse assembles the canonical user object from upstream data + role permissions.
func buildUserResponse(data map[string]interface{}, role string) authUserResponse {
	perms := rolePermissions[role]
	if perms == nil {
		perms = rolePermissions["readonly"]
	}
	id, _ := data["id"].(string)
	if id == "" {
		id, _ = data["user_id"].(string)
	}
	email, _ := data["email"].(string)
	name, _ := data["name"].(string)
	if name == "" {
		name, _ = data["username"].(string)
	}
	createdAt, _ := data["created_at"].(string)
	if createdAt == "" {
		createdAt = time.Now().UTC().Format(time.RFC3339)
	}
	mfaEnabled, _ := data["mfa_enabled"].(bool)
	return authUserResponse{
		ID: id, Email: email, Name: name, Role: role,
		Permissions: perms, MFAEnabled: mfaEnabled,
		AvatarURL: nil, CreatedAt: createdAt,
	}
}
