// Package http provides HTTP delivery layer for scan-service.
package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// Dummy agentUseCase interface for SEED-005-C
type agentUseCase interface {
	Register(ctx context.Context, req AgentRegisterReq) (AgentDto, string, error)
	List(ctx context.Context) ([]AgentDto, error)
	SubmitReport(ctx context.Context, agentID uuid.UUID, report AgentReportPayload) (ReportResultDto, error)
}

type AgentRegisterReq struct {
	Name      string   `json:"name"`
	Hostname  string   `json:"hostname"`
	IPAddress string   `json:"ip_address"`
	OS        string   `json:"os"`
	Tags      []string `json:"tags"`
}

type AgentDto struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Hostname  string    `json:"hostname"`
	IPAddress string    `json:"ip_address"`
	OS        string    `json:"os"`
	Tags      []string  `json:"tags"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type AgentReportPayload struct {
	Packages []map[string]any `json:"packages"`
}

type ReportResultDto struct {
	ID uuid.UUID
}

// AgentHandler handles agent registration and reports.
type AgentHandler struct {
	agentUC agentUseCase // Optional for now
	log     zerolog.Logger
}

// NewAgentHandler creates an AgentHandler.
func NewAgentHandler(uc agentUseCase, log zerolog.Logger) *AgentHandler {
	return &AgentHandler{agentUC: uc, log: log}
}

// RegisterAgent handles POST /api/v1/agents.
func (h *AgentHandler) RegisterAgent(w http.ResponseWriter, r *http.Request) {
	var req AgentRegisterReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, 400, map[string]string{"error": "invalid_body", "detail": err.Error()})
		return
	}

	// Fake implementation for seed purposes
	apiKeyPlaintext := "ak_live_" + uuid.New().String()
	agent := AgentDto{
		ID:        uuid.New(),
		Name:      req.Name,
		Hostname:  req.Hostname,
		IPAddress: req.IPAddress,
		OS:        req.OS,
		Tags:      req.Tags,
		Status:    "inactive",
		CreatedAt: time.Now().UTC(),
	}

	h.writeJSON(w, 201, map[string]any{
		"id":         agent.ID,
		"name":       agent.Name,
		"hostname":   agent.Hostname,
		"ip_address": agent.IPAddress,
		"os":         agent.OS,
		"tags":       agent.Tags,
		"api_key":    apiKeyPlaintext, // ONE-TIME ONLY
		"status":     agent.Status,
		"created_at": agent.CreatedAt,
	})
}

// ListAgents handles GET /api/v1/agents.
func (h *AgentHandler) ListAgents(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, 200, map[string]any{"agents": []AgentDto{}, "count": 0})
}

// GetAgent handles GET /api/v1/agents/{id}.
func (h *AgentHandler) GetAgent(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeJSON(w, 400, map[string]string{"error": "invalid_id"})
		return
	}
	h.writeJSON(w, 200, AgentDto{ID: id})
}

// SubmitReport handles POST /api/v1/agents/{id}/reports.
func (h *AgentHandler) SubmitReport(w http.ResponseWriter, r *http.Request) {
	agentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeJSON(w, 400, map[string]string{"error": "invalid_id", "detail": "invalid agent UUID"})
		return
	}

	var report AgentReportPayload
	if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
		h.writeJSON(w, 400, map[string]string{"error": "invalid_body", "detail": err.Error()})
		return
	}

	h.writeJSON(w, 202, map[string]any{
		"report_id":     uuid.New(),
		"agent_id":      agentID,
		"package_count": len(report.Packages),
		"status":        "queued_for_processing",
	})
}

func (h *AgentHandler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
