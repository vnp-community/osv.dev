// Package app — app.go
// App là container chứa tất cả service goroutines và shared infrastructure.
// Kiến trúc: goroutine-per-service, giao tiếp qua gRPC bufconn + NATS.
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/test/bufconn"

	"github.com/osv/apps/openvulnscan/internal/events"
	"github.com/osv/apps/openvulnscan/internal/runners"
	"github.com/osv/apps/openvulnscan/internal/syslog"
	"github.com/osv/apps/openvulnscan/internal/transport"

	// shared/proto
	authv1    "github.com/osv/shared/proto/gen/go/auth/v1"
	findingv1 "github.com/osv/shared/proto/gen/go/finding/v1"
	productv1 "github.com/osv/shared/proto/gen/go/product/v1"
)

// App là main application container.
type App struct {
	cfg      *Config
	Registry *Registry
	log      zerolog.Logger

	// Shared infrastructure
	db    *pgxpool.Pool
	nc    *nats.Conn
	redis *redis.Client

	// bufconn listeners — per goroutine service
	authLis         *bufconn.Listener
	findingLis      *bufconn.Listener
	scanLis         *bufconn.Listener
	vulnLis         *bufconn.Listener
	productLis      *bufconn.Listener
	notificationLis *bufconn.Listener
	ingestionLis    *bufconn.Listener
	reportLis       *bufconn.Listener
	queryLis        *bufconn.Listener

	// gRPC clients — tạo sau khi Start()
	AuthClient    authv1.AuthServiceClient
	FindingClient findingv1.FindingServiceClient
	ProductClient productv1.ProductServiceClient

	// Service runners (expose HTTP handlers)
	ScanRunner         *runners.ScanRunner
	VulnRunner         *runners.VulnRunner
	ProductRunner      *runners.ProductRunner
	NotificationRunner *runners.NotificationRunner
	IngestionRunner    *runners.IngestionRunner
	ReportRunner       *runners.ReportRunner
	QueryRunner        *runners.QueryRunner

	// SIEM syslog channel
	SyslogChannel *syslog.Channel

	// AuthHTTP: direct auth operations (not gRPC — auth proto only has ValidateToken)
	authHTTP *authHTTPHandler
}

// DB returns the shared Postgres connection pool (implements DashboardQuerier).
func (a *App) DB() *pgxpool.Pool { return a.db }

