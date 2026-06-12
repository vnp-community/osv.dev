// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

// Command admin is the entrypoint for the OSV Admin API service.
//
// The admin service provides a REST API for operational management:
//   - Source management (pause, resume, trigger sync)
//   - Import finding management (view, resolve errors)
//   - Vulnerability operations (withdraw, reprocess)
//   - System health and statistics
//   - API key management
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/osv/admin/internal/infra/http/handler"
)

var (
	httpPort = flag.Int("http-port", 8090, "HTTP REST API port")
	logLevel = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
)

func main() {
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "admin: fatal: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	h := handler.New()
	mux := http.NewServeMux()
	registerRoutes(mux, h)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", *httpPort),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	fmt.Printf("OSV Admin Service starting on port %d (log-level=%s)\n", *httpPort, *logLevel)

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		fmt.Println("admin: shutting down gracefully...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	}
}

// registerRoutes sets up all admin API routes.
func registerRoutes(mux *http.ServeMux, h *handler.Handler) {
	// Health
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"status":"ok"}`)
	})

	// ── Sources (TASK-06-01) ───────────────────────────────────────────────────
	mux.HandleFunc("GET /admin/v1/sources", h.ListSources)
	mux.HandleFunc("GET /admin/v1/sources/{name}", h.GetSource)
	mux.HandleFunc("POST /admin/v1/sources/{name}/sync", h.TriggerSync)
	mux.HandleFunc("POST /admin/v1/sources/{name}/pause", h.PauseSource)
	mux.HandleFunc("POST /admin/v1/sources/{name}/resume", h.ResumeSource)

	// ── Import Findings (TASK-06-02) ───────────────────────────────────────────
	mux.HandleFunc("GET /admin/v1/import-findings", h.ListImportFindings)
	mux.HandleFunc("POST /admin/v1/import-findings/{id}/resolve", h.ResolveImportFinding)

	// ── Vulnerability Admin (TASK-06-03) ───────────────────────────────────────
	mux.HandleFunc("POST /admin/v1/vulns/{id}/withdraw", h.WithdrawVuln)
	mux.HandleFunc("POST /admin/v1/vulns/{id}/reprocess", h.ReprocessVuln)
	mux.HandleFunc("GET /admin/v1/vulns/stats", h.VulnStats)

	// ── System Health (TASK-06-07) ─────────────────────────────────────────────
	mux.HandleFunc("GET /admin/v1/system/health", h.SystemHealth)

	// ── API Key Management (TASK-06-06) ────────────────────────────────────────
	mux.HandleFunc("GET /admin/v1/api-keys", h.ListAPIKeys)
	mux.HandleFunc("POST /admin/v1/api-keys", h.CreateAPIKey)
	mux.HandleFunc("DELETE /admin/v1/api-keys/{id}", h.RevokeAPIKey)
}
