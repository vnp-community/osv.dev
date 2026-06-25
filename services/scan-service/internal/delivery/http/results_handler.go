package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/osv/scan-service/internal/domain/entity"
	"github.com/rs/zerolog"
)

// ─── Repository interfaces ────────────────────────────────────────────────────

// FindingRepository is the minimal read interface needed by ResultsHandler.
type FindingRepository interface {
	FindByScanID(ctx context.Context, scanID uuid.UUID, page, pageSize int) ([]*entity.Finding, int64, error)
}

// WebAlertRepository is the minimal read interface needed by ResultsHandler.
type WebAlertRepository interface {
	FindByScanID(ctx context.Context, scanID uuid.UUID) ([]*entity.WebAlert, error)
}

// ─── Handler ─────────────────────────────────────────────────────────────────

// ResultsHandler handles GET /api/v1/scans/{id}/results/nmap
// and GET /api/v1/scans/{id}/results/zap
type ResultsHandler struct {
	findingRepo FindingRepository
	alertRepo   WebAlertRepository
	log         zerolog.Logger
}

// NewResultsHandler creates a ResultsHandler.
// Either repo may be nil — the handler returns graceful empty responses.
func NewResultsHandler(findingRepo FindingRepository, alertRepo WebAlertRepository, log zerolog.Logger) *ResultsHandler {
	return &ResultsHandler{findingRepo: findingRepo, alertRepo: alertRepo, log: log}
}

// ─── Nmap results ─────────────────────────────────────────────────────────────

// GetNmapResults handles GET /api/v1/scans/{id}/results/nmap
// Returns paginated host findings discovered by the nmap scanner for this scan.
func (h *ResultsHandler) GetNmapResults(w http.ResponseWriter, r *http.Request) {
	scanID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeScanJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid scan id"})
		return
	}

	if h.findingRepo == nil {
		writeScanJSON(w, http.StatusOK, nmapResultsResponse(nil, 0, 1, 50))
		return
	}

	page, pageSize := parseResultsPagination(r)

	findings, total, err := h.findingRepo.FindByScanID(r.Context(), scanID, page, pageSize)
	if err != nil {
		h.log.Error().Err(err).Str("scan_id", scanID.String()).Msg("GetNmapResults: DB error")
		writeScanJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to fetch nmap results"})
		return
	}

	writeScanJSON(w, http.StatusOK, nmapResultsResponse(findings, total, page, pageSize))
}

func nmapResultsResponse(findings []*entity.Finding, total int64, page, pageSize int) map[string]interface{} {
	if findings == nil {
		findings = []*entity.Finding{}
	}

	type hostDTO struct {
		ID        string   `json:"id"`
		IPAddress string   `json:"ip_address"`
		Hostname  string   `json:"hostname"`
		OS        string   `json:"os"`
		OpenPorts []entity.Port    `json:"open_ports"`
		Services  []entity.Service `json:"services"`
		CVEIDs    []string `json:"cve_ids"`
		Severity  string   `json:"severity"`
	}

	hosts := make([]hostDTO, 0, len(findings))
	for _, f := range findings {
		hosts = append(hosts, hostDTO{
			ID:        f.ID.String(),
			IPAddress: f.IPAddress,
			Hostname:  f.Hostname,
			OS:        f.OS,
			OpenPorts: f.OpenPorts,
			Services:  f.Services,
			CVEIDs:    nullSafeStrings(f.CVEIDs),
			Severity:  string(f.Severity),
		})
	}

	return map[string]interface{}{
		"scan_type":  "nmap",
		"hosts":      hosts,
		"total":      total,
		"page":       page,
		"page_size":  pageSize,
	}
}

// ─── ZAP results ──────────────────────────────────────────────────────────────

// GetZapResults handles GET /api/v1/scans/{id}/results/zap
// Returns web security alerts from the OWASP ZAP scanner for this scan.
func (h *ResultsHandler) GetZapResults(w http.ResponseWriter, r *http.Request) {
	scanID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeScanJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid scan id"})
		return
	}

	if h.alertRepo == nil {
		writeScanJSON(w, http.StatusOK, zapResultsResponse(nil))
		return
	}

	alerts, err := h.alertRepo.FindByScanID(r.Context(), scanID)
	if err != nil {
		h.log.Error().Err(err).Str("scan_id", scanID.String()).Msg("GetZapResults: DB error")
		writeScanJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to fetch ZAP results"})
		return
	}

	writeScanJSON(w, http.StatusOK, zapResultsResponse(alerts))
}

func zapResultsResponse(alerts []*entity.WebAlert) map[string]interface{} {
	if alerts == nil {
		alerts = []*entity.WebAlert{}
	}

	type alertDTO struct {
		ID          string `json:"id"`
		TargetURL   string `json:"target_url"`
		AlertName   string `json:"alert_name"`
		Risk        string `json:"risk"`
		Confidence  string `json:"confidence"`
		Description string `json:"description"`
		Solution    string `json:"solution"`
		Reference   string `json:"reference"`
		Evidence    string `json:"evidence"`
	}

	dtos := make([]alertDTO, 0, len(alerts))
	for _, a := range alerts {
		dtos = append(dtos, alertDTO{
			ID:          a.ID.String(),
			TargetURL:   a.TargetURL,
			AlertName:   a.AlertName,
			Risk:        a.Risk,
			Confidence:  a.Confidence,
			Description: a.Description,
			Solution:    a.Solution,
			Reference:   a.Reference,
			Evidence:    a.Evidence,
		})
	}

	// Risk summary counts for dashboard widgets
	riskCounts := map[string]int{"High": 0, "Medium": 0, "Low": 0, "Informational": 0}
	for _, a := range alerts {
		if _, ok := riskCounts[a.Risk]; ok {
			riskCounts[a.Risk]++
		}
	}

	return map[string]interface{}{
		"scan_type":   "zap",
		"alerts":      dtos,
		"total":       len(dtos),
		"risk_counts": riskCounts,
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func parseResultsPagination(r *http.Request) (page, pageSize int) {
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ = strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return
}

func nullSafeStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// ensure json import used for potential future use
var _ = json.Marshal