// New tạo App mới, wire-up tất cả dependencies.
func New(cfg *Config) (*App, error) {
	l := log.With().Str("component", "app").Logger()
	a := &App{
		cfg:      cfg,
		log:      l,
		Registry: NewRegistry(l),
	}

	// 1. Connect shared infrastructure
	if err := a.connectInfra(); err != nil {
		return nil, fmt.Errorf("connect infra: %w", err)
	}

	// 2. Setup JetStream streams
	if _, err := events.SetupJetStream(a.nc); err != nil {
		l.Warn().Err(err).Msg("jetstream setup warning")
	}

	// 3. Tạo bufconn listeners
	a.authLis         = transport.NewBufConnListener()
	a.findingLis      = transport.NewBufConnListener()
	a.scanLis         = transport.NewBufConnListener()
	a.vulnLis         = transport.NewBufConnListener()
	a.productLis      = transport.NewBufConnListener()
	a.notificationLis = transport.NewBufConnListener()
	a.ingestionLis    = transport.NewBufConnListener()
	a.reportLis       = transport.NewBufConnListener()
	a.queryLis        = transport.NewBufConnListener()

	// 4. Register service goroutines

	// T04 — auth-service (Bridge Pattern — ValidateToken + ValidateAPIKey gRPC)
	a.Registry.Register(runners.NewAuthRunner(
		runners.AuthRunnerConfig{
			DBURL:             cfg.Database.URL,
			RedisURL:          cfg.Redis.URL,
			JWTPrivateKeyPath: cfg.Auth.JWTPrivateKeyPath,
			JWTIssuer:         cfg.Auth.JWTIssuer,
			JWTAudience:       cfg.Auth.JWTAudience,
			JWTAccessTTL:      cfg.Auth.JWTAccessTTL,
			JWTRefreshTTL:     cfg.Auth.JWTRefreshTTL,
			GoogleClientID:    cfg.Auth.GoogleClientID,
			GoogleSecret:      cfg.Auth.GoogleSecret,
			GoogleRedirectURL: cfg.Auth.GoogleRedirectURL,
			GitHubClientID:    cfg.Auth.GitHubClientID,
			GitHubSecret:      cfg.Auth.GitHubSecret,
			GitHubRedirectURL: cfg.Auth.GitHubRedirectURL,
		},
		a.authLis,
	))

	// T08 — finding-service (Bridge Pattern: direct Postgres gRPC + NATS scan.completed subscriber)
	a.Registry.Register(runners.NewFindingRunner(
		runners.FindingRunnerConfig{
			DBURL: cfg.Database.URL,
		},
		a.nc,
		a.findingLis,
	))

	// T05/T06 — scan-service (Bridge Pattern: HTTP + WorkerPool)
	a.ScanRunner = runners.NewScanRunner(
		runners.ScanRunnerConfig{
			DBURL:          cfg.Database.URL,
			NmapBinary:     cfg.Scan.NmapBinary,
			ZAPApiURL:      cfg.Scan.ZAPApiURL,
			ZAPApiKey:      cfg.Scan.ZAPApiKey,
			DefaultTimeout: cfg.Scan.DefaultTimeout,
			WorkerPoolSize: cfg.Scan.WorkerPoolSize,
		},
		a.nc,
		a.scanLis,
	)
	a.Registry.Register(a.ScanRunner)

	// T09 — vulnerability-service (Bridge Pattern: MongoDB CVE HTTP handler)
	a.VulnRunner = runners.NewVulnRunner(
		runners.VulnRunnerConfig{
			MongoURI: cfg.Mongo.URI,
			MongoDB:  cfg.Mongo.Database,
		},
		a.vulnLis,
	)
	a.Registry.Register(a.VulnRunner)

	// T10 — product-service (Bridge Pattern: gRPC ProductService + HTTP REST + asset upsert on scan.completed)
	a.ProductRunner = runners.NewProductRunner(
		runners.ProductRunnerConfig{
			DBURL: cfg.Database.URL,
		},
		a.nc,
		a.productLis,
	)
	a.Registry.Register(a.ProductRunner)

	// T11 — notification-service (NATS subscriber + Webhook dispatcher)
	a.NotificationRunner = runners.NewNotificationRunner(
		runners.NotificationRunnerConfig{
			DBURL:           cfg.Database.URL,
			EmailEnabled:    cfg.Notification.Email.Enabled,
			SMTPHost:        cfg.Notification.Email.SMTPHost,
			SMTPPort:        cfg.Notification.Email.SMTPPort,
			SMTPUser:        cfg.Notification.Email.SMTPUser,
			SMTPPassword:    cfg.Notification.Email.SMTPPassword,
			FromEmail:       cfg.Notification.Email.From,
			SlackEnabled:    cfg.Notification.Slack.Enabled,
			SlackWebhookURL: cfg.Notification.Slack.WebhookURL,
			TeamsEnabled:    cfg.Notification.Teams.Enabled,
			TeamsWebhookURL: cfg.Notification.Teams.WebhookURL,
		},
		a.nc,
		a.notificationLis,
	)
	a.Registry.Register(a.NotificationRunner)

	// T11 (ingestion) — agent report enrichment via OSV API
	a.IngestionRunner = runners.NewIngestionRunner(
		runners.IngestionRunnerConfig{
			DBURL:          cfg.Database.URL,
			OSVAPIURL:      "https://api.osv.dev/v1",
			MaxConcurrency: 3,
		},
		a.nc,
		a.ingestionLis,
	)
	a.Registry.Register(a.IngestionRunner)

	// T12 — report-service (PDF/JSON/CSV/HTML generation)
	a.ReportRunner = runners.NewReportRunner(
		runners.ReportRunnerConfig{
			DBURL:       cfg.Database.URL,
			StorageType: cfg.Storage.Type,
			StoragePath: "/tmp/openvulnscan-reports",
		},
		a.reportLis,
	)
	a.Registry.Register(a.ReportRunner)

	// T07 — query-service (browse: /browse/, /vendors/search — Redis L1 + MongoDB L2)
	a.QueryRunner = runners.NewQueryRunner(
		runners.QueryRunnerConfig{
			MongoURI: cfg.Mongo.URI,
			MongoDB:  cfg.Mongo.Database,
			RedisURL: cfg.Redis.URL,
		},
		a.queryLis,
	)
	a.Registry.Register(a.QueryRunner)

	// T14 — SIEM syslog channel
	a.SyslogChannel = syslog.New(syslog.Config{
		Host:     cfg.SIEM.Host,
		Port:     cfg.SIEM.Port,
		Protocol: cfg.SIEM.Protocol,
		Enabled:  cfg.SIEM.Enabled,
		Facility: 16, // local0
	}, l)

	return a, nil
}

