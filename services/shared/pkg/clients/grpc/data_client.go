package grpc

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	cvedbv1 "github.com/osv/shared/proto/gen/go/cvedb/v1"
)

// DataClient wraps the CVEDBService gRPC client (data-service).
// Used by: apps/cli (query, relations commands), apps/osv (gateway proxy)
type DataClient struct {
	conn  *grpc.ClientConn
	cvedb cvedbv1.CVEDBServiceClient
}

// NewDataClient connects to data-service at the given address.
// Example: NewDataClient("localhost:50053") or NewDataClient("data-service:50053")
func NewDataClient(addr string) (*DataClient, error) {
	conn, err := DialTimeout(addr)
	if err != nil {
		return nil, fmt.Errorf("data client: %w", err)
	}
	return &DataClient{
		conn:  conn,
		cvedb: cvedbv1.NewCVEDBServiceClient(conn),
	}, nil
}

// LookupByPackage queries CVEs affecting the given package + version.
// Maps to OSV v1 POST /query with package+version.
func (c *DataClient) LookupByPackage(ctx context.Context, pkg, version string) ([]*cvedbv1.CVEData, error) {
	resp, err := c.cvedb.LookupCVEs(ctx, &cvedbv1.LookupCVEsRequest{
		Products: []*cvedbv1.ProductInfo{
			{Product: pkg, Version: version},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("LookupCVEs(%s@%s): %w", pkg, version, err)
	}
	return flattenCVEs(resp), nil
}

// LookupByPURL queries CVEs via Package URL.
func (c *DataClient) LookupByPURL(ctx context.Context, purl string) ([]*cvedbv1.CVEData, error) {
	resp, err := c.cvedb.LookupCVEs(ctx, &cvedbv1.LookupCVEsRequest{
		Products: []*cvedbv1.ProductInfo{
			{Purl: purl},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("LookupCVEs(purl=%s): %w", purl, err)
	}
	return flattenCVEs(resp), nil
}

// Raw returns the underlying gRPC stub for advanced usage.
func (c *DataClient) Raw() cvedbv1.CVEDBServiceClient { return c.cvedb }

// Close releases the underlying gRPC connection.
func (c *DataClient) Close() error { return c.conn.Close() }

func flattenCVEs(resp *cvedbv1.LookupCVEsResponse) []*cvedbv1.CVEData {
	var out []*cvedbv1.CVEData
	for _, batch := range resp.GetResults() {
		out = append(out, batch.GetCves()...)
	}
	return out
}
