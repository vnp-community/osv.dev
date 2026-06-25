// Package http — epss_handler.go
// Provides EPSS stats endpoint: GET /api/v2/epss/stats
// Returns distribution, top exploitable CVEs, and EPSS percentile ranges.
package http

import (
	"fmt"
	"net/http"
	"strconv"
)

// GetEPSSStats handles GET /api/v2/epss/stats
func (h *Handler) GetEPSSStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.cveRepo.GetEPSSStats(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "epss stats failed")
		return
	}
	respondJSON(w, http.StatusOK, stats)
}

// GetTopEPSS handles GET /api/v2/epss/top?min_epss=0.5&limit=20
func (h *Handler) GetTopEPSS(w http.ResponseWriter, r *http.Request) {
	minEPSS := 0.0
	if v := r.URL.Query().Get("min_epss"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			minEPSS = f
		}
	}
	limit := parseInt(r.URL.Query().Get("limit"), 20)
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	top, err := h.cveRepo.GetTopEPSS(r.Context(), minEPSS, limit)
	if err != nil {
		h.log.Error().Err(err).Msg("get top epss failed")
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("query failed: %v", err))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":     top,
		"count":    len(top),
		"min_epss": minEPSS,
	})
}

// GetEPSSDistribution handles GET /api/v2/epss/distribution
// Returns bucket counts for EPSS score ranges (very_low/low/high/critical).
func (h *Handler) GetEPSSDistribution(w http.ResponseWriter, r *http.Request) {
	dist := h.cveRepo.QueryEPSSDistribution(r.Context())
	if dist == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"very_low": 0, "low": 0, "high": 0, "critical": 0,
			"mean": nil, "median": nil,
		})
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"very_low": dist.VeryLow,
		"low":      dist.Low,
		"high":     dist.High,
		"critical": dist.Critical,
		"mean":     dist.Mean,
		"median":   dist.Median,
	})
}
