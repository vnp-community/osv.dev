package http

// CR-009: Webhook deliveries endpoints
// - GET  /api/v1/webhooks/deliveries          → list all deliveries (filterable)
// - POST /api/v1/webhooks/deliveries/{id}/retry → retry a failed delivery
// - GET  /api/v1/webhooks/stats/hourly         → 24h hourly success/failed counts
//
// These complement the existing /{id}/deliveries endpoint (per-webhook history).
// The new /deliveries endpoint is flat — queryable by webhook_id, status, etc.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DeliveryHandler handles the new delivery-related HTTP endpoints.
// It uses the pgxpool directly to query webhook_deliveries, keeping
// the domain/repository layer unchanged.
type DeliveryHandler struct {
	db *pgxpool.Pool
}

// NewDeliveryHandler creates a DeliveryHandler backed by the given pool.
func NewDeliveryHandler(db *pgxpool.Pool) *DeliveryHandler {
	return &DeliveryHandler{db: db}
}

// DeliveryDTO is the JSON shape for a single delivery record.
type DeliveryDTO struct {
	ID             string    `json:"id"`
	WebhookID      string    `json:"webhook_id"`
	Event          string    `json:"event"`
	Endpoint       string    `json:"endpoint"`
	Status         string    `json:"status"`
	ResponseTimeMs int       `json:"response_time_ms"`
	StatusCode     int       `json:"status_code"`
	RequestBody    string    `json:"request_body,omitempty"`
	ResponseBody   string    `json:"response_body,omitempty"`
	Time           time.Time `json:"time"`
}

// ListWebhookDeliveries handles GET /api/v1/webhooks/deliveries
// Supports query params: webhook_id, status (success|failed|retried), page, page_size.
// Falls back to empty list gracefully if table does not exist.
func (h *DeliveryHandler) ListWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	webhookID := q.Get("webhook_id")
	status := q.Get("status")
	page := parseIntDef(q.Get("page"), 1)
	pageSize := parseIntDef(q.Get("page_size"), 50)
	if pageSize > 200 {
		pageSize = 200
	}

	// Build dynamic WHERE
	var conds []string
	var args []interface{}
	idx := 1
	if webhookID != "" {
		conds = append(conds, fmt.Sprintf("webhook_id = $%d", idx))
		args = append(args, webhookID)
		idx++
	}
	if status != "" {
		conds = append(conds, fmt.Sprintf("status = $%d", idx))
		args = append(args, status)
		idx++
	}
	where := "WHERE 1=1"
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	// Count
	var total int
	countQ := fmt.Sprintf("SELECT COUNT(*) FROM webhook_deliveries %s", where)
	if err := h.db.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		// Table may not exist yet — return empty gracefully
		respondDeliveryJSON(w, 200, map[string]interface{}{"deliveries": []interface{}{}, "total": 0})
		return
	}

	// Data
	dataQ := fmt.Sprintf(`
		SELECT id, webhook_id, event, endpoint, status,
		       COALESCE(response_time_ms, 0), COALESCE(status_code, 0),
		       COALESCE(request_body, ''), COALESCE(response_body, ''),
		       COALESCE(time, created_at, NOW())
		FROM webhook_deliveries %s
		ORDER BY COALESCE(time, created_at, NOW()) DESC
		LIMIT $%d OFFSET $%d`, where, idx, idx+1)
	pageArgs := append(args, pageSize, (page-1)*pageSize)

	rows, err := h.db.Query(ctx, dataQ, pageArgs...)
	if err != nil {
		respondDeliveryJSON(w, 200, map[string]interface{}{"deliveries": []interface{}{}, "total": 0})
		return
	}
	defer rows.Close()

	deliveries := []DeliveryDTO{}
	for rows.Next() {
		var d DeliveryDTO
		if err := rows.Scan(
			&d.ID, &d.WebhookID, &d.Event, &d.Endpoint, &d.Status,
			&d.ResponseTimeMs, &d.StatusCode,
			&d.RequestBody, &d.ResponseBody,
			&d.Time,
		); err == nil {
			deliveries = append(deliveries, d)
		}
	}

	respondDeliveryJSON(w, 200, map[string]interface{}{
		"deliveries": deliveries,
		"total":      total,
	})
}