// Start khởi động tất cả service goroutines.
func (a *App) Start(ctx context.Context) error {
	// AuthHTTP handler cần DB/Redis — init sau khi infra connected
	a.authHTTP = newAuthHTTPHandler(a.db, a.redis, a.cfg)

	// Start tất cả service goroutines
	a.Registry.Start(ctx)

	// Đợi goroutines ready
	if err := a.waitForServices(ctx); err != nil {
		return fmt.Errorf("services not ready: %w", err)
	}

	// Tạo gRPC clients
	if err := a.connectClients(ctx); err != nil {
		return err
	}

	// Seed default admin user (nếu chưa có user nào)
	a.seedAdminUser(ctx)

	return nil
}


// Wait block cho đến khi tất cả goroutines dừng.
func (a *App) Wait() { a.Registry.Wait() }

// Shutdown đóng kết nối.
func (a *App) Shutdown() {
	if a.nc != nil {
		a.nc.Drain() //nolint:errcheck
	}
	if a.db != nil {
		a.db.Close()
	}
	if a.redis != nil {
		a.redis.Close() //nolint:errcheck
	}
}

// ── Auth HTTP Handlers (không qua gRPC — auth proto chỉ có ValidateToken) ────

// HandleLogin xử lý POST /api/v1/auth/login
func (a *App) HandleLogin(w http.ResponseWriter, r *http.Request) {
	a.authHTTP.login(w, r)
}

// HandleRegister xử lý POST /api/v1/auth/register
func (a *App) HandleRegister(w http.ResponseWriter, r *http.Request) {
	a.authHTTP.register(w, r)
}

// HandleLogout xử lý POST /api/v1/auth/logout
func (a *App) HandleLogout(w http.ResponseWriter, r *http.Request) {
	a.authHTTP.logout(w, r)
}

// HandleRefresh xử lý POST /api/v1/auth/refresh
func (a *App) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	a.authHTTP.refresh(w, r)
}

// HandleGoogleLogin redirect đến Google OAuth.
func (a *App) HandleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "https://accounts.google.com/o/oauth2/auth?response_type=code&client_id="+
		a.cfg.Auth.GoogleClientID, http.StatusTemporaryRedirect)
}

// HandleGoogleCallback xử lý OAuth callback.
func (a *App) HandleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "oauth not fully implemented"})
}

// HandleAgentDownload trả về Python agent script.
func (a *App) HandleAgentDownload(w http.ResponseWriter, r *http.Request) {
	baseURL := "http://localhost:8080"
	if r.Host != "" {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		baseURL = scheme + "://" + r.Host
	}
	script := generateAgentScript(baseURL + "/agent/report")
	w.Header().Set("Content-Type", "text/x-python")
	w.Header().Set("Content-Disposition", "attachment; filename=agent.py")
	w.Write([]byte(script)) //nolint:errcheck
}

// HandleAgentReport nhận package report từ agent.
func (a *App) HandleAgentReport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Hostname string `json:"hostname"`
		OSInfo   string `json:"os_info"`
		Packages []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"packages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}

	a.log.Info().Str("hostname", req.Hostname).Int("packages", len(req.Packages)).Msg("agent report received")

	// Publish NATS event để ingestion pipeline xử lý
	if a.nc != nil {
		payload, _ := json.Marshal(map[string]interface{}{
			"hostname":  req.Hostname,
			"os_info":   req.OSInfo,
			"packages":  req.Packages,
			"received":  time.Now().UTC(),
		})
		a.nc.Publish("agent.report.submitted", payload) //nolint:errcheck
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":   "report received",
		"hostname":  req.Hostname,
		"pkg_count": len(req.Packages),
	})
}

