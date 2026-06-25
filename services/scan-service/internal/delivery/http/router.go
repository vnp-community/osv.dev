package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/osv/scan-service/internal/domain/entity"
	schedulehttp "github.com/osv/scan-service/internal/delivery/http/schedule"
)

// ScanListResponse is the paginated response for GET /api/v1/scans.
type ScanListResponse struct {
	Scans     []interface{}          `json:"scans"`
	Total     int64                  `json:"total"`
	Page      int                    `json:"page"`
	PageSize  int                    `json:"page_size"`
	Stats     map[string]interface{} `json:"stats"`
}

// ScanRepository is the minimal interface needed by the delivery layer.
// Implemented by adapters/repository/postgres.ScanRepo.
type ScanRepository interface {
	ListRaw(ctx interface{}, page, pageSize int, status string) ([]interface{}, int64, error)
	FindByIDRaw(ctx interface{}, id string) (interface{}, error)
	CancelRaw(ctx interface{}, id string) error
}

// CreateScanRequest is the parsed create-scan request body from the API.
type CreateScanRequest struct {
	Name     string                 `json:"name"`
	Type     string                 `json:"type"`
	Targets  []string               `json:"targets"`
	Options  map[string]interface{} `json:"options"`
	Priority int                    `json:"priority"`
}

// CreateScanUseCase is the minimal use case interface for scan creation.
type CreateScanUseCase interface {
	Execute(ctx context.Context, userID uuid.UUID, req CreateScanRequest) (map[string]interface{}, error)
}

// ScanAPIHandler handles /api/v1/scans HTTP endpoints.
// When repo is nil (not yet wired), it returns graceful empty responses.
type ScanAPIHandler struct {
	repo         ScanRepository    // optional; nil = return empty
	createScanUC CreateScanUseCase // optional; nil = 503
	log          zerolog.Logger
}

// NewScanAPIHandler creates a handler. repo may be nil for a stub mode.
func NewScanAPIHandler(repo ScanRepository, log zerolog.Logger) *ScanAPIHandler {
	return &ScanAPIHandler{repo: repo, log: log}
}

// WithCreateScanUC attaches a CreateScanUseCase to the handler.
func (h *ScanAPIHandler) WithCreateScanUC(uc CreateScanUseCase) *ScanAPIHandler {
	h.createScanUC = uc
	return h
}

// ListScans handles GET /api/v1/scans
func (h *ScanAPIHandler) ListScans(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parseScanPagination(r)
	status := r.URL.Query().Get("status")

	if h.repo == nil {
		// Graceful empty response when DB not wired
		writeScanJSON(w, http.StatusOK, ScanListResponse{
			Scans: []interface{}{}, Total: 0, Page: page, PageSize: pageSize,
			Stats: map[string]interface{}{"active_scans": 0, "completed_today": 0,
				"total_findings": 0, "scheduled_scans": 0},
		})
		return
	}

	scans, total, err := h.repo.ListRaw(r.Context(), page, pageSize, status)
	if err != nil {
		h.log.Error().Err(err).Msg("list scans failed")
		writeScanJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list scans"})
		return
	}
	if scans == nil {
		scans = []interface{}{}
	}
	writeScanJSON(w, http.StatusOK, ScanListResponse{
		Scans: scans, Total: total, Page: page, PageSize: pageSize,
		Stats: map[string]interface{}{"active_scans": 0, "completed_today": 0,
			"total_findings": 0, "scheduled_scans": 0},
	})
}

// ListScheduled handles GET /api/v1/scans/scheduled
func (h *ScanAPIHandler) ListScheduled(w http.ResponseWriter, r *http.Request) {
	writeScanJSON(w, http.StatusOK, map[string]interface{}{
		"scheduled_scans": []interface{}{},
		"total":           0,
	})
}

// GetStats handles GET /api/v1/scans/stats
// Replaced by StatsHandler.HandleStats when a StatsHandler is wired.
func (h *ScanAPIHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	// Graceful empty response since stats logic is decoupled or not wired
	writeScanJSON(w, http.StatusOK, &ScanStats{
		Total: 0, Running: 0, Completed: 0, Failed: 0, Pending: 0, Cancelled: 0,
	})
}

