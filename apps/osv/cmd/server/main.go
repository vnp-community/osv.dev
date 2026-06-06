// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Command server is the main entrypoint for the OSV.dev web server.
// It composes the API gateway, vulnerability query, search, and web BFF services.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/osv/pkg/logger"
	"github.com/osv/pkg/observability"
	"github.com/rs/zerolog"
)

func main() {
	logger.InitGlobalLogger()
	defer logger.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")

	log := zerolog.New(os.Stdout).With().Timestamp().Logger()
	shutdown := observability.MustSetup("osv-server", "dev", log)
	defer shutdown()

	if err := run(ctx, projectID); err != nil {
		slog.Error("server exited with error", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, projectID string) error {
	slog.InfoContext(ctx, "OSV server starting", "project", projectID)

	// TODO: Wire up individual services:
	// - api-gateway  → gRPC gateway
	// - vulnerability-query → vuln lookup service
	// - search       → full-text search
	// - web-bff      → frontend Backend-For-Frontend

	<-ctx.Done()
	fmt.Println("Shutting down...")
	return nil
}