// HandleListFindings — GET /api/v1/findings
func (a *App) HandleListFindings(w http.ResponseWriter, r *http.Request) {
	if a.FindingClient == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "finding service not ready"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"findings": []interface{}{},
		"total":    0,
		"note":     "use gRPC for streaming",
	})
}

// HandleGetFinding — GET /api/v1/findings/{id}
func (a *App) HandleGetFinding(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"message": "not implemented"})
}

// HandleUpdateFindingStatus — PATCH /api/v1/findings/{id}/status
func (a *App) HandleUpdateFindingStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"message": "not implemented"})
}

// HandleAcceptRisk — POST /api/v1/findings/{id}/accept-risk
func (a *App) HandleAcceptRisk(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"message": "not implemented"})
}

// HandleFindingsSummary — GET /api/v1/findings/summary
func (a *App) HandleFindingsSummary(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"message": "not implemented"})
}

// HandleListAgentReports — GET /api/v1/agent/reports
func (a *App) HandleListAgentReports(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{"reports": []interface{}{}, "total": 0})
}

// HandleGetAgentReport — GET /api/v1/agent/reports/{id}
func (a *App) HandleGetAgentReport(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
}

// ── Private helpers ──────────────────────────────────────────────────────────

func (a *App) connectInfra() error {
	var err error

	// PostgreSQL
	config, err := pgxpool.ParseConfig(a.cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("parse db url: %w", err)
	}
	config.MaxConns = int32(a.cfg.Database.MaxConnections)
	if a.cfg.Database.MinConnections > 0 {
		config.MinConns = int32(a.cfg.Database.MinConnections)
	}
	a.db, err = pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return fmt.Errorf("db connect: %w", err)
	}
	if err := a.db.Ping(context.Background()); err != nil {
		return fmt.Errorf("db ping: %w", err)
	}
	a.log.Info().Msg("postgres connected")

	// NATS
	a.nc, err = nats.Connect(
		a.cfg.NATS.URL,
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			a.log.Warn().Err(err).Msg("NATS disconnected")
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			a.log.Info().Msg("NATS reconnected")
		}),
	)
	if err != nil {
		return fmt.Errorf("nats connect: %w", err)
	}
	a.log.Info().Msg("NATS connected")

	// Redis
	redisOpt, err := redis.ParseURL(a.cfg.Redis.URL)
	if err != nil {
		return fmt.Errorf("parse redis url: %w", err)
	}
	a.redis = redis.NewClient(redisOpt)
	if err := a.redis.Ping(context.Background()).Err(); err != nil {
		return fmt.Errorf("redis ping: %w", err)
	}
	a.log.Info().Msg("redis connected")

	return nil
}

