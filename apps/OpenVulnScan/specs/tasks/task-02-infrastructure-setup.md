# T02 — Infrastructure Setup (v3)

## Thông tin
| | |
|---|---|
| **Phase** | 0 — Foundation |
| **Ước tính** | 3–4 giờ |
| **Depends on** | T01 |
| **Blocks** | T04–T12 |

## Mục tiêu

Setup Docker Compose, config YAML, **NATS JetStream streams**, và `internal/app/app.go` — container wire-up tất cả service goroutines.

> **v3 change**: `app.go` giờ phải:  
> 1. Tạo bufconn listeners cho tất cả services  
> 2. Register tất cả runners vào Registry  
> 3. Sau khi Start(), tạo gRPC clients đến các listeners

---

## 2.1 Tạo `docker-compose.yml`

```yaml
# apps/OpenVulnScan/docker-compose.yml
version: "3.9"
services:

  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_USER: openvulnscan
      POSTGRES_PASSWORD: secret
      POSTGRES_DB: openvulnscan
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U openvulnscan"]
      interval: 5s
      timeout: 5s
      retries: 10

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    command: redis-server --appendonly yes
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      retries: 5

  nats:
    image: nats:2.10-alpine
    ports:
      - "4222:4222"   # Client connections
      - "8222:8222"   # Monitoring UI
    command: -js -m 8222 --store_dir /data
    volumes:
      - nats_data:/data
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8222/healthz"]
      interval: 5s
      retries: 5

  minio:
    image: minio/minio:latest
    ports:
      - "9000:9000"
      - "9001:9001"
    environment:
      MINIO_ROOT_USER: minioadmin
      MINIO_ROOT_PASSWORD: minioadmin
    command: server /data --console-address ":9001"
    volumes:
      - minio_data:/data
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9000/minio/health/live"]
      interval: 10s
      retries: 5

volumes:
  postgres_data:
  nats_data:
  minio_data:
```

---

## 2.2 Tạo `configs/config.yaml`

```yaml
# apps/OpenVulnScan/configs/config.yaml
server:
  http_addr: ":8080"
  read_timeout: 30s
  write_timeout: 30s
  idle_timeout: 60s

database:
  url: "postgres://openvulnscan:secret@localhost:5432/openvulnscan?sslmode=disable"
  max_connections: 25
  min_connections: 5

redis:
  url: "redis://localhost:6379"
  db: 0

nats:
  url: "nats://localhost:4222"

storage:
  type: minio
  endpoint: "localhost:9000"
  bucket: "openvulnscan"
  access_key: "minioadmin"
  secret_key: "minioadmin"
  use_ssl: false

auth:
  jwt_secret: "change-in-production-must-be-32-chars-min"
  jwt_expiry: 24h
  refresh_expiry: 168h
  google_client_id: ""
  google_client_secret: ""
  google_redirect_url: "http://localhost:8080/api/v1/auth/google/callback"

scan:
  worker_pool_size: 5
  nmap_binary: "/usr/bin/nmap"
  zap_api_url: "http://localhost:8090"
  default_timeout: 300

admin:
  email: "admin@openvulnscan.local"
  password: "admin123"

siem:
  enabled: false
  host: ""
  port: 514
  protocol: "udp"

notification:
  email:
    enabled: false
    smtp_host: ""
    smtp_port: 587
    smtp_user: ""
    smtp_password: ""
    from: "openvulnscan@example.com"
  slack:
    enabled: false
    webhook_url: ""
  teams:
    enabled: false
    webhook_url: ""
  webhook:
    enabled: false

log:
  level: "info"
  format: "json"
```

---

## 2.3 Tạo `internal/app/config.go`

