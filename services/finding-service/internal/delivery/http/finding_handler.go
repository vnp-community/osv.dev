// Package http — finding_handler.go
// FindingHandler handles HTTP REST requests for finding-service.
// Exposes finding CRUD and status transition endpoints on port 8085.
// gRPC server continues to run unchanged on its own port.
//
// Routes (add to router.go):
//   GET    /findings          → List   (paginated)
//   GET    /findings/{id}     → Get    (single finding)
//   PUT    /findings/{id}/close          → Close
//   PUT    /findings/{id}/reopen         → Reopen
//   PUT    /findings/{id}/false-positive → MarkFalsePositive
//   PUT    /findings/{id}/risk-accepted  → AcceptRisk
package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/osv/finding-service/internal/domain/finding"
	uc "github.com/osv/finding-service/internal/usecase/finding"
)

// FindingHandler handles HTTP requests for findings.
type FindingHandler struct {
	repo       finding.Repository
	transition *uc.StatusTransitionUseCase
	log        zerolog.Logger
}

// NewFindingHandler creates a new FindingHandler.
func NewFindingHandler(repo finding.Repository, transition *uc.StatusTransitionUseCase, log zerolog.Logger) *FindingHandler {
	return &FindingHandler{repo: repo, transition: transition, log: log}
}

// ── Response types ────────────────────────────────────────────────────────────

// FindingResponse is the HTTP JSON representation of a single Finding (GET /findings/{id}).
// Fields must match test_findings_scans.py FINDING_REQUIRED schema.
type FindingResponse struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Severity     string   `json:"severity"`
	CVE          string   `json:"cve,omitempty"`
	IsKEV        bool     `json:"is_kev"`
	Status       string   `json:"status"`
	IsDuplicate  bool     `json:"is_duplicate"`
	ProductID    string   `json:"product_id"`
	ProductName  string   `json:"product_name"`
	EngagementID string   `json:"engagement_id"`
	TestID       string   `json:"test_id"`
	SLAStatus    string   `json:"sla_status"`
	Active       bool     `json:"active"`
	CVSSv3Score  *float64 `json:"cvss_v3_score,omitempty"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
	CreatedBy    string   `json:"created_by"`
}

// ListResponse is the paginated list response (legacy, for Get).
type ListResponse struct {
	Findings []*FindingResponse `json:"findings"`
	Total    int                `json:"total"`
	Page     int                `json:"page"`
	Limit    int                `json:"limit"`
	HasMore  bool               `json:"has_more"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// List handles GET /findings — paginated list with optional filters.
// Supports both page/page_size (spec) and legacy limit/offset params.
func (h *FindingHandler) List(w http.ResponseWriter, r *http.Request) {
	// Support page/page_size (spec) as primary, fallback to limit/offset (legacy)
	var limit, offset int
	ps := parseIntParam(r, "page_size", 0)
	if ps == 0 {
		ps = parseIntParam(r, "pageSize", 0)
	}

	if ps > 0 {
		// page/page_size mode
		limit = ps
		if limit > MaxPageSize { // [FIX BUG-008] was: 200
			limit = MaxPageSize
		}
		page := parseIntParam(r, "page", 1)
		if page < 1 {
			page = 1
		}
		offset = (page - 1) * limit
	} else if r.URL.Query().Get("limit") != "" || r.URL.Query().Get("offset") != "" {
		// legacy limit/offset mode
		limit = parseIntParam(r, "limit", DefaultPageSize) // [FIX BUG-008] was: 20
		offset = parseIntParam(r, "offset", 0)
		if limit > MaxPageSize { // [FIX BUG-008] was: 200
			limit = MaxPageSize
		}
	} else {
		// fallback to default page size
		limit = DefaultPageSize
		page := parseIntParam(r, "page", 1)
		if page < 1 {
			page = 1
		}
		offset = (page - 1) * limit
	}

	filter := finding.FindingFilter{
		Limit:  limit,
		Offset: offset,
	}

	// Optional filters
	if sev := r.URL.Query().Get("severity"); sev != "" {
		filter.Severity = []finding.Severity{finding.Severity(sev)}
	}
	if activeOnly := r.URL.Query().Get("active_only"); activeOnly == "true" {
		filter.ActiveOnly = true
	}
	if pidStr := r.URL.Query().Get("product_id"); pidStr != "" {
		if pid, err := uuid.Parse(pidStr); err == nil {
			filter.ProductID = &pid
		}
	}

	// Parse status[] — supports ?status[]=active&status[]=resolved (array form from FE)
	// Also supports ?status=active (single form)
	statuses := r.URL.Query()["status[]"]
	if len(statuses) == 0 {
		if s := r.URL.Query().Get("status"); s != "" {
			statuses = []string{s}
		}
	}
	if len(statuses) > 0 {
		filter.Status = statuses
		// Map "active" status → ActiveOnly shortcut: excludes mitigated/FP/accepted/OOS/duplicate
		for _, s := range statuses {
			if s == "active" {
				filter.ActiveOnly = true
				break
			}
		}
	}

	res, err := h.repo.List(r.Context(), filter)
	if err != nil {
		h.log.Error().Err(err).
			Str("product_id", r.URL.Query().Get("product_id")).
			Str("severity", r.URL.Query().Get("severity")).
			Int("limit", limit).
			Int("offset", offset).
			Msg("FindingHandler.List: repo error")
		respondError(w, http.StatusInternalServerError, "failed to list findings")
		return
	}

	// Defensive nil guard
	if res == nil {
		res = &finding.FindingListResult{Findings: []*finding.FindingWithMeta{}}
	}
	if res.Findings == nil {
		res.Findings = []*finding.FindingWithMeta{}
	}

	responses := make([]FindingListItem, 0, len(res.Findings))
	for _, f := range res.Findings {
		responses = append(responses, toFindingListItem(f))
	}

	// Compute page number safely (avoid division by zero)
	page := 1
	if limit > 0 {
		page = (offset / limit) + 1
	}

	slaOK := res.StatusActive - res.SLABreached - res.SLAAtRisk
	if slaOK < 0 {
		slaOK = 0
	}
	slaStats := &SLASummary{
		Breached: res.SLABreached,
		AtRisk:   res.SLAAtRisk,
		OK:       slaOK,
	}

	respondJSON(w, http.StatusOK, &FindingListResponse{
		Findings: responses,
		Total:    res.Total,
		Page:     page,
		PageSize: limit,
		BySeverity: map[string]int{
			"Critical": res.SevCritical,
			"High":     res.SevHigh,
			"Medium":   res.SevMedium,
			"Low":      res.SevLow,
		},
		ByStatus: map[string]int{
			"new":            res.StatusActive,
			"resolved":       res.StatusMitigated,
			"false_positive": res.StatusFP,
			"accepted":       res.StatusRisk,
		},
		SLAStats: slaStats,
	})
}

