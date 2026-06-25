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
	"time"

	domainerr "github.com/osv/identity-service/internal/domain/error"
	"github.com/osv/identity-service/internal/domain/repository"
	"github.com/osv/identity-service/internal/domain/valueobject"
	uclogin "github.com/osv/identity-service/internal/usecase/login"
	ucregister "github.com/osv/identity-service/internal/usecase/register"
	ucrefresh "github.com/osv/identity-service/internal/usecase/refresh_token"
	ucvalidate "github.com/osv/identity-service/internal/usecase/validate_token"
	jwtpkg "github.com/osv/identity-service/internal/infrastructure/jwt"
	pginfra "github.com/osv/identity-service/internal/infra/postgres" // [FIX TASK-HC-014]
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)


// AuthHandler handles authentication HTTP endpoints.
type AuthHandler struct {
	registerUC     *ucregister.UseCase
	loginUC        *uclogin.UseCase
	refreshUC      *ucrefresh.UseCase
	logoutUC       *ucrefresh.LogoutUseCase
	validateUC     *ucvalidate.UseCase
	jwtSvc         *jwtpkg.Service
	userRepo       repository.UserRepository
	// [FIX TASK-HC-014] invitation repo for AcceptInvite — nil means disabled
	invitationRepo *pginfra.InvitationRepo
	log            zerolog.Logger
}

// NewAuthHandler creates an AuthHandler with all required use cases.
func NewAuthHandler(
	registerUC *ucregister.UseCase,
	loginUC *uclogin.UseCase,
	refreshUC *ucrefresh.UseCase,
	logoutUC *ucrefresh.LogoutUseCase,
	validateUC *ucvalidate.UseCase,
	jwtSvc *jwtpkg.Service,
	userRepo repository.UserRepository,
	log zerolog.Logger,
) *AuthHandler {
	return &AuthHandler{
		registerUC: registerUC,
		loginUC:    loginUC,
		refreshUC:  refreshUC,
		logoutUC:   logoutUC,
		validateUC: validateUC,
		jwtSvc:     jwtSvc,
		userRepo:   userRepo,
		log:        log,
	}
}

