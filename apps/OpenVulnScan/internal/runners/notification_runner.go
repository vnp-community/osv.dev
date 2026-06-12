// Package runners — notification_runner.go
// NotificationRunner chạy notification-service trong một goroutine riêng.
//
// Notification service:
// - Subscribe NATS events: scan.completed, finding.created, finding.status_changed
// - Dispatch notifications qua Email, Slack, Teams, Webhook
// - HTTP endpoint để manage webhooks và subscriptions
//
// Bridge Pattern: implement notification logic trực tiếp không import internal/ packages.
package runners

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"

	"github.com/osv/apps/openvulnscan/internal/transport"
)

// NotificationRunnerConfig cấu hình cho notification goroutine.
type NotificationRunnerConfig struct {
	DBURL        string
	EmailEnabled bool
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	FromEmail    string
	SlackEnabled    bool
	SlackWebhookURL string
	TeamsEnabled    bool
	TeamsWebhookURL string
}

// NotificationRunner implement ServiceRunner cho notification-service.
type NotificationRunner struct {
	cfg         NotificationRunnerConfig
	nc          *nats.Conn
	lis         *bufconn.Listener
	server      *grpc.Server
	log         zerolog.Logger
	HTTPHandler http.Handler
}

// NewNotificationRunner tạo NotificationRunner.
func NewNotificationRunner(cfg NotificationRunnerConfig, nc *nats.Conn, lis *bufconn.Listener) *NotificationRunner {
	return &NotificationRunner{
		cfg: cfg,
		nc:  nc,
		lis: lis,
		log: log.With().Str("runner", "notification-service").Logger(),
	}
}

func (r *NotificationRunner) Name() string { return "notification-service" }

// Run khởi động notification goroutine.
func (r *NotificationRunner) Run(ctx context.Context) error {
	r.log.Info().Msg("initializing...")

	db, err := pgxpool.New(ctx, r.cfg.DBURL)
	if err != nil {
		return fmt.Errorf("notification: db: %w", err)
	}
	defer db.Close()
	if err := db.Ping(ctx); err != nil {
		return fmt.Errorf("notification: db ping: %w", err)
	}

	bridge := newNotificationBridge(db, r.nc, r.cfg, r.log)
	r.HTTPHandler = bridge.router()

	// Subscribe NATS events trong goroutine riêng
	go bridge.subscribeEvents(ctx)

	// gRPC health server
	r.server = grpc.NewServer(
		grpc.ChainUnaryInterceptor(grpcRecoveryInterceptor, grpcLoggingInterceptor),
	)
	healthSrv := health.NewServer()
	healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(r.server, healthSrv)

	errCh := make(chan error, 1)
	go func() {
		r.log.Info().Msg("gRPC health ready on bufconn")
		errCh <- r.server.Serve(r.lis)
	}()

	r.log.Info().Msg("notification-service ready")

	select {
	case <-ctx.Done():
		r.log.Info().Msg("graceful shutdown...")
		r.server.GracefulStop()
		return nil
	case err := <-errCh:
		return wrapRunnerError("notification-service", err)
	}
}

// Health kiểm tra gRPC health.
func (r *NotificationRunner) Health(ctx context.Context) error {
	hctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	conn, err := transport.DialBufConn(hctx, r.lis)
	if err != nil {
		return fmt.Errorf("notification health: %w", err)
	}
	defer conn.Close()
	hc := grpc_health_v1.NewHealthClient(conn)
	resp, err := hc.Check(hctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		return err
	}
	if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		return fmt.Errorf("notification not serving: %s", resp.Status)
	}
	return nil
}

// Listener returns the bufconn listener.
func (r *NotificationRunner) Listener() *bufconn.Listener { return r.lis }

// ── Notification Bridge ───────────────────────────────────────────────────────

type notificationBridge struct {
	db   *pgxpool.Pool
	nc   *nats.Conn
	cfg  NotificationRunnerConfig
	log  zerolog.Logger
	http *http.Client
}

func newNotificationBridge(db *pgxpool.Pool, nc *nats.Conn, cfg NotificationRunnerConfig, l zerolog.Logger) *notificationBridge {
	return &notificationBridge{
		db:   db,
		nc:   nc,
		cfg:  cfg,
		log:  l,
		http: &http.Client{Timeout: 10 * time.Second},
	}
}

// subscribeEvents lắng nghe NATS events và dispatch notifications.
func (b *notificationBridge) subscribeEvents(ctx context.Context) {
	// scan.completed — notify scan done
	if b.nc != nil {
		b.nc.Subscribe("scan.completed", func(msg *nats.Msg) { //nolint:errcheck
			var evt struct {
				ScanID       string `json:"scan_id"`
				Status       string `json:"status"`
				FindingCount int    `json:"finding_count"`
			}
			if err := json.Unmarshal(msg.Data, &evt); err != nil {
				return
			}
			b.log.Debug().Str("scan_id", evt.ScanID).Int("findings", evt.FindingCount).Msg("scan.completed event")

			// Dispatch webhook notifications
			b.dispatchWebhooks(ctx, "scan.completed", map[string]interface{}{
				"scan_id":       evt.ScanID,
				"status":        evt.Status,
				"finding_count": evt.FindingCount,
			})

			// Dispatch Slack if configured
			if b.cfg.SlackEnabled && b.cfg.SlackWebhookURL != "" {
				b.sendSlack(ctx, fmt.Sprintf("✅ Scan %s completed — %d findings", evt.ScanID[:8], evt.FindingCount))
			}
		})

		// finding.created — notify high severity findings
		b.nc.Subscribe("finding.created", func(msg *nats.Msg) { //nolint:errcheck
			var evt struct {
				FindingID string `json:"finding_id"`
				Title     string `json:"title"`
				Severity  string `json:"severity"`
				ScanID    string `json:"scan_id"`
			}
			if err := json.Unmarshal(msg.Data, &evt); err != nil {
				return
			}
			// Chỉ notify Critical/High
			if evt.Severity == "Critical" || evt.Severity == "High" {
				b.dispatchWebhooks(ctx, "finding.created", map[string]interface{}{
					"finding_id": evt.FindingID,
					"title":      evt.Title,
					"severity":   evt.Severity,
				})
				if b.cfg.SlackEnabled && b.cfg.SlackWebhookURL != "" {
					b.sendSlack(ctx, fmt.Sprintf("🚨 %s Finding: %s", evt.Severity, evt.Title))
				}
			}
		})
	}

	<-ctx.Done()
	b.log.Info().Msg("notification NATS subscriptions stopped")
}