// Get handles GET /findings/{id} — single finding by UUID.
func (h *FindingHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid finding id")
		return
	}

	f, err := h.repo.FindByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "finding not found")
		return
	}

	respondJSON(w, http.StatusOK, toFindingResponse(f))
}

// Close handles PUT /findings/{id}/close.
func (h *FindingHandler) Close(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid finding id")
		return
	}

	mitigatedBy := r.Header.Get("X-User-ID")
	if err := h.transition.Close(r.Context(), id, mitigatedBy); err != nil {
		h.log.Error().Err(err).Str("finding_id", id.String()).Msg("FindingHandler.Close")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Reopen handles PUT /findings/{id}/reopen.
func (h *FindingHandler) Reopen(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid finding id")
		return
	}

	if err := h.transition.Reopen(r.Context(), id); err != nil {
		h.log.Error().Err(err).Str("finding_id", id.String()).Msg("FindingHandler.Reopen")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// MarkFalsePositive handles PUT /findings/{id}/false-positive.
func (h *FindingHandler) MarkFalsePositive(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid finding id")
		return
	}

	if err := h.transition.MarkFalsePositive(r.Context(), id); err != nil {
		h.log.Error().Err(err).Str("finding_id", id.String()).Msg("FindingHandler.MarkFalsePositive")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// AcceptRisk handles PUT /findings/{id}/risk-accepted.
func (h *FindingHandler) AcceptRisk(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid finding id")
		return
	}

	if err := h.transition.AcceptRisk(r.Context(), id); err != nil {
		h.log.Error().Err(err).Str("finding_id", id.String()).Msg("FindingHandler.AcceptRisk")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ── helpers ──────────────────────────────────────────────────────────────────

func toFindingResponse(f *finding.Finding) *FindingResponse {
	slaStatus, _ := computeSLAStatus(f.SLAExpirationDate)
	status := deriveStatus(f.IsMitigated, f.FalsePositive, f.RiskAccepted, f.OutOfScope, f.Duplicate)

	createdBy := ""
	if f.CreatedBy != nil {
		createdBy = *f.CreatedBy
	}

	resp := &FindingResponse{
		ID:          f.ID.String(),
		Title:       f.Title,
		Description: f.Description,
		Severity:    string(f.Severity),
		CVE:         f.CVE,
		IsKEV:       f.IsKEV,
		Status:      status,
		IsDuplicate: f.Duplicate,
		Active:      f.Active,
		CVSSv3Score: f.CVSSv3Score,
		SLAStatus:   slaStatus,
		CreatedAt:   f.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   f.UpdatedAt.Format(time.RFC3339),
		CreatedBy:   createdBy,
	}
	if f.ProductID != uuid.Nil {
		resp.ProductID = f.ProductID.String()
	}
	if f.EngagementID != uuid.Nil {
		resp.EngagementID = f.EngagementID.String()
	}
	if f.TestID != uuid.Nil {
		resp.TestID = f.TestID.String()
	}
	return resp
}

func toFindingListItem(fm *finding.FindingWithMeta) FindingListItem {
	f := fm.Finding
	cveID := f.CVE
	var pCve *string
	if cveID != "" {
		pCve = &cveID
	}

	slaStatus, daysLeft := computeSLAStatus(f.SLAExpirationDate)
	status := deriveStatus(f.IsMitigated, f.FalsePositive, f.RiskAccepted, f.OutOfScope, f.Duplicate)

	var dupID *string
	if f.DuplicateFindingID != nil {
		idStr := f.DuplicateFindingID.String()
		dupID = &idStr
	}

	var slaStr *string
	if f.SLAExpirationDate != nil {
		s := f.SLAExpirationDate.Format(time.RFC3339)
		slaStr = &s
	}
	var mitStr *string
	if f.MitigatedAt != nil {
		s := f.MitigatedAt.Format(time.RFC3339)
		mitStr = &s
	}

	var compName, compVer *string
	if f.ComponentName != "" {
		compName = &f.ComponentName
	}
	if f.ComponentVersion != "" {
		compVer = &f.ComponentVersion
	}

	return FindingListItem{
		ID:               f.ID.String(),
		Title:            f.Title,
		Description:      f.Description,
		CveID:            pCve,
		Severity:         string(f.Severity),
		CVSSv3:           f.CVSSv3Score,
		EPSSScore:        f.EPSSScore,
		IsKEV:            f.IsKEV,
		Status:           status,
		IsDuplicate:      f.Duplicate,
		DupOfID:          dupID,
		ProductID:        f.ProductID.String(),
		ProductName:      fm.ProductName,
		EngagementID:     f.EngagementID.String(),
		TestID:           f.TestID.String(),
		AssetIP:          f.AssetIP,
		AssetHostname:    f.AssetHostname,
		ComponentName:    compName,
		ComponentVersion: compVer,
		SLAExpiry:        slaStr,
		SLAStatus:        slaStatus,
		SLADaysLeft:      daysLeft,
		CreatedAt:        f.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        f.UpdatedAt.Format(time.RFC3339),
		MitigatedAt:      mitStr,
		AssignedTo:       f.AssignedTo,
		CreatedBy:        f.CreatedBy,
		JiraIssueKey:     fm.JiraIssueKey,
		JiraURL:          fm.JiraURL,
	}
}

// PatchFinding handles PATCH /api/v1/findings/{id} — partial update.
// Supports updating status via operation field, and/or severity.
func (h *FindingHandler) PatchFinding(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid finding id")
		return
	}

	var req struct {
		Operation *string `json:"operation"` // "close", "reopen", "false_positive", "risk_accepted"
		Severity  *string `json:"severity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Fetch existing finding
	f, err := h.repo.FindByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "finding not found")
		return
	}

	userID := r.Header.Get("X-User-ID")

	// Apply operation if provided
	if req.Operation != nil {
		switch *req.Operation {
		case "close", "mitigate":
			if err := h.transition.Close(r.Context(), id, userID); err != nil {
				h.log.Error().Err(err).Str("finding_id", id.String()).Msg("PatchFinding.Close")
				respondError(w, http.StatusInternalServerError, err.Error())
				return
			}
		case "reopen":
			if err := h.transition.Reopen(r.Context(), id); err != nil {
				h.log.Error().Err(err).Str("finding_id", id.String()).Msg("PatchFinding.Reopen")
				respondError(w, http.StatusInternalServerError, err.Error())
				return
			}
		case "false_positive":
			if err := h.transition.MarkFalsePositive(r.Context(), id); err != nil {
				h.log.Error().Err(err).Str("finding_id", id.String()).Msg("PatchFinding.FalsePositive")
				respondError(w, http.StatusInternalServerError, err.Error())
				return
			}
		case "risk_accepted":
			if err := h.transition.AcceptRisk(r.Context(), id); err != nil {
				h.log.Error().Err(err).Str("finding_id", id.String()).Msg("PatchFinding.AcceptRisk")
				respondError(w, http.StatusInternalServerError, err.Error())
				return
			}
		default:
			respondError(w, http.StatusBadRequest, "unknown operation: "+*req.Operation)
			return
		}
	}

	// Apply severity update if provided
	if req.Severity != nil {
		f.Severity = finding.Severity(*req.Severity)
		if err := h.repo.Save(r.Context(), f); err != nil {
			h.log.Error().Err(err).Str("finding_id", id.String()).Msg("PatchFinding.Save")
			respondError(w, http.StatusInternalServerError, "failed to update severity")
			return
		}
	}

	// Re-fetch updated finding for response
	updated, err := h.repo.FindByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to fetch updated finding")
		return
	}
	respondJSON(w, http.StatusOK, toFindingResponse(updated))
}
