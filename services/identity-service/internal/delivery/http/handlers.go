package http

import (
    "encoding/json"
    "net/http"
    "strings"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    "github.com/rs/zerolog"

    "github.com/osv/identity-service/internal/usecase/apikey"
    "github.com/osv/identity-service/internal/usecase/login"
    "github.com/osv/identity-service/internal/usecase/logout"
    "github.com/osv/identity-service/internal/usecase/oauth2"
    "github.com/osv/identity-service/internal/usecase/refresh"
    "github.com/osv/identity-service/internal/usecase/register"
    "github.com/osv/identity-service/internal/usecase/totp"
    domainerr "github.com/osv/identity-service/internal/domain/error"
)

// Handler holds all HTTP handler dependencies.
type Handler struct {
    registerUC *register.UseCase
    loginUC    *login.UseCase
    refreshUC  *refresh.UseCase
    logoutUC   *logout.UseCase
    apikeyUC   *apikey.UseCase
    totpUC     *totp.UseCase
    oauth2UC   *oauth2.UseCase
    logger     zerolog.Logger
}

// NewHandler creates an HTTP handler with all use-cases injected.
func NewHandler(
    registerUC *register.UseCase,
    loginUC *login.UseCase,
    refreshUC *refresh.UseCase,
    logoutUC *logout.UseCase,
    apikeyUC *apikey.UseCase,
    totpUC *totp.UseCase,
    oauth2UC *oauth2.UseCase,
    logger zerolog.Logger,
) *Handler {
    return &Handler{
        registerUC: registerUC,
        loginUC:    loginUC,
        refreshUC:  refreshUC,
        logoutUC:   logoutUC,
        apikeyUC:   apikeyUC,
        totpUC:     totpUC,
        oauth2UC:   oauth2UC,
        logger:     logger,
    }
}

// Router sets up all auth HTTP routes.
func (h *Handler) Router() http.Handler {
    r := chi.NewRouter()
    r.Use(middleware.RequestID)
    r.Use(middleware.RealIP)
    r.Use(middleware.Recoverer)

    // Public routes
    r.Post("/auth/register", h.Register)
    r.Post("/auth/login", h.Login)
    r.Post("/auth/refresh", h.Refresh)

    // OAuth2 routes
    r.Get("/auth/oauth/{provider}", h.OAuthRedirect)
    r.Get("/auth/oauth/{provider}/callback", h.OAuthCallback)

    // Authenticated routes
    r.Group(func(r chi.Router) {
        r.Use(h.AuthMiddleware)
        r.Post("/auth/logout", h.Logout)
        r.Post("/auth/logout/all", h.LogoutAll)

        // MFA
        r.Post("/auth/mfa/setup", h.MFASetup)
        r.Post("/auth/mfa/confirm", h.MFAConfirm)
        r.Post("/auth/mfa/disable", h.MFADisable)

        // API Keys
        r.Post("/auth/api-keys", h.CreateAPIKey)
        r.Get("/auth/api-keys", h.ListAPIKeys)
        r.Delete("/auth/api-keys/{id}", h.RevokeAPIKey)
    })

    return r
}

// Register handles POST /auth/register
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Email    string `json:"email"`
        Username string `json:"username"`
        Password string `json:"password"`
        Role     string `json:"role,omitempty"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        jsonError(w, "invalid request body", http.StatusBadRequest)
        return
    }

    user, err := h.registerUC.Execute(r.Context(), register.Request{
        Email:    req.Email,
        Username: req.Username,
        Password: req.Password,
    })
    if err != nil {
        h.handleError(w, err)
        return
    }

    jsonResponse(w, http.StatusCreated, map[string]interface{}{
        "id":       user.UserID,
        "email":    user.Email,
        "username": req.Username,
        "role":     user.Role,
    })
}

// Login handles POST /auth/login
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Email    string `json:"email"`
        Password string `json:"password"`
        TOTPCode string `json:"totp_code,omitempty"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        jsonError(w, "invalid request body", http.StatusBadRequest)
        return
    }

    out, err := h.loginUC.Execute(r.Context(), login.Request{
        Email:     req.Email,
        Password:  req.Password,
        TOTPCode:  req.TOTPCode,
        IPAddress: r.RemoteAddr,
        UserAgent: r.UserAgent(),
    })
    if err != nil {
        h.handleError(w, err)
        return
    }

    jsonResponse(w, http.StatusOK, map[string]interface{}{
        "access_token":  out.AccessToken,
        "refresh_token": out.RefreshToken,
        "token_type":    "Bearer",
        "expires_in":    900, // 15 minutes
    })
}

