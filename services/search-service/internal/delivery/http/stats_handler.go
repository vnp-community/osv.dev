package http

import (
	"net/http"

	"github.com/osv/search-service/internal/domain/repository"
)

type StatsHandler struct {
	cveRepo repository.CVERepository
}

func NewStatsHandler(repo repository.CVERepository) *StatsHandler {
	return &StatsHandler{cveRepo: repo}
}

// Dashboard handles GET /api/v2/stats/dashboard.
func (h *StatsHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	stats, err := h.cveRepo.GetDashboardStats(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to compute dashboard stats")
		return
	}
	respondJSON(w, http.StatusOK, stats)
}
