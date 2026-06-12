// Package grpcserver implements the gRPC ConverterService server.
// TASK-04-04: gRPC service interface for the converter microservice.
package grpcserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/osv/converter/internal/domain/cve5"
	"github.com/osv/converter/internal/domain/nvd"
)

// ConvertResponse is the internal (non-proto) response type used by the server.
// The actual proto-generated types would be used after protoc compilation.
type ConvertResponse struct {
	Success bool
	OSVJSON []byte
	ID      string
	Notes   []string
	Error   string
	Meta    ConversionMeta
}

// ConversionMeta holds metadata about a conversion result.
type ConversionMeta struct {
	SourceFormat string
	SourceID     string
	AffectedCnt  int
	HasKEV       bool
	HasEPSS      bool
	CWEIDs       []string
}

// ConversionStats holds aggregated stats for a source.
type ConversionStats struct {
	SourceID          string
	TotalConverted    int64
	TotalFailed       int64
	LastConvertedAt   int64
	AvgNotesPerRecord float64
}

// TriggerResponse is the response for a full re-conversion trigger.
type TriggerResponse struct {
	Accepted         bool
	JobID            string
	EstimatedRecords int64
	Error            string
}

// ConverterServer implements the ConverterService gRPC interface.
type ConverterServer struct {
	// In production this would embed the proto UnimplementedConverterServiceServer.
	stats map[string]*sourceStats
}

type sourceStats struct {
	totalConverted int64
	totalFailed    int64
	lastAt         int64
	totalNotes     int64
}

// NewConverterServer creates a new ConverterServer.
func NewConverterServer() *ConverterServer {
	return &ConverterServer{
		stats: make(map[string]*sourceStats),
	}
}

// ConvertCVE5 converts a raw CVE5 JSON payload to an OSV vulnerability record.
func (s *ConverterServer) ConvertCVE5(ctx context.Context, rawJSON []byte, sourceID string) ConvertResponse {
	logger := log.Ctx(ctx).With().Str("source_id", sourceID).Logger()

	// Parse raw CVE5 JSON
	var cveRecord cve5.CVERecord
	if err := json.Unmarshal(rawJSON, &cveRecord); err != nil {
		logger.Error().Err(err).Msg("failed to parse CVE5 JSON")
		s.recordFailure(sourceID)
		return ConvertResponse{
			Success: false,
			Error:   fmt.Sprintf("parse CVE5 JSON: %v", err),
		}
	}

	cveID := ""
	if cveRecord.CVEMetadata != nil {
		cveID = cveRecord.CVEMetadata.CVEID
	}

	// Convert using domain function
	result, err := cve5.ConvertToOSV(&cveRecord)
	if err != nil {
		logger.Error().Err(err).Str("id", cveID).Msg("CVE5 conversion failed")
		s.recordFailure(sourceID)
		return ConvertResponse{
			Success: false,
			ID:      cveID,
			Error:   err.Error(),
		}
	}

	// Marshal result to JSON
	osvJSON, err := json.Marshal(result)
	if err != nil {
		s.recordFailure(sourceID)
		return ConvertResponse{
			Success: false,
			ID:      cveID,
			Error:   fmt.Sprintf("marshal OSV: %v", err),
		}
	}

	s.recordSuccess(sourceID, 0)
	logger.Info().Str("id", result.Id).Msg("CVE5 conversion OK")

	return ConvertResponse{
		Success: true,
		OSVJSON: osvJSON,
		ID:      result.Id,
		Meta: ConversionMeta{
			SourceFormat: "cve5",
			SourceID:     sourceID,
			AffectedCnt:  len(result.Affected),
		},
	}
}