func (a *App) waitForServices(ctx context.Context) error {
	deadline := time.Now().Add(30 * time.Second)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for services")
		}
		results := a.Registry.HealthAll(ctx)
		allOK := true
		for svc, err := range results {
			if err != nil {
				a.log.Debug().Str("svc", svc).Err(err).Msg("waiting...")
				allOK = false
			}
		}
		if allOK {
			a.log.Info().Msg("all services ready ✓")
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func (a *App) connectClients(ctx context.Context) error {
	// Auth gRPC client (ValidateToken/ValidateAPIKey)
	authConn, err := transport.DialBufConn(ctx, a.authLis)
	if err != nil {
		return fmt.Errorf("auth client: %w", err)
	}
	a.AuthClient = authv1.NewAuthServiceClient(authConn)

	// Finding gRPC client
	findingConn, err := transport.DialBufConn(ctx, a.findingLis)
	if err != nil {
		return fmt.Errorf("finding client: %w", err)
	}
	a.FindingClient = findingv1.NewFindingServiceClient(findingConn)

	// Product gRPC client
	productConn, err := transport.DialBufConn(ctx, a.productLis)
	if err != nil {
		return fmt.Errorf("product client: %w", err)
	}
	a.ProductClient = productv1.NewProductServiceClient(productConn)

	a.log.Info().Msg("all gRPC clients connected")
	return nil
}

// ── Utilities ────────────────────────────────────────────────────────────────

// seedAdminUser tạo admin user mặc định nếu bảng users rỗng.
// Credentials: admin@openvulnscan.local / Admin123!
// Được gọi một lần khi khởi động (idempotent).
func (a *App) seedAdminUser(ctx context.Context) {
	if a.db == nil {
		return
	}
	const checkSQL = `SELECT COUNT(*) FROM auth.users`
	var count int
	if err := a.db.QueryRow(ctx, checkSQL).Scan(&count); err != nil {
		a.log.Warn().Err(err).Msg("seedAdmin: cannot check users table")
		return
	}
	if count > 0 {
		return // Already have users — skip seed
	}

	// bcrypt hash for "Admin123!"
	const bcryptHash = "$2a$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxMQJqhN8/LewdBPj1TCQMiy8OW"
	const insertSQL = `
		INSERT INTO auth.users (id, email, password_hash, first_name, last_name, is_active, role, created_at, updated_at)
		VALUES (
			gen_random_uuid(),
			'admin@openvulnscan.local',
			$1,
			'Admin', 'User',
			true, 'admin',
			NOW(), NOW()
		)
		ON CONFLICT (email) DO NOTHING`

	if _, err := a.db.Exec(ctx, insertSQL, bcryptHash); err != nil {
		a.log.Warn().Err(err).Msg("seedAdmin: failed to create admin user")
		return
	}
	a.log.Info().
		Str("email", "admin@openvulnscan.local").
		Msg("✓ Default admin user seeded")
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func extractToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); len(h) > 7 && h[:7] == "Bearer " {
		return h[7:]
	}
	if c, err := r.Cookie("access_token"); err == nil {
		return c.Value
	}
	return ""
}

func generateAgentScript(reportURL string) string {
	return fmt.Sprintf(`#!/usr/bin/env python3
"""OpenVulnScan Agent v1.0 — collects installed packages and reports to server"""
import subprocess, json, sys, socket, platform
try:
    import requests
except ImportError:
    subprocess.check_call([sys.executable, "-m", "pip", "install", "requests", "-q"])
    import requests

REPORT_URL = %q

def get_packages():
    """Collect installed packages from the system package manager."""
    # Try dpkg (Debian/Ubuntu)
    try:
        out = subprocess.check_output(
            ["dpkg-query", "-W", "-f=${Package}\t${Version}\n"],
            stderr=subprocess.DEVNULL
        )
        return [
            {"name": l.split("\t")[0], "version": l.split("\t")[1].strip()}
            for l in out.decode().splitlines() if "\t" in l
        ]
    except Exception:
        pass
    # Try rpm (RHEL/CentOS/Fedora)
    try:
        out = subprocess.check_output(
            ["rpm", "-qa", "--queryformat", "%%{NAME}\t%%{VERSION}\n"],
            stderr=subprocess.DEVNULL
        )
        return [
            {"name": l.split("\t")[0], "version": l.split("\t")[1].strip()}
            for l in out.decode().splitlines() if "\t" in l
        ]
    except Exception:
        pass
    # Try pip packages
    try:
        out = subprocess.check_output([sys.executable, "-m", "pip", "list", "--format=json"],
                                       stderr=subprocess.DEVNULL)
        return [{"name": p["name"], "version": p["version"]} for p in json.loads(out)]
    except Exception:
        return []

if __name__ == "__main__":
    packages = get_packages()
    payload = {
        "hostname": socket.gethostname(),
        "os_info":  platform.platform(),
        "packages": packages,
    }
    try:
        r = requests.post(REPORT_URL, json=payload, timeout=30)
        r.raise_for_status()
        print(f"✓ Reported {len(packages)} packages → {r.status_code}: {r.json().get('message', 'ok')}")
    except Exception as e:
        print(f"✗ Report failed: {e}", file=sys.stderr)
        sys.exit(1)
`, reportURL)
}

// zerolog logger shim for unused import
var _ zerolog.Logger
