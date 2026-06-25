// Package api implements the public gRPC server API for OSV.
package api

import (
	"context"
	"fmt"
	"net"

	"github.com/osv/shared/pkg/models"
	"github.com/osv/shared/pkg/logger"
	"google.golang.org/grpc"
	pb "github.com/osv/shared/pkg/clients/api"
)

type server struct {
	pb.UnimplementedOSVServer

	vulnStore      models.VulnerabilityStore
	relationsStore models.RelationsStore
}

type ServerOptions struct {
	Port           int
	VulnStore      models.VulnerabilityStore
	RelationsStore models.RelationsStore
}

// RunServer starts the gRPC server and handles graceful shutdown.
func RunServer(ctx context.Context, opts ServerOptions) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", opts.Port))
	if err != nil {
		logger.ErrorContext(ctx, "failed to listen", "error", err)
		return err
	}

	s := grpc.NewServer()
	pb.RegisterOSVServer(s, &server{
		vulnStore:      opts.VulnStore,
		relationsStore: opts.RelationsStore,
	})

	logger.InfoContext(ctx, "server listening", "port", opts.Port)

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- s.Serve(lis)
	}()

	select {
	case err := <-serveErr:
		if err != nil {
			logger.ErrorContext(ctx, "server failed to serve", "error", err)
			return err
		}
	case <-ctx.Done():
		logger.InfoContext(ctx, "received shutdown signal, shutting down server gracefully")
		s.GracefulStop()
		if err := <-serveErr; err != nil {
			logger.ErrorContext(ctx, "server failed during shutdown", "error", err)
			return err
		}
	}

	return nil
}
