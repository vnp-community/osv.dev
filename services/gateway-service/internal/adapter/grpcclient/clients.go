// Package grpcclient - DataSyncClient, ReporterClient, SBOMVEXClient adapters.
package grpcclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	dspb  "github.com/osv/shared/proto/gen/go/datasync/v1"
	rpb   "github.com/osv/shared/proto/gen/go/reporter/v1"
	sbpb  "github.com/osv/shared/proto/gen/go/sbomvex/v1"
)

// ─────────────────────────────────────────────────────────────────────────────
// DataSyncClient
// ─────────────────────────────────────────────────────────────────────────────

// SyncAllRequest configures a SyncAll call.
type SyncAllRequest struct {
	DisabledSources []string
	ForceUpdate     bool
	NVDMode         string
	NVDAPIKey       string
	Mirror          string
}

// SyncAllResult wraps SyncAllResponse.
type SyncAllResult struct {
	SourcesUpdated []string
	SourcesFailed  []string
	TotalCVEs      int32
	Duration       string
}

// SyncSourceResult wraps SyncSourceResponse.
type SyncSourceResult struct {
	SourceName  string
	TotalCVEs   int32
	Duration    string
}

// SyncStatus wraps GetSyncStatusResponse.
type SyncStatus struct {
	LastSyncTime  string
	IsRunning     bool
	SourcesStatus map[string]string
}

// DataSyncClient wraps the proto DataSyncServiceClient.
type DataSyncClient struct {
	conn   *grpc.ClientConn
	client dspb.DataSyncServiceClient
}

// NewDataSyncClient creates a new DataSyncClient.
func NewDataSyncClient(addr string, opts ...grpc.DialOption) (*DataSyncClient, error) {
	all := append(defaultDialOptions(), opts...)
	conn, err := grpc.NewClient(addr, all...)
	if err != nil {
		return nil, fmt.Errorf("datasync client: %w", err)
	}
	return &DataSyncClient{conn: conn, client: dspb.NewDataSyncServiceClient(conn)}, nil
}

// Close closes the connection.
func (c *DataSyncClient) Close() error { return c.conn.Close() }

// SyncAll triggers sync for all enabled sources.
func (c *DataSyncClient) SyncAll(ctx context.Context, req SyncAllRequest) (*SyncAllResult, error) {
	resp, err := c.client.SyncAll(ctx, &dspb.SyncAllRequest{
		DisabledSources: req.DisabledSources,
		ForceUpdate:     req.ForceUpdate,
		NvdMode:         req.NVDMode,
		NvdApiKey:       req.NVDAPIKey,
		Mirror:          req.Mirror,
	})
	if err != nil {
		return nil, fmt.Errorf("SyncAll: %w", err)
	}
	return &SyncAllResult{
		SourcesUpdated: resp.GetSourcesUpdated(),
		SourcesFailed:  resp.GetSourcesFailed(),
		TotalCVEs:      resp.GetTotalCves(),
		Duration:       resp.GetDuration(),
	}, nil
}

// SyncSource triggers sync for a single named source.
func (c *DataSyncClient) SyncSource(ctx context.Context, source, nvdMode, nvdAPIKey string) (*SyncSourceResult, error) {
	resp, err := c.client.SyncSource(ctx, &dspb.SyncSourceRequest{
		Source:    source,
		NvdMode:   nvdMode,
		NvdApiKey: nvdAPIKey,
	})
	if err != nil {
		return nil, fmt.Errorf("SyncSource: %w", err)
	}
	return &SyncSourceResult{
		SourceName: resp.GetSource(),
		TotalCVEs:  0, // SyncSourceResponse only has Source and Success
	}, nil
}

