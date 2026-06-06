// Package http provides the chi router setup for the auth service.
package http

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	jwtpkg "github.com/osv/auth-service/internal/infrastructure/jwt"
	ucapikey "github.com/osv/auth-service/internal/usecase/manage_api_key"
	uclogin "github.com/osv/auth-service/internal/usecase/login"
	ucoauth "github.com/osv/auth-service/internal/usecase/oauth"
	ucrefresh "github.com/osv/auth-service/internal/usecase/refresh_token"
	ucregister "github.com/osv/auth-service/internal/usecase/register"
	ucvalidate "github.com/osv/auth-service/internal/usecase/validate_token"
	"github.com/rs/zerolog"
)

// RouterDeps holds all handler dependencies for dependency injection.
type RouterDeps struct {
	RegisterUC *ucregister.UseCase
	LoginUC    *uclogin.UseCase
	RefreshUC  *ucrefresh.UseCase
	LogoutUC   *ucrefresh.LogoutUseCase
	ValidateUC *ucvalidate.UseCase
	OAuthUC    *ucoauth.UseCase
	APIKeyUC   *ucapikey.UseCase
	JWTSvc     *jwtpkg.Service
	Log        zerolog.Logger
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
		deps.ValidateUC, deps.JWTSvc, deps.Log,
	)
	oauthH := NewOAuthHandler(deps.OAuthUC, deps.Log)
	apiKeyH := NewAPIKeyHTTPHandler(deps.APIKeyUC, deps.Log)

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

		// OAuth2 flows
		r.Get("/oauth/google", oauthH.InitiateGoogle)
		r.Get("/oauth/google/callback", oauthH.CallbackGoogle)
		r.Get("/oauth/github", oauthH.InitiateGitHub)
		r.Get("/oauth/github/callback", oauthH.CallbackGitHub)

		// Authenticated endpoints (X-User-ID injected by api-gateway)
		r.Post("/logout", authH.Logout)
		r.Get("/me", authH.Me)

		// API key management
		r.Post("/api-keys", apiKeyH.CreateAPIKey)
		r.Get("/api-keys", apiKeyH.ListAPIKeys)
		r.Delete("/api-keys/{key_id}", apiKeyH.RevokeAPIKey)
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
