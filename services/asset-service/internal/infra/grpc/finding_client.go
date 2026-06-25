package grpc

import (
	"context"
	"fmt"

	findingpb "github.com/osv/shared/proto/gen/go/finding/v1"
	ucasset "github.com/google/osv.dev/services/asset-service/internal/usecase/asset"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// FindingGRPCClient wraps the gRPC client and adapts it to FindingClient interface.
type FindingGRPCClient struct {
	client findingpb.FindingServiceClient
}

// Ensure it implements the interface
var _ ucasset.FindingClient = (*FindingGRPCClient)(nil)

// NewFindingClient creates a gRPC FindingClient (connection is lazy in grpc.NewClient).
func NewFindingClient(target string) (*FindingGRPCClient, error) {
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to dial finding service: %w", err)
	}
	return &FindingGRPCClient{
		client: findingpb.NewFindingServiceClient(conn),
	}, nil
}

// CountBySeverity calls GetSeverityCounts using ProductId as a lookup key.
func (c *FindingGRPCClient) CountBySeverity(ctx context.Context, assetIP string) (map[string]int, error) {
	resp, err := c.client.GetSeverityCounts(ctx, &findingpb.GetSeverityCountsRequest{
		ProductId:  assetIP,
		ActiveOnly: true,
	})
	if err != nil {
		return map[string]int{}, nil // gracefully return empty on failure
	}

	return map[string]int{
		"Critical": int(resp.GetCritical()),
		"High":     int(resp.GetHigh()),
		"Medium":   int(resp.GetMedium()),
		"Low":      int(resp.GetLow()),
		"Info":     int(resp.GetInfo()),
	}, nil
}

// NoopFindingClient is a no-op that always returns empty counts.
// Used when NATS/gRPC connection to finding-service is unavailable.
type NoopFindingClient struct{}

var _ ucasset.FindingClient = (*NoopFindingClient)(nil)

func (n *NoopFindingClient) CountBySeverity(_ context.Context, _ string) (map[string]int, error) {
	return map[string]int{}, nil
}
