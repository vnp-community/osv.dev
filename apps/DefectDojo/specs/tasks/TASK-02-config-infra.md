# TASK-02: Config, Infrastructure & Migrations

**Phase**: 2 — Infrastructure Layer  
**Ước tính**: 4 giờ  
**Phụ thuộc**: TASK-01 hoàn thành  
**Output**: Config load, DB/NATS/Redis connections, migration runner, Prometheus metrics

---

## Mục tiêu

Xây dựng layer cơ sở hạ tầng: unified config, connection management, database migration runner, và metrics setup.

---

## T-02.1: Unified Config

**File**: `apps/DefectDojo/internal/config/config.go`

```go
package config

import (
    "fmt"
    "strings"
    "time"

    "github.com/spf13/viper"
)

// Config holds all configuration for the DefectDojo monolith.
type Config struct {
    // Core Infrastructure
    PostgresURL   string `mapstructure:"POSTGRES_URL"`
    NatsURL       string `mapstructure:"NATS_URL"`
    RedisURL      string `mapstructure:"REDIS_URL"`
    OpenSearchURL string `mapstructure:"OPENSEARCH_URL"`

    // Security
    JWTSecret string `mapstructure:"JWT_SECRET"`

    // Ports
    HTTPPort string `mapstructure:"HTTP_PORT"`
    GRPCPort string `mapstructure:"GRPC_PORT"`

    // Sub-configs
    Auth         AuthConfig
    AI           AIConfig
    Notification NotificationConfig
    Integration  IntegrationConfig
    Log          LogConfig

    // Runtime (computed)
    JWTExpiry     time.Duration
    RefreshExpiry time.Duration
}

type AuthConfig struct {
    JWTExpiryStr     string `mapstructure:"JWT_EXPIRY"`
    RefreshExpiryStr string `mapstructure:"REFRESH_EXPIRY"`
    OAuthGoogleKey    string `mapstructure:"OAUTH_GOOGLE_KEY"`
    OAuthGoogleSecret string `mapstructure:"OAUTH_GOOGLE_SECRET"`
    OAuthGitHubKey    string `mapstructure:"OAUTH_GITHUB_KEY"`
    OAuthGitHubSecret string `mapstructure:"OAUTH_GITHUB_SECRET"`
}

type AIConfig struct {
    Backend string `mapstructure:"AI_BACKEND"`  // ollama|openai|azure
    Model   string `mapstructure:"AI_MODEL"`
    BaseURL string `mapstructure:"AI_BASE_URL"`
    APIKey  string `mapstructure:"AI_API_KEY"`
}

type NotificationConfig struct {
    SMTPHost     string `mapstructure:"SMTP_HOST"`
    SMTPPort     int    `mapstructure:"SMTP_PORT"`
    SMTPUser     string `mapstructure:"SMTP_USER"`
    SMTPPassword string `mapstructure:"SMTP_PASSWORD"`
    SMTPFrom     string `mapstructure:"SMTP_FROM"`
    SlackToken   string `mapstructure:"SLACK_TOKEN"`
    TeamsWebhook string `mapstructure:"TEAMS_WEBHOOK_URL"`
}

type IntegrationConfig struct {
    JiraEncryptionKey string `mapstructure:"JIRA_ENCRYPTION_KEY"`
}

type LogConfig struct {
    Level  string `mapstructure:"LOG_LEVEL"`  // debug|info|warn|error
    Format string `mapstructure:"LOG_FORMAT"` // json|console
}

// Load reads config from environment variables.
func Load() (*Config, error) {
    v := viper.New()
    v.AutomaticEnv()

    // Defaults
    v.SetDefault("HTTP_PORT", "8080")
    v.SetDefault("GRPC_PORT", "9090")
    v.SetDefault("NATS_URL", "nats://localhost:4222")
    v.SetDefault("REDIS_URL", "redis://localhost:6379")
    v.SetDefault("OPENSEARCH_URL", "http://localhost:9200")
    v.SetDefault("JWT_EXPIRY", "24h")
    v.SetDefault("REFRESH_EXPIRY", "168h")
    v.SetDefault("AI_BACKEND", "ollama")
    v.SetDefault("AI_MODEL", "llama3")
    v.SetDefault("AI_BASE_URL", "http://localhost:11434")
    v.SetDefault("SMTP_PORT", 587)
    v.SetDefault("LOG_LEVEL", "info")
    v.SetDefault("LOG_FORMAT", "json")

    cfg := &Config{}
    if err := v.Unmarshal(cfg); err != nil {
        return nil, fmt.Errorf("unmarshal config: %w", err)
    }

    // Sub-configs
    cfg.Auth.JWTExpiryStr = v.GetString("JWT_EXPIRY")
    cfg.Auth.RefreshExpiryStr = v.GetString("REFRESH_EXPIRY")
    cfg.Auth.OAuthGoogleKey = v.GetString("OAUTH_GOOGLE_KEY")
    cfg.Auth.OAuthGoogleSecret = v.GetString("OAUTH_GOOGLE_SECRET")
    cfg.AI.Backend = v.GetString("AI_BACKEND")
    cfg.AI.Model = v.GetString("AI_MODEL")
    cfg.AI.BaseURL = v.GetString("AI_BASE_URL")
    cfg.AI.APIKey = v.GetString("AI_API_KEY")
    cfg.Notification.SMTPHost = v.GetString("SMTP_HOST")
    cfg.Notification.SMTPPort = v.GetInt("SMTP_PORT")
    cfg.Notification.SMTPUser = v.GetString("SMTP_USER")
    cfg.Notification.SMTPPassword = v.GetString("SMTP_PASSWORD")
    cfg.Notification.SlackToken = v.GetString("SLACK_TOKEN")
    cfg.Integration.JiraEncryptionKey = v.GetString("JIRA_ENCRYPTION_KEY")
    cfg.Log.Level = v.GetString("LOG_LEVEL")
    cfg.Log.Format = v.GetString("LOG_FORMAT")

    // Parse durations
    var err error
    cfg.JWTExpiry, err = time.ParseDuration(cfg.Auth.JWTExpiryStr)
    if err != nil {
        return nil, fmt.Errorf("parse JWT_EXPIRY: %w", err)
    }
    cfg.RefreshExpiry, err = time.ParseDuration(cfg.Auth.RefreshExpiryStr)
    if err != nil {
        return nil, fmt.Errorf("parse REFRESH_EXPIRY: %w", err)
    }

    // Validate required
    if cfg.PostgresURL == "" {
        return nil, fmt.Errorf("POSTGRES_URL is required")
    }
    if cfg.JWTSecret == "" {
        return nil, fmt.Errorf("JWT_SECRET is required")
    }
    if len(cfg.JWTSecret) < 32 {
        return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters")
    }

    return cfg, nil
}
```

