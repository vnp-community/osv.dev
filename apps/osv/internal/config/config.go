// Package config provides runtime configuration for the OSV server orchestrator.
// All values are read from environment variables with sensible defaults.
//
// This is ADDITIVE — existing config in apps/osv is not modified.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all runtime configuration for the OSV orchestrator.
type Config struct {
	// Mode determines how the server starts:
	//   "standalone" (default) — GCP Datastore + existing GCP pipeline
	//   "microservices"        — full embedded service orchestrator
	Mode string

	// HTTP is the main HTTP gateway port (default: 8080)
	HTTP Port

	// Services holds per-service addresses when Mode=microservices
	Services ServiceAddrs

	// EmbeddedInfra holds connection strings for embedded microservices
	EmbeddedInfra EmbeddedInfraConfig
}

// EmbeddedInfraConfig holds infrastructure settings for embedded services.
type EmbeddedInfraConfig struct {
	PostgresDSN string
	RedisURL    string
	JWTSecret   string
}

// Port holds a network port number.
type Port struct {
	Port int
}

// Addr returns a ":PORT" listen address string.
func (p Port) Addr() string {
	return fmt.Sprintf(":%d", p.Port)
}

// ServiceAddrs holds upstream addresses for each microservice.
type ServiceAddrs struct {
	DataServiceGRPC       string // gRPC addr for data-service (CVEDBService)
	SearchServiceHTTP     string // HTTP base URL for search-service
	AIServiceGRPC         string // gRPC addr for ai-service
	FindingServiceGRPC    string // gRPC addr for finding-service
	IdentityServiceGRPC   string // gRPC addr for identity-service
	NotificationHTTP      string // HTTP addr for notification-service
	GatewayHTTP           string // HTTP addr for gateway-service (internal)
	NATSURL               string // NATS server URL

	// CVE Search CR additions (CR-001–CR-009)
	DataServiceHTTP    string // HTTP base URL for data-service REST (CVE search/feed/query)
	RankingServiceHTTP string // HTTP base URL for ranking-service (CR-004)
	IdentityServiceHTTP string // HTTP base URL for identity-service admin endpoints (CR-007)
}

// FromEnv loads configuration from environment variables.
func FromEnv() *Config {
	return &Config{
		Mode: envOrDefault("OSV_MODE", "standalone"),
		HTTP: Port{Port: envInt("HTTP_PORT", 8080)},
		Services: ServiceAddrs{
			DataServiceGRPC:     envOrDefault("DATA_SERVICE_ADDR", "localhost:50053"),
			SearchServiceHTTP:   envOrDefault("SEARCH_SERVICE_HTTP", "http://localhost:8083"),
			AIServiceGRPC:       envOrDefault("AI_SERVICE_ADDR", "localhost:50052"),
			FindingServiceGRPC:  envOrDefault("FINDING_SERVICE_ADDR", "localhost:50060"),
			IdentityServiceGRPC: envOrDefault("IDENTITY_SERVICE_ADDR", "localhost:50051"),
			NotificationHTTP:    envOrDefault("NOTIFICATION_HTTP", "http://localhost:8084"),
			GatewayHTTP:         envOrDefault("GATEWAY_HTTP", "http://localhost:8080"),
			NATSURL:             envOrDefault("NATS_URL", "nats://localhost:4222"),
			// CVE Search CRs — new HTTP upstreams
			DataServiceHTTP:     envOrDefault("DATA_SERVICE_HTTP", "http://localhost:8082"),
			RankingServiceHTTP:  envOrDefault("RANKING_SERVICE_HTTP", "http://localhost:8088"),
			IdentityServiceHTTP: envOrDefault("IDENTITY_SERVICE_HTTP", "http://localhost:8081"),
		},
		EmbeddedInfra: EmbeddedInfraConfig{
			PostgresDSN: envOrDefault("POSTGRES_DSN", "postgres://osv:osv_dev@localhost:5432/osv?sslmode=disable"),
			RedisURL:    envOrDefault("REDIS_URL", "redis://localhost:6379/0"),
			JWTSecret:   envOrDefault("JWT_SECRET", "production-secret-key-change-me"),
		},
	}
}

// IsMicroservicesMode returns true when Mode=microservices.
func (c *Config) IsMicroservicesMode() bool {
	return strings.EqualFold(c.Mode, "microservices")
}

// Validate checks that required fields are set.
func (c *Config) Validate() error {
	if c.HTTP.Port <= 0 || c.HTTP.Port > 65535 {
		return fmt.Errorf("invalid HTTP_PORT: %d", c.HTTP.Port)
	}
	return nil
}

// String returns a human-readable config summary (safe to log).
func (c *Config) String() string {
	return fmt.Sprintf("mode=%s http=%s data=%s search=%s ai=%s nats=%s",
		c.Mode, c.HTTP.Addr(),
		c.Services.DataServiceGRPC, c.Services.SearchServiceHTTP,
		c.Services.AIServiceGRPC, c.Services.NATSURL,
	)
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	i, err := strconv.Atoi(v)
	if err != nil || i <= 0 {
		return def
	}
	return i
}
