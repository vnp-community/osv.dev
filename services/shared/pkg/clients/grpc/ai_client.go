package grpc

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	aiv1 "github.com/osv/shared/proto/gen/go/ai/v1"
)

// AIClient wraps the AIEnrichmentService gRPC client (ai-service).
// Used by: apps/cli (worker pipeline AIEnricher, cmd/enrich), apps/osv (gateway BFF)
type AIClient struct {
	conn *grpc.ClientConn
	ai   aiv1.AIEnrichmentServiceClient
}

// NewAIClient connects to ai-service at the given address.
// Example: NewAIClient("localhost:50052") or NewAIClient("ai-service:50052")
func NewAIClient(addr string) (*AIClient, error) {
	conn, err := DialTimeout(addr)
	if err != nil {
		return nil, fmt.Errorf("ai client: %w", err)
	}
	return &AIClient{
		conn: conn,
		ai:   aiv1.NewAIEnrichmentServiceClient(conn),
	}, nil
}

// EnrichCVE triggers AI enrichment for a single CVE.
// Used by: apps/cli/cmd/enrich, worker pipeline AIEnricher
func (c *AIClient) EnrichCVE(ctx context.Context, cveID string) (*aiv1.EnrichCVEResponse, error) {
	resp, err := c.ai.EnrichCVE(ctx, &aiv1.EnrichCVERequest{CveId: cveID})
	if err != nil {
		return nil, fmt.Errorf("EnrichCVE(%s): %w", cveID, err)
	}
	return resp, nil
}

// GetEPSS returns the EPSS probability score for the given CVE.
func (c *AIClient) GetEPSS(ctx context.Context, cveID string) (*aiv1.EPSSResponse, error) {
	resp, err := c.ai.GetEPSS(ctx, &aiv1.GetEPSSRequest{CveId: cveID})
	if err != nil {
		return nil, fmt.Errorf("GetEPSS(%s): %w", cveID, err)
	}
	return resp, nil
}

// BatchEnrich triggers enrichment for multiple CVEs in one call.
// Used by: apps/cli/cmd/enrich --batch
func (c *AIClient) BatchEnrich(ctx context.Context, cveIDs []string) (*aiv1.BatchEnrichResponse, error) {
	resp, err := c.ai.BatchEnrich(ctx, &aiv1.BatchEnrichRequest{CveIds: cveIDs})
	if err != nil {
		return nil, fmt.Errorf("BatchEnrich(%d CVEs): %w", len(cveIDs), err)
	}
	return resp, nil
}

// Raw returns the underlying gRPC stub for advanced usage.
func (c *AIClient) Raw() aiv1.AIEnrichmentServiceClient { return c.ai }

// Close releases the underlying gRPC connection.
func (c *AIClient) Close() error { return c.conn.Close() }
