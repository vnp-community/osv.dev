// Package grpc provides the gRPC server implementation for finding-service.
package grpc

import (
	"context"
	"errors"
	"net"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	findingpb "github.com/osv/shared/proto/gen/go/finding/v1"

	"github.com/osv/finding-service/internal/domain/engagement"
	"github.com/osv/finding-service/internal/domain/finding"
	"github.com/osv/finding-service/internal/domain/repository"
	"github.com/osv/finding-service/internal/domain/sla"
	testp "github.com/osv/finding-service/internal/domain/test"
	"github.com/osv/finding-service/internal/infra/postgres"
	"github.com/osv/finding-service/internal/usecase/dedup"
)

// Server implements findingpb.FindingServiceServer.
type Server struct {
	findingpb.UnimplementedFindingServiceServer

	findingRepo    *postgres.FindingRepo
	engagementRepo repository.EngagementRepository
	testRepo       repository.TestRepository
	slaSvc         *sla.Service
	dedupSvc       *dedup.Service
	logger         zerolog.Logger
}

// New creates the gRPC server.
func New(
	findingRepo *postgres.FindingRepo,
	engagementRepo repository.EngagementRepository,
	testRepo repository.TestRepository,
	slaSvc *sla.Service,
	dedupSvc *dedup.Service,
	logger zerolog.Logger,
) *Server {
	return &Server{
		findingRepo:    findingRepo,
		engagementRepo: engagementRepo,
		testRepo:       testRepo,
		slaSvc:         slaSvc,
		dedupSvc:       dedupSvc,
		logger:         logger,
	}
}

// Serve starts the gRPC server on the given address.
func (s *Server) Serve(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	grpcSvr := grpc.NewServer(grpc.UnaryInterceptor(s.loggingInterceptor))
	findingpb.RegisterFindingServiceServer(grpcSvr, s)
	s.logger.Info().Str("addr", addr).Msg("finding-service gRPC server starting")
	return grpcSvr.Serve(lis)
}

// BatchCreateFindings creates multiple findings from a scan result.
func (s *Server) BatchCreateFindings(ctx context.Context, req *findingpb.BatchCreateFindingsRequest) (*findingpb.BatchCreateFindingsResponse, error) {
	if req.ProductId == "" || req.EngagementId == "" || req.TestId == "" {
		return nil, status.Error(codes.InvalidArgument, "product_id, engagement_id, and test_id are required")
	}

	findings := make([]*finding.Finding, 0, len(req.Findings))
	for _, fi := range req.Findings {
		f := finding.FromProto(fi, req.ProductId, req.EngagementId, req.TestId)
		findings = append(findings, f)
	}

	ids, err := s.findingRepo.BulkCreate(ctx, findings)
	if err != nil {
		s.logger.Error().Err(err).Msg("BatchCreateFindings: failed")
		return nil, status.Errorf(codes.Internal, "bulk create failed: %v", err)
	}

	return &findingpb.BatchCreateFindingsResponse{
		FindingIds: ids,
		Created:    int32(len(ids)),
	}, nil
}

// CloseOldFindings marks findings not in current scan as inactive.
// ExcludeFindingIds contains the IDs of findings from the current scan that should stay active.
func (s *Server) CloseOldFindings(ctx context.Context, req *findingpb.CloseOldFindingsRequest) (*findingpb.CloseOldFindingsResponse, error) {
	closed, err := s.findingRepo.CloseOldFindings(ctx, req.TestId, req.EngagementId, "", req.ExcludeFindingIds)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "close old findings: %v", err)
	}
	return &findingpb.CloseOldFindingsResponse{Closed: int32(closed)}, nil
}

// FindByHashCode retrieves a finding by its hash code within an engagement or product scope.
func (s *Server) FindByHashCode(ctx context.Context, req *findingpb.FindByHashCodeRequest) (*findingpb.FindByHashCodeResponse, error) {
	testID := parseUUID(req.TestId)
	var engID *uuid.UUID
	var productID *uuid.UUID
	if req.EngagementId != nil {
		id := parseUUID(*req.EngagementId)
		engID = &id
	}
	if req.ProductId != nil {
		id := parseUUID(*req.ProductId)
		productID = &id
	}

	f, err := s.findingRepo.FindByHashCode(ctx, req.HashCode, testID, req.DeduplicationOnEngagement, engID, productID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "find by hash: %v", err)
	}
	if f == nil {
		return &findingpb.FindByHashCodeResponse{}, nil
	}

	idStr := f.ID.String()
	statusStr := "active"
	if f.IsMitigated {
		statusStr = "mitigated"
	} else if f.FalsePositive {
		statusStr = "false_positive"
	}
	return &findingpb.FindByHashCodeResponse{
		FindingId: &idStr,
		Status:    &statusStr,
	}, nil
}

