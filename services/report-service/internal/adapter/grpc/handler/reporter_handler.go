// Package handler implements the gRPC ReporterService handler.
package handler

import (
	"context"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/osv/report-service/internal/domain/entity"
	"github.com/osv/report-service/internal/usecase/generatereport"

	pb "github.com/osv/proto/gen/go/reporter/v1"
)

// ReporterHandler implements pb.ReporterServiceServer.
type ReporterHandler struct {
	pb.UnimplementedReporterServiceServer
	generateReportUC *generatereport.UseCase
	log              zerolog.Logger
}

// New creates a ReporterHandler.
func New(generateReportUC *generatereport.UseCase, log zerolog.Logger) *ReporterHandler {
	return &ReporterHandler{
		generateReportUC: generateReportUC,
		log:              log,
	}
}

// GenerateReport implements pb.ReporterServiceServer.
func (h *ReporterHandler) GenerateReport(ctx context.Context, req *pb.GenerateReportRequest) (*pb.GenerateReportResponse, error) {
	if len(req.GetCveData()) == 0 {
		return &pb.GenerateReportResponse{
			ExitCode:  0,
			TotalCves: 0,
			Reports:   map[string][]byte{},
		}, nil
	}

	// Convert proto → domain
	cveData := protoToEntityCVEData(req.GetCveData())

	// Parse requested formats
	formats := parseFormats(req.GetFormats())
	if len(formats) == 0 {
		formats = []entity.OutputFormat{entity.FormatJSON}
	}

	result, err := h.generateReportUC.Execute(ctx, generatereport.Input{
		CVEData:     cveData,
		Formats:     formats,
		ScanTarget:  req.GetScanTarget(),
		MinSeverity: req.GetMinSeverity(),
		MinScore:    float64(req.GetMinScore()),
		Theme:       req.GetTheme(),
		Quiet:       req.GetQuiet(),
	})
	if err != nil {
		h.log.Error().Err(err).Msg("GenerateReport failed")
		return nil, status.Errorf(codes.Internal, "generate report: %v", err)
	}

	// Convert domain → proto
	protoReports := make(map[string][]byte, len(result.Reports))
	for format, data := range result.Reports {
		protoReports[string(format)] = data
	}

	return &pb.GenerateReportResponse{
		Reports:   protoReports,
		ExitCode:  int32(result.ExitCode),
		TotalCves: int32(result.TotalCVEs),
	}, nil
}

// GetSupportedFormats returns the list of supported report formats.
func (h *ReporterHandler) GetSupportedFormats(_ context.Context, _ *pb.GetFormatsRequest) (*pb.GetFormatsResponse, error) {
	formats := []*pb.FormatInfo{
		{Name: "console", ContentType: "text/plain"},
		{Name: "csv",     ContentType: "text/csv"},
		{Name: "json",    ContentType: "application/json"},
		{Name: "json2",   ContentType: "application/json"},
		{Name: "html",    ContentType: "text/html"},
		{Name: "pdf",     ContentType: "application/pdf"},
	}
	return &pb.GetFormatsResponse{Formats: formats}, nil
}

// GenerateAvailableFix is a stub (requires OS package query infra).
func (h *ReporterHandler) GenerateAvailableFix(_ context.Context, _ *pb.AvailableFixRequest) (*pb.AvailableFixResponse, error) {
	return &pb.AvailableFixResponse{Fixes: []*pb.FixResult{}}, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Conversion helpers
// ─────────────────────────────────────────────────────────────────────────────

func protoToEntityCVEData(protoData []*pb.ProductCVEs) map[entity.ProductInfo][]entity.CVEData {
	result := make(map[entity.ProductInfo][]entity.CVEData, len(protoData))
	for _, pc := range protoData {
		if pc.GetProduct() == nil {
			continue
		}
		product := entity.ProductInfo{
			Vendor:  pc.GetProduct().GetVendor(),
			Product: pc.GetProduct().GetProduct(),
			Version: pc.GetProduct().GetVersion(),
			PURL:    pc.GetProduct().GetPurl(),
		}
		var cves []entity.CVEData
		for _, c := range pc.GetCves() {
			cves = append(cves, entity.CVEData{
				CVENumber:   c.GetCveNumber(),
				Severity:    c.GetSeverity(),
				Score:       float64(c.GetScore()),
				DataSource:  c.GetDataSource(),
				IsExploit:   c.GetIsExploit(),
				CVSSVector:  c.GetCvssVector(),
				CVSSVersion: int(c.GetCvssVersion()),
			})
		}
		result[product] = cves
	}
	return result
}

func parseFormats(strs []string) []entity.OutputFormat {
	formats := make([]entity.OutputFormat, 0, len(strs))
	for _, s := range strs {
		f := entity.OutputFormat(s)
		if f.IsValid() {
			formats = append(formats, f)
		}
	}
	return formats
}

// Ensure compile-time interface check.
var _ pb.ReporterServiceServer = (*ReporterHandler)(nil)
