package observability

import (
	"os"

	"github.com/rs/zerolog"
)

// InitLogger configures structured JSON zerolog logger.
// LOG_LEVEL env: "debug"|"info"|"warn"|"error" (default: "info")
func InitLogger(serviceName, version string) zerolog.Logger {
	level := zerolog.InfoLevel
	if l, err := zerolog.ParseLevel(os.Getenv("LOG_LEVEL")); err == nil {
		level = l
	}
	return zerolog.New(os.Stdout).Level(level).
		With().Timestamp().Str("service", serviceName).Str("version", version).Logger()
}