**Checklist**:
- [ ] Config struct covers all env vars
- [ ] Validation cho required fields
- [ ] Duration parsing hoạt động
- [ ] `go build` pass

---

## T-02.2: Infrastructure Connections

**File**: `apps/DefectDojo/internal/app/infra.go`

```go
package app

import (
    "context"
    "fmt"
    "time"

    "github.com/defectdojo/apps/defectdojo/internal/config"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/nats-io/nats.go"
    "github.com/redis/go-redis/v9"
    "github.com/rs/zerolog/log"
)

// connectInfra initializes all shared infrastructure connections.
func (a *App) connectInfra(ctx context.Context) error {
    var err error

    // PostgreSQL
    log.Info().Str("url", maskURL(a.cfg.PostgresURL)).Msg("connecting to PostgreSQL")
    a.db, err = pgxpool.New(ctx, a.cfg.PostgresURL)
    if err != nil {
        return fmt.Errorf("postgres: %w", err)
    }
    if err := a.db.Ping(ctx); err != nil {
        return fmt.Errorf("postgres ping: %w", err)
    }
    log.Info().Msg("PostgreSQL connected")

    // NATS
    log.Info().Str("url", a.cfg.NatsURL).Msg("connecting to NATS")
    a.nats, err = nats.Connect(a.cfg.NatsURL,
        nats.MaxReconnects(-1),
        nats.ReconnectWait(2*time.Second),
        nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
            log.Warn().Err(err).Msg("NATS disconnected")
        }),
        nats.ReconnectHandler(func(nc *nats.Conn) {
            log.Info().Str("url", nc.ConnectedUrl()).Msg("NATS reconnected")
        }),
    )
    if err != nil {
        return fmt.Errorf("nats: %w", err)
    }
    log.Info().Msg("NATS connected")

    // Redis
    log.Info().Str("url", maskURL(a.cfg.RedisURL)).Msg("connecting to Redis")
    opt, err := redis.ParseURL(a.cfg.RedisURL)
    if err != nil {
        return fmt.Errorf("redis parse URL: %w", err)
    }
    a.redis = redis.NewClient(opt)
    if _, err := a.redis.Ping(ctx).Result(); err != nil {
        return fmt.Errorf("redis ping: %w", err)
    }
    log.Info().Msg("Redis connected")

    return nil
}

// maskURL masks password in connection URL for logging.
func maskURL(url string) string {
    // Simple masking: replace password in URL
    return url // TODO: implement proper masking
}
```

