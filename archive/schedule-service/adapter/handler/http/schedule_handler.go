package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
	"github.com/osv/schedule-service/internal/domain/entity"
	"github.com/osv/schedule-service/internal/domain/repository"
	"github.com/rs/zerolog"
)

// ScheduleHandler handles HTTP requests for schedule management.
type ScheduleHandler struct {
	schedRepo repository.ScheduleRepository
	log       zerolog.Logger
}

func NewScheduleHandler(schedRepo repository.ScheduleRepository, log zerolog.Logger) *ScheduleHandler {
	return &ScheduleHandler{schedRepo: schedRepo, log: log}
}

// CreateSchedule POST /schedules
func (h *ScheduleHandler) CreateSchedule(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.Header.Get("X-User-ID")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp("unauthorized", "missing user context"))
		return
	}

	var req struct {
		Targets         []string        `json:"targets"`
		ScanType        string          `json:"scan_type"`
		CronExpr        string          `json:"cron_expr"`
		IntervalMinutes int             `json:"interval_minutes"`
		Options         json.RawMessage `json:"options"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("invalid_request", err.Error()))
		return
	}
	if req.CronExpr == "" && req.IntervalMinutes <= 0 {
		writeJSON(w, http.StatusBadRequest, errResp("validation_error", "cron_expr or interval_minutes is required"))
		return
	}

	s := &entity.ScheduledScan{
		UserID:          userID,
		Targets:         req.Targets,
		ScanType:        req.ScanType,
		CronExpr:        req.CronExpr,
		IntervalMinutes: req.IntervalMinutes,
		Status:          "active",
		Options:         req.Options,
	}
	s.NextRunAt = s.CalculateNextRun(time.Now().UTC())
	if err := h.schedRepo.Create(r.Context(), s); err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("create_failed", err.Error()))
		return
	}
	writeJSON(w, http.StatusCreated, s)
}

// ListSchedules GET /schedules
func (h *ScheduleHandler) ListSchedules(w http.ResponseWriter, r *http.Request) {
	userID, _ := uuid.Parse(r.Header.Get("X-User-ID"))
	schedules, total, err := h.schedRepo.List(r.Context(), userID, 1, 20)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("list_failed", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"schedules": schedules, "total": total})
}

// GetSchedule GET /schedules/{id}
func (h *ScheduleHandler) GetSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("invalid_id", "id must be a UUID"))
		return
	}
	s, err := h.schedRepo.FindByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errResp("not_found", "schedule not found"))
		return
	}
	writeJSON(w, http.StatusOK, s)
}

// UpdateSchedule PUT /schedules/{id}
func (h *ScheduleHandler) UpdateSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("invalid_id", "id must be a UUID"))
		return
	}
	var req struct { Status string `json:"status"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("invalid_request", err.Error()))
		return
	}
	if err := h.schedRepo.UpdateStatus(r.Context(), id, req.Status); err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("update_failed", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "schedule updated"})
}

// DeleteSchedule DELETE /schedules/{id}
func (h *ScheduleHandler) DeleteSchedule(w http.ResponseWriter, r *http.Request) {
	id, _ := uuid.Parse(chi.URLParam(r, "id"))
	if err := h.schedRepo.Delete(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("delete_failed", err.Error()))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func errResp(code, msg string) map[string]string {
	return map[string]string{"error": code, "message": msg}
}

// NewRouter builds the schedule service chi router.
func NewRouter(h *ScheduleHandler, log zerolog.Logger) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
	}))

	r.Get("/health/live",  func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	r.Get("/health/ready", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/schedules", h.CreateSchedule)
		r.Get("/schedules", h.ListSchedules)
		r.Get("/schedules/{id}", h.GetSchedule)
		r.Put("/schedules/{id}", h.UpdateSchedule)
		r.Delete("/schedules/{id}", h.DeleteSchedule)
	})
	return r
}
