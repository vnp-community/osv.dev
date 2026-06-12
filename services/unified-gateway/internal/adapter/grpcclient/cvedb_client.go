// Package grpcclient - CVEDBClient adapter.
package grpcclient

import (
	"context"
	"fmt"
	"io"

	"google.golang.org/grpc"

	pb "github.com/osv/proto/gen/go/cvedb/v1"
)

// LookupRequest wraps LookupCVEsRequest parameters.
type LookupRequest struct {
	Products         []*pb.ProductInfo
	MinScore         float32
	CheckExploits    bool
	CheckMetrics     bool
	DisabledSources  []string
	TriageData       map[string]*pb.TriageEntry
	FilterTriage     bool
}

// LookupResult wraps LookupCVEsResponse.
type LookupResult struct {
	Results         []*pb.ProductCVEs
	TotalCVEs       int32
	ProductsScanned int32
}

// DBStatus wraps GetDBStatusResponse.
type DBStatus struct {
	TotalCVEs   int64
	LastUpdated string
	Version     string
	Sources     []string
}

// CVEDBClient wraps the proto CVEDBServiceClient.
type CVEDBClient struct {
	conn   *grpc.ClientConn
	client pb.CVEDBServiceClient
}

// NewCVEDBClient creates a new CVEDBClient.
func NewCVEDBClient(addr string, opts ...grpc.DialOption) (*CVEDBClient, error) {
	all := append(defaultDialOptions(), opts...)
	conn, err := grpc.NewClient(addr, all...)
	if err != nil {
		return nil, fmt.Errorf("cvedb client: %w", err)
	}
	return &CVEDBClient{conn: conn, client: pb.NewCVEDBServiceClient(conn)}, nil
}

// Close closes the connection.
func (c *CVEDBClient) Close() error { return c.conn.Close() }

// LookupCVEs finds CVEs for a list of products.
func (c *CVEDBClient) LookupCVEs(ctx context.Context, req LookupRequest) (*LookupResult, error) {
	resp, err := c.client.LookupCVEs(ctx, &pb.LookupCVEsRequest{
		Products:        req.Products,
		MinScore:        req.MinScore,
		CheckExploits:   req.CheckExploits,
		CheckMetrics:    req.CheckMetrics,
		DisabledSources: req.DisabledSources,
		TriageData:      req.TriageData,
		FilterTriage:    req.FilterTriage,
	})
	if err != nil {
		return nil, fmt.Errorf("LookupCVEs: %w", err)
	}
	return &LookupResult{
		Results:         resp.GetResults(),
		TotalCVEs:       resp.GetTotalCves(),
		ProductsScanned: resp.GetProductsScanned(),
	}, nil
}

// InitDB initialises or upgrades the database schema.
func (c *CVEDBClient) InitDB(ctx context.Context, forceRebuild bool) error {
	_, err := c.client.InitDB(ctx, &pb.InitDBRequest{ForceRebuild: forceRebuild})
	return err
}

// GetDBStatus returns database metadata.
func (c *CVEDBClient) GetDBStatus(ctx context.Context) (*DBStatus, error) {
	resp, err := c.client.GetDBStatus(ctx, &pb.GetDBStatusRequest{})
	if err != nil {
		return nil, fmt.Errorf("GetDBStatus: %w", err)
	}
	return &DBStatus{
		TotalCVEs:   resp.GetCveCount(),
		LastUpdated: resp.GetLastUpdated(),
		Version:     resp.GetSchemaVersion(),
	}, nil
}

// ImportDB streams a database file to the CVEDB service.
func (c *CVEDBClient) ImportDB(ctx context.Context, reader io.Reader, verifyPGP bool, pgpSig []byte) error {
	stream, err := c.client.ImportDB(ctx)
	if err != nil {
		return fmt.Errorf("ImportDB stream: %w", err)
	}

	buf := make([]byte, 64*1024) // 64KB chunks
	for {
		n, rerr := reader.Read(buf)
		if n > 0 {
			if err := stream.Send(&pb.ImportDBChunk{
				Data:         buf[:n],
				VerifyPgp:    verifyPGP,
				PgpSignature: pgpSig,
			}); err != nil {
				return fmt.Errorf("ImportDB send: %w", err)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return fmt.Errorf("ImportDB read: %w", rerr)
		}
	}

	if _, err := stream.CloseAndRecv(); err != nil {
		return fmt.Errorf("ImportDB close: %w", err)
	}
	return nil
}

// ExportDB streams the exported database to the writer.
func (c *CVEDBClient) ExportDB(ctx context.Context, format string, year int, signPGP bool, writer io.Writer) error {
	stream, err := c.client.ExportDB(ctx, &pb.ExportDBRequest{
		Format:  format,
		Year:    int32(year),
		SignPgp: signPGP,
	})
	if err != nil {
		return fmt.Errorf("ExportDB stream: %w", err)
	}

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("ExportDB recv: %w", err)
		}
		if _, err := writer.Write(chunk.GetData()); err != nil {
			return fmt.Errorf("ExportDB write: %w", err)
		}
	}
	return nil
}
