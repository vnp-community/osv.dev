// Package client provides circuit-breaker-backed gRPC clients for web-bff.
// All clients use pkg/grpcutil.BaseClient for resilience and observability.
package client

import (
	"time"

	"github.com/rs/zerolog"

	"github.com/osv/pkg/grpcutil"
	"github.com/osv/pkg/resilience"
)

// NewVulnerabilityQueryClient creates a production gRPC client to the Vulnerability Query service.
// Uses circuit breaker (3 failures → 2min open) and 10s per-call timeout.
func NewVulnerabilityQueryClient(addr string, log zerolog.Logger) (*grpcutil.BaseClient, error) {
	cbConfig := resilience.CircuitBreakerConfig{
		MaxFailures:         3,
		Timeout:             2 * time.Minute,
		HalfOpenMaxRequests: 1,
	}
	return grpcutil.NewBaseClient(addr, "vulnerability-query", cbConfig, log)
}

// NewSearchServiceClient creates a production gRPC client to the Search service.
func NewSearchServiceClient(addr string, log zerolog.Logger) (*grpcutil.BaseClient, error) {
	cbConfig := resilience.CircuitBreakerConfig{
		MaxFailures:         3,
		Timeout:             2 * time.Minute,
		HalfOpenMaxRequests: 1,
	}
	return grpcutil.NewBaseClient(addr, "search", cbConfig, log)
}

// NewAIEnrichmentClient creates a production gRPC client to the AI Enrichment service.
// Uses a more lenient circuit breaker (5 failures) since AI calls can be slow.
func NewAIEnrichmentClient(addr string, log zerolog.Logger) (*grpcutil.BaseClient, error) {
	cbConfig := resilience.CircuitBreakerConfig{
		MaxFailures:         5,
		Timeout:             5 * time.Minute,
		HalfOpenMaxRequests: 1,
	}
	return grpcutil.NewBaseClient(addr, "ai-enrichment", cbConfig, log)
}
