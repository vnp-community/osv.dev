// Package http — handler_public_stats.go (gateway-service)
// CR-013: GET /api/v2/public/stats — NO authentication required.
// Fan-out to scan/finding/data services with graceful degradation and 5-min cache.
package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// PublicBFFHandler serves the no-auth public stats endpoint.
// Used by the login page to display live platform statistics without requiring authentication.
type PublicBFFHandler struct {
	scanBase    string
	findingBase string
	dataBase    string
	httpClient  *http.Client
	cache       sync.Map
}

type publicStatsEntry struct {
	data      []byte
	expiresAt time.Time
}

// NewPublicBFFHandler creates a handler that aggregates public stats from multiple services.
func NewPublicBFFHandler(services map[string]string) *PublicBFFHandler {
	return &PublicBFFHandler{
		scanBase:    services["scan"],
		findingBase: services["finding"],
		dataBase:    services["data"],
		httpClient:  &http.Client{Timeout: 3 * time.Second},
	}
}

// PublicStats is the canonical response shape for GET /api/v2/public/stats.
type PublicStats struct {
	TotalCVEs        string           `json:"total_cves"`
	ScansToday       int              `json:"scans_today"`
	FindingAccuracy  string           `json:"finding_accuracy"`
	UptimeSLA        string           `json:"uptime_sla"`
	ThreatIndicators ThreatIndicators `json:"threat_indicators"`
}

// ThreatIndicators is the nested threat info in PublicStats.
type ThreatIndicators struct {
	CriticalThreats int `json:"critical_threats"`
	KEVActive       int `json:"kev_active"`
	AssetsAtRisk    int `json:"assets_at_risk"`
}

// HandlePublicStats serves GET /api/v2/public/stats (and /api/v1/public/stats alias).
// No JWT required. Cached 5 minutes in-process. Returns X-Cache: HIT/MISS header.
func (h *PublicBFFHandler) HandlePublicStats(w http.ResponseWriter, r *http.Request) {
	// CORS — allow login page from any origin
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Max-Age", "300")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	const cacheKey = "public_stats_v1"
	const cacheTTL = 5 * time.Minute

	// Check in-process cache
	if v, ok := h.cache.Load(cacheKey); ok {
		entry := v.(*publicStatsEntry)
		if time.Now().Before(entry.expiresAt) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Cache", "HIT")
			w.WriteHeader(http.StatusOK)
			w.Write(entry.data) //nolint:errcheck
			return
		}
	}

	// Aggregate from services in parallel
	stats := h.aggregate(r.Context())
	data, _ := json.Marshal(stats)

	h.cache.Store(cacheKey, &publicStatsEntry{data: data, expiresAt: time.Now().Add(cacheTTL)})

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.WriteHeader(http.StatusOK)
	w.Write(data) //nolint:errcheck
}

func (h *PublicBFFHandler) aggregate(ctx context.Context) PublicStats {
	var (
		wg              sync.WaitGroup
		totalCVEs       = "240K+"
		scansToday      int
		criticalThreats int
		kevActive       int
	)

	wg.Add(3)
	go func() {
		defer wg.Done()
		if v := h.fetchTotalCVEs(ctx); v != "" {
			totalCVEs = v
		}
	}()
	go func() {
		defer wg.Done()
		scansToday = h.fetchScansToday(ctx)
	}()
	go func() {
		defer wg.Done()
		criticalThreats = h.fetchCriticalThreats(ctx)
		kevActive = h.fetchKEVActive(ctx)
	}()
	wg.Wait()

	return PublicStats{
		TotalCVEs:       totalCVEs,
		ScansToday:      scansToday,
		FindingAccuracy: "98.4%",
		UptimeSLA:       "99.99%",
		ThreatIndicators: ThreatIndicators{
			CriticalThreats: criticalThreats,
			KEVActive:       kevActive,
			AssetsAtRisk:    0,
		},
	}
}

func (h *PublicBFFHandler) fetchTotalCVEs(ctx context.Context) string {
	if h.dataBase == "" {
		return "240K+"
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", h.dataBase+"/api/v1/dbinfo", nil)
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return "240K+"
	}
	defer resp.Body.Close()
	var info struct {
		TotalCVEs int `json:"total_cves"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil || info.TotalCVEs == 0 {
		return "240K+"
	}
	if info.TotalCVEs >= 1_000_000 {
		return fmt.Sprintf("%dM+", info.TotalCVEs/1_000_000)
	}
	if info.TotalCVEs >= 1000 {
		return fmt.Sprintf("%dK+", info.TotalCVEs/1000)
	}
	return strconv.Itoa(info.TotalCVEs)
}

func (h *PublicBFFHandler) fetchScansToday(ctx context.Context) int {
	if h.scanBase == "" {
		return 0
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", h.scanBase+"/api/v1/scans/stats", nil)
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	var stats struct {
		CompletedToday int `json:"completed_today"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return 0
	}
	return stats.CompletedToday
}

func (h *PublicBFFHandler) fetchCriticalThreats(ctx context.Context) int {
	if h.findingBase == "" {
		return 0
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", h.findingBase+"/internal/stats", nil)
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	var result struct {
		BySeverity map[string]int `json:"by_severity"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0
	}
	return result.BySeverity["Critical"]
}

func (h *PublicBFFHandler) fetchKEVActive(ctx context.Context) int {
	if h.dataBase == "" {
		return 0
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", h.dataBase+"/api/v1/kev/stats", nil)
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	var stats struct {
		Active int `json:"active"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return 0
	}
	return stats.Active
}
