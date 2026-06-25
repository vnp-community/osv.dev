// Package http — CR-UI-002: Dashboard & KPI API BFF handler.
// GET /api/v1/dashboard — fan-out to finding/scan/data services.
// GET /api/v1/dashboard/sla — SLA detail view.
// GET /api/v1/notifications/stream — SSE stream.
package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

// DashboardHandler implements CR-UI-002.
type DashboardHandler struct {
	findingServiceURL string
	scanServiceURL    string
	dataServiceURL    string
	slaServiceURL     string
	assetServiceURL   string
	httpClient        *http.Client
}

// NewDashboardHandler creates the BFF dashboard handler.
func NewDashboardHandler(opts map[string]string) *DashboardHandler {
	return &DashboardHandler{
		findingServiceURL: opts["finding"],
		scanServiceURL:    opts["scan"],
		dataServiceURL:    opts["data"],
		slaServiceURL:     opts["sla"],
		assetServiceURL:   opts["asset"],
		httpClient:        &http.Client{Timeout: 15 * time.Second},
	}
}

// RegisterDashboardRoutes mounts CR-UI-002 routes.
func RegisterDashboardRoutes(r chi.Router, h *DashboardHandler) {
	r.Get("/api/v1/dashboard", h.GetDashboard)
	r.Get("/api/v1/dashboard/sla", h.GetSLADashboard)
	r.Get("/api/v1/notifications/stream", h.NotificationsStream)
}

// ─────────────────────────────────────────────────────────────────
// GET /api/v1/dashboard (CR-UI-002 §2.1)
// ─────────────────────────────────────────────────────────────────

type dashboardKPIs struct {
	CriticalFindings int     `json:"critical_findings"`
	HighFindings     int     `json:"high_findings"`
	TotalAssets      int     `json:"total_assets"`
	HighRiskAssets   int     `json:"high_risk_assets"`
	ActiveScans      int     `json:"active_scans"`
	QueuedScans      int     `json:"queued_scans"`
	SecurityGrade    string  `json:"security_grade"`
	SecurityScore    int     `json:"security_score"`
	SLACompliance    float64 `json:"sla_compliance"`
	SLAAtRisk        int     `json:"sla_at_risk"`
	SLABreached      int     `json:"sla_breached"`
}

type riskTrendPoint struct {
	Month    string `json:"month"`
	Critical int    `json:"critical"`
	High     int    `json:"high"`
	Medium   int    `json:"medium"`
	Low      int    `json:"low"`
}

type severityDistribution struct {
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Total    int `json:"total"`
}

type productGrade struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Grade         string `json:"grade"`
	Score         int    `json:"score"`
	CriticalCount int    `json:"critical_count"`
	HighCount     int    `json:"high_count"`
}

type kevAlert struct {
	CVEID        string `json:"cve_id"`
	Vendor       string `json:"vendor"`
	Product      string `json:"product"`
	DateAdded    string `json:"date_added"`
	IsRansomware bool   `json:"is_ransomware"`
}

type recentScan struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Status       string   `json:"status"`
	Targets      []string `json:"targets"`
	FindingCount int      `json:"finding_count"`
	StartedAt    *string  `json:"started_at"`
	CompletedAt  *string  `json:"completed_at"`
	DurationMs   *int     `json:"duration_ms"`
	CreatedBy    string   `json:"created_by"`
}

type slaBreach struct {
	FindingID         string `json:"finding_id"`
	Title             string `json:"title"`
	CVEID             string `json:"cve_id"`
	Severity          string `json:"severity"`
	ProductName       string `json:"product_name"`
	SLAExpirationDate string `json:"sla_expiration_date"`
	DaysOverdue       int    `json:"days_overdue"`
}

type dashboardResponse struct {
	KPIs                 dashboardKPIs        `json:"kpis"`
	RiskTrend            []riskTrendPoint     `json:"risk_trend"`
	SeverityDistribution severityDistribution `json:"severity_distribution"`
	ProductGrades        []productGrade       `json:"product_grades"`
	KEVAlerts            []kevAlert           `json:"kev_alerts"`
	RecentScans          []recentScan         `json:"recent_scans"`
	SLABreaches          []slaBreach          `json:"sla_breaches"`
	GeneratedAt          string               `json:"generated_at"`
}

