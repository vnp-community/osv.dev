// Package http provides the SLA handler for finding-service dashboard.
package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
)

// SLASummaryData aggregates overall SLA stats.
type SLASummaryData struct {
	CompliancePct       float64 `json:"compliance_percent"`
	Breached            int     `json:"breached"`
	AtRisk              int     `json:"at_risk"`
	OnTime              int     `json:"on_time"`
	TotalActiveFindings int     `json:"total_active_findings"`
	Ok                  bool    `json:"ok"`
}

// SLATrendPoint represents compliance trend over a month.
type SLATrendPoint struct {
	Month         string  `json:"month"`
	CompliancePct float64 `json:"compliance_pct"`
}

// SLAFindingItem is a finding tracking its SLA due date.
type SLAFindingItem struct {
	FindingID   string `json:"finding_id"`
	Title       string `json:"title"`
	Severity    string `json:"severity"`
	ProductName string `json:"product_name"`
	DaysLeft    int    `json:"days_left"` // negative = overdue
	ExpiresAt   string `json:"expires_at"`
}

// ProductSLAData tracks SLA metrics per product.
type ProductSLAData struct {
	ProductID     string  `json:"product_id"`
	ProductName   string  `json:"product_name"`
	CompliancePct float64 `json:"compliance_pct"`
	Breached      int     `json:"breached"`
	AtRisk        int     `json:"at_risk"`
}

// FindingRepository extends finding repo with SLA aggregations.
type FindingRepository interface {
	GetSLASummary(ctx context.Context, productID string) (*SLASummaryData, error)
	GetSLAComplianceTrend(ctx context.Context, productID string, months int) ([]SLATrendPoint, error)
	GetBreachedFindings(ctx context.Context, productID string, page, pageSize int) ([]SLAFindingItem, error)
	GetAtRiskFindings(ctx context.Context, productID string) ([]SLAFindingItem, error)
	GetSLAByProduct(ctx context.Context) ([]ProductSLAData, error)
}

// SLAHandler handles SLA dashboard endpoints.
type SLAHandler struct {
	findingRepo FindingRepository
}

// NewSLAHandler creates a new SLAHandler.
func NewSLAHandler(repo FindingRepository) *SLAHandler {
	return &SLAHandler{findingRepo: repo}
}

// GET /internal/sla-dashboard
func (h *SLAHandler) GetSLADashboard(w http.ResponseWriter, r *http.Request) {
	productID := r.URL.Query().Get("product_id")
	page, ps := parsePaginationSLA(r)

	// All queries in parallel
	var (
		summary   *SLASummaryData
		trend     []SLATrendPoint
		breached  []SLAFindingItem
		atRisk    []SLAFindingItem
		byProduct []ProductSLAData
		mu        sync.Mutex
		wg        sync.WaitGroup
	)

	wg.Add(5)

	go func() {
		defer wg.Done()
		d, _ := h.findingRepo.GetSLASummary(r.Context(), productID)
		mu.Lock()
		summary = d
		mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		d, _ := h.findingRepo.GetSLAComplianceTrend(r.Context(), productID, 6)
		mu.Lock()
		trend = d
		mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		d, _ := h.findingRepo.GetBreachedFindings(r.Context(), productID, page, ps)
		mu.Lock()
		breached = d
		mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		d, _ := h.findingRepo.GetAtRiskFindings(r.Context(), productID)
		mu.Lock()
		atRisk = d
		mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		d, _ := h.findingRepo.GetSLAByProduct(r.Context())
		mu.Lock()
		byProduct = d
		mu.Unlock()
	}()

	wg.Wait()

	var totalBreached, totalAtRisk int
	if summary != nil {
		totalBreached = summary.Breached
		totalAtRisk = summary.AtRisk
	}

	respondSLAJSON(w, 200, map[string]interface{}{
		"total_breached":    totalBreached,
		"total_at_risk":     totalAtRisk,
		"summary":           summary,
		"compliance_trend":  trend,
		"breached_findings": breached,
		"at_risk_findings":  atRisk,
		"by_product":        byProduct,
		"page":              page,
		"page_size":         ps,
	})
}

func parsePaginationSLA(r *http.Request) (page, pageSize int) {
	page = 1
	pageSize = 20

	if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 0 {
		page = p
	}
	if ps, err := strconv.Atoi(r.URL.Query().Get("page_size")); err == nil && ps > 0 {
		if ps > 100 {
			ps = 100
		}
		pageSize = ps
	}
	return
}

func respondSLAJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data) //nolint:errcheck
}