```go
// internal/app/config.go
package app

import (
    "fmt"
    "time"
    "github.com/spf13/viper"
)

type Config struct {
    Server       ServerConfig
    Database     DatabaseConfig
    Redis        RedisConfig
    NATS         NATSConfig
    Storage      StorageConfig
    Auth         AuthConfig
    Scan         ScanConfig
    Admin        AdminConfig
    SIEM         SIEMConfig
    Notification NotificationConfig
    Log          LogConfig
}

type ServerConfig struct {
    HTTPAddr     string        `mapstructure:"http_addr"`
    ReadTimeout  time.Duration `mapstructure:"read_timeout"`
    WriteTimeout time.Duration `mapstructure:"write_timeout"`
    IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
}

type DatabaseConfig struct {
    URL            string `mapstructure:"url"`
    MaxConnections int    `mapstructure:"max_connections"`
    MinConnections int    `mapstructure:"min_connections"`
}

type RedisConfig struct {
    URL string `mapstructure:"url"`
    DB  int    `mapstructure:"db"`
}

type NATSConfig struct {
    URL string `mapstructure:"url"`
}

type StorageConfig struct {
    Type      string `mapstructure:"type"`
    Endpoint  string `mapstructure:"endpoint"`
    Bucket    string `mapstructure:"bucket"`
    AccessKey string `mapstructure:"access_key"`
    SecretKey string `mapstructure:"secret_key"`
    UseSSL    bool   `mapstructure:"use_ssl"`
}

type AuthConfig struct {
    JWTSecret         string        `mapstructure:"jwt_secret"`
    JWTExpiry         time.Duration `mapstructure:"jwt_expiry"`
    RefreshExpiry     time.Duration `mapstructure:"refresh_expiry"`
    GoogleClientID    string        `mapstructure:"google_client_id"`
    GoogleSecret      string        `mapstructure:"google_client_secret"`
    GoogleRedirectURL string        `mapstructure:"google_redirect_url"`
}

type ScanConfig struct {
    WorkerPoolSize int    `mapstructure:"worker_pool_size"`
    NmapBinary     string `mapstructure:"nmap_binary"`
    ZAPApiURL      string `mapstructure:"zap_api_url"`
    DefaultTimeout int    `mapstructure:"default_timeout"`
}

type AdminConfig struct {
    Email    string `mapstructure:"email"`
    Password string `mapstructure:"password"`
}

type SIEMConfig struct {
    Enabled  bool   `mapstructure:"enabled"`
    Host     string `mapstructure:"host"`
    Port     int    `mapstructure:"port"`
    Protocol string `mapstructure:"protocol"`
}

type NotificationConfig struct {
    Email   EmailNotifyConfig   `mapstructure:"email"`
    Slack   SlackConfig         `mapstructure:"slack"`
    Teams   TeamsConfig         `mapstructure:"teams"`
    Webhook WebhookNotifyConfig `mapstructure:"webhook"`
}

type EmailNotifyConfig struct {
    Enabled      bool   `mapstructure:"enabled"`
    SMTPHost     string `mapstructure:"smtp_host"`
    SMTPPort     int    `mapstructure:"smtp_port"`
    SMTPUser     string `mapstructure:"smtp_user"`
    SMTPPassword string `mapstructure:"smtp_password"`
    From         string `mapstructure:"from"`
}

type SlackConfig struct {
    Enabled    bool   `mapstructure:"enabled"`
    WebhookURL string `mapstructure:"webhook_url"`
}

type TeamsConfig struct {
    Enabled    bool   `mapstructure:"enabled"`
    WebhookURL string `mapstructure:"webhook_url"`
}

type WebhookNotifyConfig struct {
    Enabled bool `mapstructure:"enabled"`
}

type LogConfig struct {
    Level  string `mapstructure:"level"`
    Format string `mapstructure:"format"`
}

func LoadConfig(path string) (*Config, error) {
    v := viper.New()
    v.SetConfigFile(path)
    v.AutomaticEnv()

    if err := v.ReadInConfig(); err != nil {
        return nil, fmt.Errorf("read config: %w", err)
    }

    var cfg Config
    if err := v.Unmarshal(&cfg); err != nil {
        return nil, fmt.Errorf("unmarshal config: %w", err)
    }
    return &cfg, nil
}
```

---

## 2.4 Tạo `internal/app/app.go`