// ReactivateFindings reactivates previously closed findings by their IDs.
func (s *Server) ReactivateFindings(ctx context.Context, req *findingpb.ReactivateFindingsRequest) (*findingpb.ReactivateFindingsResponse, error) {
	if len(req.FindingIds) == 0 {
		return &findingpb.ReactivateFindingsResponse{Reactivated: 0}, nil
	}
	ids := make([]uuid.UUID, 0, len(req.FindingIds))
	for _, idStr := range req.FindingIds {
		if id, err := uuid.Parse(idStr); err == nil {
			ids = append(ids, id)
		}
	}
	if err := s.findingRepo.BulkReactivate(ctx, ids); err != nil {
		return nil, status.Errorf(codes.Internal, "reactivate: %v", err)
	}
	return &findingpb.ReactivateFindingsResponse{Reactivated: int32(len(ids))}, nil
}

// ApplyTags adds tags to findings.
func (s *Server) ApplyTags(ctx context.Context, req *findingpb.ApplyTagsRequest) (*findingpb.ApplyTagsResponse, error) {
	if _, err := s.findingRepo.ApplyTags(ctx, req.FindingIds, req.Tags); err != nil {
		return nil, status.Errorf(codes.Internal, "apply tags: %v", err)
	}
	return &findingpb.ApplyTagsResponse{}, nil
}

// BatchUpdateSLADates updates SLA expiration dates in bulk.
func (s *Server) BatchUpdateSLADates(ctx context.Context, req *findingpb.BatchUpdateSLADatesRequest) (*findingpb.BatchUpdateSLADatesResponse, error) {
	updates := make([]finding.SLADateUpdate, 0, len(req.Updates))
	for _, u := range req.Updates {
		fid, err := uuid.Parse(u.FindingId)
		if err != nil {
			continue
		}
		var expDate interface{}
		if u.ExpirationDate != nil {
			t := u.ExpirationDate.AsTime()
			expDate = t
		}
		updates = append(updates, finding.SLADateUpdate{
			FindingID:      fid,
			ExpirationDate: expDate,
		})
	}
	if err := s.findingRepo.BulkUpdateSLADates(ctx, updates); err != nil {
		return nil, status.Errorf(codes.Internal, "batch update SLA dates: %v", err)
	}
	return &findingpb.BatchUpdateSLADatesResponse{Updated: int32(len(updates))}, nil
}

// ListFindingsForSLACheck returns findings for SLA review.
func (s *Server) ListFindingsForSLACheck(ctx context.Context, req *findingpb.ListFindingsForSLACheckRequest) (*findingpb.ListFindingsForSLACheckResponse, error) {
	ids := make([]uuid.UUID, 0, len(req.FindingIds))
	for _, idStr := range req.FindingIds {
		if id, err := uuid.Parse(idStr); err == nil {
			ids = append(ids, id)
		}
	}
	findings, err := s.findingRepo.ListForSLACheck(ctx, ids, req.ActiveOnly, req.HasSlaDate)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list SLA check: %v", err)
	}

	protoFindings := make([]*findingpb.FindingForSLA, 0, len(findings))
	for _, f := range findings {
		pf := &findingpb.FindingForSLA{
			Id:       f.ID.String(),
			Severity: string(f.Severity),
			ProductId: f.ProductID.String(),
			Date:     timestamppb.New(f.Date),
		}
		if f.SLAExpirationDate != nil {
			pf.SlaExpirationDate = timestamppb.New(*f.SLAExpirationDate)
		}
		protoFindings = append(protoFindings, pf)
	}
	return &findingpb.ListFindingsForSLACheckResponse{Findings: protoFindings}, nil
}

// ListFindingsForReport is not exposed via gRPC (use HTTP export endpoint).
func (s *Server) ListFindingsForReport(req *findingpb.ListFindingsForReportRequest, stream grpc.ServerStreamingServer[findingpb.FindingProto]) error {
	return status.Error(codes.Unimplemented, "use HTTP /api/v2/findings/export")
}

// GetOrCreateEngagement finds or creates an engagement for a scan.
func (s *Server) GetOrCreateEngagement(ctx context.Context, req *findingpb.GetOrCreateEngagementRequest) (*findingpb.GetOrCreateEngagementResponse, error) {
	if req.ProductId == "" || req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "product_id and name are required")
	}

	productID := parseUUID(req.ProductId)

	existing, err := s.engagementRepo.FindByNameAndProduct(ctx, req.Name, productID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "find engagement: %v", err)
	}
	if existing != nil {
		return &findingpb.GetOrCreateEngagementResponse{
			EngagementId: existing.ID.String(),
			Created:      false,
		}, nil
	}

	engType := engagement.TypeInteractive
	if req.EngagementType != nil && *req.EngagementType == "CI/CD" {
		engType = engagement.TypeCICD
	}
	eng, engErr := engagement.New(productID, req.Name, engType)
	if engErr != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid engagement: %v", engErr)
	}
	if req.BuildId != nil {
		eng.BuildID = *req.BuildId
	}
	if req.BranchTag != nil {
		eng.BranchTag = *req.BranchTag
	}
	if req.CommitHash != nil {
		eng.CommitHash = *req.CommitHash
	}
	if req.SourceCodeManagementUri != nil {
		eng.SourceCodeManagementURI = *req.SourceCodeManagementUri
	}
	eng.DeduplicationOnEngagement = req.DeduplicationOnEngagement

	if err := s.engagementRepo.Create(ctx, eng); err != nil {
		return nil, status.Errorf(codes.Internal, "create engagement: %v", err)
	}

	return &findingpb.GetOrCreateEngagementResponse{
		EngagementId: eng.ID.String(),
		Created:      true,
	}, nil
}

