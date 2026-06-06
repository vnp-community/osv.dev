// Package server implements the FindingService gRPC server.
package server

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	findingv1 "github.com/defectdojo/proto/finding/v1"
	"github.com/defectdojo/finding-management/internal/domain/finding"
	uc "github.com/defectdojo/finding-management/internal/usecase/finding"
)

// FindingServer implements findingv1.FindingServiceServer.
type FindingServer struct {
	findingv1.UnimplementedFindingServiceServer
	batchCreate     *uc.BatchCreateFindingsUseCase
	findByHashCode  *uc.FindByHashCodeUseCase
	closeOld        *uc.CloseOldFindingsUseCase
	transition      *uc.StatusTransitionUseCase
	batchSLA        *uc.BatchUpdateSLADatesUseCase
	repo            finding.Repository
}

func New(
	batchCreate *uc.BatchCreateFindingsUseCase,
	findByHash *uc.FindByHashCodeUseCase,
	closeOld *uc.CloseOldFindingsUseCase,
	transition *uc.StatusTransitionUseCase,
	batchSLA *uc.BatchUpdateSLADatesUseCase,
	repo finding.Repository,
) *FindingServer {
	return &FindingServer{
		batchCreate:    batchCreate,
		findByHashCode: findByHash,
		closeOld:       closeOld,
		transition:     transition,
		batchSLA:       batchSLA,
		repo:           repo,
	}
}

// BatchCreateFindings creates multiple findings (called by Scan Orchestrator).
func (s *FindingServer) BatchCreateFindings(ctx context.Context, req *findingv1.BatchCreateFindingsRequest) (*findingv1.BatchCreateFindingsResponse, error) {
	testID, err := uuid.Parse(req.TestId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid test_id")
	}
	engagementID, err := uuid.Parse(req.EngagementId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid engagement_id")
	}
	productID, err := uuid.Parse(req.ProductId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid product_id")
	}

	inputs := make([]uc.FindingInput, 0, len(req.Findings))
	for _, fi := range req.Findings {
		inputs = append(inputs, uc.FindingInput{
			Title:            fi.Title,
			Description:      fi.Description,
			Mitigation:       fi.Mitigation,
			Impact:           fi.Impact,
			References:       fi.References,
			Severity:         fi.Severity,
			CVE:              fi.Cve,
			CWE:              int(fi.Cwe),
			VulnIDFromTool:   fi.VulnIdFromTool,
			CVSSv3:           fi.CvssV3,
			CVSSv3Score:      fi.CvssV3Score,
			Active:           fi.Active,
			Verified:         fi.Verified,
			ComponentName:    fi.ComponentName,
			ComponentVersion: fi.ComponentVersion,
			FilePath:         fi.FilePath,
			LineNumber:       int(fi.LineNumber),
			Service:          fi.Service,
			HashCode:         fi.HashCode,
			Tags:             fi.Tags,
		})
	}

	out, err := s.batchCreate.Execute(ctx, uc.BatchCreateInput{
		TestID:       testID,
		EngagementID: engagementID,
		ProductID:    productID,
		Findings:     inputs,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "batch_create: %v", err)
	}

	return &findingv1.BatchCreateFindingsResponse{
		FindingIds: out.FindingIDs,
		Count:      int32(out.Count),
	}, nil
}

// FindByHashCode looks up a finding by hash code for deduplication.
func (s *FindingServer) FindByHashCode(ctx context.Context, req *findingv1.FindByHashCodeRequest) (*findingv1.FindByHashCodeResponse, error) {
	testID, _ := uuid.Parse(req.TestId)
	var engID, prodID *uuid.UUID
	if req.EngagementId != nil {
		id, _ := uuid.Parse(*req.EngagementId)
		engID = &id
	}
	if req.ProductId != nil {
		id, _ := uuid.Parse(*req.ProductId)
		prodID = &id
	}

	out, err := s.findByHashCode.Execute(ctx, uc.FindByHashCodeInput{
		HashCode:     req.HashCode,
		TestID:       testID,
		EngagementID: engID,
		ProductID:    prodID,
		OnEngagement: req.DeduplicationOnEngagement,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "find_by_hash_code: %v", err)
	}

	resp := &findingv1.FindByHashCodeResponse{}
	if out.FindingID != nil {
		id := out.FindingID.String()
		resp.FindingId = &id
		resp.Status = out.Status
	}
	return resp, nil
}

