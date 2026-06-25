# S1-AI-02 — Thêm gRPC Server (ai-service)

## ✅ Execution Status: COMPLETED
- **Executed**: 2026-06-13
- **Result**: `go build` PASSED


## Metadata
- **Task ID**: S1-AI-02
- **Service**: ai-service
- **Sprint**: 1 (P0)
- **Ước tính**: 3 giờ
- **Dependencies**: S1-AI-01 (entity.go cần có trước)
- **Spec nguồn**: `specs/develop/06_ai-service-upgrade.md` § "P0 — Thêm: gRPC Server"

## Context

```bash
# Đọc HTTP delivery hiện tại:
cat services/ai-service/internal/delivery/http/router.go

# Đọc proto definitions:
ls services/shared/proto/ai/
cat services/shared/proto/ai/*.proto 2>/dev/null | head -100

# Đọc use cases để biết exact method signatures:
cat services/ai-service/internal/usecase/enrich_cve/handler.go
cat services/ai-service/internal/usecase/batch_enrich/usecase.go
cat services/ai-service/internal/usecase/generate_embedding/usecase.go
cat services/ai-service/internal/usecase/triage_finding/triage_finding.go
cat services/ai-service/internal/usecase/epss/job.go

# Đọc gRPC pattern từ services khác:
cat services/finding-service/internal/delivery/grpc/server/finding_server.go
```

## Goal

Tạo gRPC server cho ai-service, implement các RPCs từ `shared/proto/ai/`.
gRPC server chạy song song với HTTP server hiện tại.

## Files to Create

### File 1: `services/ai-service/internal/delivery/grpc/ai_handler.go`

```go
package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"github.com/rs/zerolog"

	aipb "github.com/osv/shared/proto/ai"
	"github.com/osv/ai-service/internal/usecase/enrich_cve"
	"github.com/osv/ai-service/internal/usecase/batch_enrich"
	"github.com/osv/ai-service/internal/usecase/generate_embedding"
	"github.com/osv/ai-service/internal/usecase/triage_finding"
	"github.com/osv/ai-service/internal/usecase/epss"
)

// AIGRPCHandler implements the aipb.AIEnrichmentServiceServer interface.
type AIGRPCHandler struct {
	aipb.UnimplementedAIEnrichmentServiceServer  // safe default for new RPCs

	enrichUC    *enrich_cve.UseCase
	batchUC     *batch_enrich.UseCase
	embeddingUC *generate_embedding.UseCase
	triageUC    *triage_finding.UseCase
	epssUC      *epss.UseCase
	log         zerolog.Logger
}

// NewAIGRPCHandler creates a new AIGRPCHandler.
func NewAIGRPCHandler(
	enrichUC *enrich_cve.UseCase,
	batchUC *batch_enrich.UseCase,
	embeddingUC *generate_embedding.UseCase,
	triageUC *triage_finding.UseCase,
	epssUC *epss.UseCase,
	log zerolog.Logger,
) *AIGRPCHandler {
	return &AIGRPCHandler{
		enrichUC:    enrichUC,
		batchUC:     batchUC,
		embeddingUC: embeddingUC,
		triageUC:    triageUC,
		epssUC:      epssUC,
		log:         log,
	}
}

// EnrichCVE handles the EnrichCVE RPC.
func (h *AIGRPCHandler) EnrichCVE(ctx context.Context, req *aipb.EnrichCVERequest) (*aipb.EnrichmentResult, error) {
	if req.GetCveId() == "" {
		return nil, status.Error(codes.InvalidArgument, "cve_id is required")
	}

	result, err := h.enrichUC.Execute(ctx, req.GetCveId())
	if err != nil {
		h.log.Error().Err(err).Str("cve_id", req.GetCveId()).Msg("EnrichCVE failed")
		return nil, status.Errorf(codes.Internal, "enrichment failed: %v", err)
	}

	// Map domain result to proto response
	// Adjust field mapping based on actual EnrichmentResult and proto fields:
	return &aipb.EnrichmentResult{
		CveId:        result.CVEID,
		SummaryShort: result.SummaryShort,
		SummaryLong:  result.SummaryLong,
		Provider:     result.Provider,
	}, nil
}

// GetEnrichment retrieves cached enrichment for a CVE.
func (h *AIGRPCHandler) GetEnrichment(ctx context.Context, req *aipb.GetEnrichmentRequest) (*aipb.EnrichmentResult, error) {
	if req.GetCveId() == "" {
		return nil, status.Error(codes.InvalidArgument, "cve_id is required")
	}

	result, err := h.enrichUC.GetCached(ctx, req.GetCveId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "no enrichment found for %s", req.GetCveId())
	}

	return &aipb.EnrichmentResult{
		CveId:        result.CVEID,
		SummaryShort: result.SummaryShort,
		Provider:     result.Provider,
	}, nil
}

// GetEPSS retrieves EPSS score for a CVE.
func (h *AIGRPCHandler) GetEPSS(ctx context.Context, req *aipb.GetEPSSRequest) (*aipb.EPSSResponse, error) {
	if req.GetCveId() == "" {
		return nil, status.Error(codes.InvalidArgument, "cve_id is required")
	}

	score, err := h.epssUC.GetScore(ctx, req.GetCveId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "EPSS score not found for %s", req.GetCveId())
	}

	return &aipb.EPSSResponse{
		CveId:      req.GetCveId(),
		Score:      score.Score,
		Percentile: score.Percentile,
	}, nil
}

// TriageFinding handles the TriageFinding RPC.
func (h *AIGRPCHandler) TriageFinding(ctx context.Context, req *aipb.TriageFindingRequest) (*aipb.TriageRecommendation, error) {
	if req.GetFindingId() == "" {
		return nil, status.Error(codes.InvalidArgument, "finding_id is required")
	}

	recommendation, err := h.triageUC.Execute(ctx, req.GetFindingId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "triage failed: %v", err)
	}

	return &aipb.TriageRecommendation{
		FindingId:    req.GetFindingId(),
		Priority:     recommendation.Priority,
		Reasoning:    recommendation.Reasoning,
		Confidence:   float32(recommendation.Confidence),
	}, nil
}
```