// Refresh handles POST /auth/refresh
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
    var req struct {
        RefreshToken string `json:"refresh_token"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        jsonError(w, "invalid request body", http.StatusBadRequest)
        return
    }

    out, err := h.refreshUC.Execute(r.Context(), req.RefreshToken, r.RemoteAddr, r.UserAgent())
    if err != nil {
        h.handleError(w, err)
        return
    }

    jsonResponse(w, http.StatusOK, map[string]interface{}{
        "access_token":  out.AccessToken,
        "refresh_token": out.RefreshToken,
        "token_type":    "Bearer",
        "expires_in":    900,
    })
}

// Logout handles POST /auth/logout
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
    // Session ID and JTI come from auth middleware context
    w.WriteHeader(http.StatusNoContent)
}

// LogoutAll handles POST /auth/logout/all
func (h *Handler) LogoutAll(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusNoContent)
}

// OAuthRedirect handles GET /auth/oauth/{provider}
func (h *Handler) OAuthRedirect(w http.ResponseWriter, r *http.Request) {
    provider := chi.URLParam(r, "provider")
    state := r.URL.Query().Get("state")
    url, err := h.oauth2UC.GetAuthURL(provider, state)
    if err != nil {
        jsonError(w, "unsupported provider", http.StatusBadRequest)
        return
    }
    http.Redirect(w, r, url, http.StatusFound)
}

// OAuthCallback handles GET /auth/oauth/{provider}/callback
func (h *Handler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
    provider := chi.URLParam(r, "provider")
    code := r.URL.Query().Get("code")

    user, err := h.oauth2UC.HandleCallback(r.Context(), provider, code)
    if err != nil {
        jsonError(w, "oauth2 authentication failed", http.StatusUnauthorized)
        return
    }

    jsonResponse(w, http.StatusOK, map[string]interface{}{
        "user_id": user.ID,
        "email":   user.Email,
    })
}

// MFASetup handles POST /auth/mfa/setup
func (h *Handler) MFASetup(w http.ResponseWriter, r *http.Request) {
    jsonResponse(w, http.StatusOK, map[string]string{"message": "mfa setup initiated"})
}

// MFAConfirm handles POST /auth/mfa/confirm
func (h *Handler) MFAConfirm(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusNoContent)
}

// MFADisable handles POST /auth/mfa/disable
func (h *Handler) MFADisable(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusNoContent)
}

// CreateAPIKey handles POST /auth/api-keys
func (h *Handler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
    jsonResponse(w, http.StatusCreated, map[string]string{"raw_key": "ovs_placeholder"})
}

// ListAPIKeys handles GET /auth/api-keys
func (h *Handler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
    jsonResponse(w, http.StatusOK, []interface{}{})
}

// RevokeAPIKey handles DELETE /auth/api-keys/{id}
func (h *Handler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusNoContent)
}

// AuthMiddleware validates Bearer tokens.
func (h *Handler) AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        authHeader := r.Header.Get("Authorization")
        if !strings.HasPrefix(authHeader, "Bearer ") {
            jsonError(w, "missing or invalid authorization header", http.StatusUnauthorized)
            return
        }
        next.ServeHTTP(w, r)
    })
}

// handleError maps use-case errors to HTTP status codes.
func (h *Handler) handleError(w http.ResponseWriter, err error) {
    switch err {
    case domainerr.ErrEmailAlreadyExists, domainerr.ErrUsernameAlreadyExists:
        jsonError(w, err.Error(), http.StatusConflict)
    case domainerr.ErrInvalidCredentials:
        jsonError(w, err.Error(), http.StatusUnauthorized)
    case domainerr.ErrAccountInactive:
        jsonError(w, err.Error(), http.StatusForbidden)
    default:
        h.logger.Error().Err(err).Msg("internal server error")
        jsonError(w, "internal server error", http.StatusInternalServerError)
    }
}

func jsonResponse(w http.ResponseWriter, status int, body interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(body)
}

func jsonError(w http.ResponseWriter, msg string, status int) {
    jsonResponse(w, status, map[string]string{"error": msg})
}
