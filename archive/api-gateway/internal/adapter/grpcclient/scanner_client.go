// Package grpcclient provides gRPC client adapters for all downstream services.
package grpcclient

import (
	"context"
	"fmt"
	"io"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/osv/proto/gen/go/scanner/v1"
)

const maxMsgSize = 100 << 20 // 100MB

// defaultDialOptions returns the standard gRPC dial options.
func defaultDialOptions() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallSendMsgSize(maxMsgSize),
			grpc.MaxCallRecvMsgSize(maxMsgSize),
		),
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// ScannerClient
// ─────────────────────────────────────────────────────────────────────────────

// ScanOptions configures a ScanFile request.
type ScanOptions struct {
	Extract      bool
	SkipCheckers []string
	RunCheckers  []string
}

// ScanResult is the Go-friendly wrapper for a single detected component.
type ScanResult struct {
	Vendor   string
	Product  string
	Version  string
	PURL     string
	FilePath string
}

// CheckerInfo describes an available checker.
type CheckerInfo struct {
	Name     string
	Vendors  []string
	Products []string
}

// ScannerClient wraps the proto ScannerServiceClient.
type ScannerClient struct {
	conn   *grpc.ClientConn
	client pb.ScannerServiceClient
}

// NewScannerClient creates a new ScannerClient connected to addr.
func NewScannerClient(addr string, opts ...grpc.DialOption) (*ScannerClient, error) {
	all := append(defaultDialOptions(), opts...)
	conn, err := grpc.NewClient(addr, all...)
	if err != nil {
		return nil, fmt.Errorf("scanner client: %w", err)
	}
	return &ScannerClient{conn: conn, client: pb.NewScannerServiceClient(conn)}, nil
}

// Close closes the underlying connection.
func (c *ScannerClient) Close() error { return c.conn.Close() }

// ScanFile scans a binary file and returns detected components.
func (c *ScannerClient) ScanFile(ctx context.Context, fileBytes []byte, fileName string, opts ScanOptions) ([]ScanResult, error) {
	resp, err := c.client.ScanFile(ctx, &pb.ScanFileRequest{
		FileBytes:    fileBytes,
		FileName:     fileName,
		Extract:      opts.Extract,
		SkipCheckers: opts.SkipCheckers,
		RunCheckers:  opts.RunCheckers,
	})
	if err != nil {
		return nil, fmt.Errorf("ScanFile: %w", err)
	}
	results := make([]ScanResult, 0, len(resp.GetResults()))
	for _, r := range resp.GetResults() {
		results = append(results, ScanResult{
			Vendor: r.GetVendor(), Product: r.GetProduct(),
			Version: r.GetVersion(), PURL: r.GetPurl(), FilePath: r.GetFilePath(),
		})
	}
	return results, nil
}

// ScanPackageList parses a package manifest file.
func (c *ScannerClient) ScanPackageList(ctx context.Context, fileBytes []byte, fileName string) ([]ScanResult, string, error) {
	resp, err := c.client.ScanPackageList(ctx, &pb.ScanPackageListRequest{
		FileBytes: fileBytes,
		FileName:  fileName,
	})
	if err != nil {
		return nil, "", fmt.Errorf("ScanPackageList: %w", err)
	}
	results := make([]ScanResult, 0, len(resp.GetProducts()))
	for _, r := range resp.GetProducts() {
		results = append(results, ScanResult{
			Vendor: r.GetVendor(), Product: r.GetProduct(),
			Version: r.GetVersion(), PURL: r.GetPurl(), FilePath: r.GetFilePath(),
		})
	}
	return results, resp.GetDetectedType(), nil
}

// MergeReports merges multiple JSON scan reports.
func (c *ScannerClient) MergeReports(ctx context.Context, reports [][]byte) ([]byte, error) {
	resp, err := c.client.MergeReports(ctx, &pb.MergeReportsRequest{Reports: reports})
	if err != nil {
		return nil, fmt.Errorf("MergeReports: %w", err)
	}
	return resp.GetMergedReport(), nil
}

// ListCheckers returns all registered checkers (optionally filtered).
func (c *ScannerClient) ListCheckers(ctx context.Context, filter string) ([]CheckerInfo, error) {
	resp, err := c.client.ListCheckers(ctx, &pb.ListCheckersRequest{Filter: filter})
	if err != nil {
		return nil, fmt.Errorf("ListCheckers: %w", err)
	}
	infos := make([]CheckerInfo, 0, len(resp.GetCheckers()))
	for _, ci := range resp.GetCheckers() {
		infos = append(infos, CheckerInfo{
			Name: ci.GetName(), Vendors: ci.GetVendors(), Products: ci.GetProducts(),
		})
	}
	return infos, nil
}

// ensure io is used
var _ = io.EOF
