// Package main is the entrypoint for integration-service.
// Currently hosts: Jira integration (ticket creation from findings).
// Future: GitHub, GitLab integrations via registry pattern.
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
	log.Info().
		Str("port", "50054").
		Str("integrations", "jira").
		Msg("integration-service starting")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Info().Msg("integration-service shutting down")
}
