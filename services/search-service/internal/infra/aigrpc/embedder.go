// Package aigrpc provides an AIEmbedder implementation that calls the ai-service
// via gRPC GenerateEmbedding RPC.
package aigrpc

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	aiv1 "github.com/osv/shared/proto/gen/go/ai/v1"
)

// Embedder calls ai-service GenerateEmbedding via gRPC.
// Implements pgvector.AIEmbedder interface.
type Embedder struct {
	client aiv1.AIEnrichmentServiceClient
	conn   *grpc.ClientConn
}

// New dials the ai-service at target and returns an Embedder.
// target format: "host:port" (e.g. "ai-service:50053" or "localhost:50053").
// Returns an error if the gRPC dial fails (non-blocking dial).
func New(target string) (*Embedder, error) {
	conn, err := grpc.NewClient(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("ai-service gRPC dial %q: %w", target, err)
	}
	return &Embedder{
		client: aiv1.NewAIEnrichmentServiceClient(conn),
		conn:   conn,
	}, nil
}

// Close releases the underlying gRPC connection.
func (e *Embedder) Close() error {
	return e.conn.Close()
}

// Embed generates a vector embedding for the given text using ai-service.
// Satisfies the pgvector.AIEmbedder interface.
func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	resp, err := e.client.GenerateEmbedding(ctx, &aiv1.GenerateEmbeddingRequest{
		Text: text,
	})
	if err != nil {
		return nil, fmt.Errorf("ai-service GenerateEmbedding: %w", err)
	}
	values := resp.GetValues()
	if len(values) == 0 {
		return nil, fmt.Errorf("ai-service returned empty embedding for text (len=%d)", len(text))
	}
	return values, nil
}
