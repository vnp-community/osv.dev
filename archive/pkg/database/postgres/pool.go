// Package postgres provides a pgx/v5 connection pool factory for PostgreSQL.
// It wraps pgxpool.Pool configuration with sensible defaults and validates
// connectivity at startup time.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds connection pool settings.
type Config struct {
	// URL is the PostgreSQL DSN, e.g. postgres://user:pass@host:5432/db
	URL string `yaml:"url" env:"DATABASE_URL,required"`

	// MaxConns is the maximum number of connections in the pool (default 25).
	MaxConns int32 `yaml:"max_conns" env:"DB_MAX_CONNS"`

	// MinConns is the minimum number of idle connections (default 5).
	MinConns int32 `yaml:"min_conns" env:"DB_MIN_CONNS"`

	// MaxConnLifetime is how long a connection may be reused (default 5m).
	MaxConnLifetime time.Duration `yaml:"max_conn_lifetime"`

	// MaxConnIdleTime is how long an idle connection is kept (default 30m).
	MaxConnIdleTime time.Duration `yaml:"max_conn_idle_time"`
}

// DefaultConfig returns Config with production-ready defaults.
func DefaultConfig() *Config {
	return &Config{
		MaxConns:        25,
		MinConns:        5,
		MaxConnLifetime: 5 * time.Minute,
		MaxConnIdleTime: 30 * time.Minute,
	}
}

// NewPool creates and verifies a pgxpool.Pool using the provided Config.
// The pool is pinged before returning; if the database is unreachable the
// function returns an error immediately so callers can fail fast at startup.
func NewPool(ctx context.Context, cfg *Config) (*pgxpool.Pool, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("postgres: parse DSN: %w", err)
	}

	// Apply pool sizing overrides when non-zero.
	if cfg.MaxConns > 0 {
		poolCfg.MaxConns = cfg.MaxConns
	}
	if cfg.MinConns > 0 {
		poolCfg.MinConns = cfg.MinConns
	}
	if cfg.MaxConnLifetime > 0 {
		poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	}
	if cfg.MaxConnIdleTime > 0 {
		poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: create pool: %w", err)
	}

	// Verify connectivity at startup.
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: ping failed: %w", err)
	}

	return pool, nil
}

// MustNewPool is like NewPool but panics on error. Use in main() where a
// database connection is mandatory.
func MustNewPool(ctx context.Context, cfg *Config) *pgxpool.Pool {
	pool, err := NewPool(ctx, cfg)
	if err != nil {
		panic(err)
	}
	return pool
}
