// Package http provides HTTP REST handlers for the auth service.
// Routes: POST /api/v1/auth/register, POST /api/v1/auth/login,
//         POST /api/v1/auth/refresh, POST /api/v1/auth/logout,
//         GET  /api/v1/auth/me, GET /.well-known/jwks.json
package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	domainerr "github.com/defectdojo/identity/internal/domain/error"
	uclogin "github.com/defectdojo/identity/internal/usecase/login"
	ucregister "github.com/defectdojo/identity/internal/usecase/register"
	ucrefresh "github.com/defectdojo/identity/internal/usecase/refresh_token"
	ucvalidate "github.com/defectdojo/identity/internal/usecase/validate_token"
	jwtpkg "github.com/defectdojo/identity/internal/infrastructure/jwt"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// AuthHandler handles authentication HTTP endpoints.
type AuthHandler struct {
	registerUC *ucregister.UseCase
	loginUC    *uclogin.UseCase
	refreshUC  *ucrefresh.UseCase
	logoutUC   *ucrefresh.LogoutUseCase
	validateUC *ucvalidate.UseCase
	jwtSvc     *jwtpkg.Service
	log        zerolog.Logger
}

// NewAuthHandler creates an AuthHandler with all required use cases.
func NewAuthHandler(
	registerUC *ucregister.UseCase,
	loginUC *uclogin.UseCase,
	refreshUC *ucrefresh.UseCase,
	logoutUC *ucrefresh.LogoutUseCase,
	validateUC *ucvalidate.UseCase,
	jwtSvc *jwtpkg.Service,
	log zerolog.Logger,
) *AuthHandler {
	return &AuthHandler{
		registerUC: registerUC,
		loginUC:    loginUC,
		refreshUC:  refreshUC,
		logoutUC:   logoutUC,
		validateUC: validateUC,
		jwtSvc:     jwtSvc,
		log:        log,
	}
}

// Register handles POST /api/v1/auth/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("invalid_request", err.Error()))
		return
	}

	resp, err := h.registerUC.Execute(r.Context(), ucregister.Request{
		Email:    req.Email,
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		switch {
		case errors.Is(err, domainerr.ErrEmailAlreadyExists):
			writeJSON(w, http.StatusConflict, errResp("email_exists", "email is already registered"))
		default:
			h.log.Error().Err(err).Msg("register failed")
			writeJSON(w, http.StatusBadRequest, errResp("register_failed", err.Error()))
		}
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"user_id": resp.UserID,
		"email":   resp.Email,
		"role":    resp.Role,
		"message": "registration successful. please verify your email.",
	})
}

// Login handles POST /api/v1/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		TOTPCode string `json:"totp_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("invalid_request", err.Error()))
		return
	}

	resp, err := h.loginUC.Execute(r.Context(), uclogin.Request{
		Email:     req.Email,
		Password:  req.Password,
		TOTPCode:  req.TOTPCode,
		IPAddress: r.RemoteAddr,
		UserAgent: r.Header.Get("User-Agent"),
	})
	if err != nil {
		switch {
		case errors.Is(err, domainerr.ErrInvalidCredentials):
			writeJSON(w, http.StatusUnauthorized, errResp("invalid_credentials", "invalid email or password"))
		case errors.Is(err, domainerr.ErrAccountLocked):
			writeJSON(w, http.StatusTooManyRequests, errResp("account_locked", "too many failed attempts, try again in 15 minutes"))
		case errors.Is(err, domainerr.ErrMFARequired):
			writeJSON(w, http.StatusUnauthorized, errResp("mfa_required", "provide your TOTP code"))
		case errors.Is(err, domainerr.ErrInvalidMFACode):
			writeJSON(w, http.StatusUnauthorized, errResp("invalid_mfa_code", "invalid TOTP code"))
		case errors.Is(err, domainerr.ErrAccountInactive):
			writeJSON(w, http.StatusForbidden, errResp("account_inactive", "account is inactive"))
		default:
			h.log.Error().Err(err).Msg("login failed")
			writeJSON(w, http.StatusInternalServerError, errResp("internal_error", "login failed"))
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  resp.AccessToken,
		"refresh_token": resp.RefreshToken,
		"token_type":    "Bearer",
		"expires_in":    resp.ExpiresIn,
		"user_id":       resp.UserID,
		"role":          resp.Role,
	})
}

// Refresh handles POST /api/v1/auth/refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("invalid_request", err.Error()))
		return
	}
	if req.RefreshToken == "" {
		writeJSON(w, http.StatusBadRequest, errResp("missing_token", "refresh_token is required"))
		return
	}

	resp, err := h.refreshUC.Execute(r.Context(), ucrefresh.Request{
		RefreshToken: req.RefreshToken,
		IPAddress:    r.RemoteAddr,
		UserAgent:    r.Header.Get("User-Agent"),
	})
	if err != nil {
		switch {
		case errors.Is(err, domainerr.ErrTokenRevoked):
			writeJSON(w, http.StatusUnauthorized, errResp("token_revoked", "refresh token has been revoked"))
		case errors.Is(err, domainerr.ErrTokenExpired):
			writeJSON(w, http.StatusUnauthorized, errResp("token_expired", "refresh token has expired"))
		case errors.Is(err, domainerr.ErrInvalidToken):
			writeJSON(w, http.StatusUnauthorized, errResp("invalid_token", "invalid refresh token"))
		default:
			h.log.Error().Err(err).Msg("refresh failed")
			writeJSON(w, http.StatusInternalServerError, errResp("internal_error", "refresh failed"))
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  resp.AccessToken,
		"refresh_token": resp.RefreshToken,
		"token_type":    "Bearer",
		"expires_in":    resp.ExpiresIn,
	})
}

// Logout handles POST /api/v1/auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from X-User-ID header (injected by api-gateway)
	userIDStr := r.Header.Get("X-User-ID")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp("unauthorized", "missing user context"))
		return
	}

	// Optionally blacklist current access token JTI
	if token := extractBearer(r); token != "" {
		h.validateUC.RevokeToken(r.Context(), token) //nolint:errcheck
	}

	// Revoke session(s)
	var req struct {
		RefreshToken string `json:"refresh_token"` // optional: single device logout
		AllDevices   bool   `json:"all_devices"`
	}
	json.NewDecoder(r.Body).Decode(&req) //nolint:errcheck

	rt := ""
	if !req.AllDevices {
		rt = req.RefreshToken
	}
	h.logoutUC.Execute(r.Context(), userID, rt) //nolint:errcheck

	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out successfully"})
}

// Me handles GET /api/v1/auth/me — returns current user info from JWT claims.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	// Claims injected by api-gateway via X-User-* headers
	userID := r.Header.Get("X-User-ID")
	role := r.Header.Get("X-User-Role")
	permsHeader := r.Header.Get("X-User-Permissions")

	var perms []string
	if permsHeader != "" {
		perms = strings.Split(permsHeader, ",")
	}

	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, errResp("unauthorized", "not authenticated"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":     userID,
		"role":        role,
		"permissions": perms,
	})
}

// JWKS handles GET /.well-known/jwks.json
func (h *AuthHandler) JWKS(w http.ResponseWriter, r *http.Request) {
	jwks, err := h.jwtSvc.PublicKeyJWKS()
	if err != nil {
		h.log.Error().Err(err).Msg("failed to generate JWKS")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	w.Write(jwks)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func errResp(code, message string) map[string]string {
	return map[string]string{"error": code, "message": message}
}

func extractBearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	parts := strings.SplitN(h, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
		return parts[1]
	}
	return ""
}