// CloseOldFindings mitigates findings from a previous test run not in current scan.
func (s *FindingServer) CloseOldFindings(ctx context.Context, req *findingv1.CloseOldFindingsRequest) (*findingv1.CloseOldFindingsResponse, error) {
	testID, _ := uuid.Parse(req.TestId)
	engagementID, _ := uuid.Parse(req.EngagementId)
	mitigatedBy, _ := uuid.Parse(req.MitigatedById)

	excludeIDs := make([]uuid.UUID, 0, len(req.ExcludeFindingIds))
	for _, id := range req.ExcludeFindingIds {
		if uid, err := uuid.Parse(id); err == nil {
			excludeIDs = append(excludeIDs, uid)
		}
	}

	out, err := s.closeOld.Execute(ctx, uc.CloseOldFindingsInput{
		TestID:                    testID,
		EngagementID:              engagementID,
		DeduplicationOnEngagement: req.DeduplicationOnEngagement,
		ExcludeFindingIDs:         excludeIDs,
		MitigatedByID:             mitigatedBy,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "close_old_findings: %v", err)
	}
	return &findingv1.CloseOldFindingsResponse{Closed: int32(out.Closed)}, nil
}

// BatchUpdateSLADates updates SLA expiration dates (called by SLA Service).
func (s *FindingServer) BatchUpdateSLADates(ctx context.Context, req *findingv1.BatchUpdateSLADatesRequest) (*findingv1.BatchUpdateSLADatesResponse, error) {
	updates := make([]uc.SLAUpdate, 0, len(req.Updates))
	for _, u := range req.Updates {
		id, _ := uuid.Parse(u.FindingId)
		updates = append(updates, uc.SLAUpdate{
			FindingID:      id,
			ExpirationDate: u.ExpirationDate.AsTime(),
		})
	}
	if err := s.batchSLA.Execute(ctx, uc.BatchUpdateSLADatesInput{Updates: updates}); err != nil {
		return nil, status.Errorf(codes.Internal, "batch_update_sla: %v", err)
	}
	return &findingv1.BatchUpdateSLADatesResponse{Updated: int32(len(updates))}, nil
}

// ListFindingsForSLACheck returns findings for SLA computation (called by SLA Service).
func (s *FindingServer) ListFindingsForSLACheck(ctx context.Context, req *findingv1.ListFindingsForSLACheckRequest) (*findingv1.ListFindingsForSLACheckResponse, error) {
	var ids []uuid.UUID
	for _, id := range req.FindingIds {
		if uid, err := uuid.Parse(id); err == nil {
			ids = append(ids, uid)
		}
	}
	findings, err := s.repo.ListForSLACheck(ctx, ids, req.ActiveOnly, req.HasSlaDate)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list_for_sla_check: %v", err)
	}

	protos := make([]*findingv1.FindingForSLA, 0, len(findings))
	for _, f := range findings {
		proto := &findingv1.FindingForSLA{
			Id:        f.ID.String(),
			Severity:  string(f.Severity),
			ProductId: f.ProductID.String(),
			Date:      timestamppb.New(f.Date),
		}
		if f.SLAExpirationDate != nil {
			proto.SlaExpirationDate = timestamppb.New(*f.SLAExpirationDate)
		}
		protos = append(protos, proto)
	}
	return &findingv1.ListFindingsForSLACheckResponse{Findings: protos}, nil
}

// ListFindingsForReport streams findings for report generation.
func (s *FindingServer) ListFindingsForReport(req *findingv1.ListFindingsForReportRequest, stream findingv1.FindingService_ListFindingsForReportServer) error {
	ctx := stream.Context()
	filter := finding.FindingFilter{
		ActiveOnly: req.ActiveOnly,
	}
	if req.ProductId != "" {
		id, _ := uuid.Parse(req.ProductId)
		filter.ProductID = &id
	}
	for _, sv := range req.Severity {
		filter.Severity = append(filter.Severity, finding.Severity(sv))
	}

	ch, err := s.repo.ListForReport(ctx, filter)
	if err != nil {
		return status.Errorf(codes.Internal, "list_for_report: %v", err)
	}

	for f := range ch {
		proto := toFindingProto(f)
		if err := stream.Send(proto); err != nil {
			return err
		}
	}
	return nil
}

func toFindingProto(f *finding.Finding) *findingv1.FindingProto {
	proto := &findingv1.FindingProto{
		Id:               f.ID.String(),
		Title:            f.Title,
		Description:      f.Description,
		Severity:         string(f.Severity),
		Cve:              f.CVE,
		Cwe:              int32(f.CWE),
		Active:           f.Active,
		Verified:         f.Verified,
		FalsePositive:    f.FalsePositive,
		Duplicate:        f.Duplicate,
		IsMitigated:      f.IsMitigated,
		ComponentName:    f.ComponentName,
		ComponentVersion: f.ComponentVersion,
		FilePath:         f.FilePath,
		HashCode:         f.HashCode,
		ProductId:        f.ProductID.String(),
		TestId:           f.TestID.String(),
		Date:             timestamppb.New(f.Date),
	}
	if f.MitigatedAt != nil {
		proto.MitigatedAt = timestamppb.New(*f.MitigatedAt)
	}
	if f.SLAExpirationDate != nil {
		proto.SlaExpirationDate = timestamppb.New(*f.SLAExpirationDate)
	}
	_ = time.Now() // silence import
	return proto
}