// GetScan handles GET /api/v1/scans/{id}
func (h *ScanAPIHandler) GetScan(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if h.repo == nil {
		writeScanJSON(w, http.StatusNotFound, map[string]string{"error": "scan not found"})
		return
	}
	scan, err := h.repo.FindByIDRaw(r.Context(), id)
	if err != nil {
		writeScanJSON(w, http.StatusNotFound, map[string]string{"error": "scan not found"})
		return
	}
	writeScanJSON(w, http.StatusOK, scan)
}

// CreateScan handles POST /api/v1/scans
func (h *ScanAPIHandler) CreateScan(w http.ResponseWriter, r *http.Request) {
	var req CreateScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeScanJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if len(req.Targets) == 0 {
		writeScanJSON(w, http.StatusBadRequest, map[string]string{"error": "targets is required"})
		return
	}
	if h.createScanUC == nil {
		writeScanJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error":   "scan_service_not_ready",
			"message": "Scan service backend not fully initialized",
		})
		return
	}
	// Extract user ID from gateway-injected header
	userIDStr := r.Header.Get("X-User-ID")
	userID, _ := uuid.Parse(userIDStr) // zero UUID if missing — repo validates

	result, err := h.createScanUC.Execute(r.Context(), userID, req)
	if err != nil {
		h.log.Error().Err(err).Msg("create scan failed")
		writeScanJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeScanJSON(w, http.StatusCreated, result)
}

// ParseScanType maps frontend type strings to domain ScanType.
func ParseScanType(t string) entity.ScanType {
	switch t {
	case "nmap_discovery":
		return entity.ScanTypeDiscovery
	case "nmap_full", "full":
		return entity.ScanTypeFull
	case "web", "zap":
		return entity.ScanTypeWeb
	default:
		return entity.ScanTypeDiscovery
	}
}

// ImportScan handles POST /api/v1/scans/import
// [FIX TASK-HC-011] Returns 501 Not Implemented — import feature is planned, not yet built.
func (h *ScanAPIHandler) ImportScan(w http.ResponseWriter, r *http.Request) {
	writeScanJSON(w, http.StatusNotImplemented, map[string]string{
		"error":   "not_implemented",
		"feature": "scan-import",
		"message": "Scan import feature is planned for a future release",
	})
}