// GetSyncStatus returns sync status for all sources.
func (c *DataSyncClient) GetSyncStatus(ctx context.Context) (*SyncStatus, error) {
	resp, err := c.client.GetSyncStatus(ctx, &dspb.GetSyncStatusRequest{})
	if err != nil {
		return nil, fmt.Errorf("GetSyncStatus: %w", err)
	}
	srcStatus := make(map[string]string, len(resp.GetSources()))
	for _, s := range resp.GetSources() {
		srcStatus[s.GetName()] = s.GetLastSync()
	}
	return &SyncStatus{
		LastSyncTime:  resp.GetLastSync(),
		IsRunning:     false, // not in proto
		SourcesStatus: srcStatus,
	}, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// ReporterClient
// ─────────────────────────────────────────────────────────────────────────────

// GenerateReportRequest configures a report generation.
type GenerateReportRequest struct {
	Products    []*rpb.ProductCVEs
	Formats     []string
	ScanTarget  string
	MinSeverity string
	MinScore    float32
	Theme       string
}

// GenerateReportResult wraps GenerateReportResponse.
type GenerateReportResult struct {
	Reports  map[string][]byte // format → bytes
	ExitCode int32
}

// FormatInfo describes a supported output format.
type FormatInfo struct {
	Name        string
	ContentType string
}

// ReporterClient wraps the proto ReporterServiceClient.
type ReporterClient struct {
	conn   *grpc.ClientConn
	client rpb.ReporterServiceClient
}

// NewReporterClient creates a new ReporterClient.
func NewReporterClient(addr string, opts ...grpc.DialOption) (*ReporterClient, error) {
	all := append(defaultDialOptions(), opts...)
	conn, err := grpc.NewClient(addr, all...)
	if err != nil {
		return nil, fmt.Errorf("reporter client: %w", err)
	}
	return &ReporterClient{conn: conn, client: rpb.NewReporterServiceClient(conn)}, nil
}

// Close closes the connection.
func (c *ReporterClient) Close() error { return c.conn.Close() }

// GenerateReport generates reports in one or more formats.
func (c *ReporterClient) GenerateReport(ctx context.Context, req GenerateReportRequest) (*GenerateReportResult, error) {
	resp, err := c.client.GenerateReport(ctx, &rpb.GenerateReportRequest{
		CveData:     req.Products,
		Formats:     req.Formats,
		ScanTarget:  req.ScanTarget,
		MinSeverity: req.MinSeverity,
		MinScore:    req.MinScore,
		Theme:       req.Theme,
	})
	if err != nil {
		return nil, fmt.Errorf("GenerateReport: %w", err)
	}
	return &GenerateReportResult{
		Reports:  resp.GetReports(),
		ExitCode: resp.GetExitCode(),
	}, nil
}

// GetSupportedFormats returns the list of supported report formats.
func (c *ReporterClient) GetSupportedFormats(ctx context.Context) ([]FormatInfo, error) {
	resp, err := c.client.GetSupportedFormats(ctx, &rpb.GetFormatsRequest{})
	if err != nil {
		return nil, fmt.Errorf("GetSupportedFormats: %w", err)
	}
	infos := make([]FormatInfo, 0, len(resp.GetFormats()))
	for _, f := range resp.GetFormats() {
		infos = append(infos, FormatInfo{Name: f.GetName(), ContentType: f.GetContentType()})
	}
	return infos, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// SBOMVEXClient
// ─────────────────────────────────────────────────────────────────────────────

// ParseSBOMResult wraps the parsed SBOM product list.
type ParseSBOMResult struct {
	Products []*sbpb.ProductInfo
	Format   string
}

// ParseVEXResult wraps triage data from a VEX file.
type ParseVEXResult struct {
	TriageData map[string]*sbpb.TriageEntry
}

// GenerateSBOMRequest configures SBOM generation.
type GenerateSBOMRequest struct {
	Products   []*sbpb.ProductInfo
	Format     string
	ScanTarget string
	ToolName   string
	ToolVersion string
}

// GenerateVEXRequest configures VEX generation.
type GenerateVEXRequest struct {
	Products  []*sbpb.ProductInfo
	CVEData   []*sbpb.ProductCVEs
	Format    string
}

// SBOMVEXClient wraps the proto SBOMVEXServiceClient.
type SBOMVEXClient struct {
	conn   *grpc.ClientConn
	client sbpb.SBOMVEXServiceClient
}

// NewSBOMVEXClient creates a new SBOMVEXClient.
func NewSBOMVEXClient(addr string, opts ...grpc.DialOption) (*SBOMVEXClient, error) {
	all := append(defaultDialOptions(), opts...)
	conn, err := grpc.NewClient(addr, all...)
	if err != nil {
		return nil, fmt.Errorf("sbomvex client: %w", err)
	}
	return &SBOMVEXClient{conn: conn, client: sbpb.NewSBOMVEXServiceClient(conn)}, nil
}

// Close closes the connection.
func (c *SBOMVEXClient) Close() error { return c.conn.Close() }

// ParseSBOM parses an SBOM file and extracts product/version info.
func (c *SBOMVEXClient) ParseSBOM(ctx context.Context, fileBytes []byte, fileName, format string) (*ParseSBOMResult, error) {
	resp, err := c.client.ParseSBOM(ctx, &sbpb.ParseSBOMRequest{
		FileBytes: fileBytes,
		FileName:  fileName,
		Format:    format,
	})
	if err != nil {
		return nil, fmt.Errorf("ParseSBOM: %w", err)
	}
	return &ParseSBOMResult{
		Products: resp.GetProducts(),
		Format:   resp.GetDetectedFormat(),
	}, nil
}

// GenerateSBOM generates an SBOM from a product list.
func (c *SBOMVEXClient) GenerateSBOM(ctx context.Context, req GenerateSBOMRequest) ([]byte, error) {
	resp, err := c.client.GenerateSBOM(ctx, &sbpb.GenerateSBOMRequest{
		Format: req.Format,
	})
	if err != nil {
		return nil, fmt.Errorf("GenerateSBOM: %w", err)
	}
	return resp.GetDocument(), nil
}

// ParseVEX parses a VEX file and returns triage data.
func (c *SBOMVEXClient) ParseVEX(ctx context.Context, fileBytes []byte, fileName, format string) (*ParseVEXResult, error) {
	resp, err := c.client.ParseVEX(ctx, &sbpb.ParseVEXRequest{
		FileBytes: fileBytes,
		FileName:  fileName,
		Format:    format,
	})
	if err != nil {
		return nil, fmt.Errorf("ParseVEX: %w", err)
	}
	return &ParseVEXResult{TriageData: resp.GetTriageData()}, nil
}

// GenerateVEX generates a VEX document.
func (c *SBOMVEXClient) GenerateVEX(ctx context.Context, req GenerateVEXRequest) ([]byte, error) {
	resp, err := c.client.GenerateVEX(ctx, &sbpb.GenerateVEXRequest{
		CveData: req.CVEData,
		Format:  req.Format,
	})
	if err != nil {
		return nil, fmt.Errorf("GenerateVEX: %w", err)
	}
	return resp.GetDocument(), nil
}

// DetectSBOMFormat detects the SBOM format of given bytes.
func (c *SBOMVEXClient) DetectSBOMFormat(ctx context.Context, fileBytes []byte) (string, error) {
	resp, err := c.client.DetectSBOMFormat(ctx, &sbpb.DetectFormatRequest{FileBytes: fileBytes})
	if err != nil {
		return "", fmt.Errorf("DetectSBOMFormat: %w", err)
	}
	return resp.GetFormat(), nil
}
