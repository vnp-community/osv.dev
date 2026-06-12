// Package config provides OVS-specific configuration for the API gateway.
package config

import "time"

// OVSConfig holds the full configuration for the OpenVulnScan API gateway.
type OVSConfig struct {
	Server         ServerConfig              `yaml:"server"`
	Auth           OVSAuthConfig             `yaml:"auth"`
	Upstreams      map[string]UpstreamConfig `yaml:"upstreams"`
	Redis          RedisConfig               `yaml:"redis"`
	RateLimit      RateLimitConfig           `yaml:"rate_limit"`
	CircuitBreaker CBConfig                  `yaml:"circuit_breaker"`
	CORS           CORSConfig                `yaml:"cors"`
}

// ServerConfig holds HTTP/gRPC server settings.
type ServerConfig struct {
	HTTPPort     string        `yaml:"http_port"`
	GRPCPort     string        `yaml:"grpc_port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout"`
}

// OVSAuthConfig holds auth-service connection settings.
type OVSAuthConfig struct {
	// GRPCAddress is the auth-service gRPC endpoint, e.g. "auth-service:9001"
	GRPCAddress string `yaml:"grpc_address"`
	// CacheTTL is how long validated JWT results are cached in Redis.
	CacheTTL time.Duration `yaml:"cache_ttl"`
	// SkipPaths are URL path prefixes that bypass authentication.
	SkipPaths []string `yaml:"skip_paths"`
}

// UpstreamConfig holds the URL and timeout for one upstream service.
type UpstreamConfig struct {
	Address string        `yaml:"address"`
	Timeout time.Duration `yaml:"timeout"`
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	URL string `yaml:"url"`
	DB  int    `yaml:"db"`
}

// RateLimitConfig holds rate limiting settings.
type RateLimitConfig struct {
	Enabled     bool                      `yaml:"enabled"`
	DefaultTier string                    `yaml:"default_tier"`
	Tiers       map[string]TierConfig     `yaml:"tiers"`
}

// TierConfig defines requests-per-window for a rate limit tier.
type TierConfig struct {
	Requests int           `yaml:"requests"`
	Window   time.Duration `yaml:"window"`
}

// CBConfig holds circuit breaker settings.
type CBConfig struct {
	Threshold    int           `yaml:"threshold"`
	ResetTimeout time.Duration `yaml:"reset_timeout"`
}

// CORSConfig holds CORS settings.
type CORSConfig struct {
	AllowedOrigins []string `yaml:"allowed_origins"`
	AllowedMethods []string `yaml:"allowed_methods"`
	AllowedHeaders []string `yaml:"allowed_headers"`
	MaxAge         int      `yaml:"max_age"`
}

// DefaultOVSConfig returns safe defaults for local development.
func DefaultOVSConfig() *OVSConfig {
	return &OVSConfig{
		Server: ServerConfig{
			HTTPPort:     "8080",
			GRPCPort:     "50051",
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		Auth: OVSAuthConfig{
			GRPCAddress: "localhost:9001",
			CacheTTL:    60 * time.Second,
			SkipPaths: []string{
				"/health/live", "/health/ready", "/metrics",
				"/.well-known/jwks.json", "/api/v1/auth",
			},
		},
		CircuitBreaker: CBConfig{
			Threshold:    5,
			ResetTimeout: 30 * time.Second,
		},
		CORS: CORSConfig{
			AllowedOrigins: []string{"*"},
			AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{"Authorization", "Content-Type", "X-Request-ID"},
			MaxAge:         3600,
		},
	}
}
