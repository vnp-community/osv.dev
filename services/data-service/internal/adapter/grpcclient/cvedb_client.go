// Package grpcclient provides a gRPC client for the CVEDB service.
// Used by syncall use case to push fetched CVE data to the CVEDB service.
package grpcclient

import (
	"context"

	"github.com/osv/data-service/internal/adapter/external/sources"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// CVEDBClient is a gRPC client for the CVEDB service.
type CVEDBClient struct {
	conn *grpc.ClientConn
	log  zerolog.Logger
}

// NewCVEDBClient creates a new CVEDBClient connected to the given address.
func NewCVEDBClient(addr string, log zerolog.Logger) (*CVEDBClient, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}
	return &CVEDBClient{conn: conn, log: log.With().Str("component", "cvedb-grpc-client").Logger()}, nil
}

// PopulateDB pushes CVE data to the CVEDB service.
// This is a best-effort operation; errors are logged but not propagated.
func (c *CVEDBClient) PopulateDB(ctx context.Context, data sources.CVEData) error {
	c.log.Debug().
		Str("source", data.Source).
		Int("severities", len(data.Severities)).
		Int("ranges", len(data.Ranges)).
		Msg("PopulateDB called (gRPC stub)")
	// TODO: wire to real proto once CVEDB service exposes PopulateDB via gRPC.
	// For now this is a no-op stub to satisfy the build.
	return nil
}

// Close closes the gRPC connection.
func (c *CVEDBClient) Close() error {
	return c.conn.Close()
}
