// Package http provides the HTTP delivery layer for the notification service.
package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"

	domainerrors "github.com/globalcve/notification-service/internal/domain/errors"
	"github.com/globalcve/notification-service/internal/domain/aggregate/webhook"
	"github.com/globalcve/notification-service/internal/usecase/dispatch"
	"github.com/globalcve/notification-service/internal/usecase/manage"
	"github.com/globalcve/notification-service/internal/usecase/register"
)

// contextKey for owner ID.
type contextKey string

const ownerIDKey contextKey = "ownerID"

// Handler holds all HTTP handlers for the notification service.
type Handler struct {
	registerUC *register.UseCase
	manageUC   *manage.UseCase
	dispatchUC *dispatch.UseCase
	log        zerolog.Logger
}

// NewHandler creates an HTTP Handler.
func NewHandler(regUC *register.UseCase, mngUC *manage.UseCase, dispUC *dispatch.UseCase, log zerolog.Logger) *Handler {
	return &Handler{registerUC: regUC, manageUC: mngUC, dispatchUC: dispUC, log: log}
}

// Health handles GET /health.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "notification-service"})
}

// CreateWebhook handles POST /api/v2/webhooks.
func (h *Handler) CreateWebhook(w http.ResponseWriter, r *http.Request) {
	ownerID := ownerFromCtx(r.Context())
	if ownerID == "" {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var body struct {
		URL    string   `json:"url"`
		Events []string `json:"events"`
		Secret string   `json:"secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	req := &register.Request{
		URL:     body.URL,
		Events:  body.Events,
		Secret:  body.Secret,
		OwnerID: ownerID,
	}
	wh, err := h.registerUC.Execute(r.Context(), req)
	switch err {
	case nil:
		respondJSON(w, http.StatusCreated, webhookResponse(wh))
	case webhook.ErrInvalidURL:
		respondError(w, http.StatusBadRequest, "webhook URL must start with https://")
	case webhook.ErrEmptyEvents:
		respondError(w, http.StatusBadRequest, "at least one event type required")
	default:
		h.log.Error().Err(err).Msg("create webhook failed")
		respondError(w, http.StatusInternalServerError, "internal error")
	}
}

// ListWebhooks handles GET /api/v2/webhooks.
func (h *Handler) ListWebhooks(w http.ResponseWriter, r *http.Request) {
	ownerID := ownerFromCtx(r.Context())
	if ownerID == "" {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	webhooks, err := h.manageUC.ListByOwner(r.Context(), ownerID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	resp := make([]map[string]interface{}, 0, len(webhooks))
	for _, wh := range webhooks {
		resp = append(resp, webhookResponse(wh))
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"webhooks": resp, "count": len(resp)})
}

// DeleteWebhook handles DELETE /api/v2/webhooks/{id}.
func (h *Handler) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	ownerID := ownerFromCtx(r.Context())
	if ownerID == "" {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	id := chi.URLParam(r, "id")
	err := h.manageUC.Delete(r.Context(), id, ownerID)
	switch err {
	case nil:
		w.WriteHeader(http.StatusNoContent)
	case domainerrors.ErrWebhookNotFound:
		respondError(w, http.StatusNotFound, "webhook not found")
	case domainerrors.ErrUnauthorized:
		respondError(w, http.StatusForbidden, "forbidden")
	default:
		h.log.Error().Err(err).Str("id", id).Msg("delete webhook failed")
		respondError(w, http.StatusInternalServerError, "internal error")
	}
}

// GetStats handles GET /internal/webhooks/stats.
func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.manageUC.GetStats(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, stats)
}

// NewRouter creates the chi router for the notification service.
func NewRouter(h *Handler, jwtSecret string, log zerolog.Logger) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(requestLogger(log))

	r.Get("/health", h.Health)
	r.Get("/internal/webhooks/stats", h.GetStats)

	r.Group(func(r chi.Router) {
		r.Use(authMiddleware(jwtSecret))
		r.Post("/api/v2/webhooks", h.CreateWebhook)
		r.Get("/api/v2/webhooks", h.ListWebhooks)
		r.Delete("/api/v2/webhooks/{id}", h.DeleteWebhook)
	})

	return r
}

// --- middleware ---

func authMiddleware(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth == "" || !strings.HasPrefix(strings.ToLower(auth), "bearer ") {
				respondError(w, http.StatusUnauthorized, "bearer token required")
				return
			}
			tokenStr := strings.TrimPrefix(strings.TrimPrefix(auth, "Bearer "), "bearer ")

			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(jwtSecret), nil
			})
			if err != nil || !token.Valid {
				respondError(w, http.StatusUnauthorized, "invalid token")
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				respondError(w, http.StatusUnauthorized, "invalid claims")
				return
			}
			ownerID, _ := claims["sub"].(string)
			ctx := context.WithValue(r.Context(), ownerIDKey, ownerID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func requestLogger(log zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			log.Info().Str("method", r.Method).Str("path", r.URL.Path).
				Int("status", ww.Status()).Dur("latency", time.Since(start)).Msg("request")
		})
	}
}

// --- helpers ---

func ownerFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(ownerIDKey).(string)
	return v
}

func webhookResponse(wh *webhook.Webhook) map[string]interface{} {
	return map[string]interface{}{
		"id":         wh.ID(),
		"owner_id":   wh.OwnerID(),
		"url":        wh.URL(),
		"events":     wh.Events(),
		"active":     wh.IsActive(),
		"created_at": wh.CreatedAt(),
	}
}

func respondJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func respondError(w http.ResponseWriter, code int, msg string) {
	respondJSON(w, code, map[string]string{"error": msg})
}
