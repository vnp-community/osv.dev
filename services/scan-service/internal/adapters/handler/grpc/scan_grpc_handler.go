// Package grpc implements the scan gRPC server.
package grpc

import (
	"context"
	"fmt"
	"net"

	"github.com/google/uuid"
	"github.com/osv/scan-service/internal/domain/repository"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"github.com/rs/zerolog"

	// Generated proto
	scanpb "github.com/osv/shared/proto/gen/go/scan/v1"
)

// ScanGRPCServer implements scanpb.ScanServiceServer.
type ScanGRPCServer struct {
	scanpb.UnimplementedScanServiceServer
	scanRepo    repository.ScanRepository
	findingRepo repository.FindingRepository
	log         zerolog.Logger
}

func NewScanGRPCServer(
	scanRepo repository.ScanRepository,
	findingRepo repository.FindingRepository,
	log zerolog.Logger,
) *ScanGRPCServer {
	return &ScanGRPCServer{scanRepo: scanRepo, findingRepo: findingRepo, log: log}
}

// GetScanStatus returns current scan status and progress.
func (s *ScanGRPCServer) GetScanStatus(ctx context.Context, req *scanpb.GetScanStatusRequest) (*scanpb.GetScanStatusResponse, error) {
	scanID, err := uuid.Parse(req.GetScanId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid scan_id")
	}

	scan, err := s.scanRepo.FindByID(ctx, scanID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "scan not found: %v", err)
	}

	resp := &scanpb.GetScanStatusResponse{
		ScanId:   scan.ID.String(),
		Status:   string(scan.Status),
		Progress: int32(scan.Progress),
		ScanType: string(scan.ScanType),
	}
	if scan.StartedAt != nil {
		resp.StartedAt = timestamppb.New(*scan.StartedAt)
	}
	if scan.CompletedAt != nil {
		resp.CompletedAt = timestamppb.New(*scan.CompletedAt)
	}
	return resp, nil
}

// GetFindingsByScan returns all findings for a scan.
func (s *ScanGRPCServer) GetFindingsByScan(ctx context.Context, req *scanpb.GetFindingsByScanRequest) (*scanpb.GetFindingsByScanResponse, error) {
	scanID, err := uuid.Parse(req.GetScanId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid scan_id")
	}

	findings, _, err := s.findingRepo.FindByScanID(ctx, scanID, 1, 1000)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "query findings: %v", err)
	}

	var pbFindings []*scanpb.Finding
	for _, f := range findings {
		pbFindings = append(pbFindings, &scanpb.Finding{
			FindingId: f.ID.String(),
			ScanId:    f.ScanID.String(),
			IpAddress: f.IPAddress,
			Hostname:  f.Hostname,
			CveIds:    f.CVEIDs,
			Severity:  string(f.Severity),
		})
	}

	return &scanpb.GetFindingsByScanResponse{Findings: pbFindings}, nil
}

// StartGRPCServer starts the gRPC server on the given port.
func StartGRPCServer(srv *ScanGRPCServer, port string, log zerolog.Logger) error {
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	grpcSrv := grpc.NewServer()
	scanpb.RegisterScanServiceServer(grpcSrv, srv)
	log.Info().Str("port", port).Msg("scan gRPC server starting")
	return grpcSrv.Serve(lis)
}