**Checklist**:
- [ ] PostgreSQL pool với ping check
- [ ] NATS với reconnect logic
- [ ] Redis client với ping check
- [ ] Proper error wrapping

---

## T-02.3: Database Migration Runner

**File**: `apps/DefectDojo/internal/migration/runner.go`

```go
// Package migration runs all service database migrations in dependency order.
package migration

import (
    "context"
    "fmt"
    "os"
    "path/filepath"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/rs/zerolog/log"
)

// MigrationSet defines one service's migration directory.
type MigrationSet struct {
    Schema  string // PostgreSQL schema name
    Service string // Human-readable service name
    Dir     string // Absolute path to migrations directory
}

// OrderedMigrations defines the migration order (dependency-based).
// Thứ tự phải khớp với dependency graph trong architecture.
func orderedMigrations(servicesRoot string) []MigrationSet {
    return []MigrationSet{
        // Tier 1: No dependencies
        {Schema: "auth",          Service: "auth-service",          Dir: filepath.Join(servicesRoot, "auth-service/migrations")},
        {Schema: "product",       Service: "product-service",       Dir: filepath.Join(servicesRoot, "product-service/migrations")},
        {Schema: "vulnerability", Service: "vulnerability-service", Dir: filepath.Join(servicesRoot, "vulnerability-service/migrations")},
        // Tier 2: Depends on Tier 1
        {Schema: "finding",       Service: "finding-service",       Dir: filepath.Join(servicesRoot, "finding-service/migrations")},
        {Schema: "scan",          Service: "scan-service",          Dir: filepath.Join(servicesRoot, "scan-service/migrations")},
        // Tier 3: Depends on Tier 1+2
        {Schema: "notification",  Service: "notification-service",  Dir: filepath.Join(servicesRoot, "notification-service/migrations")},
        {Schema: "report",        Service: "report-service",        Dir: filepath.Join(servicesRoot, "report-service/migrations")},
        {Schema: "integration",   Service: "integration-service",   Dir: filepath.Join(servicesRoot, "integration-service/migrations")},
    }
}

// RunAll executes all migrations in dependency order.
// servicesRoot is the absolute path to services/ directory.
func RunAll(ctx context.Context, pool *pgxpool.Pool, servicesRoot string) error {
    migrations := orderedMigrations(servicesRoot)

    for _, m := range migrations {
        log.Info().Str("service", m.Service).Str("schema", m.Schema).Msg("running migrations")

        // Check if migration dir exists
        if _, err := os.Stat(m.Dir); os.IsNotExist(err) {
            log.Warn().Str("dir", m.Dir).Msg("migration directory not found, skipping")
            continue
        }

        if err := runServiceMigration(ctx, pool, m); err != nil {
            return fmt.Errorf("migrate %s: %w", m.Service, err)
        }

        log.Info().Str("service", m.Service).Msg("migrations complete")
    }

    return nil
}

// runServiceMigration detects migration format and runs appropriately.
func runServiceMigration(ctx context.Context, pool *pgxpool.Pool, m MigrationSet) error {
    // TODO: Implement based on T-00.6 audit results.
    // Detect format: goose, golang-migrate, atlas, raw SQL
    // Example for raw SQL:
    //   return runRawSQL(ctx, pool, m.Dir, m.Schema)
    // Example for goose:
    //   return runGoose(ctx, pool, m.Dir, m.Schema)

    // Placeholder — implement after T-00.6 audit
    log.Info().Str("service", m.Service).Msg("migration placeholder — implement after audit")
    return nil
}

// ensureSchema creates PostgreSQL schema if it doesn't exist.
func ensureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
    _, err := pool.Exec(ctx, fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schema))
    return err
}
```

> **Lưu ý**: Implementation cụ thể của `runServiceMigration` phụ thuộc vào kết quả T-00.6 (migration tool audit).

**Checklist**:
- [ ] Runner framework hoàn chỉnh
- [ ] Thứ tự migration đúng dependency graph
- [ ] `ensureSchema` tạo schema trước khi migrate
- [ ] Error wrapping rõ ràng

---

## T-02.4: Logging Setup

**File**: `apps/DefectDojo/internal/config/logging.go`

```go
package config

import (
    "os"
    "strings"

    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"
)

// SetupLogging configures zerolog based on config.
func SetupLogging(cfg *LogConfig) {
    // Set level
    level, err := zerolog.ParseLevel(strings.ToLower(cfg.Level))
    if err != nil {
        level = zerolog.InfoLevel
    }
    zerolog.SetGlobalLevel(level)

    // Set format
    if cfg.Format == "console" {
        log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05"})
    } else {
        zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
    }

    // Add service name to all log entries
    log.Logger = log.With().Str("app", "defectdojo-go").Logger()
}
```

