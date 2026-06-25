// Package http provides HTTP handlers for SLA configuration endpoints.
package http

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	ucconfig "github.com/osv/sla-service/internal/usecase/config"
)

// SLAConfigHandler handles /api/v2/sla-configurations endpoints.
type SLAConfigHandler struct {
	createUC *ucconfig.CreateSLAConfigUseCase
	updateUC *ucconfig.UpdateSLAConfigUseCase
	deleteUC *ucconfig.DeleteSLAConfigUseCase
	assignUC *ucconfig.AssignProductUseCase
	repo     ucconfig.Repository
}

// NewSLAConfigHandler creates a new handler.
func NewSLAConfigHandler(
	cr *ucconfig.CreateSLAConfigUseCase,
	ur *ucconfig.UpdateSLAConfigUseCase,
	dr *ucconfig.DeleteSLAConfigUseCase,
	ar *ucconfig.AssignProductUseCase,
	repo ucconfig.Repository,
) *SLAConfigHandler {
	return &SLAConfigHandler{createUC: cr, updateUC: ur, deleteUC: dr, assignUC: ar, repo: repo}
}

// GlobalConfig represents the global SLA days for each severity.
type GlobalConfig struct {
	CriticalDays int `json:"critical_days"`
	HighDays     int `json:"high_days"`
	MediumDays   int `json:"medium_days"`
	LowDays      int `json:"low_days"`
}

// ProductOverride represents SLA days overridden for a specific product.
type ProductOverride struct {
	ProductID    string `json:"product_id"`
	CriticalDays int    `json:"critical_days"`
	HighDays     int    `json:"high_days"`
	MediumDays   int    `json:"medium_days"`
	LowDays      int    `json:"low_days"`
}

// SLAConfigResponse is the JSON schema for GET and PUT /api/v1/sla/config.
type SLAConfigResponse struct {
	Global           GlobalConfig      `json:"global"`
	ProductOverrides []ProductOverride `json:"product_overrides"`
}

// GetConfig handles GET /api/v1/sla/config
// FIX: returns { global: { critical_days, high_days, medium_days, low_days }, product_overrides: [] }
// Falls back to technical design defaults if no config exists.
func (h *SLAConfigHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	// Try to get the default (is_default=true) SLA configuration
	cfg, err := h.repo.FindDefault(r.Context())

	// Use defaults from technical-design.md §5.3 if none configured
	criticalDays, highDays, mediumDays, lowDays := 7, 30, 90, 180
	if err == nil && cfg != nil {
		criticalDays = cfg.Critical
		highDays     = cfg.High
		mediumDays   = cfg.Medium
		lowDays      = cfg.Low
	}

	writeJSON(w, http.StatusOK, SLAConfigResponse{
		Global: GlobalConfig{
			CriticalDays: criticalDays,
			HighDays:     highDays,
			MediumDays:   mediumDays,
			LowDays:      lowDays,
		},
		// product_overrides populated from assignment table in future
		ProductOverrides: []ProductOverride{},
	})
}