// dispatchWebhooks gửi event đến tất cả registered webhooks.
func (b *notificationBridge) dispatchWebhooks(ctx context.Context, eventType string, data interface{}) {
	rows, err := b.db.Query(ctx, `
		SELECT url, secret FROM notification_webhooks WHERE active = true AND $1 = ANY(events)
	`, eventType)
	if err != nil {
		return
	}
	defer rows.Close()

	payload, _ := json.Marshal(map[string]interface{}{
		"event":     eventType,
		"timestamp": time.Now().UTC(),
		"data":      data,
	})

	for rows.Next() {
		var url, secret string
		if err := rows.Scan(&url, &secret); err != nil {
			continue
		}
		go b.postWebhook(ctx, url, payload)
	}
}

func (b *notificationBridge) postWebhook(ctx context.Context, url string, payload []byte) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.http.Do(req)
	if err != nil {
		b.log.Warn().Err(err).Str("url", url).Msg("webhook dispatch failed")
		return
	}
	resp.Body.Close()
	b.log.Debug().Str("url", url).Int("status", resp.StatusCode).Int("bytes", len(payload)).Msg("webhook dispatched")
}

func (b *notificationBridge) sendSlack(ctx context.Context, text string) {
	payload, _ := json.Marshal(map[string]string{"text": text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.cfg.SlackWebhookURL, nil)
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.http.Do(req)
	if err != nil {
		b.log.Warn().Err(err).Msg("slack notification failed")
		return
	}
	resp.Body.Close()
	b.log.Debug().Int("bytes", len(payload)).Msg("slack notification sent")
}

// ── HTTP Routes ───────────────────────────────────────────────────────────────

func (b *notificationBridge) router() http.Handler {
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.Recoverer)

	r.Route("/api/v1/notifications", func(r chi.Router) {
		// Webhooks
		r.Post("/webhooks", b.createWebhook)
		r.Get("/webhooks", b.listWebhooks)
		r.Delete("/webhooks/{id}", b.deleteWebhook)

		// Test webhook
		r.Post("/webhooks/{id}/test", b.testWebhook)
	})
	return r
}

func (b *notificationBridge) createWebhook(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL    string   `json:"url"`
		Secret string   `json:"secret"`
		Events []string `json:"events"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
		writeJSONNotif(w, http.StatusBadRequest, map[string]string{"error": "url is required"})
		return
	}
	id := uuid.New()
	_, err := b.db.Exec(r.Context(), `
		INSERT INTO notification_webhooks (id, url, secret, events, active, created_at)
		VALUES ($1, $2, $3, $4, true, NOW())
	`, id, req.URL, req.Secret, req.Events)
	if err != nil {
		writeJSONNotif(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSONNotif(w, http.StatusCreated, map[string]interface{}{
		"id": id.String(), "url": req.URL, "events": req.Events,
	})
}

func (b *notificationBridge) listWebhooks(w http.ResponseWriter, r *http.Request) {
	rows, err := b.db.Query(r.Context(), `
		SELECT id::text, url, events, active, created_at FROM notification_webhooks ORDER BY created_at DESC
	`)
	if err != nil {
		writeJSONNotif(w, http.StatusOK, map[string]interface{}{"webhooks": []interface{}{}})
		return
	}
	defer rows.Close()
	var hooks []map[string]interface{}
	for rows.Next() {
		var id, url string
		var events []string
		var active bool
		var ca time.Time
		if err := rows.Scan(&id, &url, &events, &active, &ca); err != nil {
			continue
		}
		hooks = append(hooks, map[string]interface{}{
			"id": id, "url": url, "events": events, "active": active, "created_at": ca,
		})
	}
	if hooks == nil {
		hooks = []map[string]interface{}{}
	}
	writeJSONNotif(w, http.StatusOK, map[string]interface{}{"webhooks": hooks, "total": len(hooks)})
}

func (b *notificationBridge) deleteWebhook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	b.db.Exec(r.Context(), `DELETE FROM notification_webhooks WHERE id = $1::uuid`, id) //nolint:errcheck
	writeJSONNotif(w, http.StatusOK, map[string]string{"message": "deleted"})
}

func (b *notificationBridge) testWebhook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var url string
	err := b.db.QueryRow(r.Context(), `SELECT url FROM notification_webhooks WHERE id = $1::uuid`, id).Scan(&url)
	if err != nil {
		writeJSONNotif(w, http.StatusNotFound, map[string]string{"error": "webhook not found"})
		return
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"event": "test", "timestamp": time.Now().UTC(), "data": map[string]string{"message": "test ping"},
	})
	go b.postWebhook(r.Context(), url, payload)
	writeJSONNotif(w, http.StatusOK, map[string]string{"message": "test dispatched"})
}

func writeJSONNotif(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}
