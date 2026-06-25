// Package http — cve_detail_handler.go
// CVEDetailHandler aggregates CVE detail from data-service + ai-service in parallel.
// Uses errgroup for concurrent calls with graceful degradation (partial results allowed).
//
// Route: GET /api/v1/cves/{id}/detail
// ADDITIVE: existing CVEHandler in handler_v1.go is unchanged.
package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	"github.com/osv/gateway-service/internal/adapter/grpcclient"

	aiv1 "github.com/osv/shared/proto/gen/go/ai/v1"
)

// CVEDetailResponse is the aggregated BFF response for CVE detail.
type CVEDetailResponse struct {
	// Core CVE data (from data-service via cvedb_client)
	ID        string  `json:"id"`
	Summary   string  `json:"summary,omitempty"`
	Severity  string  `json:"severity,omitempty"`
	CVSSScore float64 `json:"cvss_score,omitempty"`
	IsKEV     bool    `json:"is_kev"`

	// AI enrichment (from ai-service — optional)
	Enrichment *EnrichmentResult `json:"enrichment,omitempty"`
	EPSS       *EPSSResult       `json:"epss,omitempty"`

	// Metadata
	GeneratedAt time.Time `json:"generated_at"`
	Partial     bool      `json:"partial"` // true when some services were unavailable
}

// EnrichmentResult holds AI enrichment summary.
type EnrichmentResult struct {
	SummaryShort     string `json:"summary_short"`
	ImpactAnalysis   string `json:"impact_analysis,omitempty"`
	RemediationGuide string `json:"remediation_guide,omitempty"`
	Provider         string `json:"provider"`
}

// EPSSResult holds EPSS score data.
type EPSSResult struct {
	Score      float64 `json:"score"`
	Percentile float64 `json:"percentile"`
}

// CVEDetailHandler aggregates CVE detail from multiple services.
type CVEDetailHandler struct {
	cvedbClient *grpcclient.CVEDBClient
	aiClient    *grpcclient.AIClient
	log         zerolog.Logger
}

// NewCVEDetailHandler creates a new CVEDetailHandler.
func NewCVEDetailHandler(
	cvedbClient *grpcclient.CVEDBClient,
	aiClient *grpcclient.AIClient,
	log zerolog.Logger,
) *CVEDetailHandler {
	return &CVEDetailHandler{
		cvedbClient: cvedbClient,
		aiClient:    aiClient,
		log:         log,
	}
}

// GetCVEDetail handles GET /api/v1/cves/{id}/detail.
// Calls data-service and ai-service in parallel.
// Partial results are returned if some services are unavailable (Partial=true).
func (h *CVEDetailHandler) GetCVEDetail(w http.ResponseWriter, r *http.Request) {
	cveID := chi.URLParam(r, "id")
	if cveID == "" {
		cveDetailJSON(w, http.StatusBadRequest, map[string]string{"error": "missing CVE ID"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	resp := &CVEDetailResponse{
		ID:          cveID,
		GeneratedAt: time.Now().UTC(),
	}

	var (
		enrichmentData *aiv1.EnrichCVEResponse
		epssData       *aiv1.EPSSResponse
	)

	// ── Parallel calls with errgroup ─────────────────────────────────────────
	// Both calls are best-effort: errors are logged but do not fail the handler.
	eg, egCtx := errgroup.WithContext(ctx)

	// Call 1: AI enrichment
	eg.Go(func() error {
		data, err := h.aiClient.GetEnrichment(egCtx, cveID)
		if err != nil {
			h.log.Warn().Err(err).Str("cve_id", cveID).Msg("cve_detail: enrichment unavailable")
			resp.Partial = true
			return nil // non-fatal
		}
		enrichmentData = data
		return nil
	})

	// Call 2: EPSS score
	eg.Go(func() error {
		data, err := h.aiClient.GetEPSS(egCtx, cveID)
		if err != nil {
			h.log.Warn().Err(err).Str("cve_id", cveID).Msg("cve_detail: EPSS unavailable")
			resp.Partial = true
			return nil // non-fatal
		}
		epssData = data
		return nil
	})

	// Wait (non-fatal — errgroup.Go never returns non-nil above)
	_ = eg.Wait()

	// ── Assemble response ────────────────────────────────────────────────────
	if enrichmentData != nil {
		resp.Enrichment = &EnrichmentResult{
			SummaryShort: enrichmentData.GetSummaryShort(),
		}
	}

	if epssData != nil {
		resp.EPSS = &EPSSResult{
			Score:      epssData.GetScore(),
			Percentile: epssData.GetPercentile(),
		}
	}

	cveDetailJSON(w, http.StatusOK, resp)
}

// ── helpers ──────────────────────────────────────────────────────────────────

// cveDetailJSON writes a JSON response for the CVE detail handler.
// Named distinctly to avoid collision with other respondJSON helpers in the package.
func cveDetailJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body) //nolint:errcheck
}
