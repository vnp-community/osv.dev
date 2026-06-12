// Package apiv2 implements OSV API v2 extended endpoints: enrichment, related, batch, timeline.
package apiv2

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// ── Domain types ──────────────────────────────────────────────────────────────

// EnrichmentData holds AI-enriched metadata for a vulnerability.
type EnrichmentData struct {
	VulnID          string     `json:"vuln_id"`
	KEV             *KEVData   `json:"kev,omitempty"`
	EPSS            *EPSSData  `json:"epss,omitempty"`
	Tags            []string   `json:"tags,omitempty"`
	CWEIDs          []string   `json:"cwe_ids,omitempty"`
	ExploitAvailable bool      `json:"exploit_available"`
	AISummary       string     `json:"ai_summary,omitempty"`
	EnrichedAt      time.Time  `json:"enriched_at"`
}

// KEVData is the CISA KEV catalog entry for a vulnerability.
type KEVData struct {
	IsKEV          bool      `json:"is_kev"`
	DateAdded      string    `json:"date_added,omitempty"`
	DueDate        string    `json:"due_date,omitempty"`
	RequiredAction string    `json:"required_action,omitempty"`
}

// EPSSData is the EPSS exploit prediction score for a vulnerability.
type EPSSData struct {
	Score      float64 `json:"score"`
	Percentile float64 `json:"percentile"`
	Tier       string  `json:"tier"` // CRITICAL(≥0.9), HIGH(≥0.7), MEDIUM(≥0.4), LOW
	Date       string  `json:"date"`
}

// RelatedVuln is a vulnerability related to the queried one.
type RelatedVuln struct {
	VulnID     string  `json:"vuln_id"`
	Summary    string  `json:"summary"`
	Similarity float32 `json:"similarity,omitempty"`
	Reason     string  `json:"reason"` // "alias", "same_package", "semantic"
}

// TimelineEvent is a single event in a vulnerability's lifecycle.
type TimelineEvent struct {
	Type      string    `json:"type"`  // published, modified, kev_added, fix_released, withdrawn, epss_changed
	Timestamp time.Time `json:"timestamp"`
	Value     any       `json:"value,omitempty"` // optional, e.g. EPSS score
	Actor     string    `json:"actor,omitempty"`
}

// BatchGetRequest is the request body for batch-get.
type BatchGetRequest struct {
	IDs []string `json:"ids"`
}

// ── Enrichment Repository port ────────────────────────────────────────────────

// EnrichmentStore reads enrichment data persisted by the ai-enrichment service.
type EnrichmentStore interface {
	GetEnrichment(ctx context.Context, vulnID string) (*EnrichmentData, error)
	GetRelated(ctx context.Context, vulnID string, limit int) ([]*RelatedVuln, error)
	GetTimeline(ctx context.Context, vulnID string) ([]*TimelineEvent, error)
	BatchGetEnrichment(ctx context.Context, ids []string) (map[string]*EnrichmentData, error)
}

// ── Handler ───────────────────────────────────────────────────────────────────

// Handler implements the /v2/vulns/* family of endpoints.
type Handler struct {
	store EnrichmentStore
	log   zerolog.Logger
}

// NewHandler creates a new API v2 handler.
func NewHandler(store EnrichmentStore, log zerolog.Logger) *Handler {
	return &Handler{store: store, log: log}
}

// RegisterRoutes registers /v2/* routes onto mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v2/vulns/", h.dispatch)
	mux.HandleFunc("/v2/vulns/batch-get", h.BatchGet)
}

// dispatch routes /v2/vulns/{id}/{action} to the appropriate handler.
func (h *Handler) dispatch(w http.ResponseWriter, r *http.Request) {
	// Path: /v2/vulns/{id}[/{action}]
	path := strings.TrimPrefix(r.URL.Path, "/v2/vulns/")
	parts := strings.SplitN(path, "/", 2)

	vulnID := parts[0]
	if vulnID == "" {
		http.Error(w, `{"error":"vuln_id required"}`, http.StatusBadRequest)
		return
	}

	if len(parts) == 1 {
		http.Error(w, `{"error":"use /v2/vulns/{id}/enrichment or /timeline or /related"}`, http.StatusNotFound)
		return
	}

	switch parts[1] {
	case "enrichment":
		h.GetEnrichment(w, r, vulnID)
	case "related":
		h.GetRelated(w, r, vulnID)
	case "timeline":
		h.GetTimeline(w, r, vulnID)
	default:
		http.Error(w, `{"error":"unknown action"}`, http.StatusNotFound)
	}
}

// GetEnrichment handles GET /v2/vulns/{id}/enrichment.
func (h *Handler) GetEnrichment(w http.ResponseWriter, r *http.Request, vulnID string) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	enrichment, err := h.store.GetEnrichment(r.Context(), vulnID)
	if err != nil {
		h.log.Error().Err(err).Str("vuln_id", vulnID).Msg("GetEnrichment failed")
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if enrichment == nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("no enrichment found for %s", vulnID))
		return
	}
	writeJSON(w, http.StatusOK, enrichment)
}

// GetRelated handles GET /v2/vulns/{id}/related.
func (h *Handler) GetRelated(w http.ResponseWriter, r *http.Request, vulnID string) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	related, err := h.store.GetRelated(r.Context(), vulnID, 10)
	if err != nil {
		h.log.Error().Err(err).Str("vuln_id", vulnID).Msg("GetRelated failed")
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"vuln_id": vulnID,
		"related": related,
	})
}

// GetTimeline handles GET /v2/vulns/{id}/timeline.
func (h *Handler) GetTimeline(w http.ResponseWriter, r *http.Request, vulnID string) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	events, err := h.store.GetTimeline(r.Context(), vulnID)
	if err != nil {
		h.log.Error().Err(err).Str("vuln_id", vulnID).Msg("GetTimeline failed")
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"vuln_id": vulnID,
		"events":  events,
	})
}

// BatchGet handles POST /v2/vulns/batch-get.
func (h *Handler) BatchGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	var req BatchGetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if len(req.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "ids must not be empty")
		return
	}
	if len(req.IDs) > 100 {
		writeError(w, http.StatusBadRequest, "max 100 IDs per request")
		return
	}

	results, err := h.store.BatchGetEnrichment(r.Context(), req.IDs)
	if err != nil {
		h.log.Error().Err(err).Msg("BatchGetEnrichment failed")
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"results": results,
		"count":   len(results),
	})
}

// ── ETag helpers ──────────────────────────────────────────────────────────────

func etag(data []byte) string {
	return fmt.Sprintf(`"%x"`, sha256.Sum256(data))[:18] + `"`
}

// ── Response helpers ──────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("ETag", etag(body))
	w.WriteHeader(status)
	w.Write(body) //nolint:errcheck
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"error":%q}`, msg)
}

// ── EPSSTier helper ───────────────────────────────────────────────────────────

// EPSSTier converts a percentile to a tier label.
func EPSSTier(percentile float64) string {
	switch {
	case percentile >= 0.9:
		return "CRITICAL"
	case percentile >= 0.7:
		return "HIGH"
	case percentile >= 0.4:
		return "MEDIUM"
	default:
		return "LOW"
	}
}