// RetryWebhookDelivery handles POST /api/v1/webhooks/deliveries/{id}/retry
// Only failed/retried deliveries can be retried. Returns 422 for successful ones.
// The actual HTTP re-dispatch is fire-and-forget to avoid blocking the response.
func (h *DeliveryHandler) RetryWebhookDelivery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	deliveryID := chi.URLParam(r, "id")

	// Load original delivery
	var orig struct {
		WebhookID   string
		Event       string
		Endpoint    string
		Status      string
		RequestBody string
	}
	err := h.db.QueryRow(ctx, `
		SELECT webhook_id, event, endpoint, status, COALESCE(request_body, '')
		FROM webhook_deliveries WHERE id = $1
	`, deliveryID).Scan(
		&orig.WebhookID, &orig.Event, &orig.Endpoint,
		&orig.Status, &orig.RequestBody,
	)
	if err != nil {
		respondDeliveryJSON(w, 404, map[string]string{"error": "NOT_FOUND", "message": "delivery not found"})
		return
	}

	// Cannot retry successful delivery
	if orig.Status == "success" {
		respondDeliveryJSON(w, 422, map[string]string{
			"error":   "CANNOT_RETRY",
			"message": "cannot retry a successful delivery",
		})
		return
	}

	// Load webhook URL (for re-dispatch)
	var webhookURL string
	h.db.QueryRow(ctx, `SELECT url FROM webhooks WHERE id = $1`, orig.WebhookID).Scan(&webhookURL) //nolint:errcheck

	newID := fmt.Sprintf("dlv_retry_%s_%d", deliveryID[:8], time.Now().UnixMilli())

	// Async re-dispatch — non-blocking
	go func() {
		status := "failed"
		statusCode := 0
		elapsed := 0
		if webhookURL != "" {
			client := &http.Client{Timeout: 10 * time.Second}
			start := time.Now()
			resp, httpErr := client.Post(webhookURL, "application/json",
				strings.NewReader(orig.RequestBody))
			elapsed = int(time.Since(start).Milliseconds())
			if httpErr == nil {
				defer resp.Body.Close()
				statusCode = resp.StatusCode
				if resp.StatusCode < 400 {
					status = "success"
				}
			}
		}

		h.db.Exec(ctx, `
			INSERT INTO webhook_deliveries
				(id, webhook_id, event, endpoint, status, response_time_ms, status_code, request_body, time)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
			ON CONFLICT (id) DO NOTHING
		`, newID, orig.WebhookID, orig.Event, orig.Endpoint, status, elapsed, statusCode, orig.RequestBody) //nolint:errcheck
	}()

	respondDeliveryJSON(w, 200, map[string]interface{}{
		"id":          newID,
		"webhook_id":  orig.WebhookID,
		"event":       orig.Event,
		"endpoint":    orig.Endpoint,
		"status":      "queued",
		"original_id": deliveryID,
		"queued_at":   time.Now().UTC(),
	})
}

// HourlyStatsDTO is one hour bucket in the 24h chart.
type HourlyStatsDTO struct {
	H       string `json:"h"`
	Success int    `json:"success"`
	Failed  int    `json:"failed"`
}

// GetWebhookHourlyStats handles GET /api/v1/webhooks/stats/hourly
// Returns up to 24 buckets showing success/failed counts per UTC hour.
// Gracefully returns empty array if table doesn't exist.
func (h *DeliveryHandler) GetWebhookHourlyStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := h.db.Query(ctx, `
		SELECT
			to_char(date_trunc('hour', COALESCE(time, created_at) AT TIME ZONE 'UTC'), 'HH24:00') AS hour_label,
			COUNT(*) FILTER (WHERE status = 'success')                    AS success,
			COUNT(*) FILTER (WHERE status IN ('failed', 'retried'))       AS failed
		FROM webhook_deliveries
		WHERE COALESCE(time, created_at) >= NOW() - INTERVAL '24 hours'
		GROUP BY date_trunc('hour', COALESCE(time, created_at) AT TIME ZONE 'UTC')
		ORDER BY date_trunc('hour', COALESCE(time, created_at) AT TIME ZONE 'UTC')
	`)
	if err != nil {
		// Table may not exist — empty graceful response
		respondDeliveryJSON(w, 200, []HourlyStatsDTO{})
		return
	}
	defer rows.Close()

	stats := []HourlyStatsDTO{}
	for rows.Next() {
		var s HourlyStatsDTO
		if err := rows.Scan(&s.H, &s.Success, &s.Failed); err == nil {
			stats = append(stats, s)
		}
	}

	respondDeliveryJSON(w, 200, stats)
}

func respondDeliveryJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func parseIntDef(s string, def int) int {
	if s == "" {
		return def
	}
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err == nil && n > 0 {
		return n
	}
	return def
}
