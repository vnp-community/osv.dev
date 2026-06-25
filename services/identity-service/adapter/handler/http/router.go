// Package http provides the chi router setup for the auth service.
package http

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	jwtpkg "github.com/osv/identity-service/internal/infrastructure/jwt"
	ucapikey "github.com/osv/identity-service/internal/usecase/manage_api_key"
	uclogin "github.com/osv/identity-service/internal/usecase/login"
	ucoauth "github.com/osv/identity-service/internal/usecase/oauth"
	ucrefresh "github.com/osv/identity-service/internal/usecase/refresh_token"
	ucregister "github.com/osv/identity-service/internal/usecase/register"
	uctotp "github.com/osv/identity-service/internal/usecase/totp"
	ucvalidate "github.com/osv/identity-service/internal/usecase/validate_token"
	ucadmin "github.com/osv/identity-service/internal/usecase/admin_user" // [FIX TASK-HC-014]
	"github.com/osv/identity-service/internal/domain/repository"
	postgres "github.com/osv/identity-service/adapter/repository/postgres" // TASK-002 FIX: NotifPrefRepository
	pginfra "github.com/osv/identity-service/internal/infra/postgres"       // [FIX TASK-HC-014]
	"github.com/rs/zerolog"
)

// RouterDeps holds all handler dependencies for dependency injection.
type RouterDeps struct {
	RegisterUC    *ucregister.UseCase
	LoginUC       *uclogin.UseCase
	RefreshUC     *ucrefresh.UseCase
	LogoutUC      *ucrefresh.LogoutUseCase
	ValidateUC    *ucvalidate.UseCase
	OAuthUC       *ucoauth.UseCase
	APIKeyUC      *ucapikey.UseCase
	TotpUC        *uctotp.UseCase
	UserRepo      repository.UserRepository
	SessionRepo   SessionRepository
	NotifPrefRepo postgres.NotifPrefRepository
	JWTSvc        *jwtpkg.Service
	Log           zerolog.Logger
	// [FIX TASK-HC-009] SettingsHandler wires platform_settings DB endpoints
	SettingsHandler SettingsGetter
	// [FIX TASK-HC-010] RBACRepo enables DB-backed role metadata; set from embedded.go
	RBACRepo *RBACRepo
	// [FIX TASK-HC-014] Invitation flow
	InviteUC       *ucadmin.InviteUserUseCase
	InvitationRepo *pginfra.InvitationRepo
	AppBaseURL     string
}

// SettingsGetter is the minimal interface for the settings handler.
type SettingsGetter interface {
	GetSettings(w http.ResponseWriter, r *http.Request)
	UpdateSettings(w http.ResponseWriter, r *http.Request)
}

