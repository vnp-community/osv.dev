// config/config.go
package config

import (
	"os"
	"strconv"
)

// Config holds all configuration for the Alias Relations Service.
type Config struct {
	Server struct {
		GRPCPort    int
		HTTPPort    int
		MetricsPort int
	}
	Firestore struct {
		ProjectID string
	}
	NATS struct {
		URL        string
		StreamName string
	}
	Telemetry struct {
		OTLPEndpoint string
		ServiceName  string
	}
	AIEnrichment struct {
		ServiceAddr string // gRPC address of AI Enrichment service
	}
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	cfg := &Config{}

	cfg.Server.GRPCPort = envInt("GRPC_PORT", 50051)
	cfg.Server.HTTPPort = envInt("HTTP_PORT", 8080)
	cfg.Server.MetricsPort = envInt("METRICS_PORT", 9090)

	cfg.Firestore.ProjectID = envStr("GCP_PROJECT", "osv-dev")
	cfg.NATS.URL = envStr("NATS_URL", "nats://localhost:4222")
	cfg.NATS.StreamName = envStr("NATS_STREAM", "OSV-EVENTS")

	cfg.Telemetry.OTLPEndpoint = envStr("OTLP_ENDPOINT", "")
	cfg.Telemetry.ServiceName = envStr("SERVICE_NAME", "alias-relations")

	cfg.AIEnrichment.ServiceAddr = envStr("AI_ENRICHMENT_ADDR", "ai-enrichment:50051")

	return cfg
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