// GetOrCreateTest finds or creates a test within an engagement.
func (s *Server) GetOrCreateTest(ctx context.Context, req *findingpb.GetOrCreateTestRequest) (*findingpb.GetOrCreateTestResponse, error) {
	if req.EngagementId == "" || req.ScanType == "" {
		return nil, status.Error(codes.InvalidArgument, "engagement_id and scan_type are required")
	}

	engID := parseUUID(req.EngagementId)

	existing, err := s.testRepo.FindByEngagementAndType(ctx, engID, req.ScanType)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "find test: %v", err)
	}
	if existing != nil {
		return &findingpb.GetOrCreateTestResponse{
			TestId:  existing.ID.String(),
			Created: false,
		}, nil
	}

	title := req.ScanType
	if req.Title != nil {
		title = *req.Title
	}
	t := testp.New(engID, req.ScanType, title)
	if req.Version != nil {
		t.Version = *req.Version
	}
	if req.BranchTag != nil {
		t.BranchTag = *req.BranchTag
	}
	if req.BuildId != nil {
		t.BuildID = *req.BuildId
	}
	if req.CommitHash != nil {
		t.CommitHash = *req.CommitHash
	}

	if err := s.testRepo.Create(ctx, t); err != nil {
		return nil, status.Errorf(codes.Internal, "create test: %v", err)
	}

	return &findingpb.GetOrCreateTestResponse{
		TestId:  t.ID.String(),
		Created: true,
	}, nil
}

// CheckProductPermission validates user permission for a product.
// In embedded/monolith mode, auth is handled at the gateway layer.
func (s *Server) CheckProductPermission(ctx context.Context, req *findingpb.CheckProductPermissionRequest) (*findingpb.CheckProductPermissionResponse, error) {
	return &findingpb.CheckProductPermissionResponse{Allowed: true}, nil
}

// ExistsFalsePositiveByHash checks if a hash was previously marked false-positive.
func (s *Server) ExistsFalsePositiveByHash(ctx context.Context, req *findingpb.ExistsFalsePositiveByHashRequest) (*findingpb.ExistsFalsePositiveByHashResponse, error) {
	var productID uuid.UUID
	if req.ProductId != "" {
		productID = parseUUID(req.ProductId)
	}
	exists, err := s.findingRepo.ExistsFalsePositiveByHash(ctx, req.HashCode, productID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "check false positive: %v", err)
	}
	return &findingpb.ExistsFalsePositiveByHashResponse{Exists: exists}, nil
}

// GetSeverityCounts returns finding counts grouped by severity for a product.
func (s *Server) GetSeverityCounts(ctx context.Context, req *findingpb.GetSeverityCountsRequest) (*findingpb.GetSeverityCountsResponse, error) {
	critical, high, medium, low, info, err := s.findingRepo.CountBySeverityForProduct(ctx, req.ProductId, req.ActiveOnly)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "count by severity: %v", err)
	}
	return &findingpb.GetSeverityCountsResponse{
		Critical: critical,
		High:     high,
		Medium:   medium,
		Low:      low,
		Info:     info,
	}, nil
}

// CountFindings returns total finding count for a product.
func (s *Server) CountFindings(ctx context.Context, req *findingpb.CountFindingsRequest) (*findingpb.CountFindingsResponse, error) {
	count, err := s.findingRepo.Count(ctx, req.ProductId, req.ActiveOnly)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "count findings: %v", err)
	}
	return &findingpb.CountFindingsResponse{Count: count}, nil
}

// parseUUID safely parses a UUID string, returning uuid.Nil on error.
func parseUUID(s string) uuid.UUID {
	id, _ := uuid.Parse(s)
	return id
}

// loggingInterceptor logs all gRPC calls.
func (s *Server) loggingInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	resp, err := handler(ctx, req)
	if err != nil {
		s.logger.Error().Err(err).Str("method", info.FullMethod).Msg("gRPC error")
	} else {
		s.logger.Debug().Str("method", info.FullMethod).Msg("gRPC call")
	}
	if errors.Is(err, finding.ErrInvalidTransition) {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return resp, err
}
