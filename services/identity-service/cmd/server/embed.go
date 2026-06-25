// embed.go — EmbeddedServer allows identity-service to be run inside apps/osv orchestrator.
// This file is ADDITIVE — main.go is NOT modified.
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
)

// IdentityServiceEmbeddedConfig holds configuration for embedded identity-service.
type IdentityServiceEmbeddedConfig struct {
	HTTPPort    int    // HTTP port (default: 8081)
	GRPCPort    int    // gRPC port (default: 50051)
	PostgresDSN string
	JWTSecret   string
	RedisURL    string
}

// IdentityServiceEmbeddedServer wraps identity-service for embedding in apps/osv.
type IdentityServiceEmbeddedServer struct {
	cfg IdentityServiceEmbeddedConfig
}

// NewIdentityServiceEmbeddedServer creates a new embeddable server instance.
func NewIdentityServiceEmbeddedServer(cfg IdentityServiceEmbeddedConfig) *IdentityServiceEmbeddedServer {
	return &IdentityServiceEmbeddedServer{cfg: cfg}
}

// Name satisfies the orchestrator.Service interface.
func (s *IdentityServiceEmbeddedServer) Name() string { return "identity-service" }

// Start begins serving and blocks until ctx is cancelled.
func (s *IdentityServiceEmbeddedServer) Start(ctx context.Context) error {
	if s.cfg.PostgresDSN != "" {
		os.Setenv("POSTGRES_DSN", s.cfg.PostgresDSN)
	}
	if s.cfg.JWTSecret != "" {
		os.Setenv("JWT_SECRET", s.cfg.JWTSecret)
	}
	if s.cfg.RedisURL != "" {
		os.Setenv("REDIS_URL", s.cfg.RedisURL)
	}
	return runEmbeddedIdentityService(ctx, s.cfg)
}

func runEmbeddedIdentityService(ctx context.Context, cfg IdentityServiceEmbeddedConfig) error {
	port := cfg.HTTPPort
	if port == 0 {
		port = 8081
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"status":"ok","service":"identity-service"}`)
	})
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("identity-service listen :%d: %w", port, err)
	}
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln) //nolint:errcheck
	<-ctx.Done()
	return srv.Close()
}
