// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

// Command converter is the entrypoint for the OSV Converter service.
//
// The converter service transforms vulnerability records from external formats
// (CVE v5, NVD JSON v2, Alpine secdb, etc.) into the OSV vulnerability format,
// and publishes them to the event bus for downstream ingestion.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

var (
	port    = flag.Int("port", 8080, "gRPC server port")
	logLevel = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
)

func main() {
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "converter: fatal: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	fmt.Printf("OSV Converter Service starting on port %d (log-level=%s)\n", *port, *logLevel)

	// TODO(Phase 3): Wire up:
	//   1. gRPC server with ConverterServiceServer
	//   2. NATS subscriber for raw CVE events
	//   3. services/pkg/clients/kev for KEV enrichment
	//   4. services/pkg/clients/epss for EPSS scoring
	//   5. Health check endpoint

	// Wait for shutdown signal
	<-ctx.Done()
	fmt.Println("converter: shutting down...")
	return nil
}
