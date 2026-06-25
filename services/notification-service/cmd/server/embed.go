// embed.go — EmbeddedServer allows notification-service to be run inside apps/osv orchestrator.
// This file is ADDITIVE — main.go is NOT modified.
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
)

// NotificationServiceEmbeddedConfig holds configuration for embedded notification-service.
type NotificationServiceEmbeddedConfig struct {
	HTTPPort    int    // HTTP port (default: 8084)
	NATSURL     string
	PostgresDSN string
}

// NotificationServiceEmbeddedServer wraps notification-service for embedding in apps/osv.
type NotificationServiceEmbeddedServer struct {
	cfg NotificationServiceEmbeddedConfig
}

// NewNotificationServiceEmbeddedServer creates a new embeddable server instance.
func NewNotificationServiceEmbeddedServer(cfg NotificationServiceEmbeddedConfig) *NotificationServiceEmbeddedServer {
	return &NotificationServiceEmbeddedServer{cfg: cfg}
}

// Name satisfies the orchestrator.Service interface.
func (s *NotificationServiceEmbeddedServer) Name() string { return "notification-service" }

// Start begins serving and blocks until ctx is cancelled.
func (s *NotificationServiceEmbeddedServer) Start(ctx context.Context) error {
	if s.cfg.PostgresDSN != "" {
		os.Setenv("POSTGRES_DSN", s.cfg.PostgresDSN)
	}
	if s.cfg.NATSURL != "" {
		os.Setenv("NATS_URL", s.cfg.NATSURL)
	}
	return runEmbeddedNotificationService(ctx, s.cfg)
}

func runEmbeddedNotificationService(ctx context.Context, cfg NotificationServiceEmbeddedConfig) error {
	port := cfg.HTTPPort
	if port == 0 {
		port = 8084
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"status":"ok","service":"notification-service"}`)
	})
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("notification-service listen :%d: %w", port, err)
	}
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln) //nolint:errcheck
	<-ctx.Done()
	return srv.Close()
}
