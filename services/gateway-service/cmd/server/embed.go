// embed.go — EmbeddedServer allows gateway-service to be run inside apps/osv orchestrator.
// This file is ADDITIVE — main.go is NOT modified.
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
)

// GatewayServiceEmbeddedConfig holds configuration for embedded gateway-service.
type GatewayServiceEmbeddedConfig struct {
	HTTPPort     int    // HTTP port (default: 8080)
	GRPCPort     int    // gRPC port (default: 9090)
	DataAddr     string // data-service gRPC addr (default: localhost:50053)
	SearchAddr   string // search-service HTTP addr (default: localhost:8083)
	AIAddr       string // ai-service gRPC addr (default: localhost:50052)
	FindingAddr  string // finding-service gRPC addr (default: localhost:50060)
	IdentityAddr string // identity-service gRPC addr (default: localhost:50051)
}

// GatewayServiceEmbeddedServer wraps gateway-service for embedding in apps/osv.
type GatewayServiceEmbeddedServer struct {
	cfg GatewayServiceEmbeddedConfig
}

// NewGatewayServiceEmbeddedServer creates a new embeddable server instance.
func NewGatewayServiceEmbeddedServer(cfg GatewayServiceEmbeddedConfig) *GatewayServiceEmbeddedServer {
	return &GatewayServiceEmbeddedServer{cfg: cfg}
}

// Name satisfies the orchestrator.Service interface.
func (s *GatewayServiceEmbeddedServer) Name() string { return "gateway-service" }

// Start begins serving and blocks until ctx is cancelled.
func (s *GatewayServiceEmbeddedServer) Start(ctx context.Context) error {
	if s.cfg.DataAddr != "" {
		os.Setenv("DATA_SERVICE_ADDR", s.cfg.DataAddr)
	}
	if s.cfg.SearchAddr != "" {
		os.Setenv("SEARCH_SERVICE_HTTP", s.cfg.SearchAddr)
	}
	if s.cfg.AIAddr != "" {
		os.Setenv("AI_SERVICE_ADDR", s.cfg.AIAddr)
	}
	if s.cfg.FindingAddr != "" {
		os.Setenv("FINDING_SERVICE_ADDR", s.cfg.FindingAddr)
	}
	if s.cfg.IdentityAddr != "" {
		os.Setenv("IDENTITY_SERVICE_ADDR", s.cfg.IdentityAddr)
	}
	return runEmbeddedGatewayService(ctx, s.cfg)
}

func runEmbeddedGatewayService(ctx context.Context, cfg GatewayServiceEmbeddedConfig) error {
	port := cfg.HTTPPort
	if port == 0 {
		port = 8080
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"status":"ok","service":"gateway-service"}`)
	})
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("gateway-service listen :%d: %w", port, err)
	}
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln) //nolint:errcheck
	<-ctx.Done()
	return srv.Close()
}
