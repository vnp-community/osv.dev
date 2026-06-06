// Package handler implements the gRPC ScannerService handler.
package handler

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/osv/scanner/internal/domain/service"
	"github.com/osv/scanner/internal/infrastructure/extractor"
	"github.com/osv/scanner/internal/parsers"
	"github.com/osv/scanner/internal/usecase/mergereport"
	"github.com/osv/scanner/internal/usecase/scanfile"
	"github.com/osv/scanner/internal/usecase/scanpackagelist"

	pb "github.com/osv/proto/gen/go/scanner/v1"
)

// ScannerHandler implements pb.ScannerServiceServer.
type ScannerHandler struct {
	pb.UnimplementedScannerServiceServer

	scanFileUC    *scanfile.UseCase
	scanPkgListUC *scanpackagelist.UseCase
	mergeUC       *mergereport.UseCase
	checkerSvc    *service.CheckerService
	log           zerolog.Logger
}

// New creates a ScannerHandler with all dependencies wired.
func New(checkerSvc *service.CheckerService, log zerolog.Logger) *ScannerHandler {
	// Build extractor list
	exts := []extractor.Extractor{
		&extractor.TarExtractor{},
		&extractor.ZipExtractor{},
		&extractor.GzipExtractor{},
		&extractor.Bzip2Extractor{},
		&extractor.XzExtractor{},
		&extractor.ZstdExtractor{},
		&extractor.RPMExtractor{},
		&extractor.DebExtractor{},
	}

	return &ScannerHandler{
		scanFileUC:    scanfile.NewUseCase(checkerSvc, exts, parsers.All()),
		scanPkgListUC: scanpackagelist.NewUseCase(),
		mergeUC:       mergereport.NewUseCase(),
		checkerSvc:    checkerSvc,
		log:           log,
	}
}

// ScanFile scans a binary file or directory for software components.
func (h *ScannerHandler) ScanFile(ctx context.Context, req *pb.ScanFileRequest) (*pb.ScanFileResponse, error) {
	if len(req.GetFileBytes()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "file_bytes is required")
	}
	if req.GetFileName() == "" {
		return nil, status.Error(codes.InvalidArgument, "file_name is required")
	}

	out, err := h.scanFileUC.Execute(ctx, scanfile.Input{
		FileBytes:    req.GetFileBytes(),
		FileName:     req.GetFileName(),
		Extract:      req.GetExtract(),
		SkipCheckers: req.GetSkipCheckers(),
		RunCheckers:  req.GetRunCheckers(),
	})
	if err != nil {
		h.log.Error().Err(err).Str("file", req.GetFileName()).Msg("ScanFile failed")
		return nil, status.Errorf(codes.Internal, "scan failed: %v", err)
	}

	results := make([]*pb.ScanResult, 0, len(out.Results))
	for _, r := range out.Results {
		results = append(results, &pb.ScanResult{
			Vendor:   r.Vendor,
			Product:  r.Product,
			Version:  r.Version,
			Purl:     r.PURL,
			FilePath: r.FilePath,
		})
	}

	h.log.Info().
		Str("file", req.GetFileName()).
		Int("results", len(results)).
		Int("scanned_files", out.ScannedFiles).
		Msg("ScanFile completed")

	return &pb.ScanFileResponse{
		Results:      results,
		ScannedFiles: int32(out.ScannedFiles),
	}, nil
}

// ScanPackageList parses a package list file (go.mod, requirements.txt, etc.)
func (h *ScannerHandler) ScanPackageList(ctx context.Context, req *pb.ScanPackageListRequest) (*pb.ScanPackageListResponse, error) {
	if len(req.GetFileBytes()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "file_bytes is required")
	}
	if req.GetFileName() == "" {
		return nil, status.Error(codes.InvalidArgument, "file_name is required")
	}

	out, err := h.scanPkgListUC.Execute(ctx, scanpackagelist.Input{
		FileBytes: req.GetFileBytes(),
		FileName:  req.GetFileName(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "scan package list failed: %v", err)
	}

	products := make([]*pb.ScanResult, 0, len(out.Products))
	for _, p := range out.Products {
		products = append(products, &pb.ScanResult{
			Vendor:   p.Vendor,
			Product:  p.Product,
			Version:  p.Version,
			Purl:     p.PURL,
			FilePath: p.FilePath,
		})
	}

	return &pb.ScanPackageListResponse{
		Products:     products,
		DetectedType: out.DetectedType,
	}, nil
}

// ScanSBOM is a placeholder that delegates to SBOM service (not implemented here).
func (h *ScannerHandler) ScanSBOM(_ context.Context, req *pb.ScanSBOMRequest) (*pb.ScanSBOMResponse, error) {
	// SBOM scanning delegates to sbomvex-service; scanner service only handles binary/manifest files
	return nil, status.Error(codes.Unimplemented, "ScanSBOM: delegate to sbomvex-service via api-gateway")
}

// MergeReports merges multiple intermediate JSON scan reports.
func (h *ScannerHandler) MergeReports(_ context.Context, req *pb.MergeReportsRequest) (*pb.MergeReportsResponse, error) {
	if len(req.GetReports()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "reports is required")
	}

	out, err := h.mergeUC.Execute(mergereport.Input{
		Reports: req.GetReports(),
	})
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "merge failed: %v", err)
	}

	merged, err := out.ToJSON()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "serialize: %v", err)
	}

	return &pb.MergeReportsResponse{
		MergedReport: merged,
	}, nil
}

// ListCheckers returns all registered checkers.
func (h *ScannerHandler) ListCheckers(_ context.Context, req *pb.ListCheckersRequest) (*pb.ListCheckersResponse, error) {
	allCheckers := h.checkerSvc.Checkers()

	filter := req.GetFilter()
	var infos []*pb.CheckerInfo

	for _, c := range allCheckers {
		if filter != "" && c.Name() != filter {
			continue
		}

		vendors := make([]string, 0, len(c.VendorProducts()))
		products := make([]string, 0, len(c.VendorProducts()))
		for _, vp := range c.VendorProducts() {
			vendors = append(vendors, vp.Vendor)
			products = append(products, vp.Product)
		}

		infos = append(infos, &pb.CheckerInfo{
			Name:     c.Name(),
			Vendors:  vendors,
			Products: products,
		})
	}

	return &pb.ListCheckersResponse{
		Checkers: infos,
	}, nil
}

// Ensure compile-time interface check.
var _ pb.ScannerServiceServer = (*ScannerHandler)(nil)

// grpcStatusError wraps an error as a gRPC internal status error.
func grpcStatusError(err error) error {
	return status.Errorf(codes.Internal, "%v", err)
}

// ensure fmt is used
var _ = fmt.Sprintf
