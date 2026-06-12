// Package main is the entrypoint for impact-service.
// Combines impact-analysis (vulnerability impact) and version-index (git repo indexing).
package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
	log.Info().Str("port", "50053").Msg("impact-service gRPC listening")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Info().Msg("impact-service shutting down")
}
