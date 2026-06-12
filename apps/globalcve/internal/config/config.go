// Package config loads application configuration from file and environment.
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config is the root configuration for the GlobalCVE monolithic app.
type Config struct {
	Server       ServerConfig       `mapstructure:"server"`
	Database     DatabaseConfig     `mapstructure:"database"`
	Redis        RedisConfig        `mapstructure:"redis"`
	OpenSearch   OpenSearchConfig   `mapstructure:"opensearch"`
	NATS         NATSConfig         `mapstructure:"nats"`
	RateLimit    RateLimitConfig    `mapstructure:"rate_limit"`
	CORS         CORSConfig         `mapstructure:"cors"`
	Auth         AuthConfig         `mapstructure:"auth"`
	Cache        CacheConfig        `mapstructure:"cache"`
	Sources      SourcesConfig      `mapstructure:"sources"`
	CISA         CISAConfig         `mapstructure:"cisa"`
	Scheduler    SchedulerConfig    `mapstructure:"scheduler"`
	Observability ObservabilityConfig `mapstructure:"observability"`
	Migrations   MigrationsConfig   `mapstructure:"migrations"`
}

type ServerConfig struct {
	GatewayPort      int           `mapstructure:"gateway_port"`
	CVESearchPort    int           `mapstructure:"cvesearch_port"`
	CVESyncPort      int           `mapstructure:"cvesync_port"`
	KEVServicePort   int           `mapstructure:"kevservice_port"`
	NotificationPort int           `mapstructure:"notification_port"`
	ReadTimeout      time.Duration `mapstructure:"read_timeout"`
	WriteTimeout     time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout  time.Duration `mapstructure:"shutdown_timeout"`
}

type DatabaseConfig struct {
	URL          string        `mapstructure:"url"`
	MaxOpenConns int           `mapstructure:"max_open_conns"`
	MaxIdleConns int           `mapstructure:"max_idle_conns"`
	ConnTimeout  time.Duration `mapstructure:"conn_timeout"`
}

type RedisConfig struct {
	URL         string        `mapstructure:"url"`
	PoolSize    int           `mapstructure:"pool_size"`
	DialTimeout time.Duration `mapstructure:"dial_timeout"`
}

type OpenSearchConfig struct {
	URL     string        `mapstructure:"url"`
	Index   string        `mapstructure:"index"`
	Timeout time.Duration `mapstructure:"timeout"`
}

type NATSConfig struct {
	URL         string `mapstructure:"url"`
	StreamCVE   string `mapstructure:"stream_cve"`
	StreamKEV   string `mapstructure:"stream_kev"`
	StreamAlert string `mapstructure:"stream_alert"`
}

type RateLimitConfig struct {
	MaxRequests int           `mapstructure:"max_requests"`
	Window      time.Duration `mapstructure:"window"`
}

type CORSConfig struct {
	AllowedOrigins []string `mapstructure:"allowed_origins"`
}

type AuthConfig struct {
	JWTSecret      string `mapstructure:"jwt_secret"`
	APIKeysEnabled bool   `mapstructure:"api_keys_enabled"`
}

type CacheConfig struct {
	SearchTTL int `mapstructure:"search_ttl"`
	SingleTTL int `mapstructure:"single_ttl"`
	KEVTTL    int `mapstructure:"kev_ttl"`
	CheckTTL  int `mapstructure:"check_ttl"`
}

type SourceConfig struct {
	APIKEY      string        `mapstructure:"api_key"`
	BaseURL     string        `mapstructure:"base_url"`
	URL         string        `mapstructure:"url"`
	FeedURL     string        `mapstructure:"feed_url"`
	CSVURL      string        `mapstructure:"csv_url"`
	ReleaseURL  string        `mapstructure:"release_url"`
	RateLimitMs int           `mapstructure:"rate_limit_ms"`
	PageSize    int           `mapstructure:"page_size"`
	Timeout     time.Duration `mapstructure:"timeout"`
}

type SourcesConfig struct {
	NVD      SourceConfig `mapstructure:"nvd"`
	CIRCL    SourceConfig `mapstructure:"circl"`
	JVN      SourceConfig `mapstructure:"jvn"`
	ExploitDB SourceConfig `mapstructure:"exploitdb"`
	CVEOrg   SourceConfig `mapstructure:"cveorg"`
	EPSS     SourceConfig `mapstructure:"epss"`
	CAPEC    SourceConfig `mapstructure:"capec"`
	CWE      SourceConfig `mapstructure:"cwe"`
}

type CISAConfig struct {
	KEVURL       string        `mapstructure:"kev_url"`
	SyncInterval time.Duration `mapstructure:"sync_interval"`
	Timeout      time.Duration `mapstructure:"timeout"`
}

type SchedulerConfig struct {
	Enabled        bool   `mapstructure:"enabled"`
	NVDCVECron     string `mapstructure:"nvd_cve_cron"`
	JVNCron        string `mapstructure:"jvn_cron"`
	CIRCLCron      string `mapstructure:"circl_cron"`
	ExploitDBCron  string `mapstructure:"exploitdb_cron"`
	CVEOrgCron     string `mapstructure:"cveorg_cron"`
	EPSSCron       string `mapstructure:"epss_cron"`
	CPECron        string `mapstructure:"cpe_cron"`
	CAPECCWECron   string `mapstructure:"capec_cwe_cron"`
	KEVCron        string `mapstructure:"kev_cron"`
}

type ObservabilityConfig struct {
	LogLevel        string `mapstructure:"log_level"`
	MetricsPort     int    `mapstructure:"metrics_port"`
	TracingEndpoint string `mapstructure:"tracing_endpoint"`
}

type MigrationsConfig struct {
	Dir string `mapstructure:"dir"`
}

// Load reads config from file and environment variables.
func Load(cfgFile string) (*Config, error) {
	v := viper.New()

	v.SetConfigFile(cfgFile)
	v.SetConfigType("yaml")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Defaults
	v.SetDefault("server.gateway_port", 8080)
	v.SetDefault("server.cvesearch_port", 8081)
	v.SetDefault("server.cvesync_port", 8082)
	v.SetDefault("server.kevservice_port", 8083)
	v.SetDefault("server.notification_port", 8084)
	v.SetDefault("server.read_timeout", "30s")
	v.SetDefault("server.write_timeout", "30s")
	v.SetDefault("server.shutdown_timeout", "15s")
	v.SetDefault("database.max_open_conns", 50)
	v.SetDefault("database.max_idle_conns", 10)
	v.SetDefault("redis.pool_size", 20)
	v.SetDefault("opensearch.index", "cves")
	v.SetDefault("cache.search_ttl", 300)
	v.SetDefault("cache.single_ttl", 3600)
	v.SetDefault("cache.kev_ttl", 3600)
	v.SetDefault("cache.check_ttl", 300)
	v.SetDefault("rate_limit.max_requests", 60)
	v.SetDefault("rate_limit.window", "60s")
	v.SetDefault("migrations.dir", "./migrations")
	v.SetDefault("observability.log_level", "info")
	v.SetDefault("observability.metrics_port", 9090)
	v.SetDefault("scheduler.enabled", true)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	return &cfg, nil
}