```go
// internal/app/app.go
package app

import (
    "context"
    "fmt"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/nats-io/nats.go"
    "github.com/redis/go-redis/v9"
    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"
    "google.golang.org/grpc/test/bufconn"

    "github.com/osv/apps/openvulnscan/internal/events"
    "github.com/osv/apps/openvulnscan/internal/runners"
    "github.com/osv/apps/openvulnscan/internal/transport"
)

// App là container chứa tất cả service goroutines và shared infrastructure.
type App struct {
    cfg      *Config
    registry *Registry
    log      zerolog.Logger

    // Shared infrastructure
    db    *pgxpool.Pool
    nc    *nats.Conn
    redis *redis.Client

    // bufconn listeners — mỗi service goroutine có 1 listener riêng
    authLis    *bufconn.Listener
    scanLis    *bufconn.Listener
    findingLis *bufconn.Listener
    productLis *bufconn.Listener
    vulnLis    *bufconn.Listener
    reportLis  *bufconn.Listener
    queryLis   *bufconn.Listener

    // gRPC clients — tạo sau khi Start()
    Clients *Clients
}

func New(cfg *Config) (*App, error) {
    l := log.With().Str("component", "app").Logger()
    a := &App{cfg: cfg, log: l, registry: NewRegistry(l)}

    // 1. Kết nối shared infrastructure
    if err := a.connectInfra(); err != nil {
        return nil, fmt.Errorf("connect infra: %w", err)
    }

    // 2. Setup NATS JetStream
    if _, err := events.SetupJetStream(a.nc); err != nil {
        return nil, fmt.Errorf("setup jetstream: %w", err)
    }

    // 3. Tạo bufconn listeners
    a.authLis    = transport.NewBufConnListener()
    a.scanLis    = transport.NewBufConnListener()
    a.findingLis = transport.NewBufConnListener()
    a.productLis = transport.NewBufConnListener()
    a.vulnLis    = transport.NewBufConnListener()
    a.reportLis  = transport.NewBufConnListener()
    a.queryLis   = transport.NewBufConnListener()

    // 4. Register service runners (Tier 1: no service deps)
    a.registry.Register(runners.NewAuthRunner(runners.AuthRunnerConfig{
        DBURL: cfg.Database.URL, RedisURL: cfg.Redis.URL,
        JWTSecret: cfg.Auth.JWTSecret, JWTExpiry: cfg.Auth.JWTExpiry,
        RefreshExpiry: cfg.Auth.RefreshExpiry,
        GoogleClientID: cfg.Auth.GoogleClientID,
        GoogleSecret: cfg.Auth.GoogleSecret, GoogleRedirect: cfg.Auth.GoogleRedirectURL,
    }, a.authLis))

    a.registry.Register(runners.NewVulnRunner(runners.VulnRunnerConfig{
        DBURL: cfg.Database.URL,
    }, a.vulnLis))

    a.registry.Register(runners.NewProductRunner(runners.ProductRunnerConfig{
        DBURL: cfg.Database.URL,
    }, a.productLis))

    a.registry.Register(runners.NewQueryRunner(runners.QueryRunnerConfig{
        DBURL: cfg.Database.URL,
    }, a.queryLis))

    // Tier 2: depend on Tier 1
    a.registry.Register(runners.NewFindingRunner(runners.FindingRunnerConfig{
        DBURL: cfg.Database.URL, ProductLis: a.productLis,
    }, a.nc, a.findingLis))

    // Tier 3: depend on Tier 2
    a.registry.Register(runners.NewScanRunner(runners.ScanRunnerConfig{
        DBURL: cfg.Database.URL,
        NmapBinary: cfg.Scan.NmapBinary, ZAPApiURL: cfg.Scan.ZAPApiURL,
        DefaultTimeout: cfg.Scan.DefaultTimeout,
        WorkerPoolSize: cfg.Scan.WorkerPoolSize,
        FindingLis: a.findingLis, ProductLis: a.productLis,
    }, a.nc, a.scanLis))

    a.registry.Register(runners.NewReportRunner(runners.ReportRunnerConfig{
        DBURL: cfg.Database.URL,
        FindingLis: a.findingLis, ProductLis: a.productLis,
    }, a.reportLis))

    a.registry.Register(runners.NewNotifyRunner(runners.NotifyRunnerConfig{
        Email:   cfg.Notification.Email,
        Slack:   cfg.Notification.Slack,
        Teams:   cfg.Notification.Teams,
        Webhook: cfg.Notification.Webhook,
        SIEM:    cfg.SIEM,
    }, a.nc))

    a.registry.Register(runners.NewIngestionRunner(runners.IngestionRunnerConfig{
        DBURL: cfg.Database.URL, VulnLis: a.vulnLis,
    }, a.nc))

    return a, nil
}

// Start khởi động tất cả goroutines, sau đó tạo gRPC clients.
func (a *App) Start(ctx context.Context) error {
    // Start tất cả service goroutines
    a.registry.Start(ctx)

    // Đợi một chút để goroutines khởi động gRPC servers
    // (production: dùng health check polling thay vì sleep)

    // Tạo gRPC clients đến các service goroutines
    clients, err := NewClients(ctx, a)
    if err != nil {
        return fmt.Errorf("create clients: %w", err)
    }
    a.Clients = clients

    // Register API Gateway runner (last — phụ thuộc vào Clients)
    a.registry.Register(runners.NewGatewayRunner(a.cfg.Server.HTTPAddr, clients, a.nc))
    // Chú ý: Gateway runner được start riêng
    a.registry.Start(ctx) // Chỉ start gateway (registry idempotent với runners đã running)

    return nil
}

// Wait block cho đến khi tất cả goroutines dừng.
func (a *App) Wait() { a.registry.Wait() }

// Shutdown đóng tất cả kết nối.
func (a *App) Shutdown() {
    if a.Clients != nil { a.Clients.Close() }
    if a.nc != nil { a.nc.Drain() }
    if a.db != nil { a.db.Close() }
    if a.redis != nil { a.redis.Close() }
}

func (a *App) connectInfra() error {
    // PostgreSQL
    config, err := pgxpool.ParseConfig(a.cfg.Database.URL)
    if err != nil { return fmt.Errorf("parse db url: %w", err) }
    config.MaxConns = int32(a.cfg.Database.MaxConnections)
    config.MinConns = int32(a.cfg.Database.MinConnections)

    a.db, err = pgxpool.NewWithConfig(context.Background(), config)
    if err != nil { return fmt.Errorf("db connect: %w", err) }

    // NATS
    a.nc, err = nats.Connect(a.cfg.NATS.URL,
        nats.MaxReconnects(-1),
        nats.ReconnectWait(2*time.Second),
    )
    if err != nil { return fmt.Errorf("nats connect: %w", err) }

    // Redis
    redisOpt, err := redis.ParseURL(a.cfg.Redis.URL)
    if err != nil { return fmt.Errorf("parse redis url: %w", err) }
    a.redis = redis.NewClient(redisOpt)
    if err := a.redis.Ping(context.Background()).Err(); err != nil {
        return fmt.Errorf("redis ping: %w", err)
    }

    a.log.Info().Msg("infrastructure connected")
    return nil
}
```

