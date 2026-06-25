package gateway

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/osv/gateway-service/internal/auth"
	handlers "github.com/osv/gateway-service/internal/bff/handlers"
	"github.com/osv/gateway-service/internal/proxy"
)

// EmbeddedConfig holds the configuration for embedded gateway-service.
type EmbeddedConfig struct {
	JWTSecret        string
	IdentityAddr     string
	DataAddr         string
	SearchAddr       string
	FindingAddr      string
	ScanAddr         string
	NotificationAddr string
	AIAddr           string
	RankingAddr      string
	AssetAddr        string // asset-service HTTP address
	ProductAddr      string // product-service HTTP address
	SLAAddr          string // sla-service HTTP address
	JiraAddr         string // jira-service HTTP address
	AuditAddr        string // audit-service HTTP address
}

// WireEmbedded wires up the gateway-service into the provided HTTP Mux.
// It initializes all BFF handlers, reverse proxies, rate limiters, and authentication middlewares.
func WireEmbedded(
	ctx context.Context,
	log zerolog.Logger,
	rdb *redis.Client,
	cfg EmbeddedConfig,
	mux *http.ServeMux,
) error {
	r := chi.NewRouter()

	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Recoverer)
	r.Use(cors.AllowAll().Handler)
	r.Use(chiMiddleware.Timeout(30 * time.Second))

	// ── Service addresses — read from EmbeddedConfig, fallback to defaults ──────
	// Spec: 01-architecture.md §2.2 Port Map
	identityHTTP     := coalesce(cfg.IdentityAddr, "http://localhost:8081")
	dataHTTP         := coalesce(cfg.DataAddr, "http://localhost:8082")
	searchHTTP       := coalesce(cfg.SearchAddr, os.Getenv("SEARCH_SERVICE_HTTP"), "http://localhost:8083") // [FIX BUG-001]
	findingHTTP      := coalesce(cfg.FindingAddr, "http://localhost:8085")
	slaHTTP          := coalesce(cfg.SLAAddr, "http://localhost:8086")
	notificationHTTP := coalesce(cfg.NotificationAddr, "http://localhost:8087")
	scanHTTP         := coalesce(cfg.ScanAddr, "http://localhost:8088")
	jiraHTTP         := coalesce(cfg.JiraAddr, "http://localhost:8089")
	auditHTTP        := coalesce(cfg.AuditAddr, "http://localhost:8090")
	assetHTTP        := coalesce(cfg.AssetAddr, "http://localhost:8091")
	productHTTP      := coalesce(cfg.ProductAddr, "http://localhost:8092")
	aiHTTP           := coalesce(cfg.AIAddr, "http://localhost:9103")
	rankingHTTP      := coalesce(cfg.RankingAddr, "http://localhost:8093")

	// Warn when any service addr falls back to localhost — will fail in container/K8s
	if strings.Contains(searchHTTP, "localhost") {
		log.Warn().Str("fallback", searchHTTP).
			Msg("SEARCH_SERVICE_HTTP not set, using localhost — CVE search will fail in container deployment")
	}

	// 1. Register PUBLIC auth BFF routes (no JWT required).
	// Protected auth routes (/me, /logout, /mfa/*, /profile, /api-keys) fall through
	// to authMiddleware(httpProxy) below which validates JWT first.
	authBFF := &handlers.PublicAuthHandler{IdentityURL: strings.TrimRight(identityHTTP, "/")}
	r.Post("/api/v1/auth/login",              authBFF.Login)
	r.Post("/api/v1/auth/refresh",            authBFF.Refresh)
	r.Get("/api/v1/auth/oauth/{provider}",    authBFF.OAuthRedirect)
	r.Get("/api/v1/auth/callback",            authBFF.OAuthCallback)

	// 2. Setup HTTP Proxy (used for routes not handled by BFF handlers above)
	upstreamURLs := map[string]string{
		// ── Canonical service names (used by RegisterUIAPIRoutes BFF handlers) ──
		"identity-service":     identityHTTP,
		"data-service":         dataHTTP,
		"search-service":       searchHTTP,  // [FIX BUG-001] was: "http://localhost:8083"
		"notification-service": notificationHTTP,
		"finding-service":      findingHTTP,
		"ai-service":           aiHTTP,
		"scan-service":         scanHTTP,
		"asset-service":        assetHTTP,
		"product-service":      productHTTP,
		"sla-service":          slaHTTP,
		"jira-service":         jiraHTTP,
		"audit-service":        auditHTTP,
		"ranking-service":      rankingHTTP,

		// ── Logical aliases used by dd_routes.go OVS proxy routes ──
		// Keys MUST match the Upstream field values in proxy.OVSRoutes / dd_routes.go
		"identity":          identityHTTP,
		"finding-mgmt":      findingHTTP,   // /api/v2/findings, /api/v2/finding-groups
		"sla":               slaHTTP,        // /api/v2/sla-configurations
		"notification":      notificationHTTP, // /api/v2/notification-rules, /api/v2/notifications
		"product-mgmt":      findingHTTP,   // /api/v2/products, /api/v2/engagements, /api/v2/tests
		"search":            searchHTTP,    // [FIX BUG-001] was: "http://localhost:8083"
		"ai":                aiHTTP,
		"scan-orchestrator": scanHTTP,
		"agent-service":     scanHTTP,
		"report":            findingHTTP,
		"audit":             findingHTTP,
		"jira":              jiraHTTP,
	}

	httpProxy, err := proxy.NewHTTPProxy(proxy.OVSRoutes, upstreamURLs, rdb, log)
	if err != nil {
		return err
	}

	// 3. Build auth middleware from OVSRoutes skip-paths
	var skipPaths []string
	for _, route := range proxy.OVSRoutes {
		if route.SkipAuth {
			skipPaths = append(skipPaths, route.PathPrefix)
		}
	}
	// Public auth routes registered above are also skip paths
	skipPaths = append(skipPaths,
		"/api/v1/auth/login",
		"/api/v1/auth/refresh",
		"/api/v1/auth/oauth",
		"/api/v1/auth/callback",
		"/api/v2/public/stats",
		"/api/v1/public/stats",
		// Public data routes — no auth required
		"/api/v1/kev",
		"/api/v1/epss/top",
		"/api/v1/epss/distribution",
		"/api/v1/cve/",
		"/api/v1/dbinfo",
		"/health",
		"/readyz",
	)

	apiKeyValidator := auth.NewAPIKeyValidator(nil, rdb)
	authMiddleware := auth.AuthVerify(cfg.JWTSecret, skipPaths, apiKeyValidator)

	// 4. Register BFF handlers with auth middleware.
	//    Use r.With(authMiddleware) to register routes directly into chi's trie.
	protected := r.With(authMiddleware)
	admin := r.With(authMiddleware, auth.RequireRole("Admin"))

	// SSE auth: also accept ?token=<jwt> (browser EventSource cannot set headers)
	sseAuth := r.With(sseTokenAuth(cfg.JWTSecret, skipPaths, apiKeyValidator))

	// Dashboard BFF (CR-UI-002) — fan-out to finding/scan/data services
	dashBFF := handlers.NewDashboardHandler(map[string]string{
		"finding": findingHTTP,
		"scan":    scanHTTP,
		"data":    dataHTTP,
		"sla":     findingHTTP, // SLA is in finding-service
		"asset":   assetHTTP,
	})
	handlers.RegisterDashboardRoutes(protected, dashBFF)

	// CR-013: Public stats (no-auth) — fan-out to scan + finding + data services
	publicBFF := handlers.NewPublicBFFHandler(map[string]string{
		"scan":    scanHTTP,
		"finding": findingHTTP,
		"data":    dataHTTP,
	})
	r.Get("/api/v2/public/stats", publicBFF.HandlePublicStats)
	r.Get("/api/v1/public/stats", publicBFF.HandlePublicStats)

	// UI API BFF (CR-UI-003..010) — all other authenticated API routes
	uiAPI := handlers.NewUIAPIHandler(map[string]string{
		"data":         dataHTTP,
		"search":       searchHTTP, // [FIX BUG-001] was: "http://localhost:8083"
		"finding":      findingHTTP,
		"scan":         scanHTTP,
		"asset":        assetHTTP,
		"product":      productHTTP, // product-service at :8092
		"ai":           aiHTTP,
		"report":       findingHTTP,
		"notification": notificationHTTP,
		"identity":     identityHTTP,
		"jira":         jiraHTTP,
		"sla":          slaHTTP, // sla-service at :8093
		"audit":        auditHTTP,
		"ranking":      rankingHTTP,
	})
	handlers.RegisterUIAPIRoutes(protected, admin, sseAuth, uiAPI)

	// 5. Generic proxy fallback — catches ALL routes NOT handled by BFF handlers above.
	//    r.NotFound is ONLY called when no registered route matches — this ensures
	//    BFF trie routes always take precedence over the proxy fallback.
	proxyHandler := authMiddleware(httpProxy)
	r.NotFound(proxyHandler.ServeHTTP)

	mux.Handle("/api/v1/", r)
	mux.Handle("/api/v2/", r)
	mux.Handle("/cve/", r)
	mux.Handle("/info", r)
	mux.Handle("/.well-known/", r)
	mux.Handle("/internal/", r)
	mux.Handle("/", r)
	return nil
}

// sseTokenAuth wraps authMiddleware to also accept JWT via ?token= query param.
// Required for browser EventSource which cannot set Authorization headers.
func sseTokenAuth(secret string, skipPaths []string, apiKeyValidator *auth.APIKeyValidator) func(http.Handler) http.Handler {
	base := auth.AuthVerify(secret, skipPaths, apiKeyValidator)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Inject ?token= as Authorization: Bearer <token> if not already set
			if r.Header.Get("Authorization") == "" {
				if tok := r.URL.Query().Get("token"); tok != "" {
					r = r.Clone(r.Context())
					r.Header.Set("Authorization", "Bearer "+tok)
				}
			}
			base(next).ServeHTTP(w, r)
		})
	}
}

// coalesce returns the first non-empty string value.
// Used to prefer EmbeddedConfig values over hardcoded defaults.
func coalesce(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