### File 2: `services/ai-service/internal/delivery/grpc/server.go`

```go
package grpc

import (
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"github.com/rs/zerolog"

	aipb "github.com/osv/shared/proto/ai"
)

// NewGRPCServer creates and configures a gRPC server with the AI handler registered.
func NewGRPCServer(handler *AIGRPCHandler, log zerolog.Logger) *grpc.Server {
	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			loggingInterceptor(log),
			recoveryInterceptor(log),
		),
	)

	// Register AI service
	aipb.RegisterAIEnrichmentServiceServer(srv, handler)

	// Enable gRPC reflection for development tools (grpcurl, Evans, etc.)
	reflection.Register(srv)

	return srv
}

// StartGRPCServer starts the gRPC server on the given port.
// This should be called in a goroutine.
func StartGRPCServer(srv *grpc.Server, port int, log zerolog.Logger) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("grpc listen on port %d: %w", port, err)
	}

	log.Info().Int("port", port).Msg("gRPC server starting")
	return srv.Serve(lis)
}

// loggingInterceptor logs each gRPC call.
func loggingInterceptor(log zerolog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		log.Debug().Str("method", info.FullMethod).Msg("grpc request")
		resp, err := handler(ctx, req)
		if err != nil {
			log.Error().Err(err).Str("method", info.FullMethod).Msg("grpc error")
		}
		return resp, err
	}
}

// recoveryInterceptor recovers from panics in handlers.
func recoveryInterceptor(log zerolog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Error().Interface("panic", r).Str("method", info.FullMethod).Msg("grpc panic recovered")
				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}
```

## Files to Extend

### Extend: `services/ai-service/cmd/server/main.go`

```go
// Thêm imports:
// grpc_delivery "github.com/osv/ai-service/internal/delivery/grpc"
// "google.golang.org/grpc"

// Sau khi init các use cases:
aiHandler := grpc_delivery.NewAIGRPCHandler(
    enrichUC,
    batchUC,
    embeddingUC,
    triageUC,
    epssUC,
    logger,
)

grpcSrv := grpc_delivery.NewGRPCServer(aiHandler, logger)

// Start gRPC server (non-blocking):
go func() {
    if err := grpc_delivery.StartGRPCServer(grpcSrv, 50056, logger); err != nil {
        log.Fatal().Err(err).Msg("gRPC server failed")
    }
}()

// Existing HTTP server continues...
```

## Verification

```bash
cd services/ai-service && go build ./...

# Test gRPC với grpcurl:
grpcurl -plaintext localhost:50056 list
# Expected: ai.AIEnrichmentService

grpcurl -plaintext -d '{"cve_id": "CVE-2024-12345"}' \
    localhost:50056 ai.AIEnrichmentService/GetEnrichment
```

## Notes

- Điều chỉnh `aipb` package name theo actual proto package name trong `shared/proto/ai/`
- Nếu proto chưa có một số RPCs, dùng `UnimplementedAIEnrichmentServiceServer` để skip
- Port 50056 — kiểm tra không conflict với services khác
- Nếu use case method names khác, đọc actual file và điều chỉnh
