// wire.go — Dependency wiring for the OSV orchestrator.
// Builds the list of embedded Service implementations based on runtime config.
//
// This is ADDITIVE — apps/osv/cmd/server/main.go is NOT modified here.
// wire.go is called from cmd/server/orchestrator_runner.go (new file).
package config

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/osv/apps/osv/internal/orchestrator"
	"github.com/osv/apps/osv/internal/orchestrator/adapters"
	asset "github.com/google/osv.dev/services/asset-service"
	audit "github.com/osv/audit-service"
	aiembed "github.com/osv/ai-service/embed"
	dataembed "github.com/osv/data-service/embed"
	finding "github.com/osv/finding-service"
	"github.com/osv/gateway-service"
	"github.com/osv/identity-service"
	jirasvc "github.com/osv/jira-service"
	notif "github.com/osv/notification-service"
	product "github.com/google/osv.dev/services/product-service"
	ranking "github.com/osv/ranking-service"
	scan "github.com/osv/scan-service"
	search "github.com/osv/search-service"
	sla "github.com/osv/sla-service"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// WireServices constructs all embedded service adapters based on cfg.
// Returns the list of services to run under the Supervisor.
// In microservices mode, uses in-memory gRPC and mounted HTTP handlers.
func WireServices(cfg *Config) []orchestrator.Service {
	// Initialize the shared DB/NATS connections
	ctx := context.Background()
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()

	// 1. Init DB Pool
	dbPool, err := pgxpool.New(ctx, cfg.EmbeddedInfra.PostgresDSN)
	if err != nil {
		slog.Error("failed to connect to postgres", "err", err)
	}

	// 2. Init Redis
	rdbOpts, err := redis.ParseURL(cfg.EmbeddedInfra.RedisURL)
	if err != nil {
		slog.Error("failed to parse redis url", "err", err)
	}
	rdb := redis.NewClient(rdbOpts)

	// Resolve JWT private key path:
	// 1. Prefer JWT_PRIVATE_KEY_PATH env (Docker: /run/secrets/jwt_private.pem)
	// 2. Fallback: auto-generate ephemeral key at secrets/jwt_private.pem (local dev)
	privKeyPath := os.Getenv("JWT_PRIVATE_KEY_PATH")
	if privKeyPath == "" {
		privKeyPath = "secrets/jwt_private.pem"
	}
	if _, err := os.Stat(privKeyPath); os.IsNotExist(err) {
		os.MkdirAll("secrets", 0755)
		key, _ := rsa.GenerateKey(rand.Reader, 4096)
		pemData := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(key),
		})
		os.WriteFile(privKeyPath, pemData, 0600)
	}


	// ── Identity Service Wiring ──────────────────────────────────────────
	identitySvc := adapters.NewEmbeddedService("identity-service", 8081)
	err = identity.WireEmbedded(ctx, logger, dbPool, rdb, privKeyPath, identitySvc.Mux, identitySvc.Server)
	if err != nil {
		slog.Error("failed to wire embedded identity-service", "err", err)
	}

	// ── Other Placeholder Services ───────────────────────────────────────
	// FIX BUG-002: data-service now uses full wiring (Sprint C) instead of placeholder
	dataSvc := dataembed.NewDataServiceEmbeddedServer(dataembed.DataServiceEmbeddedConfig{
		HTTPPort:    8082,
		GRPCPort:    50053,
		NATSURL:     cfg.Services.NATSURL,
		MongoURI:    os.Getenv("MONGO_URI"),
		MongoDB:     os.Getenv("MONGO_DB"),
		PostgresDSN: cfg.EmbeddedInfra.PostgresDSN,
		NVDAPIKey:   os.Getenv("NVD_API_KEY"),
	})
	// ── Search Service Wiring ─────────────────────────────────────────────
	searchSvc := adapters.NewEmbeddedService("search-service", 8083)
	if err := search.WireEmbedded(ctx, logger, dbPool, searchSvc.Mux); err != nil {
		slog.Error("failed to wire embedded search-service", "err", err)
	}
	aiSvc := aiembed.NewAIServiceEmbeddedServer(aiembed.AIServiceEmbeddedConfig{
		HTTPPort:    9103,
		NATSURL:     cfg.Services.NATSURL,
		PostgresDSN: cfg.EmbeddedInfra.PostgresDSN,
		MongoURI:    os.Getenv("MONGO_URI"),
	})

	// ── Notification Service Wiring ───────────────────────────────────────
	notifSvc := adapters.NewEmbeddedService("notification-service", 8087)
	if err := notif.WireEmbedded(ctx, logger, dbPool, notifSvc.Mux); err != nil {
		slog.Error("failed to wire embedded notification-service", "err", err)
	}

	// ── Scan Service Wiring ────────────────────────────────────────────
	scanSvc := adapters.NewEmbeddedService("scan-service", 8088)
	if err = scan.WireEmbedded(ctx, logger, dbPool, scanSvc.Mux); err != nil {
		slog.Error("failed to wire embedded scan-service", "err", err)
	}

	// ── SLA Service Wiring ───────────────────────────────
	slaSvc := adapters.NewEmbeddedService("sla-service", 8086) // port 8086 per architecture.md
	if err := sla.WireEmbedded(ctx, logger, dbPool, slaSvc.Mux); err != nil {
		slog.Error("failed to wire embedded sla-service", "err", err)
	}

	// ── Asset Service Wiring ───────────────────────────────
	assetSvc := adapters.NewEmbeddedService("asset-service", 8091)
	if err := asset.WireEmbedded(ctx, logger, dbPool, assetSvc.Mux); err != nil {
		slog.Error("failed to wire embedded asset-service", "err", err)
	}

	// ── Product Service Wiring ─────────────────────────────
	productSvc := adapters.NewEmbeddedService("product-service", 8092)
	if err := product.WireEmbedded(ctx, logger, dbPool, productSvc.Mux); err != nil {
		slog.Error("failed to wire embedded product-service", "err", err)
	}


	jiraSvc := adapters.NewEmbeddedService("jira-service", 8089)
	if err := jirasvc.WireEmbedded(ctx, logger, dbPool, jiraSvc.Mux); err != nil {
		slog.Error("failed to wire jira-service", "err", err)
	}

	// 10. Start Gateway Service (BFF/Proxy)
	// Gateway verifies JWT tokens issued by identity-service (RS256).
	// We load the same RSA private key and derive the public key PEM,
	// then pass it to the gateway so AuthVerify can validate RS256 tokens.
	jwtSecretForGateway := cfg.EmbeddedInfra.JWTSecret // fallback HMAC
	if privKeyData, err := os.ReadFile(privKeyPath); err == nil {
		block, _ := pem.Decode(privKeyData)
		if block != nil {
			if privKey, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
				pubKeyDER, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
				if err == nil {
					pubKeyPEM := string(pem.EncodeToMemory(&pem.Block{
						Type:  "PUBLIC KEY",
						Bytes: pubKeyDER,
					}))
					jwtSecretForGateway = pubKeyPEM
				}
			}
		}
	}

	gatewaySvc := adapters.NewEmbeddedService("gateway-service", cfg.HTTP.Port)
	err = gateway.WireEmbedded(ctx, logger, rdb, gateway.EmbeddedConfig{
		JWTSecret:        jwtSecretForGateway,
		IdentityAddr:     os.Getenv("IDENTITY_SERVICE_ADDR"),
		AssetAddr:        os.Getenv("ASSET_SERVICE_ADDR"),
		SearchAddr:       os.Getenv("SEARCH_SERVICE_ADDR"),
		AIAddr:           os.Getenv("AI_SERVICE_ADDR"),
		JiraAddr:         os.Getenv("JIRA_SERVICE_ADDR"),
		DataAddr:         os.Getenv("DATA_SERVICE_ADDR"),
		FindingAddr:      os.Getenv("FINDING_SERVICE_ADDR"),
		ScanAddr:         os.Getenv("SCAN_SERVICE_ADDR"),
		NotificationAddr: os.Getenv("NOTIFICATION_SERVICE_ADDR"),
		ProductAddr:      os.Getenv("PRODUCT_SERVICE_ADDR"),
		SLAAddr:          os.Getenv("SLA_SERVICE_ADDR"),
		AuditAddr:        os.Getenv("AUDIT_SERVICE_ADDR"),
		RankingAddr:      os.Getenv("RANKING_SERVICE_ADDR"),
	}, gatewaySvc.Mux)

	if err != nil {
		slog.Error("failed to wire embedded gateway-service", "err", err)
	}

	// ── Finding Service Wiring ────────────────────────────────────────────
	findingSvc := adapters.NewEmbeddedService("finding-service", 8085)
	if err = finding.WireEmbedded(ctx, logger, dbPool, findingSvc.Mux); err != nil {
		slog.Error("failed to wire embedded finding-service", "err", err)
	}

	// ── Audit Service Wiring ──────────────────────────────────────────────
	// audit-service shares the identity-service HTTP mux (port 8081) since
	// /api/v1/audit-log is an admin route in the same identity namespace.
	if err := audit.WireEmbedded(ctx, logger, dbPool, identitySvc.Mux); err != nil {
		slog.Error("failed to wire embedded audit-service", "err", err)
	}

	// ── Ranking Service Wiring ───────────────────────────────────────────
	// ranking-service uses MongoDB — soft-fail if Mongo unavailable
	rankingSvc := adapters.NewEmbeddedService("ranking-service", 8093)
	if err := ranking.WireEmbedded(ctx, logger, rankingSvc.Mux); err != nil {
		slog.Error("failed to wire embedded ranking-service", "err", err)
	}

	return []orchestrator.Service{
		dataSvc,
		searchSvc,
		aiSvc,
		identitySvc, // audit routes also mounted on this mux
		notifSvc,
		findingSvc,
		slaSvc,
		assetSvc,
		productSvc,
		jiraSvc,
		scanSvc,
		rankingSvc,
		gatewaySvc,
	}
}
