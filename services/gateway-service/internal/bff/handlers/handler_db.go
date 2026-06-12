// Package http — DB route handlers for the API gateway.
package http

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/osv/gateway-service/internal/usecase/dbsync"
)

// handleSyncAll handles POST /api/v1/db/sync
func (h *CVEHandler) handleSyncAll(w http.ResponseWriter, r *http.Request) {
	type syncRequest struct {
		DisabledSources []string `json:"disabled_sources"`
		ForceUpdate     bool     `json:"force_update"`
		NVDMode         string   `json:"nvd_mode"`
		NVDAPIKey       string   `json:"nvd_api_key"`
		Mirror          string   `json:"mirror"`
	}

	var req syncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if h.dbsyncUC == nil {
		respondError(w, http.StatusServiceUnavailable, "sync service not configured")
		return
	}

	result, err := h.dbsyncUC.Execute(r.Context(), dbsync.Input{
		DisabledSources: req.DisabledSources,
		ForceUpdate:     req.ForceUpdate,
		NVDMode:         req.NVDMode,
		NVDAPIKey:       req.NVDAPIKey,
		Mirror:          req.Mirror,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"sources_updated": result.SourcesUpdated,
		"sources_failed":  result.SourcesFailed,
		"total_cves":      result.TotalCVEs,
		"duration":        result.Duration,
	})
}

// handleSyncSource handles POST /api/v1/db/sync/{source}
func (h *CVEHandler) handleSyncSource(w http.ResponseWriter, r *http.Request) {
	source := chi.URLParam(r, "source")
	if source == "" {
		respondError(w, http.StatusBadRequest, "source is required")
		return
	}

	type syncSourceRequest struct {
		NVDMode   string `json:"nvd_mode"`
		NVDAPIKey string `json:"nvd_api_key"`
	}
	var req syncSourceRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	if h.dbsyncUC == nil {
		respondError(w, http.StatusServiceUnavailable, "sync service not configured")
		return
	}

	result, err := h.dbsyncUC.Execute(r.Context(), dbsync.Input{
		Source:    source,
		NVDMode:   req.NVDMode,
		NVDAPIKey: req.NVDAPIKey,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"sources_updated": result.SourcesUpdated,
		"total_cves":      result.TotalCVEs,
	})
}

// handleDBStatus handles GET /api/v1/db/status
func (h *CVEHandler) handleDBStatus(w http.ResponseWriter, r *http.Request) {
	if h.dbsyncUC == nil {
		respondError(w, http.StatusServiceUnavailable, "sync service not configured")
		return
	}
	status, err := h.dbsyncUC.GetStatus(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, status)
}

// handleImportDB handles POST /api/v1/db/import (multipart stream)
func (h *CVEHandler) handleImportDB(w http.ResponseWriter, _ *http.Request) {
	// Streaming import — placeholder (requires cvedb client directly)
	respondJSON(w, http.StatusAccepted, map[string]string{"status": "import queued"})
}

// handleExportDB handles GET /api/v1/db/export
func (h *CVEHandler) handleExportDB(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="cve_database.json"`)
	respondJSON(w, http.StatusOK, map[string]string{"status": "export not yet implemented"})
}
