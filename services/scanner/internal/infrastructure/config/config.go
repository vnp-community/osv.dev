// Package config holds scanner service configuration.
package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all scanner service configuration.
type Config struct {
	GRPCPort       int    // gRPC listen port (default: 50054)
	LogLevel       string // "debug"|"info"|"warn"|"error"
	MaxDepth       int    // max archive extraction depth (default: 10)
	MaxFileSize    int64  // max file size to scan in bytes (default: 100MB)
	ShutdownSecs   int    // graceful shutdown timeout in seconds (default: 30)
}

// ShutdownTimeout returns the graceful shutdown timeout.
func (c Config) ShutdownTimeout() time.Duration {
	if c.ShutdownSecs <= 0 {
		return 30 * time.Second
	}
	return time.Duration(c.ShutdownSecs) * time.Second
}

// Load reads configuration from environment variables.
func Load() Config {
	return Config{
		GRPCPort:    envInt("SCANNER_GRPC_PORT", 50054),
		LogLevel:    envStr("SCANNER_LOG_LEVEL", "info"),
		MaxDepth:    envInt("SCANNER_MAX_DEPTH", 10),
		MaxFileSize: int64(envInt("SCANNER_MAX_FILE_SIZE_MB", 100)) * 1024 * 1024,
	}
}

func envStr(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func envInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}