// CancelScan handles POST /api/v1/scans/{id}/cancel
func (h *ScanAPIHandler) CancelScan(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if h.repo == nil {
		writeScanJSON(w, http.StatusNotFound, map[string]string{"error": "scan not found"})
		return
	}
	if err := h.repo.CancelRaw(r.Context(), id); err != nil {
		writeScanJSON(w, http.StatusNotFound, map[string]string{"error": "scan not found or cannot be cancelled"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetScanHistory handles GET /api/v1/scans/history — TASK-009 FIX
// Returns completed/failed/cancelled scans (terminal status).
// CRITICAL: this literal route MUST be registered BEFORE /api/v1/scans/{id}
// to prevent chi from capturing "history" as an ID value.
func (h *ScanAPIHandler) GetScanHistory(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parseScanPagination(r)

	// Allow status override; default to terminal statuses
	statusFilter := r.URL.Query().Get("status")
	if statusFilter == "" {
		statusFilter = "completed"
	}

	if h.repo == nil {
		writeScanJSON(w, http.StatusOK, map[string]interface{}{
			"scans": []interface{}{},
			"total": 0,
			"page":      page,
			"page_size": pageSize,
		})
		return
	}

	scans, total, err := h.repo.ListRaw(r.Context(), page, pageSize, statusFilter)
	if err != nil {
		h.log.Error().Err(err).Msg("GetScanHistory failed")
		writeScanJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to fetch scan history"})
		return
	}
	if scans == nil {
		scans = []interface{}{}
	}
	writeScanJSON(w, http.StatusOK, map[string]interface{}{
		"scans":     scans,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func parseScanPagination(r *http.Request) (page, pageSize int) {
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ = strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return
}

func writeScanJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

// NewRouter sets up the chi router for scan-service HTTP endpoints.
func NewRouter(
	importHandler   *ImportHandler,
	parserHandler   *ParserHandler,
	agentHandler    *AgentHandler,
	log             zerolog.Logger,
) http.Handler {
	return NewRouterFull(importHandler, parserHandler, agentHandler, nil, nil, nil, nil, log)
}

// NewRouterWithScan builds the router with optional ScanAPIHandler for /api/v1/scans.
func NewRouterWithScan(
	importHandler   *ImportHandler,
	parserHandler   *ParserHandler,
	agentHandler    *AgentHandler,
	scanHandler     *ScanAPIHandler,
	scheduleHandler *schedulehttp.ScheduleHandler,
	log             zerolog.Logger,
) http.Handler {
	return NewRouterFull(importHandler, parserHandler, agentHandler, scanHandler, scheduleHandler, nil, nil, log)
}

// NewRouterFull builds the router with all optional handlers including StatsHandler.
func NewRouterFull(
	importHandler   *ImportHandler,
	parserHandler   *ParserHandler,
	agentHandler    *AgentHandler,
	scanHandler     *ScanAPIHandler,
	scheduleHandler *schedulehttp.ScheduleHandler,
	statsHandler    *StatsHandler,
	resultsHandler  *ResultsHandler,
	log             zerolog.Logger,
) http.Handler {
	r := chi.NewRouter()

	// Setup CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// ── /api/v1/scans — scan CRUD endpoints ──────────────────────────────────
	// Use provided handler or default stub (returns graceful empty responses)
	sh := scanHandler
	if sh == nil {
		sh = NewScanAPIHandler(nil, log)
	}
	r.Route("/api/v1", func(r chi.Router) {
		// Literal paths BEFORE wildcard {id}
		// SEED-005-C: Scheduled scan CRUD — mount ScheduleHandler if provided, else stub
		if scheduleHandler != nil {
			r.Post("/scans/scheduled", scheduleHandler.CreateSchedule)
			r.Get("/scans/scheduled", scheduleHandler.ListSchedules)
			r.Get("/scans/scheduled/{id}", scheduleHandler.GetSchedule)
			r.Put("/scans/scheduled/{id}", scheduleHandler.UpdateSchedule)
			r.Delete("/scans/scheduled/{id}", scheduleHandler.DeleteSchedule)
		} else {
			r.Get("/scans/scheduled", sh.ListScheduled) // stub fallback
		}
		r.Get("/scans", sh.ListScans)
		r.Post("/scans", sh.CreateScan)
		r.Post("/scans/import", sh.ImportScan)

		// TASK-009 FIX: /scans/history MUST be before /scans/{id} to prevent shadowing
		r.Get("/scans/history", sh.GetScanHistory)

		// TASK-008: Stats routes — literal paths MUST be BEFORE /{id}
		// Use StatsHandler if wired, else fall back to ScanAPIHandler stub
		if statsHandler != nil {
			r.Get("/scans/stats/weekly", statsHandler.HandleWeeklyStats) // literal BEFORE /{id}
			r.Get("/scans/stats", statsHandler.HandleStats)               // literal BEFORE /{id}
		} else {
			r.Get("/scans/stats/weekly", func(w http.ResponseWriter, _ *http.Request) {
				writeScanJSON(w, http.StatusOK, map[string]interface{}{"data": []DailyStats{}})
			})
			r.Get("/scans/stats", sh.GetStats) // ScanAPIHandler stub
		}
		// FIX BUG: /scans/{id}/results/nmap and /scans/{id}/results/zap
		// CRITICAL: these MUST be registered BEFORE /scans/{id} to avoid chi
		// capturing the literal "results" segment as the {id} value.
		rh := resultsHandler
		if rh == nil {
			// Graceful stub: always returns empty results (no DB, no panic)
			rh = NewResultsHandler(nil, nil, log)
		}
		r.Get("/scans/{id}/results/nmap", rh.GetNmapResults)
		r.Get("/scans/{id}/results/zap", rh.GetZapResults)

		r.Get("/scans/{id}", sh.GetScan)
		r.Post("/scans/{id}/cancel", sh.CancelScan)

		// Agent routes (SEED-005-C)
		if agentHandler != nil {
			r.Post("/agents", agentHandler.RegisterAgent)
			r.Get("/agents", agentHandler.ListAgents)
			r.Get("/agents/{id}", agentHandler.GetAgent)
			r.Post("/agents/{id}/reports", agentHandler.SubmitReport)
		}
	})

	// API v2 routes
	r.Route("/api/v2", func(r chi.Router) {
		if importHandler != nil {
			r.Post("/reimport-scan", importHandler.ReimportScan)
			r.Get("/test-imports/{id}", importHandler.GetTestImport)
		}
		if parserHandler != nil {
			r.Get("/parsers", parserHandler.ListParsers)
		}
	})

	return r
}

