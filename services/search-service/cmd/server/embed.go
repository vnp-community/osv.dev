// embed.go — EmbeddedServer allows search-service to be run inside apps/osv orchestrator.
// This file is ADDITIVE — main.go is NOT modified.
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
)

// SearchServiceEmbeddedConfig holds configuration for embedded search-service.
type SearchServiceEmbeddedConfig struct {
	HTTPPort    int    // REST API port (default: 8083)
	GRPCPort    int    // gRPC port (default: 50056)
	NATSURL     string
	MongoURI    string
	PostgresDSN string // for pgvector
}

// SearchServiceEmbeddedServer wraps search-service for embedding in apps/osv.
type SearchServiceEmbeddedServer struct {
	cfg SearchServiceEmbeddedConfig
}

// NewSearchServiceEmbeddedServer creates a new embeddable server instance.
func NewSearchServiceEmbeddedServer(cfg SearchServiceEmbeddedConfig) *SearchServiceEmbeddedServer {
	return &SearchServiceEmbeddedServer{cfg: cfg}
}

// Name satisfies the orchestrator.Service interface.
func (s *SearchServiceEmbeddedServer) Name() string { return "search-service" }

// Start begins serving and blocks until ctx is cancelled.
func (s *SearchServiceEmbeddedServer) Start(ctx context.Context) error {
	if s.cfg.MongoURI != "" {
		os.Setenv("MONGO_URI", s.cfg.MongoURI)
	}
	if s.cfg.PostgresDSN != "" {
		os.Setenv("POSTGRES_DSN", s.cfg.PostgresDSN)
	}
	if s.cfg.NATSURL != "" {
		os.Setenv("NATS_URL", s.cfg.NATSURL)
	}
	return runEmbeddedSearchService(ctx, s.cfg)
}

func runEmbeddedSearchService(ctx context.Context, cfg SearchServiceEmbeddedConfig) error {
	port := cfg.HTTPPort
	if port == 0 {
		port = 8083
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"status":"ok","service":"search-service"}`)
	})
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("search-service listen :%d: %w", port, err)
	}
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln) //nolint:errcheck
	<-ctx.Done()
	return srv.Close()
}
