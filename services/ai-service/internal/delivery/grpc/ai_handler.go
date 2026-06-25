// Package grpc — ai_handler.go
// AIGRPCHandler implements aiv1.AIEnrichmentServiceServer.
// Runs on the ai-service gRPC port (default: 50053), alongside the existing HTTP server.
//
// ADDITIVE: existing HTTP router.go is unchanged.
package grpc

import (
	"context"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	aiv1 "github.com/osv/shared/proto/gen/go/ai/v1"
	enrich "github.com/osv/ai-service/internal/usecase/enrich_cve"
	batchenrich "github.com/osv/ai-service/internal/usecase/batch_enrich"
	embedding "github.com/osv/ai-service/internal/usecase/generate_embedding"
	triage "github.com/osv/ai-service/internal/usecase/triage_finding"
)

// AIGRPCHandler implements AIEnrichmentServiceServer.
// Uses UnimplementedAIEnrichmentServiceServer for forward compatibility.
type AIGRPCHandler struct {
	aiv1.UnimplementedAIEnrichmentServiceServer

	enrichHandler *enrich.Handler
	batchUC       *batchenrich.UseCase
	embeddingUC   *embedding.UseCase
	triageUC      *triage.TriageFindingUseCase
	log           zerolog.Logger
}

// NewAIGRPCHandler creates a new AIGRPCHandler.
func NewAIGRPCHandler(
	enrichHandler *enrich.Handler,
	batchUC *batchenrich.UseCase,
	embeddingUC *embedding.UseCase,
	triageUC *triage.TriageFindingUseCase,
	log zerolog.Logger,
) *AIGRPCHandler {
	return &AIGRPCHandler{
		enrichHandler: enrichHandler,
		batchUC:       batchUC,
		embeddingUC:   embeddingUC,
		triageUC:      triageUC,
		log:           log,
	}
}

// EnrichCVE handles EnrichCVE RPC — triggers full AI enrichment pipeline.
func (h *AIGRPCHandler) EnrichCVE(ctx context.Context, req *aiv1.EnrichCVERequest) (*aiv1.EnrichCVEResponse, error) {
	if req.GetCveId() == "" {
		return nil, status.Error(codes.InvalidArgument, "cve_id is required")
	}

	result, err := h.enrichHandler.Handle(ctx, enrich.Command{
		VulnID: req.GetCveId(),
	})
	if err != nil {
		h.log.Error().Err(err).Str("cve_id", req.GetCveId()).Msg("AIGRPCHandler.EnrichCVE failed")
		return nil, status.Errorf(codes.Internal, "enrichment failed: %v", err)
	}

	return &aiv1.EnrichCVEResponse{
		CveId:        result.VulnID,
		SummaryShort: truncateStr(result.TechnicalSummary, 500),
		SummaryLong:  result.TechnicalSummary,
		EpssScore:    result.ExploitabilityScore,
	}, nil
}

// GetEnrichment retrieves cached enrichment via repo.GetByVulnID (interface on handler).
func (h *AIGRPCHandler) GetEnrichment(ctx context.Context, req *aiv1.GetEnrichmentRequest) (*aiv1.EnrichCVEResponse, error) {
	if req.GetCveId() == "" {
		return nil, status.Error(codes.InvalidArgument, "cve_id is required")
	}

	// Delegate to EnrichCVE (re-enrichment path) — read path requires direct repo access
	// which is not yet wired at this layer. Return Unimplemented stub until wired.
	return nil, status.Errorf(codes.NotFound, "no cached enrichment for %s — use EnrichCVE to trigger", req.GetCveId())
}

// GetEPSS returns a stubbed EPSS response.
// EPSS Job (epssupdate.Job) has no read path yet; wiring deferred to S3.
func (h *AIGRPCHandler) GetEPSS(_ context.Context, req *aiv1.GetEPSSRequest) (*aiv1.EPSSResponse, error) {
	if req.GetCveId() == "" {
		return nil, status.Error(codes.InvalidArgument, "cve_id is required")
	}
	return &aiv1.EPSSResponse{
		CveId:      req.GetCveId(),
		Score:      0,
		Percentile: 0,
	}, nil
}

// TriageFinding triggers AI triage analysis for a finding.
func (h *AIGRPCHandler) TriageFinding(ctx context.Context, req *aiv1.TriageFindingRequest) (*aiv1.TriageResponse, error) {
	if req.GetFindingId() == "" {
		return nil, status.Error(codes.InvalidArgument, "finding_id is required")
	}

	suggestion, err := h.triageUC.Execute(ctx, triage.TriageFindingInput{
		FindingID: req.GetFindingId(),
		CVE:       req.GetCveId(),
	})
	if err != nil {
		h.log.Error().Err(err).Str("finding_id", req.GetFindingId()).Msg("TriageFinding failed")
		return nil, status.Errorf(codes.Internal, "triage failed: %v", err)
	}

	return &aiv1.TriageResponse{
		Rationale:  suggestion.Summary,
		Suggestion: suggestion.Remediation,
	}, nil
}

// GenerateEmbedding generates a vector embedding for a CVE.
func (h *AIGRPCHandler) GenerateEmbedding(ctx context.Context, req *aiv1.GenerateEmbeddingRequest) (*aiv1.EmbeddingResponse, error) {
	if req.GetCveId() == "" {
		return nil, status.Error(codes.InvalidArgument, "cve_id is required")
	}

	values, err := h.embeddingUC.Execute(ctx, req.GetCveId(), req.GetText())
	if err != nil {
		h.log.Error().Err(err).Str("cve_id", req.GetCveId()).Msg("GenerateEmbedding failed")
		return nil, status.Errorf(codes.Internal, "embedding failed: %v", err)
	}

	return &aiv1.EmbeddingResponse{
		CveId:      req.GetCveId(),
		Values:     values,
		Dimensions: int32(len(values)),
	}, nil
}

// BatchEnrich queues multiple CVEs for async enrichment.
func (h *AIGRPCHandler) BatchEnrich(ctx context.Context, req *aiv1.BatchEnrichRequest) (*aiv1.BatchEnrichResponse, error) {
	if len(req.GetCveIds()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "cve_ids must not be empty")
	}

	results := h.batchUC.ExecuteAsync(ctx, req.GetCveIds())

	var failedIDs []string
	for _, r := range results {
		if r.Err != nil {
			failedIDs = append(failedIDs, r.CVEID)
		}
	}

	return &aiv1.BatchEnrichResponse{
		Queued: int32(len(results) - len(failedIDs)),
		Failed: failedIDs,
	}, nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