---

## T-02.5: Prometheus Metrics

**File**: `apps/DefectDojo/internal/metrics/metrics.go`

```go
// Package metrics defines Prometheus metrics for the DefectDojo monolith.
package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
    // ── Service health ───────────────────────────────────────────────────────
    ServiceHealth = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "dd_service_health",
            Help: "Service health status: 1=healthy, 0=unhealthy",
        },
        []string{"service"},
    )

    // ── Finding metrics ───────────────────────────────────────────────────────
    FindingsCreated = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "dd_findings_created_total", Help: "Total findings created"},
        []string{"severity", "product_id"},
    )
    FindingsSLABreached = prometheus.NewCounter(
        prometheus.CounterOpts{Name: "dd_sla_breaches_total", Help: "Total SLA breaches"},
    )

    // ── Scan metrics ──────────────────────────────────────────────────────────
    ScansImported = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "dd_scans_imported_total", Help: "Total scans imported"},
        []string{"scan_type", "status"},
    )
    ScanProcessingDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "dd_scan_processing_seconds",
            Help:    "Scan processing duration in seconds",
            Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600},
        },
        []string{"scan_type"},
    )

    // ── HTTP metrics ──────────────────────────────────────────────────────────
    HTTPRequests = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "dd_http_requests_total", Help: "Total HTTP requests"},
        []string{"method", "path", "status_code"},
    )
    HTTPDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "dd_http_request_duration_seconds",
            Help:    "HTTP request duration in seconds",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method", "path"},
    )

    // ── NATS metrics ──────────────────────────────────────────────────────────
    NATSMessages = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "dd_nats_messages_total", Help: "Total NATS messages"},
        []string{"subject", "status"}, // status: published|consumed|failed
    )
)

func init() {
    prometheus.MustRegister(
        ServiceHealth,
        FindingsCreated,
        FindingsSLABreached,
        ScansImported,
        ScanProcessingDuration,
        HTTPRequests,
        HTTPDuration,
        NATSMessages,
    )
}
```

**Checklist**:
- [ ] Tất cả metrics định nghĩa
- [ ] prometheus.MustRegister trong init()
- [ ] Labels consistent với service names

---

## T-02.6: Health Check Contracts

**File**: `apps/DefectDojo/internal/health/health.go`

```go
// Package health defines health check interfaces and HTTP handlers.
package health

import (
    "context"
    "encoding/json"
    "net/http"
    "sync"
    "time"
)

// Checker is implemented by every service runner.
type Checker interface {
    Name() string
    Health(ctx context.Context) error
}

// Registry holds all health checkers.
type Registry struct {
    mu       sync.RWMutex
    checkers map[string]Checker
}

func NewRegistry() *Registry {
    return &Registry{checkers: make(map[string]Checker)}
}

func (r *Registry) Register(c Checker) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.checkers[c.Name()] = c
}

// HealthResponse is the JSON response for /health endpoint.
type HealthResponse struct {
    Status   string            `json:"status"` // "ok" | "degraded" | "unhealthy"
    Services map[string]string `json:"services"`
    Uptime   string            `json:"uptime"`
}

// Handler returns an HTTP handler for /health endpoint.
func (r *Registry) Handler() http.HandlerFunc {
    start := time.Now()
    return func(w http.ResponseWriter, req *http.Request) {
        ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
        defer cancel()

        r.mu.RLock()
        checkers := make(map[string]Checker, len(r.checkers))
        for k, v := range r.checkers {
            checkers[k] = v
        }
        r.mu.RUnlock()

        statuses := make(map[string]string, len(checkers))
        overall := "ok"
        for name, checker := range checkers {
            if err := checker.Health(ctx); err != nil {
                statuses[name] = "unhealthy: " + err.Error()
                overall = "degraded"
            } else {
                statuses[name] = "ok"
            }
        }

        resp := HealthResponse{
            Status:   overall,
            Services: statuses,
            Uptime:   time.Since(start).Round(time.Second).String(),
        }

        status := http.StatusOK
        if overall != "ok" {
            status = http.StatusServiceUnavailable
        }

        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(status)
        json.NewEncoder(w).Encode(resp)
    }
}
```

---

## Definition of Done

- [ ] Config load từ env vars hoạt động
- [ ] Validation reject thiếu POSTGRES_URL, JWT_SECRET
- [ ] PostgreSQL pool connect được
- [ ] NATS connect được
- [ ] Redis connect được
- [ ] Migration runner framework hoàn chỉnh (kể cả placeholder)
- [ ] Prometheus metrics đăng ký thành công
- [ ] Health registry hoạt động
- [ ] `go build ./...` pass
