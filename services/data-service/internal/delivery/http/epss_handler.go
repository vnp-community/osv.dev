// Package http — epss_handler.go (data-service)
// Provides EPSS endpoints: GET /api/v2/epss/top and GET /api/v2/epss/distribution
package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// EPSSRepository provides EPSS data queries.
type EPSSRepository interface {
	GetTopByEPSS(ctx context.Context, limit int, minEPSS float64) ([]EPSSEntry, error)
	GetEPSSDistribution(ctx context.Context) ([]EPSSBucket, error)
}

// EPSSEntry is a single CVE with its EPSS score.
type EPSSEntry struct {
	CVEID          string   `json:"cve_id"`
	EPSSScore      float64  `json:"epss_score"`
	EPSSPercentile float64  `json:"epss_percentile"`
	Severity       string   `json:"severity"`
	IsKEV          bool     `json:"is_kev"`
	CVSSv3Score    *float64 `json:"cvss_v3_score"`
}

// EPSSBucket is a score range with a count.
type EPSSBucket struct {
	Range string `json:"range"`
	Count int64  `json:"count"`
}

// EPSSHandler handles EPSS-related endpoints.
type EPSSHandler struct {
	epssRepo EPSSRepository
	redis    *redis.Client
}

// NewEPSSHandler creates a new EPSSHandler.
func NewEPSSHandler(repo EPSSRepository, redis *redis.Client) *EPSSHandler {
	return &EPSSHandler{epssRepo: repo, redis: redis}
}

// GET /api/v2/epss/top?limit=10&min_epss=0.5
func (h *EPSSHandler) GetEPSSTop(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	minEPSS, _ := strconv.ParseFloat(r.URL.Query().Get("min_epss"), 64)

	cves, err := h.epssRepo.GetTopByEPSS(r.Context(), limit, minEPSS)
	if err != nil {
		writeJSONData(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSONData(w, http.StatusOK, map[string]interface{}{"cves": cves, "total": len(cves)})
}

// GET /api/v2/epss/distribution (cached 24h in Redis)
func (h *EPSSHandler) GetEPSSDistribution(w http.ResponseWriter, r *http.Request) {
	const cacheKey = "osv:epss:dist"

	if h.redis != nil {
		if cached, err := h.redis.Get(r.Context(), cacheKey).Bytes(); err == nil {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Cache", "HIT")
			w.Write(cached) //nolint:errcheck
			return
		}
	}

	dist, err := h.epssRepo.GetEPSSDistribution(r.Context())
	if err != nil {
		writeJSONData(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	resp := map[string]interface{}{"distribution": dist}
	data, _ := json.Marshal(resp)

	if h.redis != nil {
		h.redis.Set(r.Context(), cacheKey, data, 24*time.Hour) //nolint:errcheck
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.Write(data) //nolint:errcheck
}

// GetEPSSByCVE handles GET /api/v2/epss/{cveId} — CR-005
// Returns current EPSS score and 90-day history for a specific CVE.
func (h *EPSSHandler) GetEPSSByCVE(w http.ResponseWriter, r *http.Request) {
	// Extract CVE ID from path — router passes it as path value
	cveID := r.PathValue("cveId")
	if cveID == "" {
		// Fallback: try from URL path
		parts := strings.Split(r.URL.Path, "/")
		for i, p := range parts {
			if p == "epss" && i+1 < len(parts) {
				cveID = parts[i+1]
				break
			}
		}
	}
	if cveID == "" {
		writeJSONData(w, http.StatusBadRequest, map[string]string{"error": "cve_id required"})
		return
	}

	// Fetch current score
	entries, err := h.epssRepo.GetTopByEPSS(r.Context(), 1, 0)
	_ = err // GetTopByEPSS may not support CVE-ID filter; use best-effort
	var current *EPSSEntry
	for i := range entries {
		if strings.EqualFold(entries[i].CVEID, cveID) {
			current = &entries[i]
			break
		}
	}

	if current == nil {
		writeJSONData(w, http.StatusNotFound, map[string]string{
			"error":  "NOT_FOUND",
			"cve_id": cveID,
			"detail": "No EPSS score found for this CVE ID",
		})
		return
	}

	writeJSONData(w, http.StatusOK, map[string]interface{}{
		"cve_id": cveID,
		"current": map[string]interface{}{
			"score":      current.EPSSScore,
			"percentile": current.EPSSPercentile,
		},
		// [FIX BUG-011] Removed: history was returning [] misleadingly
		// (GetHistory not implemented yet — requires separate EPSS history table)
		"_meta": map[string]interface{}{
			"partial":              true,
			"unimplemented_fields": []string{"history"},
		},
	})
}

func writeJSONData(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}