// NewRouter builds the chi router with all auth service routes.
// Route map:
//   POST   /api/v1/auth/register
//   POST   /api/v1/auth/login
//   POST   /api/v1/auth/refresh
//   POST   /api/v1/auth/logout            (requires X-User-ID header from gateway)
//   GET    /api/v1/auth/me                (requires X-User-ID header from gateway)
//   GET    /api/v1/auth/providers
//   GET    /api/v1/auth/oauth/google
//   GET    /api/v1/auth/oauth/google/callback
//   GET    /api/v1/auth/oauth/github
//   GET    /api/v1/auth/oauth/github/callback
//   POST   /api/v1/auth/api-keys
//   GET    /api/v1/auth/api-keys
//   DELETE /api/v1/auth/api-keys/{key_id}
//   GET    /.well-known/jwks.json
//   GET    /health/live
//   GET    /health/ready
func NewRouter(deps RouterDeps) http.Handler {
	authH := NewAuthHandler(
		deps.RegisterUC, deps.LoginUC, deps.RefreshUC, deps.LogoutUC,
		deps.ValidateUC, deps.JWTSvc, deps.UserRepo, deps.Log,
	)
	oauthH := NewOAuthHandler(deps.OAuthUC, deps.Log)
	apiKeyH := NewAPIKeyHTTPHandler(deps.APIKeyUC, deps.Log)
	totpH := NewTOTPHandler(deps.TotpUC, deps.Log)
	adminH := NewAdminHandler(deps.UserRepo, deps.APIKeyUC, deps.Log)
	// [FIX TASK-HC-010] Wire RBACRepo when provided
	if deps.RBACRepo != nil {
		adminH = adminH.WithRBACRepo(deps.RBACRepo)
	}
	// [FIX TASK-HC-014] Wire InviteUseCase when provided
	if deps.InviteUC != nil {
		adminH = adminH.WithInviteUC(deps.InviteUC, deps.AppBaseURL)
	}
	// [FIX TASK-HC-014] Rebuild authH with invitation repo for accept-invite
	authH = NewAuthHandlerWithInvitation(
		deps.RegisterUC, deps.LoginUC, deps.RefreshUC, deps.LogoutUC,
		deps.ValidateUC, deps.JWTSvc, deps.UserRepo,
		deps.InvitationRepo, deps.Log,
	)
	profileH := NewProfileHandler(deps.UserRepo, deps.SessionRepo, deps.NotifPrefRepo, deps.Log)

	r := chi.NewRouter()

	// ── Middleware ──────────────────────────────────────────────────────────
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(zerolog_middleware(deps.Log))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Request-ID"},
		AllowCredentials: false,
		MaxAge:           3600,
	}))

	// ── Health ──────────────────────────────────────────────────────────────
	r.Get("/health", HealthCheck)
	r.Get("/health/live", HealthCheck)
	r.Get("/health/ready", HealthCheck)

	// ── JWKS (public key for external validators) ───────────────────────────
	r.Get("/.well-known/jwks.json", authH.JWKS)

	// ── Auth routes ─────────────────────────────────────────────────────────
	r.Route("/api/v1/auth", func(r chi.Router) {
		// Public endpoints (no auth required)
		r.Post("/register", authH.Register)
		r.Post("/login", authH.Login)
		r.Post("/refresh", authH.Refresh)
		r.Get("/providers", GetProviders)
		// [FIX TASK-HC-014] Accept invitation endpoint
		r.Get("/accept-invite", authH.AcceptInvite)

		// OAuth2 flows
		r.Get("/oauth/google", oauthH.InitiateGoogle)
		r.Get("/oauth/google/callback", oauthH.CallbackGoogle)
		r.Get("/oauth/github", oauthH.InitiateGitHub)
		r.Get("/oauth/github/callback", oauthH.CallbackGitHub)

		// Authenticated endpoints (X-User-ID injected by api-gateway)
		r.Post("/logout", authH.Logout)
		r.Get("/me", authH.Me)

		// TOTP management
		r.Get("/totp/setup", totpH.Setup)
		r.Post("/totp/setup", totpH.Setup)
		r.Post("/totp/verify", totpH.Verify)
		r.Delete("/totp", totpH.Disable)

		// API key management
		r.Post("/api-keys", apiKeyH.CreateAPIKey)
		r.Get("/api-keys", apiKeyH.ListAPIKeys)
		r.Delete("/api-keys/{key_id}", apiKeyH.RevokeAPIKey)

		// Profile management
		r.Get("/profile", profileH.GetProfile)
		r.Patch("/profile", profileH.UpdateProfile)
		r.Post("/profile/change-password", profileH.ChangePassword)
		r.Get("/profile/sessions", profileH.ListSessions)
		r.Delete("/profile/sessions/{sessionId}", profileH.RevokeSession)
		r.Get("/profile/notifications/settings", profileH.GetNotifSettings)
		r.Put("/profile/notifications/settings", profileH.UpdateNotifSettings)
	})

	// CR-012: Top-level /api/v1/api-keys aliases
	// OVSRoutes proxies /api/v1/api-keys → identity:8081 (no path rewrite).
	// These routes ensure requests reach the correct handlers.
	r.Get("/api/v1/api-keys", apiKeyH.ListAPIKeys)
	r.Post("/api/v1/api-keys", apiKeyH.CreateAPIKey)
	r.Delete("/api/v1/api-keys/{key_id}", apiKeyH.RevokeAPIKey)

	// Top-level /api/v1/profile aliases
	r.Get("/api/v1/profile", profileH.GetProfile)
	r.Patch("/api/v1/profile", profileH.UpdateProfile)
	r.Post("/api/v1/profile/change-password", profileH.ChangePassword)
	r.Get("/api/v1/profile/sessions", profileH.ListSessions)
	r.Delete("/api/v1/profile/sessions/{sessionId}", profileH.RevokeSession)
	r.Get("/api/v1/profile/notifications/settings", profileH.GetNotifSettings)
	r.Put("/api/v1/profile/notifications/settings", profileH.UpdateNotifSettings)

	// ── Admin routes ────────────────────────────────────────────────────────
	r.Route("/api/v1/admin", func(r chi.Router) {
		r.Get("/users", adminH.ListUsers)

		// SEED-001: Direct & bulk user creation
		// IMPORTANT: literal /users/bulk MUST be registered before /users/{id}
		r.Post("/users", adminH.CreateUser)
		r.Post("/users/bulk", adminH.BulkCreateUsers) // literal before wildcard
		r.Post("/users/invite", adminH.InviteUser)

		r.Get("/users/{id}", adminH.GetUser)          // CR-001
		r.Patch("/users/{id}", adminH.UpdateUser)
		r.Post("/users/{id}/unlock", adminH.UnlockUser)
		r.Post("/users/{id}/api-keys", adminH.CreateAPIKeyForUser) // SEED-001

		// SEED-001: Role assignment
		r.Post("/users/{id}/roles", adminH.AssignRole)

		r.Get("/roles", adminH.GetRBACMatrix)

		// [FIX TASK-HC-009] Platform settings from PostgreSQL
		if deps.SettingsHandler != nil {
			r.Get("/settings", deps.SettingsHandler.GetSettings)
			r.Put("/settings", deps.SettingsHandler.UpdateSettings)
			r.Patch("/settings", deps.SettingsHandler.UpdateSettings)
		}
	})

	return r

}

// zerolog_middleware returns a chi-compatible middleware that logs requests.
func zerolog_middleware(log zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			log.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", ww.Status()).
				Dur("latency", time.Since(start)).
				Str("request_id", middleware.GetReqID(r.Context())).
				Msg("http")
		})
	}
}

// StartHTTPServer starts the HTTP server with the given router (used by main.go).
func StartHTTPServer(port string, handler http.Handler) *http.Server {
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return srv
}
