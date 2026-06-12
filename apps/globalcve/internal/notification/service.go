// Package notification is the Notification service goroutine.
// Handles webhooks, alerts, and event dispatching.
package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog/log"

	"github.com/globalcve/mono/internal/config"
	infraNATS "github.com/globalcve/mono/internal/infra/nats"
)

// Webhook represents a registered webhook endpoint.
type Webhook struct {
	ID        int64     `json:"id"`
	URL       string    `json:"url"`
	Events    []string  `json:"events"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

// Service is the Notification goroutine service.
type Service struct {
	cfg    config.Config
	pool   *pgxpool.Pool
	nats   *infraNATS.Client
	server *http.Server
}

// New creates a new Notification service.
func New(cfg config.Config, pool *pgxpool.Pool, nats *infraNATS.Client) *Service {
	return &Service{cfg: cfg, pool: pool, nats: nats}
}

// Start launches the Notification service goroutine.
func (s *Service) Start(ctx context.Context) error {
	// Subscribe to NATS events
	if s.nats != nil {
		go s.subscribeEvents(ctx)
	}

	// HTTP server
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

	r.Get("/health", s.handleHealth)
	r.Route("/api/v2/webhooks", func(r chi.Router) {
		r.Get("/", s.handleListWebhooks)
		r.Post("/", s.handleCreateWebhook)
		r.Delete("/{id}", s.handleDeleteWebhook)
	})

	addr := fmt.Sprintf(":%d", s.cfg.Server.NotificationPort)
	s.server = &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  s.cfg.Server.ReadTimeout,
		WriteTimeout: s.cfg.Server.WriteTimeout,
	}

	log.Ctx(ctx).Info().Str("addr", addr).Msg("notification: starting server")

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = s.server.Shutdown(shutdownCtx)
	}()

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("notification server: %w", err)
	}
	return nil
}

// subscribeEvents subscribes to NATS JetStream events and dispatches webhooks.
func (s *Service) subscribeEvents(ctx context.Context) {
	consumer, err := s.nats.JS.CreateOrUpdateConsumer(ctx, s.cfg.NATS.StreamAlert, jetstream.ConsumerConfig{
		Durable:       "notification-service",
		FilterSubject: "alert.>",
	})
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("notification: failed to create consumer")
		return
	}

	msgCtx, err := consumer.Messages()
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("notification: failed to start consuming")
		return
	}
	defer msgCtx.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := msgCtx.Next()
			if err != nil {
				continue
			}
			s.dispatchWebhooks(ctx, msg.Subject(), msg.Data())
			msg.Ack()
		}
	}
}

// dispatchWebhooks sends webhook notifications to all enabled subscribers.
func (s *Service) dispatchWebhooks(ctx context.Context, event string, payload []byte) {
	// Query enabled webhooks that subscribe to this event
	rows, err := s.pool.Query(ctx,
		`SELECT id, url FROM webhooks WHERE enabled = TRUE AND $1 = ANY(events)`,
		event,
	)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("notification: query webhooks failed")
		return
	}
	defer rows.Close()

	client := &http.Client{Timeout: 10 * time.Second}
	for rows.Next() {
		var id int64
		var url string
		if err := rows.Scan(&id, &url); err != nil {
			continue
		}

		go func(webhookURL string, webhookID int64) {
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL,
				bytes.NewReader(payload),
			)
			if err != nil {
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-GlobalCVE-Event", event)

			resp, err := client.Do(req)
			if err != nil {
				log.Ctx(ctx).Warn().Err(err).Str("url", webhookURL).Msg("notification: webhook delivery failed")
				return
			}
			resp.Body.Close()
			log.Ctx(ctx).Info().Str("url", webhookURL).Int("status", resp.StatusCode).Msg("notification: webhook delivered")
		}(url, id)
	}
}

func (s *Service) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "notification"})
}

func (s *Service) handleListWebhooks(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(),
		`SELECT id, url, events, enabled, created_at FROM webhooks ORDER BY created_at DESC`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var webhooks []Webhook
	for rows.Next() {
		var wh Webhook
		if err := rows.Scan(&wh.ID, &wh.URL, &wh.Events, &wh.Enabled, &wh.CreatedAt); err == nil {
			webhooks = append(webhooks, wh)
		}
	}
	writeJSON(w, http.StatusOK, webhooks)
}

func (s *Service) handleCreateWebhook(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL    string   `json:"url"`
		Events []string `json:"events"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var id int64
	err := s.pool.QueryRow(r.Context(),
		`INSERT INTO webhooks (url, events) VALUES ($1, $2) RETURNING id`,
		req.URL, req.Events,
	).Scan(&id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func (s *Service) handleDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	_, err := s.pool.Exec(r.Context(), `DELETE FROM webhooks WHERE id = $1`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
