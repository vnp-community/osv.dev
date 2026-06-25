// Package pipeline — ai_enricher.go provides an AIEnricher that delegates
// vulnerability enrichment to ai-service via gRPC.
//
// The AIEnricher implements the existing pipeline.Enricher interface and is
// NON-DESTRUCTIVE — it only fills empty/missing fields, never overwrites.
//
// Activation: set AI_ENRICHER_ADDR=<ai-service-host:port>
// Example: AI_ENRICHER_ADDR=localhost:50052
//
// This file is ADDITIVE — existing enrichers in registry/registry.go are NOT modified.
package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	aiv1 "github.com/osv/shared/proto/gen/go/ai/v1"
	"github.com/ossf/osv-schema/bindings/go/osvschema"
)

// Compile-time check that AIEnricher implements Enricher.
var _ Enricher = (*AIEnricher)(nil)

// AIEnricher enriches a vulnerability using ai-service gRPC.
// Fills empty Summary and Details fields from AI-generated content.
// Non-destructive: never overwrites existing data.
type AIEnricher struct {
	addr   string
	conn   *grpc.ClientConn
	client aiv1.AIEnrichmentServiceClient
}

// NewAIEnricher creates an AIEnricher connected to ai-service.
// addr example: "localhost:50052" or "ai-service:50052"
func NewAIEnricher(addr string) (*AIEnricher, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("ai enricher: connect %s: %w", addr, err)
	}
	return &AIEnricher{
		addr:   addr,
		conn:   conn,
		client: aiv1.NewAIEnrichmentServiceClient(conn),
	}, nil
}

// Enrich implements the Enricher interface.
// Calls ai-service.EnrichCVE and fills empty Summary/Details fields.
// Non-fatal: if ai-service is unreachable, logs warning and continues the pipeline.
func (e *AIEnricher) Enrich(ctx context.Context, vuln *osvschema.Vulnerability, params *EnrichParams) error {
	if vuln == nil || vuln.GetId() == "" {
		return nil
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := e.client.EnrichCVE(timeoutCtx, &aiv1.EnrichCVERequest{
		CveId: vuln.GetId(),
	})
	if err != nil {
		// Non-fatal: AI enrichment failure must not block the main pipeline.
		slog.WarnContext(ctx, "ai enrichment skipped (non-fatal)",
			slog.String("id", vuln.GetId()),
			slog.String("addr", e.addr),
			slog.Any("error", err),
		)
		return nil
	}

	// Non-destructive: only fill empty fields.
	if vuln.Summary == "" && resp.GetSummaryShort() != "" {
		vuln.Summary = resp.GetSummaryShort()
	}
	if vuln.Details == "" && resp.GetSummaryLong() != "" {
		vuln.Details = resp.GetSummaryLong()
	}

	return nil
}

// Close releases the gRPC connection. Call when the enricher is no longer needed.
func (e *AIEnricher) Close() error {
	if e.conn != nil {
		return e.conn.Close()
	}
	return nil
}
