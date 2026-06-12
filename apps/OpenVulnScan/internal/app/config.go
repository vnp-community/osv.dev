// Package app — config.go
// Config struct cho toàn bộ OpenVulnScan monolith.
package app

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config là config root của OpenVulnScan.
type Config struct {
	Server       ServerConfig
	Database     DatabaseConfig
	Redis        RedisConfig
	NATS         NATSConfig
	Storage      StorageConfig
	Mongo        MongoConfig
	Auth         AuthConfig
	Scan         ScanConfig
	Admin        AdminConfig
	SIEM         SIEMConfig
	Notification NotificationConfig
	Log          LogConfig
}

// ServerConfig chứa config cho HTTP API gateway.
type ServerConfig struct {
	HTTPAddr     string        `mapstructure:"http_addr"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
}

// DatabaseConfig chứa config PostgreSQL.
type DatabaseConfig struct {
	URL            string `mapstructure:"url"`
	MaxConnections int    `mapstructure:"max_connections"`
	MinConnections int    `mapstructure:"min_connections"`
}

// RedisConfig chứa config Redis.
type RedisConfig struct {
	URL string `mapstructure:"url"`
	DB  int    `mapstructure:"db"`
}

// NATSConfig chứa config NATS JetStream.
type NATSConfig struct {
	URL string `mapstructure:"url"`
}

// MongoConfig chứa config MongoDB (dùng bởi vulnerability-service).
type MongoConfig struct {
	URI      string `mapstructure:"uri"`
	Database string `mapstructure:"database"`
}

// StorageConfig chứa config object storage (MinIO/S3).
type StorageConfig struct {
	Type      string `mapstructure:"type"`
	Endpoint  string `mapstructure:"endpoint"`
	Bucket    string `mapstructure:"bucket"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
	UseSSL    bool   `mapstructure:"use_ssl"`
}

// AuthConfig chứa config cho auth-service goroutine.
type AuthConfig struct {
	// JWT RSA
	JWTPrivateKeyPath string        `mapstructure:"jwt_private_key_path"`
	JWTIssuer         string        `mapstructure:"jwt_issuer"`
	JWTAudience       []string      `mapstructure:"jwt_audience"`
	JWTAccessTTL      time.Duration `mapstructure:"jwt_access_ttl"`
	JWTRefreshTTL     time.Duration `mapstructure:"jwt_refresh_ttl"`

	// OAuth
	GoogleClientID    string `mapstructure:"google_client_id"`
	GoogleSecret      string `mapstructure:"google_client_secret"`
	GoogleRedirectURL string `mapstructure:"google_redirect_url"`
	GitHubClientID    string `mapstructure:"github_client_id"`
	GitHubSecret      string `mapstructure:"github_client_secret"`
	GitHubRedirectURL string `mapstructure:"github_redirect_url"`
}

// ScanConfig chứa config cho scan-service goroutine.
type ScanConfig struct {
	WorkerPoolSize int    `mapstructure:"worker_pool_size"`
	NmapBinary     string `mapstructure:"nmap_binary"`
	ZAPApiURL      string `mapstructure:"zap_api_url"`
	ZAPApiKey      string `mapstructure:"zap_api_key"`
	DefaultTimeout int    `mapstructure:"default_timeout"`
}

// AdminConfig chứa thông tin admin user mặc định.
type AdminConfig struct {
	Email    string `mapstructure:"email"`
	Password string `mapstructure:"password"`
}

// SIEMConfig chứa config cho SIEM syslog forwarding.
type SIEMConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Protocol string `mapstructure:"protocol"`
}

// NotificationConfig chứa config cho notification-service goroutine.
type NotificationConfig struct {
	Email   EmailNotifyConfig   `mapstructure:"email"`
	Slack   SlackConfig         `mapstructure:"slack"`
	Teams   TeamsConfig         `mapstructure:"teams"`
	Webhook WebhookNotifyConfig `mapstructure:"webhook"`
}

// EmailNotifyConfig chứa config SMTP.
type EmailNotifyConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	SMTPHost     string `mapstructure:"smtp_host"`
	SMTPPort     int    `mapstructure:"smtp_port"`
	SMTPUser     string `mapstructure:"smtp_user"`
	SMTPPassword string `mapstructure:"smtp_password"`
	From         string `mapstructure:"from"`
}

// SlackConfig chứa config Slack webhook.
type SlackConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	WebhookURL string `mapstructure:"webhook_url"`
}

// TeamsConfig chứa config Microsoft Teams webhook.
type TeamsConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	WebhookURL string `mapstructure:"webhook_url"`
}

// WebhookNotifyConfig chứa config generic webhook.
type WebhookNotifyConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// LogConfig chứa config logging.
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// LoadConfig đọc config từ file YAML với environment variable override.
func LoadConfig(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.AutomaticEnv()
	v.SetEnvPrefix("OVS")

	// Defaults
	v.SetDefault("server.http_addr", ":8080")
	v.SetDefault("server.read_timeout", "30s")
	v.SetDefault("server.write_timeout", "30s")
	v.SetDefault("server.idle_timeout", "60s")
	v.SetDefault("database.max_connections", 25)
	v.SetDefault("database.min_connections", 5)
	v.SetDefault("scan.worker_pool_size", 5)
	v.SetDefault("scan.nmap_binary", "/usr/bin/nmap")
	v.SetDefault("scan.default_timeout", 300)
	v.SetDefault("auth.jwt_issuer", "https://auth.openvulnscan.io")
	v.SetDefault("auth.jwt_audience", []string{"openvulnscan"})
	v.SetDefault("auth.jwt_access_ttl", "15m")
	v.SetDefault("auth.jwt_refresh_ttl", "168h")
	v.SetDefault("auth.jwt_private_key_path", "secrets/jwt_private.pem")
	v.SetDefault("mongo.uri", "mongodb://localhost:27017")
	v.SetDefault("mongo.database", "cvedb")
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &cfg, nil
}