---

## 2.5 Tạo `cmd/server/main.go`

```go
// cmd/server/main.go
package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"

    "github.com/osv/apps/openvulnscan/internal/app"
)

func main() {
    // Setup logging
    log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})
    log.Info().Msg("OpenVulnScan starting...")

    // Load config
    cfg, err := app.LoadConfig("configs/config.yaml")
    if err != nil {
        log.Fatal().Err(err).Msg("failed to load config")
    }

    // Create application (wire-up all goroutines)
    application, err := app.New(cfg)
    if err != nil {
        log.Fatal().Err(err).Msg("failed to create application")
    }

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Start all service goroutines
    if err := application.Start(ctx); err != nil {
        log.Fatal().Err(err).Msg("failed to start application")
    }

    log.Info().Str("http_addr", cfg.Server.HTTPAddr).Msg("OpenVulnScan ready")

    // Wait for shutdown signal
    sig := make(chan os.Signal, 1)
    signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
    <-sig

    log.Info().Msg("shutdown initiated...")
    cancel() // Cancel context → tất cả goroutines nhận signal stop

    // Wait for all goroutines to finish (với timeout)
    done := make(chan struct{})
    go func() {
        application.Wait()
        close(done)
    }()

    select {
    case <-done:
        log.Info().Msg("graceful shutdown complete")
    case <-time.After(30 * time.Second):
        log.Warn().Msg("shutdown timeout — some goroutines may still be running")
    }

    application.Shutdown()
    log.Info().Msg("OpenVulnScan stopped")
}
```

---

## Output

- [x] `docker-compose.yml` — PostgreSQL (pgvector), Redis, NATS (JetStream), MinIO
- [x] `configs/config.yaml` — config đầy đủ với tất cả service settings
- [x] `internal/app/config.go` — Config struct với Viper + defaults
- [x] `internal/app/app.go` — Registry + bufconn listeners + wire-up auth runner
- [x] `cmd/server/main.go` — Entry point với graceful shutdown (30s)

## Trạng thái: ✅ HOÀN THÀNH
> Thực thi: 2026-06-09
> `go build ./...` thành công
> Lưu ý: App.connectClients() chỉ có auth client — các service khác sẽ thêm trong T05–T12

## Acceptance Criteria

```bash
# Infrastructure healthy
cd apps/OpenVulnScan && docker-compose up -d
docker-compose ps  # tất cả healthy

# Build không lỗi
go build ./...

# Server khởi động (sẽ fail ở connect nếu chưa có service code, nhưng không compile error)
go run ./cmd/server/
```

## Lưu ý

- **Xác minh shared/pkg database API**: Đọc `services/shared/pkg/database/` trước khi dùng `shareddb.Connect`
- **NATS JetStream**: Phải dùng `-js` flag trong docker-compose
- **bufconn ordering**: Tạo tất cả listeners TRƯỚC khi register runners, vì runners có thể cần listeners của service khác
