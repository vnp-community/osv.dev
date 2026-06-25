// Command server starts the auth-service HTTP and gRPC servers.
// Wires all dependencies: DB → Redis → JWT → Use Cases → Handlers → Servers.
package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	authv1 "github.com/osv/shared/proto/gen/go/auth/v1"

	// Adapters
	grpcHandler "github.com/osv/identity-service/adapter/handler/grpc"
	httpHandler "github.com/osv/identity-service/adapter/handler/http"
	pgRepo "github.com/osv/identity-service/adapter/repository/postgres"

	// Infrastructure
	"github.com/osv/identity-service/internal/infrastructure/cache"
	"github.com/osv/identity-service/internal/infrastructure/jwt"
	"github.com/osv/identity-service/internal/infrastructure/oauth"

	// Use cases
	ucapikey "github.com/osv/identity-service/internal/usecase/manage_api_key"
	uclogin "github.com/osv/identity-service/internal/usecase/login"
	ucoauth "github.com/osv/identity-service/internal/usecase/oauth"
	ucrefresh "github.com/osv/identity-service/internal/usecase/refresh_token"
	ucregister "github.com/osv/identity-service/internal/usecase/register"
	uctotp "github.com/osv/identity-service/internal/usecase/totp"
	ucvalidate "github.com/osv/identity-service/internal/usecase/validate_token"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	logger := log.With().Str("service", "auth-service").Logger()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// ── PostgreSQL ────────────────────────────────────────────────────────
	// IDENTITY_DATABASE_URL takes priority, falls back to DATABASE_URL, then default.
	dbURL := getEnvFallback(
		"postgres://osv:osv_dev@localhost:5432/osvdb?sslmode=disable",
		"IDENTITY_DATABASE_URL",
		"DATABASE_URL",
	)
	dbPool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("postgres connect failed")
	}
	defer dbPool.Close()
	if err := dbPool.Ping(ctx); err != nil {
		logger.Fatal().Err(err).Msg("postgres ping failed")
	}
	logger.Info().Str("db", dbURL).Msg("postgres connected")

	// ── Redis ─────────────────────────────────────────────────────────────
	rdbOpts, err := redis.ParseURL(getEnv("REDIS_URL", "redis://localhost:6379"))
	if err != nil {
		logger.Fatal().Err(err).Msg("invalid REDIS_URL")
	}
	rdb := redis.NewClient(rdbOpts)
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Fatal().Err(err).Msg("redis connect failed")
	}
	logger.Info().Msg("redis connected")

	// ── Repositories ──────────────────────────────────────────────────────
	userRepo := pgRepo.NewUserRepo(dbPool)
	sessionRepo := pgRepo.NewSessionRepo(dbPool)
	apiKeyRepo := pgRepo.NewAPIKeyRepo(dbPool)
	oauthAcctRepo := pgRepo.NewOAuthAccountRepo(dbPool)

	// ── Infrastructure ────────────────────────────────────────────────────
	tokenCache := cache.NewTokenCache(rdb)

	jwtSvc, err := jwt.NewService(jwt.Config{
		PrivateKeyPath:  getEnv("JWT_PRIVATE_KEY_PATH", "secrets/jwt_private.pem"),
		Issuer:          getEnv("JWT_ISSUER", "https://auth.openvulnscan.io"),
		Audience:        []string{getEnv("JWT_AUDIENCE", "openvulnscan")},
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("JWT service init failed")
	}

	googleProvider := oauth.NewGoogleProvider(
		getEnv("GOOGLE_CLIENT_ID", ""),
		getEnv("GOOGLE_CLIENT_SECRET", ""),
		getEnv("GOOGLE_REDIRECT_URL", "http://localhost:9101/api/v1/auth/oauth/google/callback"),
	)
	githubProvider := oauth.NewGitHubProvider(
		getEnv("GITHUB_CLIENT_ID", ""),
		getEnv("GITHUB_CLIENT_SECRET", ""),
		getEnv("GITHUB_REDIRECT_URL", "http://localhost:9101/api/v1/auth/oauth/github/callback"),
	)

	// ── Use Cases ─────────────────────────────────────────────────────────
	registerUC := ucregister.NewUseCase(userRepo)
	loginUC := uclogin.NewUseCase(userRepo, sessionRepo, tokenCache, jwtSvc)
	refreshUC := ucrefresh.NewUseCase(userRepo, sessionRepo, jwtSvc)
	logoutUC := ucrefresh.NewLogoutUseCase(sessionRepo)
	validateUC := ucvalidate.NewUseCase(jwtSvc, tokenCache)
	oauthUC := ucoauth.NewUseCase(userRepo, sessionRepo, oauthAcctRepo, jwtSvc, googleProvider, githubProvider)
	apiKeyUC := ucapikey.NewUseCase(apiKeyRepo)
	totpUC := uctotp.New(userRepo)

	notifPrefRepo := pgRepo.NewPostgresNotifPrefRepo(dbPool)

	// ── gRPC Server ───────────────────────────────────────────────────────
	grpcPort := getEnvFallback("9001", "IDENTITY_GRPC_PORT", "GRPC_PORT")
	grpcLis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		logger.Fatal().Err(err).Str("port", grpcPort).Msg("gRPC listen failed")
	}

	grpcSrv := grpc.NewServer()
	authGRPCHandler := grpcHandler.NewAuthGRPCHandler(jwtSvc, tokenCache, apiKeyRepo, logger)
	authv1.RegisterAuthServiceServer(grpcSrv, authGRPCHandler)

	go func() {
		logger.Info().Str("port", grpcPort).Msg("gRPC server starting")
		if err := grpcSrv.Serve(grpcLis); err != nil {
			logger.Error().Err(err).Msg("gRPC server error")
		}
	}()

	// ── HTTP Server ───────────────────────────────────────────────────────
	httpPort := getEnvFallback("9101", "IDENTITY_HTTP_PORT", "HTTP_PORT")
	router := httpHandler.NewRouter(httpHandler.RouterDeps{
		RegisterUC:    registerUC,
		LoginUC:       loginUC,
		RefreshUC:     refreshUC,
		LogoutUC:      logoutUC,
		ValidateUC:    validateUC,
		OAuthUC:       oauthUC,
		APIKeyUC:      apiKeyUC,
		TotpUC:        totpUC,
		UserRepo:      userRepo,
		SessionRepo:   sessionRepo,
		NotifPrefRepo: notifPrefRepo,
		JWTSvc:        jwtSvc,
		Log:           logger,
	})

	httpSrv := httpHandler.StartHTTPServer(httpPort, router)

	go func() {
		logger.Info().Str("port", httpPort).Msg("HTTP server starting")
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error().Err(err).Msg("HTTP server error")
		}
	}()

	logger.Info().
		Str("grpc", ":"+grpcPort).
		Str("http", ":"+httpPort).
		Msg("auth-service ready")

	// ── Graceful Shutdown ─────────────────────────────────────────────────
	<-ctx.Done()
	logger.Info().Msg("shutting down auth-service...")

	shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	grpcSrv.GracefulStop()
	if err := httpSrv.Shutdown(shutCtx); err != nil {
		logger.Error().Err(err).Msg("HTTP shutdown error")
	}
	logger.Info().Msg("auth-service stopped")
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// getEnvFallback returns the value of the first non-empty env var from keys,
// or defaultVal if all are empty. Supports IDENTITY_-prefixed overrides.
func getEnvFallback(defaultVal string, keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return defaultVal
}