// UpdateConfig handles PUT /api/v1/sla/config
// Updates the default SLA configuration.
func (h *SLAConfigHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Global GlobalConfig `json:"global"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiErr("invalid request body"))
		return
	}

	// Validate ordering: critical ≤ high ≤ medium ≤ low
	g := req.Global
	if g.CriticalDays <= 0 || g.HighDays <= 0 || g.MediumDays <= 0 || g.LowDays <= 0 {
		writeJSON(w, http.StatusBadRequest, apiErr("all day values must be positive"))
		return
	}
	if g.CriticalDays > g.HighDays || g.HighDays > g.MediumDays || g.MediumDays > g.LowDays {
		writeJSON(w, http.StatusBadRequest, apiErr("day values must satisfy: critical ≤ high ≤ medium ≤ low"))
		return
	}

	// Find or update default config
	cfg, _ := h.repo.FindDefault(r.Context())
	if cfg == nil {
		// Create new default config
		newCfg, err := h.createUC.Execute(r.Context(), ucconfig.CreateSLAConfigInput{
			Name:      "Default",
			Critical:  g.CriticalDays,
			High:      g.HighDays,
			Medium:    g.MediumDays,
			Low:       g.LowDays,
			IsDefault: true,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiErr(err.Error()))
			return
		}
		cfg = newCfg
	} else {
		updated, err := h.updateUC.Execute(r.Context(), ucconfig.UpdateSLAConfigInput{
			ID:       cfg.ID,
			Critical: g.CriticalDays,
			High:     g.HighDays,
			Medium:   g.MediumDays,
			Low:      g.LowDays,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiErr(err.Error()))
			return
		}
		cfg = updated
	}

	writeJSON(w, http.StatusOK, SLAConfigResponse{
		Global: GlobalConfig{
			CriticalDays: cfg.Critical,
			HighDays:     cfg.High,
			MediumDays:   cfg.Medium,
			LowDays:      cfg.Low,
		},
		ProductOverrides: []ProductOverride{},
	})
}

// List handles GET /api/v2/sla-configurations
func (h *SLAConfigHandler) List(w http.ResponseWriter, r *http.Request) {
	cfgs, err := h.repo.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiErr("failed to list SLA configurations"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"count":   len(cfgs),
		"results": cfgs,
	})
}

// Create handles POST /api/v2/sla-configurations
func (h *SLAConfigHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in ucconfig.CreateSLAConfigInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, apiErr("invalid request body"))
		return
	}

	cfg, err := h.createUC.Execute(r.Context(), in)
	if err != nil {
		switch err {
		case ucconfig.ErrInvalidDays, ucconfig.ErrInvalidOrdering:
			writeJSON(w, http.StatusBadRequest, apiErr(err.Error()))
		default:
			writeJSON(w, http.StatusInternalServerError, apiErr(err.Error()))
		}
		return
	}
	writeJSON(w, http.StatusCreated, cfg)
}

// Get handles GET /api/v2/sla-configurations/{id}
func (h *SLAConfigHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiErr("invalid id"))
		return
	}
	cfg, err := h.repo.FindByID(r.Context(), id)
	if err != nil || cfg == nil {
		writeJSON(w, http.StatusNotFound, apiErr("SLA configuration not found"))
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// Update handles PUT /api/v2/sla-configurations/{id}
func (h *SLAConfigHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiErr("invalid id"))
		return
	}
	var in ucconfig.UpdateSLAConfigInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, apiErr("invalid request body"))
		return
	}
	in.ID = id

	cfg, err := h.updateUC.Execute(r.Context(), in)
	if err != nil {
		switch err {
		case ucconfig.ErrSLAConfigNotFound:
			writeJSON(w, http.StatusNotFound, apiErr(err.Error()))
		case ucconfig.ErrInvalidOrdering:
			writeJSON(w, http.StatusBadRequest, apiErr(err.Error()))
		default:
			writeJSON(w, http.StatusInternalServerError, apiErr(err.Error()))
		}
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// Delete handles DELETE /api/v2/sla-configurations/{id}
func (h *SLAConfigHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiErr("invalid id"))
		return
	}

	if err := h.deleteUC.Execute(r.Context(), id); err != nil {
		switch err {
		case ucconfig.ErrConfigInUse:
			writeJSON(w, http.StatusConflict, apiErr(err.Error()))
		default:
			writeJSON(w, http.StatusInternalServerError, apiErr(err.Error()))
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Assign handles POST /api/v2/sla-configurations/{id}/assign/{product_id}
func (h *SLAConfigHandler) Assign(w http.ResponseWriter, r *http.Request) {
	slaID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiErr("invalid sla_configuration id"))
		return
	}
	productID, err := uuid.Parse(r.PathValue("product_id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiErr("invalid product_id"))
		return
	}
	assignedBy, _ := uuid.Parse(r.Header.Get("X-User-ID"))

	if err := h.assignUC.Execute(r.Context(), ucconfig.AssignProductInput{
		ProductID:          productID,
		SLAConfigurationID: slaID,
		AssignedByID:       assignedBy,
	}); err != nil {
		switch err {
		case ucconfig.ErrSLAConfigNotFound:
			writeJSON(w, http.StatusNotFound, apiErr(err.Error()))
		default:
			writeJSON(w, http.StatusInternalServerError, apiErr(err.Error()))
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "assigned"})
}

// BulkCreateConfigs handles POST /api/v2/sla-configurations/bulk
func (h *SLAConfigHandler) BulkCreateConfigs(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Configurations []ucconfig.CreateSLAConfigInput `json:"configurations"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiErr("invalid request body"))
		return
	}

	// Validate: không được có nhiều hơn 1 is_default=true
	defaultCount := 0
	for _, c := range req.Configurations {
		if c.IsDefault {
			defaultCount++
		}
	}
	if defaultCount > 1 {
		writeJSON(w, http.StatusBadRequest, apiErr("only one configuration can be default"))
		return
	}

	results := make([]map[string]any, 0, len(req.Configurations))
	created := 0
	for _, cfg := range req.Configurations {
		resCfg, err := h.createUC.Execute(r.Context(), cfg)
		if err != nil {
			results = append(results, map[string]any{
				"name": cfg.Name, "status": "error", "message": err.Error(),
			})
		} else {
			results = append(results, map[string]any{
				"name": cfg.Name, "status": "created", "id": resCfg.ID,
			})
			created++
		}
	}

	writeJSON(w, http.StatusMultiStatus, map[string]any{
		"created_count": created,
		"failed_count":  len(results) - created,
		"results":       results,
	})
}

// BulkAssign handles POST /api/v2/sla-configurations/assign-bulk
func (h *SLAConfigHandler) BulkAssign(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Assignments []struct {
			ProductID          uuid.UUID `json:"product_id"`
			SLAConfigurationID uuid.UUID `json:"sla_configuration_id"`
		} `json:"assignments"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiErr("invalid request body"))
		return
	}

	assignedBy, _ := uuid.Parse(r.Header.Get("X-User-ID"))
	results := make([]map[string]any, 0, len(req.Assignments))
	assigned := 0
	for _, a := range req.Assignments {
		err := h.assignUC.Execute(r.Context(), ucconfig.AssignProductInput{
			ProductID:          a.ProductID,
			SLAConfigurationID: a.SLAConfigurationID,
			AssignedByID:       assignedBy,
		})
		if err != nil {
			results = append(results, map[string]any{"product_id": a.ProductID, "status": "error"})
		} else {
			results = append(results, map[string]any{"product_id": a.ProductID, "status": "assigned"})
			assigned++
		}
	}
	writeJSON(w, http.StatusMultiStatus, map[string]any{"assigned_count": assigned, "results": results})
}
