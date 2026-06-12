package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
	"github.com/osv/agent-service/internal/domain/entity"
	"github.com/osv/agent-service/internal/domain/repository"
	submitreport "github.com/osv/agent-service/internal/usecase/submit_report"
	"github.com/rs/zerolog"
)

// AgentHandler handles HTTP requests for agent management and report submission.
type AgentHandler struct {
	submitUC   *submitreport.UseCase
	agentRepo  repository.AgentRepository
	reportRepo repository.AgentReportRepository
	log        zerolog.Logger
}

func NewAgentHandler(
	submitUC *submitreport.UseCase,
	agentRepo repository.AgentRepository,
	reportRepo repository.AgentReportRepository,
	log zerolog.Logger,
) *AgentHandler {
	return &AgentHandler{submitUC: submitUC, agentRepo: agentRepo, reportRepo: reportRepo, log: log}
}

// SubmitReport POST /agents/report — called by agent binary using API key auth
func (h *AgentHandler) SubmitReport(w http.ResponseWriter, r *http.Request) {
	// API key validation comes from X-API-Key-ID header (set by api-gateway after ValidateAPIKey)
	apiKeyIDStr := r.Header.Get("X-API-Key-ID")
	userIDStr   := r.Header.Get("X-User-ID")
	apiKeyID, err := uuid.Parse(apiKeyIDStr)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp("unauthorized", "missing API key context"))
		return
	}
	userID, _ := uuid.Parse(userIDStr)

	var req struct {
		Hostname      string           `json:"hostname"`
		IPAddress     string           `json:"ip_address"`
		OSInfo        string           `json:"os_info"`
		KernelVersion string           `json:"kernel_version"`
		Packages      []entity.Package `json:"packages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("invalid_request", err.Error()))
		return
	}

	out, err := h.submitUC.Execute(r.Context(), submitreport.Input{
		APIKeyID:      apiKeyID,
		UserID:        userID,
		Hostname:      req.Hostname,
		IPAddress:     req.IPAddress,
		OSInfo:        req.OSInfo,
		KernelVersion: req.KernelVersion,
		Packages:      req.Packages,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("submit_failed", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// ListAgents GET /agents
func (h *AgentHandler) ListAgents(w http.ResponseWriter, r *http.Request) {
	filter := repository.AgentFilter{Page: 1, PageSize: 20}
	agents, total, err := h.agentRepo.List(r.Context(), filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("list_failed", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"agents": agents, "total": total})
}

// GetAgent GET /agents/{id}
func (h *AgentHandler) GetAgent(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("invalid_id", "id must be a UUID"))
		return
	}
	agent, err := h.agentRepo.FindByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errResp("not_found", "agent not found"))
		return
	}
	writeJSON(w, http.StatusOK, agent)
}

// DeleteAgent DELETE /agents/{id}
func (h *AgentHandler) DeleteAgent(w http.ResponseWriter, r *http.Request) {
	id, _ := uuid.Parse(chi.URLParam(r, "id"))
	if err := h.agentRepo.Delete(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("delete_failed", err.Error()))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListReportsByAgent GET /agents/{id}/reports
func (h *AgentHandler) ListReportsByAgent(w http.ResponseWriter, r *http.Request) {
	id, _ := uuid.Parse(chi.URLParam(r, "id"))
	reports, total, err := h.reportRepo.FindByAgentID(r.Context(), id, 1, 20)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("list_failed", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reports": reports, "total": total})
}

// GetLatestReport GET /agents/{id}/reports/latest
func (h *AgentHandler) GetLatestReport(w http.ResponseWriter, r *http.Request) {
	id, _ := uuid.Parse(chi.URLParam(r, "id"))
	report, err := h.reportRepo.FindLatestByAgentID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errResp("not_found", "no reports found"))
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func errResp(code, msg string) map[string]string {
	return map[string]string{"error": code, "message": msg}
}

// NewRouter builds the agent service chi router.
func NewRouter(h *AgentHandler, log zerolog.Logger) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "DELETE", "OPTIONS"},
	}))

	r.Get("/health/live",  func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	r.Get("/health/ready", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })

	// Agent report submission (API key auth via X-API-Key-ID header)
	r.Post("/agents/report", h.SubmitReport)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/agents", h.ListAgents)
		r.Get("/agents/{id}", h.GetAgent)
		r.Delete("/agents/{id}", h.DeleteAgent)
		r.Get("/agents/{id}/reports", h.ListReportsByAgent)
		r.Get("/agents/{id}/reports/latest", h.GetLatestReport)
	})
	return r
}
