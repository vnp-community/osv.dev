// Package client provides gRPC client factories for the gateway-service BFF.
package client

import (
	"github.com/osv/shared/pkg/grpcutil"
	"github.com/rs/zerolog"
)

// VulnerabilityQueryClient wraps the gRPC connection to the vulnerability-query service.
type VulnerabilityQueryClient struct {
	*grpcutil.BaseClient
}

// ScanServiceClient wraps the gRPC connection to the scan-service.
type ScanServiceClient struct {
	*grpcutil.BaseClient
}

// SearchServiceClient wraps the gRPC connection to the search-service.
type SearchServiceClient struct {
	*grpcutil.BaseClient
}

// NewVulnerabilityQueryClient creates a resilient gRPC client to the vulnerability-query service.
func NewVulnerabilityQueryClient(addr string, log zerolog.Logger) (*VulnerabilityQueryClient, error) {
	base, err := grpcutil.NewBaseClient(grpcutil.ConnectionConfig{
		Target: addr,
	}, log)
	if err != nil {
		return nil, err
	}
	return &VulnerabilityQueryClient{BaseClient: base}, nil
}

// NewScanServiceClient creates a resilient gRPC client to the scan-service.
func NewScanServiceClient(addr string, log zerolog.Logger) (*ScanServiceClient, error) {
	base, err := grpcutil.NewBaseClient(grpcutil.ConnectionConfig{
		Target: addr,
	}, log)
	if err != nil {
		return nil, err
	}
	return &ScanServiceClient{BaseClient: base}, nil
}

// NewSearchServiceClient creates a resilient gRPC client to the search-service.
func NewSearchServiceClient(addr string, log zerolog.Logger) (*SearchServiceClient, error) {
	base, err := grpcutil.NewBaseClient(grpcutil.ConnectionConfig{
		Target: addr,
	}, log)
	if err != nil {
		return nil, err
	}
	return &SearchServiceClient{BaseClient: base}, nil
}
