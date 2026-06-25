package identity

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"

	grpcHandler "github.com/osv/identity-service/adapter/handler/grpc"
	httpHandler "github.com/osv/identity-service/adapter/handler/http"
	pgRepo "github.com/osv/identity-service/adapter/repository/postgres"
	authv1 "github.com/osv/shared/proto/gen/go/auth/v1"
	"github.com/osv/identity-service/internal/infrastructure/cache"
	"github.com/osv/identity-service/internal/infrastructure/jwt"
	"github.com/osv/identity-service/internal/infrastructure/oauth"
	identityhttp "github.com/osv/identity-service/internal/delivery/http" // [FIX TASK-HC-009]
	pginfra "github.com/osv/identity-service/internal/infra/postgres"     // [FIX TASK-HC-014]
	smtpinfra "github.com/osv/identity-service/internal/infra/smtp"        // [FIX TASK-HC-014]
	ucadmin "github.com/osv/identity-service/internal/usecase/admin_user"  // [FIX TASK-HC-014]
	uclogin "github.com/osv/identity-service/internal/usecase/login"
	ucapikey "github.com/osv/identity-service/internal/usecase/manage_api_key"
	ucoauth "github.com/osv/identity-service/internal/usecase/oauth"
	ucrefresh "github.com/osv/identity-service/internal/usecase/refresh_token"
	ucregister "github.com/osv/identity-service/internal/usecase/register"
	ucvalidate "github.com/osv/identity-service/internal/usecase/validate_token"
)

// WireEmbedded initializes all usecases and handlers for the identity service,
// and mounts them onto the provided HTTP Mux and gRPC server.
// This is used by the OSV Monolith to embed the service without exposing internal packages.
func WireEmbedded(
	ctx context.Context,
	logger zerolog.Logger,
	dbPool *pgxpool.Pool,
	rdb *redis.Client,
	jwtKeyPath string,
	mux *http.ServeMux,
	grpcSrv *grpc.Server,
) error {
	userRepo := pgRepo.NewUserRepo(dbPool)
	sessionRepo := pgRepo.NewSessionRepo(dbPool)
	apiKeyRepo := pgRepo.NewAPIKeyRepo(dbPool)
	oauthAcctRepo := pgRepo.NewOAuthAccountRepo(dbPool)

	tokenCache := cache.NewTokenCache(rdb)

	jwtSvc, err := jwt.NewService(jwt.Config{
		PrivateKeyPath:  jwtKeyPath,
		Issuer:          "https://c12.openledger.vn",
		Audience:        []string{"openvulnscan"},
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
	})
	if err != nil {
		return err
	}

	// MOCK-011 FIX: đọc OAuth credentials từ environment variables
	googleClientID     := os.Getenv("GOOGLE_CLIENT_ID")
	googleClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	googleRedirectURL  := os.Getenv("GOOGLE_REDIRECT_URL")
	if googleRedirectURL == "" {
		googleRedirectURL = "http://localhost:8080/api/v1/auth/callback?provider=google"
	}

	githubClientID     := os.Getenv("GITHUB_CLIENT_ID")
	githubClientSecret := os.Getenv("GITHUB_CLIENT_SECRET")
	githubRedirectURL  := os.Getenv("GITHUB_REDIRECT_URL")
	if githubRedirectURL == "" {
		githubRedirectURL = "http://localhost:8080/api/v1/auth/callback?provider=github"
	}

	googleProvider := oauth.NewGoogleProvider(googleClientID, googleClientSecret, googleRedirectURL)
	githubProvider := oauth.NewGitHubProvider(githubClientID, githubClientSecret, githubRedirectURL)

	if googleClientID == "" {
		logger.Warn().Msg("identity-service: GOOGLE_CLIENT_ID not set — Google OAuth login disabled")
	}
	if githubClientID == "" {
		logger.Warn().Msg("identity-service: GITHUB_CLIENT_ID not set — GitHub OAuth login disabled")
	}

	registerUC := ucregister.NewUseCase(userRepo)
	loginUC := uclogin.NewUseCase(userRepo, sessionRepo, tokenCache, jwtSvc)
	refreshUC := ucrefresh.NewUseCase(userRepo, sessionRepo, jwtSvc)
	logoutUC := ucrefresh.NewLogoutUseCase(sessionRepo)
	validateUC := ucvalidate.NewUseCase(jwtSvc, tokenCache)
	oauthUC := ucoauth.NewUseCase(userRepo, sessionRepo, oauthAcctRepo, jwtSvc, googleProvider, githubProvider)
	apiKeyUC := ucapikey.NewUseCase(apiKeyRepo)

	// [FIX TASK-HC-009] Wire platform settings from PostgreSQL
	settingsRepo := identityhttp.NewPlatformSettingsRepo(dbPool)
	settingsH := identityhttp.NewSettingsHandler(settingsRepo)

	// [FIX TASK-HC-010] Wire RBAC repo from PostgreSQL
	rbacRepo := httpHandler.NewRBACRepo(dbPool)

	// [FIX TASK-HC-014] Wire invitation repo + SMTP sender + InviteUserUseCase
	invitationRepo := pginfra.NewInvitationRepo(dbPool)
	var emailSender ucadmin.EmailSenderIface
	if smtpHost := os.Getenv("SMTP_HOST"); smtpHost != "" {
		emailSender = smtpinfra.New(
			smtpHost,
			os.Getenv("SMTP_PORT"),
			os.Getenv("SMTP_USER"),
			os.Getenv("SMTP_PASSWORD"),
			os.Getenv("SMTP_FROM"),
		)
		logger.Info().Str("smtp_host", smtpHost).Msg("identity-service: SMTP sender configured")
	} else {
		logger.Warn().Msg("identity-service: SMTP_HOST not set — invitation emails disabled")
	}
	appBaseURL := os.Getenv("APP_BASE_URL")
	if appBaseURL == "" {
		appBaseURL = "https://c12.openledger.vn"
	}
	inviteUC := ucadmin.NewInviteUserUseCase(userRepo, invitationRepo, emailSender, logger)

	router := httpHandler.NewRouter(httpHandler.RouterDeps{
		RegisterUC:      registerUC,
		LoginUC:         loginUC,
		RefreshUC:       refreshUC,
		LogoutUC:        logoutUC,
		ValidateUC:      validateUC,
		OAuthUC:         oauthUC,
		APIKeyUC:        apiKeyUC,
		UserRepo:        userRepo,
		SessionRepo:     sessionRepo,
		NotifPrefRepo:   pgRepo.NewPostgresNotifPrefRepo(dbPool),
		JWTSvc:          jwtSvc,
		Log:             logger,
		SettingsHandler: settingsH,  // [FIX TASK-HC-009]
		RBACRepo:        rbacRepo,   // [FIX TASK-HC-010]
		InviteUC:        inviteUC,   // [FIX TASK-HC-014]
		InvitationRepo:  invitationRepo, // [FIX TASK-HC-014]
		AppBaseURL:      appBaseURL, // [FIX TASK-HC-014]
	})

	mux.Handle("/", router)

	authGRPCHandler := grpcHandler.NewAuthGRPCHandler(jwtSvc, tokenCache, apiKeyRepo, logger)
	authv1.RegisterAuthServiceServer(grpcSrv, authGRPCHandler)

	return nil
}