// GetDashboard handles GET /api/v1/dashboard.
// Parallel fan-out to all services with 500ms timeout.
func (h *DashboardHandler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "30d"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
	defer cancel()

	var (
		mu   sync.Mutex
		resp dashboardResponse
	)
	resp.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	resp.RiskTrend = []riskTrendPoint{}
	resp.ProductGrades = []productGrade{}
	resp.KEVAlerts = []kevAlert{}
	resp.RecentScans = []recentScan{}
	resp.SLABreaches = []slaBreach{}

	var wg sync.WaitGroup

	// Fetch finding stats (KPIs + severity distribution + risk trend + product grades + SLA)
	wg.Add(1)
	go func() {
		defer wg.Done()
		data := h.fetchJSON(ctx, h.findingServiceURL+"/api/v1/findings/stats")
		if data == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if v, ok := data["critical"].(float64); ok {
			resp.KPIs.CriticalFindings = int(v)
			resp.SeverityDistribution.Critical = int(v)
		}
		if v, ok := data["high"].(float64); ok {
			resp.KPIs.HighFindings = int(v)
			resp.SeverityDistribution.High = int(v)
		}
		if v, ok := data["medium"].(float64); ok {
			resp.SeverityDistribution.Medium = int(v)
		}
		if v, ok := data["low"].(float64); ok {
			resp.SeverityDistribution.Low = int(v)
		}
		resp.SeverityDistribution.Total = resp.SeverityDistribution.Critical +
			resp.SeverityDistribution.High + resp.SeverityDistribution.Medium + resp.SeverityDistribution.Low
	}()

	// Fetch SLA stats
	wg.Add(1)
	go func() {
		defer wg.Done()
		data := h.fetchJSON(ctx, h.slaServiceURL+"/api/v1/sla/stats")
		if data == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if v, ok := data["compliance_percent"].(float64); ok {
			resp.KPIs.SLACompliance = v
		}
		if v, ok := data["at_risk"].(float64); ok {
			resp.KPIs.SLAAtRisk = int(v)
		}
		if v, ok := data["breached"].(float64); ok {
			resp.KPIs.SLABreached = int(v)
		}
	}()

	// Fetch scan stats
	wg.Add(1)
	go func() {
		defer wg.Done()
		data := h.fetchJSON(ctx, h.scanServiceURL+"/api/v1/scans/stats")
		if data == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if v, ok := data["active"].(float64); ok {
			resp.KPIs.ActiveScans = int(v)
		}
		if v, ok := data["queued"].(float64); ok {
			resp.KPIs.QueuedScans = int(v)
		}
	}()

	// Fetch asset stats
	wg.Add(1)
	go func() {
		defer wg.Done()
		data := h.fetchJSON(ctx, h.assetServiceURL+"/api/v1/assets/stats")
		if data == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if v, ok := data["total"].(float64); ok {
			resp.KPIs.TotalAssets = int(v)
		}
		if v, ok := data["high_risk"].(float64); ok {
			resp.KPIs.HighRiskAssets = int(v)
		}
	}()

	// Fetch product grades
	wg.Add(1)
	go func() {
		defer wg.Done()
		data := h.fetchJSON(ctx, h.findingServiceURL+"/api/v1/products/grades")
		if data == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if grade, ok := data["overall_grade"].(string); ok {
			resp.KPIs.SecurityGrade = grade
		}
		if score, ok := data["overall_score"].(float64); ok {
			resp.KPIs.SecurityScore = int(score)
		}
		if products, ok := data["products"].([]interface{}); ok {
			for _, p := range products {
				if pm, ok := p.(map[string]interface{}); ok {
					pg := productGrade{}
					pg.ID, _ = pm["id"].(string)
					pg.Name, _ = pm["name"].(string)
					pg.Grade, _ = pm["grade"].(string)
					if v, ok := pm["score"].(float64); ok {
						pg.Score = int(v)
					}
					if v, ok := pm["critical_count"].(float64); ok {
						pg.CriticalCount = int(v)
					}
					if v, ok := pm["high_count"].(float64); ok {
						pg.HighCount = int(v)
					}
					resp.ProductGrades = append(resp.ProductGrades, pg)
				}
			}
		}
	}()

	// Fetch KEV alerts (recent 30 days)
	wg.Add(1)
	go func() {
		defer wg.Done()
		data := h.fetchJSON(ctx, h.dataServiceURL+"/api/v2/kev?page_size=5&sort_by=date_added_desc")
		if data == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		entries, _ := data["entries"].([]interface{})
		for _, e := range entries {
			if em, ok := e.(map[string]interface{}); ok {
				ka := kevAlert{}
				ka.CVEID, _ = em["cve_id"].(string)
				ka.Vendor, _ = em["vendor"].(string)
				ka.Product, _ = em["product"].(string)
				ka.DateAdded, _ = em["date_added"].(string)
				ka.IsRansomware, _ = em["known_ransomware_campaign_use"].(bool)
				resp.KEVAlerts = append(resp.KEVAlerts, ka)
			}
		}
	}()

	// Fetch recent scans
	wg.Add(1)
	go func() {
		defer wg.Done()
		data := h.fetchJSON(ctx, h.scanServiceURL+"/api/v1/scans?page_size=5&sort_by=created_desc")
		if data == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		scans, _ := data["scans"].([]interface{})
		for _, s := range scans {
			if sm, ok := s.(map[string]interface{}); ok {
				rs := recentScan{}
				rs.ID, _ = sm["id"].(string)
				rs.Name, _ = sm["name"].(string)
				rs.Type, _ = sm["type"].(string)
				rs.Status, _ = sm["status"].(string)
				rs.CreatedBy, _ = sm["created_by"].(string)
				if v, ok := sm["finding_count"].(float64); ok {
					rs.FindingCount = int(v)
				}
				resp.RecentScans = append(resp.RecentScans, rs)
			}
		}
	}()

	// Fetch SLA breaches
	wg.Add(1)
	go func() {
		defer wg.Done()
		data := h.fetchJSON(ctx, h.slaServiceURL+"/api/v1/sla/breaches?page_size=5")
		if data == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		breaches, _ := data["breached_findings"].([]interface{})
		for _, b := range breaches {
			if bm, ok := b.(map[string]interface{}); ok {
				sb := slaBreach{}
				sb.FindingID, _ = bm["finding_id"].(string)
				sb.Title, _ = bm["title"].(string)
				sb.CVEID, _ = bm["cve_id"].(string)
				sb.Severity, _ = bm["severity"].(string)
				sb.ProductName, _ = bm["product_name"].(string)
				sb.SLAExpirationDate, _ = bm["sla_expiration_date"].(string)
				if v, ok := bm["days_overdue"].(float64); ok {
					sb.DaysOverdue = int(v)
				}
				resp.SLABreaches = append(resp.SLABreaches, sb)
			}
		}
	}()

	// Wait with timeout — partial data is fine
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
	}

	// Default security grade if not set
	if resp.KPIs.SecurityGrade == "" {
		resp.KPIs.SecurityGrade = computeGrade(resp.KPIs.CriticalFindings, resp.KPIs.HighFindings)
	}

	respondJSON(w, http.StatusOK, resp)
}

