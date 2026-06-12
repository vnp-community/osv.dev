package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
	createscan "github.com/osv/scan-service/internal/usecase/create_scan"
	executescan "github.com/osv/scan-service/internal/usecase/execute_scan"
	"github.com/osv/scan-service/internal/domain/entity"
	"github.com/osv/scan-service/internal/domain/repository"
	"github.com/osv/scan-service/internal/adapters/worker"
	"github.com/rs/zerolog"
)

// ScanHandler handles HTTP requests for scan operations.
type ScanHandler struct {
	createUC *createscan.UseCase
	executeUC *executescan.UseCase
	scanRepo  repository.ScanRepository
	findingRepo repository.FindingRepository
	pool      *worker.WorkerPool
	log       zerolog.Logger
}

func NewScanHandler(
	createUC *createscan.UseCase,
	executeUC *executescan.UseCase,
	scanRepo repository.ScanRepository,
	findingRepo repository.FindingRepository,
	pool *worker.WorkerPool,
	log zerolog.Logger,
) *ScanHandler {
	return &ScanHandler{
		createUC: createUC, executeUC: executeUC,
		scanRepo: scanRepo, findingRepo: findingRepo,
		pool: pool, log: log,
	}
}

// CreateScan handles POST /scans → 202 Accepted
func (h *ScanHandler) CreateScan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Targets  []string          `json:"targets"`
		ScanType entity.ScanType   `json:"scan_type"`
		Priority int               `json:"priority"`
		Options  entity.ScanOptions `json:"options"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("invalid_request", err.Error()))
		return
	}

	userIDStr := r.Header.Get("X-User-ID")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp("unauthorized", "missing user context"))
		return
	}

	resp, err := h.createUC.Execute(r.Context(), createscan.Request{
		UserID: userID, Targets: req.Targets,
		ScanType: req.ScanType, Priority: req.Priority, Options: req.Options,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("create_failed", err.Error()))
		return
	}

	// Submit to worker pool asynchronously
	h.pool.Submit(worker.ScanJob{ScanID: resp.ScanID, UserID: userID}) //nolint:errcheck

	writeJSON(w, http.StatusAccepted, map[string]any{
		"scan_id":   resp.ScanID,
		"status":    resp.Status,
		"targets":   resp.Targets,
		"scan_type": resp.ScanType,
		"message":   "scan queued",
	})
}

// GetScan handles GET /scans/{id}
func (h *ScanHandler) GetScan(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("invalid_id", "id must be a UUID"))
		return
	}
	scan, err := h.scanRepo.FindByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errResp("not_found", "scan not found"))
		return
	}
	writeJSON(w, http.StatusOK, scan)
}

// ListScans handles GET /scans
func (h *ScanHandler) ListScans(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.Header.Get("X-User-ID")
	userID, _ := uuid.Parse(userIDStr)
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

	filter := repository.ScanFilter{Page: page, PageSize: pageSize}
	if userID != uuid.Nil { filter.UserID = &userID }

	scans, total, err := h.scanRepo.List(r.Context(), filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("list_failed", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"scans": scans, "total": total})
}

// CancelScan handles DELETE /scans/{id}
func (h *ScanHandler) CancelScan(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("invalid_id", "id must be a UUID"))
		return
	}
	h.pool.Cancel(id)
	h.scanRepo.UpdateStatus(r.Context(), id, entity.ScanStatusCancelled) //nolint:errcheck
	writeJSON(w, http.StatusOK, map[string]string{"message": "scan cancelled"})
}

// GetFindings handles GET /scans/{id}/findings
func (h *ScanHandler) GetFindings(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("invalid_id", "id must be a UUID"))
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	findings, total, err := h.findingRepo.FindByScanID(r.Context(), id, page, pageSize)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("query_failed", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"findings": findings, "total": total})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func errResp(code, msg string) map[string]string {
	return map[string]string{"error": code, "message": msg}
}

// NewRouter builds the scan service router.
func NewRouter(h *ScanHandler, log zerolog.Logger) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{AllowedOrigins: []string{"*"}, AllowedMethods: []string{"GET","POST","DELETE","OPTIONS"}}))

	r.Get("/health/live", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/scans", h.CreateScan)
		r.Get("/scans", h.ListScans)
		r.Get("/scans/{id}", h.GetScan)
		r.Delete("/scans/{id}", h.CancelScan)
		r.Get("/scans/{id}/findings", h.GetFindings)
	})
	return r
}
