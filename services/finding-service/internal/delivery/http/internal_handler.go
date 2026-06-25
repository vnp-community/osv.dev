package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/osv/finding-service/internal/usecase"
	"github.com/rs/zerolog/log"
)

// InternalHandler serves /internal/* endpoints for BFF consumption
// These endpoints MUST NOT be exposed to the public internet
// (Gateway must not route external traffic to these paths)
type InternalHandler struct {
	statsUC *usecase.StatsUseCase
}

func NewInternalHandler(statsUC *usecase.StatsUseCase) *InternalHandler {
	return &InternalHandler{statsUC: statsUC}
}

// GET /internal/stats
func (h *InternalHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.statsUC.GetDashboardStats(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("failed to get dashboard stats")
		http.Error(w, `{"error":"INTERNAL_ERROR"}`, 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// GetFindingStats serves GET /api/v1/findings/stats
// Returns severity counts in {critical, high, medium, low} format for the
// gateway-service DashboardHandler (BFF fan-out consumer).
func (h *InternalHandler) GetFindingStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.statsUC.GetDashboardStats(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("failed to get finding stats")
		http.Error(w, `{"error":"INTERNAL_ERROR"}`, 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{
		"critical": stats.CriticalFindings,
		"high":     stats.HighFindings,
		"medium":   stats.MediumFindings,
		"low":      stats.LowFindings,
		"total":    stats.CriticalFindings + stats.HighFindings + stats.MediumFindings + stats.LowFindings,
	})
}

// GET /internal/risk-trend?period=30d
func (h *InternalHandler) GetRiskTrend(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "30d"
	}

	trend, err := h.statsUC.GetRiskTrend(r.Context(), period)
	if err != nil {
		log.Error().Err(err).Msg("failed to get risk trend")
		http.Error(w, `{"error":"INTERNAL_ERROR"}`, 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(trend)
}

// GET /internal/product-grades
func (h *InternalHandler) GetProductGrades(w http.ResponseWriter, r *http.Request) {
	grades, err := h.statsUC.GetProductGrades(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("failed to get product grades")
		http.Error(w, `{"error":"INTERNAL_ERROR"}`, 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(grades)
}

// GetProductGradesPublic serves GET /api/v1/products/grades
// Returns {overall_grade, overall_score, products} format for the
// gateway-service DashboardHandler. Computes aggregate grade from all products.
func (h *InternalHandler) GetProductGradesPublic(w http.ResponseWriter, r *http.Request) {
	grades, err := h.statsUC.GetProductGrades(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("failed to get product grades for public endpoint")
		http.Error(w, `{"error":"INTERNAL_ERROR"}`, 500)
		return
	}

	// Compute overall grade from aggregate: if any critical→F, else compute avg
	overallGrade := "A"
	overallScore := 100
	if len(grades) > 0 {
		totalScore := 0
		for _, g := range grades {
			totalScore += g.Score
			// Lowest grade wins
			if gradeRank(g.Grade) > gradeRank(overallGrade) {
				overallGrade = g.Grade
			}
		}
		overallScore = totalScore / len(grades)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"overall_grade": overallGrade,
		"overall_score": overallScore,
		"products":      grades,
	})
}

// gradeRank returns the numeric rank (higher = worse) for comparison
func gradeRank(grade string) int {
	switch grade {
	case "A":
		return 0
	case "B":
		return 1
	case "C":
		return 2
	case "D":
		return 3
	default: // F
		return 4
	}
}

// GET /internal/sla-breaches?limit=5
func (h *InternalHandler) GetSLABreaches(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 5
	}

	breaches, err := h.statsUC.GetSLABreaches(r.Context(), limit)
	if err != nil {
		log.Error().Err(err).Msg("failed to get SLA breaches")
		http.Error(w, `{"error":"INTERNAL_ERROR"}`, 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(breaches)
}

// POST /internal/findings/count-by-cve-ids
func (h *InternalHandler) CountByCVEIds(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CVEIds []string `json:"cve_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"VALIDATION_ERROR"}`, 400)
		return
	}

	count, err := h.statsUC.CountActiveByCVEIds(r.Context(), req.CVEIds)
	if err != nil {
		http.Error(w, `{"error":"INTERNAL_ERROR"}`, 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"count": count})
}
