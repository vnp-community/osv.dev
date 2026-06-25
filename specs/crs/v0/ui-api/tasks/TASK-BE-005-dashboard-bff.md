# TASK-BE-005 — gateway: Dashboard BFF Handler

| Field | Value |
|-------|-------|
| **Task ID** | TASK-BE-005 |
| **Service** | `apps/osv` (gateway) |
| **Solution Ref** | [SOL-UI-002 §2.2–2.4](../solutions/SOL-UI-002-dashboard-bff-sse.md) |
| **Priority** | 🔴 P0 |
| **Depends On** | TASK-BE-006 (finding-service /internal/* endpoints) |
| **Estimated** | 5h |
| **Status** | ✅ DONE |

---

## Context

`GET /api/v1/dashboard` là endpoint **blocking P0** — app không load được nếu thiếu. Gateway hiện là pure reverse proxy, chưa có logic aggregate. Dashboard cần dữ liệu từ 5 services đồng thời:
- `finding-service:8085/internal/stats` — KPI counts
- `finding-service:8085/internal/risk-trend?period=X` — monthly trend
- `finding-service:8085/internal/product-grades` — product grades
- `data-service:8082/v1/kev?limit=5` — recent KEV alerts
- `finding-service:8085/internal/sla-breaches` — SLA breach data

---

## Goal

Tạo BFF handler trong gateway với:
1. Fan-out concurrent HTTP calls đến 5 services
2. Redis cache 60s per (user, period)
3. `GET /api/v1/dashboard/sla` endpoint cho SLA detail page

---

## Target Files

| Action | File Path |
|--------|-----------|
| CREATE | `apps/osv/internal/gateway/bff/dashboard.go` |
| CREATE | `apps/osv/internal/gateway/bff/types.go` |
| CREATE | `apps/osv/internal/gateway/bff/clients.go` |
| MODIFY | `apps/osv/internal/gateway/router.go` |

---

## Implementation

### File 1: `apps/osv/internal/gateway/bff/types.go`

```go
package bff

import "time"

// DashboardResponse is the complete dashboard API response
type DashboardResponse struct {
	KPIs                *KPIData          `json:"kpis"`
	RiskTrend           []TrendPoint      `json:"risk_trend"`
	SeverityDistribution *SeverityDist    `json:"severity_distribution"`
	ProductGrades       []ProductGrade    `json:"product_grades"`
	KEVAlerts           []KEVAlert        `json:"kev_alerts"`
	RecentScans         []ScanSummary     `json:"recent_scans"`
	SLABreaches         []SLABreachItem   `json:"sla_breaches"`
}

type KPIData struct {
	CriticalFindings int     `json:"critical_findings"`
	HighFindings     int     `json:"high_findings"`
	MediumFindings   int     `json:"medium_findings"`
	LowFindings      int     `json:"low_findings"`
	TotalAssets      int     `json:"total_assets"`
	ActiveScans      int     `json:"active_scans"`
	SecurityGrade    string  `json:"security_grade"`  // A-F
	SecurityScore    int     `json:"security_score"`  // 0-100
	SLACompliance    float64 `json:"sla_compliance"`  // 0.0-100.0
	SLAAtRisk        int     `json:"sla_at_risk"`
	SLABreached      int     `json:"sla_breached"`
}

type TrendPoint struct {
	Month    string `json:"month"` // "2026-01"
	Critical int    `json:"critical"`
	High     int    `json:"high"`
	Medium   int    `json:"medium"`
	Low      int    `json:"low"`
}

type SeverityDist struct {
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Total    int `json:"total"`
}

type ProductGrade struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Grade         string `json:"grade"`
	Score         int    `json:"score"`
	CriticalCount int    `json:"critical_count"`
	HighCount     int    `json:"high_count"`
	Trend         string `json:"trend"` // "improving" | "worsening" | "stable"
}

type KEVAlert struct {
	CveID       string `json:"cve_id"`
	Description string `json:"description"`
	DateAdded   string `json:"date_added"`
	Vendor      string `json:"vendor"`
	Product     string `json:"product"`
	IsKnownRansomware bool `json:"is_known_ransomware"`
}

type ScanSummary struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	Status       string  `json:"status"`
	FindingCount int     `json:"finding_count"`
	CreatedAt    string  `json:"created_at"`
}

type SLABreachItem struct {
	FindingID    string `json:"finding_id"`
	Title        string `json:"title"`
	Severity     string `json:"severity"`
	ProductName  string `json:"product_name"`
	DaysOverdue  int    `json:"days_overdue"`
	ExpiresAt    string `json:"expires_at"`
}
```

### File 2: `apps/osv/internal/gateway/bff/clients.go`

```go
package bff

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// InternalClient makes HTTP calls to internal services
type InternalClient struct {
	httpClient *http.Client
}

func NewInternalClient() *InternalClient {
	return &InternalClient{
		httpClient: &http.Client{Timeout: 3 * time.Second},
	}
}

func (c *InternalClient) get(ctx context.Context, url string, dst interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

// FetchStats fetches KPI stats from finding-service
func (c *InternalClient) FetchStats(ctx context.Context, findingSvcAddr string) (*KPIData, error) {
	var result KPIData
	err := c.get(ctx, "http://"+findingSvcAddr+"/internal/stats", &result)
	return &result, err
}

// FetchRiskTrend fetches monthly risk trend from finding-service
func (c *InternalClient) FetchRiskTrend(ctx context.Context, findingSvcAddr, period string) ([]TrendPoint, error) {
	var result []TrendPoint
	url := fmt.Sprintf("http://%s/internal/risk-trend?period=%s", findingSvcAddr, period)
	err := c.get(ctx, url, &result)
	return result, err
}

// FetchProductGrades fetches product grades from finding-service
func (c *InternalClient) FetchProductGrades(ctx context.Context, findingSvcAddr string) ([]ProductGrade, error) {
	var result []ProductGrade
	err := c.get(ctx, "http://"+findingSvcAddr+"/internal/product-grades", &result)
	return result, err
}

// FetchRecentKEV fetches last 5 KEV entries from data-service
func (c *InternalClient) FetchRecentKEV(ctx context.Context, dataSvcAddr string) ([]KEVAlert, error) {
	var result struct {
		Entries []KEVAlert `json:"entries"`
	}
	url := fmt.Sprintf("http://%s/v1/kev?limit=5&sort_by=date_added_desc", dataSvcAddr)
	err := c.get(ctx, url, &result)
	return result.Entries, err
}

// FetchSLABreaches fetches top SLA breached findings from finding-service
func (c *InternalClient) FetchSLABreaches(ctx context.Context, findingSvcAddr string) ([]SLABreachItem, error) {
	var result []SLABreachItem
	err := c.get(ctx, "http://"+findingSvcAddr+"/internal/sla-breaches?limit=5", &result)
	return result, err
}
```

### File 3: `apps/osv/internal/gateway/bff/dashboard.go`

```go
package bff

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// DashboardBFF handles the dashboard aggregate endpoint
type DashboardBFF struct {
	client        *InternalClient
	redis         *redis.Client
	findingSvcAddr string  // "finding-service:8085"
	dataSvcAddr    string  // "data-service:8082"
}

// NewDashboardBFF creates a new dashboard BFF handler
func NewDashboardBFF(
	redis *redis.Client,
	findingSvcAddr, dataSvcAddr string,
) *DashboardBFF {
	return &DashboardBFF{
		client:         NewInternalClient(),
		redis:          redis,
		findingSvcAddr: findingSvcAddr,
		dataSvcAddr:    dataSvcAddr,
	}
}

// HandleDashboard — GET /api/v1/dashboard
func (bff *DashboardBFF) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "30d"
	}
	// Validate period
	validPeriods := map[string]bool{"30d": true, "90d": true, "1y": true}
	if !validPeriods[period] {
		period = "30d"
	}

	userID   := r.Header.Get("X-User-ID")
	cacheKey := "dashboard:" + userID + ":" + period

	// Try cache first
	if cached, err := bff.redis.Get(r.Context(), cacheKey).Bytes(); err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		w.Write(cached)
		return
	}

	// Fan-out to services concurrently
	resp := bff.fetchAll(r.Context(), period)

	// Compute severity distribution from KPIs
	if resp.KPIs != nil {
		resp.SeverityDistribution = &SeverityDist{
			Critical: resp.KPIs.CriticalFindings,
			High:     resp.KPIs.HighFindings,
			Medium:   resp.KPIs.MediumFindings,
			Low:      resp.KPIs.LowFindings,
		}
		resp.SeverityDistribution.Total =
			resp.SeverityDistribution.Critical +
			resp.SeverityDistribution.High +
			resp.SeverityDistribution.Medium +
			resp.SeverityDistribution.Low
	}

	data, _ := json.Marshal(resp)

	// Cache 60s
	bff.redis.Set(r.Context(), cacheKey, data, 60*time.Second)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.Write(data)
}

// HandleDashboardSLA — GET /api/v1/dashboard/sla
func (bff *DashboardBFF) HandleDashboardSLA(w http.ResponseWriter, r *http.Request) {
	// Delegate to sla-service internal endpoint
	// This is a thin proxy to sla-service dashboard endpoint
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	var result map[string]interface{}
	err := bff.client.get(ctx, "http://"+bff.findingSvcAddr+"/internal/sla-dashboard", &result)
	if err != nil {
		log.Error().Err(err).Msg("failed to fetch SLA dashboard")
		http.Error(w, `{"error":"SERVICE_UNAVAILABLE","message":"SLA service unavailable"}`, 503)
		return
	}

	data, _ := json.Marshal(result)
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// fetchAll performs concurrent fan-out to all required services
func (bff *DashboardBFF) fetchAll(ctx context.Context, period string) *DashboardResponse {
	resp := &DashboardResponse{}
	var mu sync.Mutex
	var wg sync.WaitGroup

	type task struct {
		name string
		fn   func()
	}

	tasks := []task{
		{"kpis", func() {
			data, err := bff.client.FetchStats(ctx, bff.findingSvcAddr)
			if err != nil {
				log.Warn().Err(err).Msg("failed to fetch KPI stats")
				return
			}
			mu.Lock(); resp.KPIs = data; mu.Unlock()
		}},
		{"risk_trend", func() {
			data, err := bff.client.FetchRiskTrend(ctx, bff.findingSvcAddr, period)
			if err != nil {
				log.Warn().Err(err).Msg("failed to fetch risk trend")
				return
			}
			mu.Lock(); resp.RiskTrend = data; mu.Unlock()
		}},
		{"product_grades", func() {
			data, err := bff.client.FetchProductGrades(ctx, bff.findingSvcAddr)
			if err != nil {
				log.Warn().Err(err).Msg("failed to fetch product grades")
				return
			}
			mu.Lock(); resp.ProductGrades = data; mu.Unlock()
		}},
		{"kev_alerts", func() {
			data, err := bff.client.FetchRecentKEV(ctx, bff.dataSvcAddr)
			if err != nil {
				log.Warn().Err(err).Msg("failed to fetch KEV alerts")
				return
			}
			mu.Lock(); resp.KEVAlerts = data; mu.Unlock()
		}},
		{"sla_breaches", func() {
			data, err := bff.client.FetchSLABreaches(ctx, bff.findingSvcAddr)
			if err != nil {
				log.Warn().Err(err).Msg("failed to fetch SLA breaches")
				return
			}
			mu.Lock(); resp.SLABreaches = data; mu.Unlock()
		}},
	}

	wg.Add(len(tasks))
	for _, t := range tasks {
		tt := t
		go func() { defer wg.Done(); tt.fn() }()
	}
	wg.Wait()

	return resp
}
```

### Gateway Router changes:

```go
// apps/osv/internal/gateway/router.go — thêm vào setup function

// Initialize BFF
dashBFF := bff.NewDashboardBFF(redisClient, "finding-service:8085", "data-service:8082")

// Dashboard BFF routes (handled in gateway process, not forwarded)
mux.Handle("GET /api/v1/dashboard",     am.Authenticate(http.HandlerFunc(dashBFF.HandleDashboard)))
mux.Handle("GET /api/v1/dashboard/sla", am.Authenticate(http.HandlerFunc(dashBFF.HandleDashboardSLA)))
```

---

## Verification

```bash
cd apps/osv
go build ./...

# With services running:
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/dashboard | jq 'keys'
# Expected: ["kev_alerts","kpis","product_grades","risk_trend","severity_distribution","sla_breaches"]

# Check Redis cache (2nd call)
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/dashboard -v 2>&1 | grep "X-Cache"
# Expected: X-Cache: HIT
```

---

## Checklist

- [x] `types.go` định nghĩa đủ 7 response structs
- [x] `clients.go` có đủ 5 `Fetch*` methods với `context.WithTimeout(3s)`
- [x] `dashboard.go` fan-out 5 services concurrently với `sync.WaitGroup`
- [x] Partial failure: nếu 1 service fail, response vẫn trả về với nil cho field đó (graceful degradation)
- [x] Redis cache key: `dashboard:{userID}:{period}`, TTL 60s
- [x] Cache HIT response có `X-Cache: HIT` header
- [x] `period` validated: chỉ nhận "30d" | "90d" | "1y", default "30d"
- [x] `go build ./...` thành công
- [x] `HandleDashboardSLA` proxies sang finding-service internal

## Notes for AI

- Addresses của services (`finding-service:8085`) nên đọc từ config/env thay vì hardcode
- BFF không forward qua gateway để tránh circular routing — gọi trực tiếp internal HTTP
- Nếu Redis không available, tiếp tục hoạt động mà không cache (không fail)