// NewAuthHandlerWithInvitation creates an AuthHandler with invitation repo injected.
// [FIX TASK-HC-014]
func NewAuthHandlerWithInvitation(
	registerUC *ucregister.UseCase,
	loginUC *uclogin.UseCase,
	refreshUC *ucrefresh.UseCase,
	logoutUC *ucrefresh.LogoutUseCase,
	validateUC *ucvalidate.UseCase,
	jwtSvc *jwtpkg.Service,
	userRepo repository.UserRepository,
	invitationRepo *pginfra.InvitationRepo,
	log zerolog.Logger,
) *AuthHandler {
	h := NewAuthHandler(registerUC, loginUC, refreshUC, logoutUC, validateUC, jwtSvc, userRepo, log)
	h.invitationRepo = invitationRepo
	return h
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
		Email      string `json:"email"`
		Password   string `json:"password"`
		MFACode    string `json:"mfa_code"`
		RememberMe bool   `json:"remember_me"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("invalid_request", err.Error()))
		return
	}

	resp, err := h.loginUC.Execute(r.Context(), uclogin.Request{
		Email:      req.Email,
		Password:   req.Password,
		TOTPCode:   req.MFACode,
		IPAddress:  r.RemoteAddr,
		UserAgent:  r.UserAgent(),
		RememberMe: req.RememberMe,
	})
	if err != nil {
		switch {
		case errors.Is(err, domainerr.ErrInvalidCredentials):
			writeJSON(w, http.StatusUnauthorized, errResp("invalid_credentials", "invalid email or password"))
		case errors.Is(err, domainerr.ErrAccountLocked):
			writeJSON(w, http.StatusTooManyRequests, errResp("account_locked", "too many failed attempts, try again in 15 minutes"))
		case errors.Is(err, domainerr.ErrMFARequired):
			// CR-001: frontend expects {mfa_required:true, detail:"...", error:"mfa_required"}
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error":        "mfa_required",
				"detail":       "MFA is required. Provide your TOTP code in the totp_code field.",
				"mfa_required": true,
			})
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

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    resp.RefreshToken,
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   7 * 24 * 3600,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":       resp.AccessToken,
		"refresh_token":      resp.RefreshToken,
		"expires_in":         resp.ExpiresIn,
		"refresh_expires_in": resp.RefreshExpiresIn,
		"user_id":            resp.UserID,
		"role":               resp.Role,
	})
}

// Refresh handles POST /api/v1/auth/refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
		writeJSON(w, http.StatusBadRequest, errResp("invalid_request", err.Error()))
		return
	}
	if req.RefreshToken == "" {
		if cookie, err := r.Cookie("refresh_token"); err == nil {
			req.RefreshToken = cookie.Value
		}
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

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    resp.RefreshToken,
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   7 * 24 * 3600,
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"access_token":       resp.AccessToken,
		"refresh_token":      resp.RefreshToken,
		"expires_in":         resp.ExpiresIn,
		"refresh_expires_in": resp.RefreshExpiresIn,
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
		if rt == "" {
			if cookie, err := r.Cookie("refresh_token"); err == nil {
				rt = cookie.Value
			}
		}
	}
	h.logoutUC.Execute(r.Context(), userID, rt) //nolint:errcheck

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

// Me handles GET /api/v1/auth/me — returns current user info from JWT claims.
// CR-001: also returns mfa_enabled by querying userRepo.
// FIX: wraps response in {"user": {...}} as required by frontend spec.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	// Claims injected by api-gateway via X-User-* headers
	userIDStr := r.Header.Get("X-User-ID")
	role := r.Header.Get("X-User-Role")
	permsHeader := r.Header.Get("X-User-Permissions")

	var perms []string
	if permsHeader != "" {
		perms = strings.Split(permsHeader, ",")
	}

	if userIDStr == "" {
		writeJSON(w, http.StatusUnauthorized, errResp("unauthorized", "not authenticated"))
		return
	}

	// Defaults from JWT headers (always available)
	userInfo := map[string]any{
		"id":          userIDStr,
		"role":        role,
		"permissions": perms,
		"mfa_enabled": false,
		"email":       "",
		"name":        "",
		"username":    "",
		"created_at":  "",
	}

	// CR-001: enrich from DB (email, username, mfa_enabled) if userRepo available
	if h.userRepo != nil {
		if uid, err := uuid.Parse(userIDStr); err == nil {
			if user, err := h.userRepo.FindByID(r.Context(), uid); err == nil {
				userInfo["mfa_enabled"] = user.MFAEnabled
				userInfo["email"]       = user.Email
				userInfo["name"]        = user.Username // "name" is the spec alias for username
				userInfo["username"]    = user.Username
				userInfo["created_at"]  = user.CreatedAt.Format(time.RFC3339)
				if len(perms) == 0 {
					// Fill permissions from role if not injected by gateway
					userInfo["permissions"] = valueobject.PermissionsFor(string(user.Role))
				}
			}
		}
	}

	// FIX: wrap in "user" key — frontend reads response.user, not response directly
	writeJSON(w, http.StatusOK, map[string]any{
		"user": userInfo,
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

// extractBearer extracts the Bearer token from the Authorization header.
func extractBearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	parts := strings.SplitN(h, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
		return parts[1]
	}
	return ""
}

// AcceptInvite handles GET /api/v1/auth/accept-invite?token=...
// [FIX TASK-HC-014] Validates invitation token and activates user account.
func (h *AuthHandler) AcceptInvite(w http.ResponseWriter, r *http.Request) {
	if h.invitationRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, errResp("not_configured", "invitation system not available"))
		return
	}
	token := r.URL.Query().Get("token")
	if token == "" {
		writeJSON(w, http.StatusBadRequest, errResp("missing_token", "token query parameter is required"))
		return
	}
	inv, err := h.invitationRepo.FindByToken(r.Context(), token)
	if err != nil || inv == nil {
		writeJSON(w, http.StatusBadRequest, errResp("invalid_token", "invitation token is invalid or expired"))
		return
	}
	// Activate the user account
	if err := h.userRepo.Activate(r.Context(), inv.UserID); err != nil {
		h.log.Error().Err(err).Str("user_id", inv.UserID.String()).Msg("AcceptInvite: activate failed")
		writeJSON(w, http.StatusInternalServerError, errResp("activation_failed", "failed to activate account"))
		return
	}
	if err := h.invitationRepo.MarkAccepted(r.Context(), token); err != nil {
		h.log.Warn().Err(err).Msg("AcceptInvite: MarkAccepted failed (non-fatal)")
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "account_activated",
		"user_id": inv.UserID.String(),
	})
}
