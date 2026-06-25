// Package http — tool_handler.go
// ToolHandler handles HTTP REST endpoints for ToolConfiguration entities.
// Passwords and API keys are NEVER returned in responses.
package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/osv/finding-service/internal/domain/tool"
	tool_uc "github.com/osv/finding-service/internal/usecase/tool"
)

// ToolHandler handles tool configuration management.
type ToolHandler struct {
	createUC *tool_uc.CreateToolConfigUseCase
	updateUC *tool_uc.UpdateToolConfigUseCase
	deleteUC *tool_uc.DeleteToolConfigUseCase
	getUC    *tool_uc.GetToolConfigUseCase
	listUC   *tool_uc.ListToolConfigsUseCase
	log      zerolog.Logger
}

// NewToolHandler creates a new ToolHandler.
func NewToolHandler(
	create *tool_uc.CreateToolConfigUseCase,
	update *tool_uc.UpdateToolConfigUseCase,
	delete *tool_uc.DeleteToolConfigUseCase,
	get *tool_uc.GetToolConfigUseCase,
	list *tool_uc.ListToolConfigsUseCase,
	log zerolog.Logger,
) *ToolHandler {
	return &ToolHandler{
		createUC: create,
		updateUC: update,
		deleteUC: delete,
		getUC:    get,
		listUC:   list,
		log:      log,
	}
}

// RegisterRoutes registers all tool-configuration routes on the router.
func (h *ToolHandler) RegisterRoutes(r chi.Router) {
	r.Get("/api/v2/tool-configurations", h.List)
	r.Post("/api/v2/tool-configurations", h.Create)
	r.Get("/api/v2/tool-configurations/{id}", h.Get)
	r.Put("/api/v2/tool-configurations/{id}", h.Update)
	r.Delete("/api/v2/tool-configurations/{id}", h.Delete)
}

// toolResponse is the JSON representation of a ToolConfiguration.
// IMPORTANT: password and api_key are always masked as "***".
type toolResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ToolType    string `json:"tool_type"`
	URL         string `json:"url"`
	AuthType    string `json:"authentication_type"`
	Username    string `json:"username"`
	Password    string `json:"password"`   // always "***"
	APIKey      string `json:"api_key"`    // always "***"
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func toToolResponse(tc *tool.ToolConfiguration) *toolResponse {
	return &toolResponse{
		ID:          tc.ID.String(),
		Name:        tc.Name,
		Description: tc.Description,
		ToolType:    tc.ToolType,
		URL:         tc.URL,
		AuthType:    string(tc.AuthType),
		Username:    tc.Username,
		Password:    "***",
		APIKey:      "***",
		CreatedAt:   tc.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   tc.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// List handles GET /api/v2/tool-configurations
func (h *ToolHandler) List(w http.ResponseWriter, r *http.Request) {
	tools, err := h.listUC.Execute(r.Context())
	if err != nil {
		h.log.Error().Err(err).Msg("ToolHandler.List")
		respondError(w, http.StatusInternalServerError, "failed to list tool configurations")
		return
	}
	responses := make([]*toolResponse, 0, len(tools))
	for _, tc := range tools {
		responses = append(responses, toToolResponse(tc))
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"count":   len(responses),
		"results": responses,
	})
}

// Get handles GET /api/v2/tool-configurations/{id}
func (h *ToolHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid tool configuration id")
		return
	}
	tc, err := h.getUC.Execute(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "tool configuration not found")
		return
	}
	respondJSON(w, http.StatusOK, toToolResponse(tc))
}

// Create handles POST /api/v2/tool-configurations
// Request: {"name":"...", "tool_type":"GitHub", "url":"...", "authentication_type":"api_key", "password":"..."}
func (h *ToolHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		ToolType    string `json:"tool_type"`
		URL         string `json:"url"`
		AuthType    string `json:"authentication_type"`
		Username    string `json:"username"`
		Password    string `json:"password"`
		APIKey      string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	tc, err := h.createUC.Execute(r.Context(), tool_uc.CreateToolConfigInput{
		Name:        req.Name,
		Description: req.Description,
		ToolType:    req.ToolType,
		URL:         req.URL,
		AuthType:    tool.AuthType(req.AuthType),
		Username:    req.Username,
		Password:    req.Password,
		APIKey:      req.APIKey,
	})
	if err != nil {
		h.log.Error().Err(err).Msg("ToolHandler.Create")
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, toToolResponse(tc))
}

// Update handles PUT /api/v2/tool-configurations/{id}
func (h *ToolHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid tool configuration id")
		return
	}
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		ToolType    string `json:"tool_type"`
		URL         string `json:"url"`
		AuthType    string `json:"authentication_type"`
		Username    string `json:"username"`
		Password    string `json:"password"`
		APIKey      string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	tc, err := h.updateUC.Execute(r.Context(), tool_uc.UpdateToolConfigInput{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		ToolType:    req.ToolType,
		URL:         req.URL,
		AuthType:    tool.AuthType(req.AuthType),
		Username:    req.Username,
		Password:    req.Password,
		APIKey:      req.APIKey,
	})
	if err != nil {
		if err == tool_uc.ErrNotFound {
			respondError(w, http.StatusNotFound, "tool configuration not found")
			return
		}
		h.log.Error().Err(err).Msg("ToolHandler.Update")
		respondError(w, http.StatusInternalServerError, "failed to update tool configuration")
		return
	}
	respondJSON(w, http.StatusOK, toToolResponse(tc))
}

// Delete handles DELETE /api/v2/tool-configurations/{id}
func (h *ToolHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid tool configuration id")
		return
	}
	if err := h.deleteUC.Execute(r.Context(), id); err != nil {
		if err == tool_uc.ErrNotFound {
			respondError(w, http.StatusNotFound, "tool configuration not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to delete tool configuration")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
