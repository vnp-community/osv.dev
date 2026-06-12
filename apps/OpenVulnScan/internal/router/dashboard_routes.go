// Package router — dashboard_routes.go
// Dashboard routes: compose stats từ scan + finding data via Postgres.
// Không có business logic mới — pure aggregation queries.
package router

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DashboardQuerier provides DB access for dashboard routes.
// Implemented by App via the DB() method.
type DashboardQuerier interface {
	DB() *pgxpool.Pool
}

// DashboardStats aggregated response for GET /api/v1/dashboard.
type DashboardStats struct {
	TotalScans        int            `json:"total_scans"`
	ActiveScans       int            `json:"active_scans"`
	CompletedScans    int            `json:"completed_scans"`
	FailedScans       int            `json:"failed_scans"`
	TotalFindings     int            `json:"total_findings"`
	ActiveFindings    int            `json:"active_findings"`
	CriticalFindings  int            `json:"critical_findings"`
	HighFindings      int            `json:"high_findings"`
	MediumFindings    int            `json:"medium_findings"`
	LowFindings       int            `json:"low_findings"`
	RecentScans       []interface{}  `json:"recent_scans"`
	TopCVEs           []interface{}  `json:"top_cves"`
	SeverityBreakdown map[string]int `json:"severity_breakdown"`
	GeneratedAt       time.Time      `json:"generated_at"`
}

// mountDashboardRoutes mounts all dashboard routes.
func mountDashboardRoutes(r chi.Router, q DashboardQuerier) {
	r.Get("/api/v1/dashboard", dashboardSummaryHandler(q))
	r.Get("/api/v1/dashboard/timeline", dashboardTimelineHandler(q))
	r.Get("/api/v1/dashboard/top-vulnerabilities", dashboardTopVulnsHandler(q))
	r.Get("/api/v1/dashboard/scan-activity", dashboardScanActivityHandler(q))
}

// dashboardSummaryHandler returns aggregated stats.
func dashboardSummaryHandler(q DashboardQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		db := q.DB()
		if db == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not ready"})
			return
		}

		stats := DashboardStats{GeneratedAt: time.Now().UTC()}

		// 1. Scan counts by status
		db.QueryRow(ctx, `
			SELECT
				COUNT(*) AS total,
				COUNT(*) FILTER (WHERE status = 'running')   AS active,
				COUNT(*) FILTER (WHERE status = 'completed') AS completed,
				COUNT(*) FILTER (WHERE status = 'failed')    AS failed
			FROM scans
		`).Scan(&stats.TotalScans, &stats.ActiveScans, &stats.CompletedScans, &stats.FailedScans) //nolint:errcheck

		// 2. Finding counts by severity
		db.QueryRow(ctx, `
			SELECT
				COUNT(*)                                                               AS total,
				COUNT(*) FILTER (WHERE active = true)                                  AS active,
				COUNT(*) FILTER (WHERE severity = 'Critical' AND active = true)        AS critical,
				COUNT(*) FILTER (WHERE severity = 'High'     AND active = true)        AS high,
				COUNT(*) FILTER (WHERE severity = 'Medium'   AND active = true)        AS medium,
				COUNT(*) FILTER (WHERE severity = 'Low'      AND active = true)        AS low
			FROM findings
		`).Scan(&stats.TotalFindings, &stats.ActiveFindings, &stats.CriticalFindings,
			&stats.HighFindings, &stats.MediumFindings, &stats.LowFindings) //nolint:errcheck

		stats.SeverityBreakdown = map[string]int{
			"critical": stats.CriticalFindings,
			"high":     stats.HighFindings,
			"medium":   stats.MediumFindings,
			"low":      stats.LowFindings,
		}

		// 3. Recent scans (last 5)
		stats.RecentScans = queryRecentScans(ctx, db, 5)

		// 4. Top CVEs by finding count
		stats.TopCVEs = queryTopCVEs(ctx, db, 10)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats) //nolint:errcheck
	}
}

