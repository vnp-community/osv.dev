// Package config định nghĩa tất cả runtime configuration cho scan-service.
// Tất cả giá trị đều có thể override qua environment variables.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"
)

// ScanConfig là single source of truth cho mọi config của scan-service.
// Load từ environment variables qua [Load].
type ScanConfig struct {
	// ZAP Scanner config
	ZAPBaseURL           string        // env: ZAP_BASE_URL          default: http://localhost:8080 (warns)
	ZAPAPIKey            string        // env: ZAP_API_KEY           optional
	ZAPHTTPTimeout       time.Duration // env: ZAP_HTTP_TIMEOUT      default: 30s
	ZAPSpiderTimeout     time.Duration // env: ZAP_SPIDER_TIMEOUT    default: 5m (300s)
	ZAPActiveScanTimeout time.Duration // env: ZAP_ACTIVE_SCAN_TIMEOUT default: 10m (600s)
	ZAPPollInterval      time.Duration // env: ZAP_POLL_INTERVAL     default: 5s

	// Scheduler config
	SchedulerTickInterval time.Duration // env: SCHEDULER_TICK_INTERVAL  default: 1m
	SchedulerLeaderTTL    time.Duration // env: SCHEDULER_LEADER_TTL     default: 90s
}

// Load đọc ScanConfig từ environment variables với sensible defaults.
// Logs WARN khi ZAP_BASE_URL trỏ về localhost.
func Load() ScanConfig {
	zapURL := envHTTPAddr("ZAP_BASE_URL", "localhost", 8080)

	return ScanConfig{
		ZAPBaseURL:           zapURL,
		ZAPAPIKey:            os.Getenv("ZAP_API_KEY"),
		ZAPHTTPTimeout:       envDuration("ZAP_HTTP_TIMEOUT", 30*time.Second),
		ZAPSpiderTimeout:     envDuration("ZAP_SPIDER_TIMEOUT", 5*time.Minute),
		ZAPActiveScanTimeout: envDuration("ZAP_ACTIVE_SCAN_TIMEOUT", 10*time.Minute),
		ZAPPollInterval:      envDuration("ZAP_POLL_INTERVAL", 5*time.Second),
		SchedulerTickInterval: envDuration("SCHEDULER_TICK_INTERVAL", 1*time.Minute),
		SchedulerLeaderTTL:    envDuration("SCHEDULER_LEADER_TTL", 90*time.Second),
	}
}

// envHTTPAddr reads an HTTP URL from env var.
// Falls back to http://defaultHost:defaultPort with a WARN log.
func envHTTPAddr(key, defaultHost string, defaultPort int) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	fallback := fmt.Sprintf("http://%s:%d", defaultHost, defaultPort)
	slog.Warn("env var not set, using localhost fallback — configure in production",
		"env_key", key,
		"fallback", fallback,
	)
	return fallback
}

// envDuration parses a duration string from an env var (e.g. "5m", "30s").
// Returns defaultVal on missing or invalid input.
func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		slog.Warn("invalid duration value for env var, using default",
			"env_key", key,
			"value", v,
			"fallback", def.String(),
		)
		return def
	}
	return d
}

// envInt parses an integer from an env var. Returns defaultVal on missing or invalid input.
func envInt(key string, def int) int { //nolint:unused
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}
