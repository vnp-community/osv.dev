// embed.go — EmbeddedServer allows scan-service to be run inside apps/osv orchestrator.
// This file is ADDITIVE — main.go is NOT modified.
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
)

// ScanServiceEmbeddedConfig holds configuration for embedded scan-service.
type ScanServiceEmbeddedConfig struct {
	HTTPPort int    // HTTP port (default: 8087)
	NATSURL  string
}

// ScanServiceEmbeddedServer wraps scan-service for embedding in apps/osv.
type ScanServiceEmbeddedServer struct {
	cfg ScanServiceEmbeddedConfig
}

// NewScanServiceEmbeddedServer creates a new embeddable server instance.
func NewScanServiceEmbeddedServer(cfg ScanServiceEmbeddedConfig) *ScanServiceEmbeddedServer {
	return &ScanServiceEmbeddedServer{cfg: cfg}
}

// Name satisfies the orchestrator.Service interface.
func (s *ScanServiceEmbeddedServer) Name() string { return "scan-service" }

// Start begins serving and blocks until ctx is cancelled.
func (s *ScanServiceEmbeddedServer) Start(ctx context.Context) error {
	if s.cfg.NATSURL != "" {
		os.Setenv("NATS_URL", s.cfg.NATSURL)
	}
	return runEmbeddedScanService(ctx, s.cfg)
}

func runEmbeddedScanService(ctx context.Context, cfg ScanServiceEmbeddedConfig) error {
	port := cfg.HTTPPort
	if port == 0 {
		port = 8087
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"status":"ok","service":"scan-service"}`)
	})
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("scan-service listen :%d: %w", port, err)
	}
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln) //nolint:errcheck
	<-ctx.Done()
	return srv.Close()
}