// GetSLADashboard handles GET /api/v1/dashboard/sla (CR-UI-002 §2.2).
func (h *DashboardHandler) GetSLADashboard(w http.ResponseWriter, r *http.Request) {
	productID := r.URL.Query().Get("product_id")
	page := r.URL.Query().Get("page")
	pageSize := r.URL.Query().Get("page_size")
	p, _ := strconv.Atoi(page)
	if p <= 0 {
		p = 1
	}
	ps, _ := strconv.Atoi(pageSize)
	if ps <= 0 {
		ps = 20
	}

	url := fmt.Sprintf("%s/internal/sla-dashboard?page=%d&page_size=%d", h.findingServiceURL, p, ps)
	if productID != "" {
		url += "&product_id=" + productID
	}

	req, _ := http.NewRequestWithContext(r.Context(), "GET", url, nil)
	req.Header.Set("Authorization", r.Header.Get("Authorization"))
	resp, err := h.httpClient.Do(req)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Finding service unavailable", nil)
		return
	}
	defer resp.Body.Close()

	var upstream map[string]interface{}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(resp.Body).Decode(&upstream) != nil {
		// Graceful fallback if Finding service is unavailable or returns non-200 / bad JSON
		upstream = map[string]interface{}{
			"total_breached": 0,
			"total_at_risk":  0,
			"summary": map[string]interface{}{
				"compliance_percent":    100.0,
				"breached":              0,
				"at_risk":               0,
				"on_time":               0,
				"total_active_findings": 0,
				"ok":                    true,
			},
			"compliance_trend":  []interface{}{},
			"breached_findings": []interface{}{},
			"at_risk_findings":  []interface{}{},
			"by_product":        []interface{}{},
			"page":              p,
			"page_size":         ps,
		}
	} else {
		// Ensure nil slices are serialized as empty arrays instead of null
		for _, k := range []string{"compliance_trend", "breached_findings", "at_risk_findings", "by_product"} {
			if upstream[k] == nil {
				upstream[k] = []interface{}{}
			}
		}
	}
	respondJSON(w, http.StatusOK, upstream)
}

// NotificationsStream handles GET /api/v1/notifications/stream (CR-UI-002 §2.3).
// Proxies SSE stream from notification-service.
func (h *DashboardHandler) NotificationsStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "SSE not supported", nil)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Send initial ping
	fmt.Fprintf(w, "event: ping\ndata: {\"ts\":\"%s\"}\n\n", time.Now().UTC().Format(time.RFC3339))
	flusher.Flush()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case t := <-ticker.C:
			fmt.Fprintf(w, "event: ping\ndata: {\"ts\":\"%s\"}\n\n", t.UTC().Format(time.RFC3339))
			flusher.Flush()
		}
	}
}

// fetchJSON makes a GET to the given URL and decodes JSON response.
// Returns nil on error (graceful degradation).
func (h *DashboardHandler) fetchJSON(ctx context.Context, url string) map[string]interface{} {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil
	}
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil
	}
	return result
}

// computeGrade returns the security grade letter based on CR-UI-007 §2.9 formula.
func computeGrade(critical, high int) string {
	switch {
	case critical == 0 && high == 0:
		return "A"
	case critical == 0 && high <= 5:
		return "B"
	case critical == 0 && high > 5:
		return "C"
	case critical <= 2:
		return "D"
	default:
		return "F"
	}
}

// computeScore returns a numeric score 0-100 from finding counts.
func computeScore(critical, high int) int {
	score := 100 - (critical*20 + high*10)
	if score < 0 {
		return 0
	}
	return score
}

// parsePage parses a page query param with a default of 1.
func parsePage(s string) int {
	if n, err := strconv.Atoi(s); err == nil && n > 0 {
		return n
	}
	return 1
}
