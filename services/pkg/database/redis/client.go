// Package redis provides a go-redis/v9 client factory for Redis.
// It wraps connection configuration with sensible defaults and validates
// connectivity at startup time.
package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Config holds Redis client settings.
type Config struct {
	// URL is the Redis connection URL, e.g. redis://:password@host:6379/0
	URL string `yaml:"url" env:"REDIS_URL,required"`

	// PoolSize is the maximum number of socket connections (default 10).
	PoolSize int `yaml:"pool_size" env:"REDIS_POOL_SIZE"`

	// MinIdleConns is the minimum idle connections kept open (default 2).
	MinIdleConns int `yaml:"min_idle_conns"`

	// DialTimeout is the connection timeout (default 5s).
	DialTimeout time.Duration `yaml:"dial_timeout"`

	// ReadTimeout is the per-command read timeout (default 3s).
	ReadTimeout time.Duration `yaml:"read_timeout"`

	// WriteTimeout is the per-command write timeout (default 3s).
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

// DefaultConfig returns Config with production-ready defaults.
func DefaultConfig() *Config {
	return &Config{
		PoolSize:     10,
		MinIdleConns: 2,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}
}

// NewClient creates and verifies a *redis.Client using the provided Config.
// The client is pinged before returning; if Redis is unreachable the function
// returns an error immediately.
func NewClient(cfg *Config) (*redis.Client, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	opts, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("redis: parse URL: %w", err)
	}

	// Apply overrides when non-zero.
	if cfg.PoolSize > 0 {
		opts.PoolSize = cfg.PoolSize
	}
	if cfg.MinIdleConns > 0 {
		opts.MinIdleConns = cfg.MinIdleConns
	}
	if cfg.DialTimeout > 0 {
		opts.DialTimeout = cfg.DialTimeout
	}
	if cfg.ReadTimeout > 0 {
		opts.ReadTimeout = cfg.ReadTimeout
	}
	if cfg.WriteTimeout > 0 {
		opts.WriteTimeout = cfg.WriteTimeout
	}

	client := redis.NewClient(opts)

	// Verify connectivity at startup.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis: ping failed: %w", err)
	}

	return client, nil
}

// MustNewClient is like NewClient but panics on error. Use in main() where
// Redis connectivity is mandatory.
func MustNewClient(cfg *Config) *redis.Client {
	c, err := NewClient(cfg)
	if err != nil {
		panic(err)
	}
	return c
}
