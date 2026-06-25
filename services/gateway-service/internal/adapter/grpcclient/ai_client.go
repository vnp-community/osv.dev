// Package grpcclient — ai_client.go
// AIClient wraps the ai-service gRPC AIEnrichmentServiceClient.
// Used by gateway BFF to fetch enrichment data and EPSS scores for CVE detail pages.
package grpcclient

import (
	"context"
	"time"

	"google.golang.org/grpc"

	aiv1 "github.com/osv/shared/proto/gen/go/ai/v1"
)

// AIClient wraps the ai-service gRPC AIEnrichmentServiceClient.
type AIClient struct {
	conn   *grpc.ClientConn
	client aiv1.AIEnrichmentServiceClient
}

// NewAIClient creates a new AIClient connected to addr.
func NewAIClient(addr string, opts ...grpc.DialOption) (*AIClient, error) {
	all := append(defaultDialOptions(), opts...)
	conn, err := grpc.NewClient(addr, all...)
	if err != nil {
		return nil, err
	}
	return &AIClient{
		conn:   conn,
		client: aiv1.NewAIEnrichmentServiceClient(conn),
	}, nil
}

// Close tears down the underlying gRPC connection.
func (c *AIClient) Close() error { return c.conn.Close() }

// GetEnrichment retrieves the AI enrichment result for a CVE ID.
// Returns nil, nil if the CVE has not been enriched yet.
func (c *AIClient) GetEnrichment(ctx context.Context, cveID string) (*aiv1.EnrichCVEResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return c.client.GetEnrichment(ctx, &aiv1.GetEnrichmentRequest{CveId: cveID})
}

// GetEPSS retrieves the EPSS exploitation probability score for a CVE ID.
func (c *AIClient) GetEPSS(ctx context.Context, cveID string) (*aiv1.EPSSResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return c.client.GetEPSS(ctx, &aiv1.GetEPSSRequest{CveId: cveID})
}

// TriageFinding requests AI triage for a finding.
func (c *AIClient) TriageFinding(ctx context.Context, findingID, cveID string) (*aiv1.TriageResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return c.client.TriageFinding(ctx, &aiv1.TriageFindingRequest{
		FindingId: findingID,
		CveId:     cveID,
	})
}