// dashboardTimelineHandler returns scan activity over the last 30 days.
func dashboardTimelineHandler(q DashboardQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		db := q.DB()
		if db == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not ready"})
			return
		}

		rows, err := db.Query(ctx, `
			SELECT
				DATE(created_at) AS date,
				COUNT(*) AS scan_count,
				COUNT(*) FILTER (WHERE status = 'completed') AS completed_count,
				COUNT(*) FILTER (WHERE status = 'failed') AS failed_count
			FROM scans
			WHERE created_at >= NOW() - INTERVAL '30 days'
			GROUP BY DATE(created_at)
			ORDER BY date DESC
		`)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{"timeline": []interface{}{}})
			return
		}
		defer rows.Close()

		timeline := collectTimeline(rows)
		writeJSON(w, http.StatusOK, map[string]interface{}{"timeline": timeline, "days": 30})
	}
}

// dashboardTopVulnsHandler returns top CVEs ranked by finding count.
func dashboardTopVulnsHandler(q DashboardQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		db := q.DB()
		if db == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not ready"})
			return
		}
		topVulns := queryTopCVEs(r.Context(), db, 20)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"top_vulnerabilities": topVulns,
			"total":               len(topVulns),
		})
	}
}

// dashboardScanActivityHandler returns hourly scan activity for the last 24h.
func dashboardScanActivityHandler(q DashboardQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		db := q.DB()
		if db == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not ready"})
			return
		}

		rows, err := db.Query(ctx, `
			SELECT
				DATE_TRUNC('hour', created_at) AS hour,
				COUNT(*) AS scan_count
			FROM scans
			WHERE created_at >= NOW() - INTERVAL '24 hours'
			GROUP BY DATE_TRUNC('hour', created_at)
			ORDER BY hour DESC
		`)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{"activity": []interface{}{}})
			return
		}
		defer rows.Close()

		var activity []map[string]interface{}
		for rows.Next() {
			var hour time.Time
			var count int
			if err := rows.Scan(&hour, &count); err != nil {
				continue
			}
			activity = append(activity, map[string]interface{}{
				"hour":       hour.Format("2006-01-02 15:00"),
				"scan_count": count,
			})
		}
		if activity == nil {
			activity = []map[string]interface{}{}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"activity": activity})
	}
}

// ── Query helpers ──────────────────────────────────────────────────────────────

func queryRecentScans(ctx context.Context, db *pgxpool.Pool, limit int) []interface{} {
	rows, err := db.Query(ctx, `
		SELECT id::text, target, status, created_at, completed_at
		FROM scans
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return []interface{}{}
	}
	defer rows.Close()

	var result []interface{}
	for rows.Next() {
		var id, target, status string
		var createdAt time.Time
		var completedAt *time.Time
		if err := rows.Scan(&id, &target, &status, &createdAt, &completedAt); err != nil {
			continue
		}
		result = append(result, map[string]interface{}{
			"id": id, "target": target, "status": status,
			"created_at": createdAt, "completed_at": completedAt,
		})
	}
	if result == nil {
		return []interface{}{}
	}
	return result
}

func queryTopCVEs(ctx context.Context, db *pgxpool.Pool, limit int) []interface{} {
	rows, err := db.Query(ctx, `
		SELECT cve, severity, COUNT(*) AS count
		FROM findings
		WHERE cve IS NOT NULL AND cve != '' AND active = true
		GROUP BY cve, severity
		ORDER BY count DESC, severity DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return []interface{}{}
	}
	defer rows.Close()

	var result []interface{}
	for rows.Next() {
		var cve, severity string
		var count int
		if err := rows.Scan(&cve, &severity, &count); err != nil {
			continue
		}
		result = append(result, map[string]interface{}{
			"cve": cve, "severity": severity, "affected_hosts": count,
		})
	}
	if result == nil {
		return []interface{}{}
	}
	return result
}

func collectTimeline(rows pgx.Rows) []map[string]interface{} {
	var timeline []map[string]interface{}
	for rows.Next() {
		var date time.Time
		var scanCount, completedCount, failedCount int
		if err := rows.Scan(&date, &scanCount, &completedCount, &failedCount); err != nil {
			continue
		}
		timeline = append(timeline, map[string]interface{}{
			"date":            date.Format("2006-01-02"),
			"scan_count":      scanCount,
			"completed_count": completedCount,
			"failed_count":    failedCount,
		})
	}
	if timeline == nil {
		return []map[string]interface{}{}
	}
	return timeline
}