// ConvertNVD converts a raw NVD JSON v2.0 payload to an OSV vulnerability record.
func (s *ConverterServer) ConvertNVD(ctx context.Context, rawJSON []byte, sourceID string) ConvertResponse {
	logger := log.Ctx(ctx).With().Str("source_id", sourceID).Logger()

	var nvdVuln nvd.NVDVulnerability
	if err := json.Unmarshal(rawJSON, &nvdVuln); err != nil {
		s.recordFailure(sourceID)
		return ConvertResponse{
			Success: false,
			Error:   fmt.Sprintf("parse NVD JSON: %v", err),
		}
	}

	result, err := nvd.ConvertNVDToOSV(nvdVuln.CVE)
	if err != nil {
		logger.Error().Err(err).Msg("NVD conversion failed")
		s.recordFailure(sourceID)
		return ConvertResponse{
			Success: false,
			Error:   err.Error(),
		}
	}

	osvJSON, err := json.Marshal(result.OSV)
	if err != nil {
		s.recordFailure(sourceID)
		return ConvertResponse{
			Success: false,
			Error:   fmt.Sprintf("marshal OSV: %v", err),
		}
	}

	s.recordSuccess(sourceID, int64(len(result.Warnings)))
	logger.Info().Str("id", result.CVEID).Msg("NVD conversion OK")

	return ConvertResponse{
		Success: true,
		OSVJSON: osvJSON,
		ID:      result.CVEID,
		Notes:   result.Warnings,
		Meta: ConversionMeta{
			SourceFormat: "nvd",
			SourceID:     sourceID,
			AffectedCnt:  len(result.OSV.Affected),
		},
	}
}

// BatchConvert converts multiple records concurrently.
func (s *ConverterServer) BatchConvert(ctx context.Context, items []BatchItem) BatchConvertResult {
	results := make([]ConvertResponse, len(items))

	// Process sequentially for now; can be parallelised with goroutines + errgroup
	for i, item := range items {
		switch item.Format {
		case "cve5":
			results[i] = s.ConvertCVE5(ctx, item.Payload, item.SourceID)
		case "nvd":
			results[i] = s.ConvertNVD(ctx, item.Payload, item.SourceID)
		default:
			results[i] = ConvertResponse{
				Success: false,
				Error:   fmt.Sprintf("unknown format %q", item.Format),
			}
		}
	}

	total := len(results)
	successCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		}
	}

	return BatchConvertResult{
		Results:      results,
		Total:        total,
		SuccessCount: successCount,
		FailureCount: total - successCount,
	}
}

// BatchItem is a single item in a batch convert request.
type BatchItem struct {
	Format   string // "cve5" or "nvd"
	SourceID string
	Payload  []byte
}

// BatchConvertResult is the response for a batch conversion.
type BatchConvertResult struct {
	Results      []ConvertResponse
	Total        int
	SuccessCount int
	FailureCount int
}

// GetStats returns conversion statistics for a source.
func (s *ConverterServer) GetStats(_ context.Context, sourceID string) ConversionStats {
	st, ok := s.stats[sourceID]
	if !ok {
		return ConversionStats{SourceID: sourceID}
	}

	avgNotes := 0.0
	if st.totalConverted > 0 {
		avgNotes = float64(st.totalNotes) / float64(st.totalConverted)
	}

	return ConversionStats{
		SourceID:          sourceID,
		TotalConverted:    st.totalConverted,
		TotalFailed:       st.totalFailed,
		LastConvertedAt:   st.lastAt,
		AvgNotesPerRecord: avgNotes,
	}
}

// TriggerFullConversion triggers a full re-conversion job for a source.
func (s *ConverterServer) TriggerFullConversion(_ context.Context, sourceID string, startFromID string) TriggerResponse {
	jobID := fmt.Sprintf("job-%s-%d", sourceID, s.stats[sourceID].lastAt)
	log.Info().
		Str("source_id", sourceID).
		Str("start_from", startFromID).
		Str("job_id", jobID).
		Msg("full conversion triggered")

	return TriggerResponse{
		Accepted: true,
		JobID:    jobID,
	}
}

func (s *ConverterServer) recordSuccess(sourceID string, notes int64) {
	st := s.ensureStats(sourceID)
	st.totalConverted++
	st.totalNotes += notes
}

func (s *ConverterServer) recordFailure(sourceID string) {
	st := s.ensureStats(sourceID)
	st.totalFailed++
}

func (s *ConverterServer) ensureStats(sourceID string) *sourceStats {
	if _, ok := s.stats[sourceID]; !ok {
		s.stats[sourceID] = &sourceStats{}
	}
	return s.stats[sourceID]
}
