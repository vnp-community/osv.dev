// Package repository defines CVE persistence interfaces for the search service.
package repository

import (
	"context"
	"time"

	"github.com/osv/search-service/internal/domain/entity"
)

// EPSSDistribution holds EPSS score bucket counts.
type EPSSDistribution struct {
	VeryLow  int64    `json:"very_low"`  // 0–0.1
	Low      int64    `json:"low"`       // 0.1–0.5
	High     int64    `json:"high"`      // 0.5–0.9
	Critical int64    `json:"critical"`  // 0.9–1.0
	Mean     *float64 `json:"mean"`
	Median   *float64 `json:"median"`
}

// EPSSTopEntry holds a CVE ID and its EPSS score.
type EPSSTopEntry struct {
	CVEID    string  `json:"cve_id"`
	EPSS     float64 `json:"eps"`
	EPSSPct  float64 `json:"percentile"`
	Severity string  `json:"severity"`
	Date     string  `json:"date"`
}

// EPSSPercentiles holds EPSS score at key percentiles.
type EPSSPercentiles struct {
	P50 float64 `json:"p50"`
	P90 float64 `json:"p90"`
	P95 float64 `json:"p95"`
	P99 float64 `json:"p99"`
}

// CVERepository is the read-side persistence interface.
type CVERepository interface {
	Search(ctx context.Context, filter *entity.SearchFilter) ([]*entity.CVE, int64, error)
	FindByID(ctx context.Context, id string) (*entity.CVE, error)
	FindByIDs(ctx context.Context, ids []string) ([]*entity.CVE, error)
	Count(ctx context.Context) (int64, error)
	GetAggregations(ctx context.Context) (map[string]interface{}, error)

	// CR-GCV-002: EPSS stats methods
	QueryEPSSDistribution(ctx context.Context) *EPSSDistribution
	GetTopEPSS(ctx context.Context, minEPSS float64, limit int) ([]EPSSTopEntry, error)
	GetEPSSPercentiles(ctx context.Context) (*EPSSPercentiles, error)
	CountWithEPSS(ctx context.Context) (int64, error)
	GetEPSSStats(ctx context.Context) (*EPSSStats, error)
	GetDashboardStats(ctx context.Context) (*DashboardStats, error)
}

type DashboardStats struct {
	TotalCVEs      int64              `json:"total_cves"`
	BySeverity     map[string]int64   `json:"by_severity"`
	NewThisWeek    int64              `json:"new_this_week"`
	NewThisMonth   int64              `json:"new_this_month"`
	TopVendors     []VendorCVECount   `json:"top_vendors"`
	EPSSDistrib    EPSSDistribution   `json:"epss_distribution"`
	TotalKEV       int64              `json:"total_kev"`
	TotalExploit   int64              `json:"total_exploit"`
	LastUpdated    time.Time          `json:"last_updated"`
}

type VendorCVECount struct {
	Vendor string `json:"vendor"`
	Count  int64  `json:"count"`
}

// EPSSStats aggregates EPSS scoring data.
type EPSSStats struct {
	TotalScored   int64        `json:"total_scored"`
	AvgEPSS       float64      `json:"avg_epss"`
	HighRiskCount int64        `json:"high_risk_count"`
	TopCVEs       []EPSSEntry  `json:"top_cves"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

// EPSSEntry represents a single CVE in the top EPSS list.
type EPSSEntry struct {
	CVEID          string  `json:"cve_id"`
	EPSS           float64 `json:"epss"`
	EPSSPercentile float64 `json:"epss_percentile"`
	Severity       string  `json:"severity"`
}

// CVECacheRepository caches search results in Redis.
type CVECacheRepository interface {
	GetSearchResult(ctx context.Context, key string) ([]byte, error)
	SetSearchResult(ctx context.Context, key string, data []byte, ttlSec int) error
	InvalidatePattern(ctx context.Context, pattern string) error
}

