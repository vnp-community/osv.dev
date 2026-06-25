package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/osv/shared/pkg/middleware/auth"
)

// injectClaimsFromHeader reads the X-User-ID header set by the API gateway
// and injects an OVSClaims into the request context so handlers can call
// auth.OVSClaimsFromContext without a full JWT re-validation step.
func injectClaimsFromHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		if userID != "" {
			claims := &auth.OVSClaims{}
			claims.UserID = userID
			ctx := auth.InjectOVSClaims(r.Context(), claims)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

// SetupRouter creates the chi router for the notification service.
func SetupRouter(wh *WebhookHandler, sh *SubscriptionHandler, ih *InternalHandler, ah *AlertsHandler, sse *SSEHandler, rh *RuleHandler, dh *DeliveryHandler) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	// Inject OVS user claims from the X-User-ID header forwarded by the API gateway.
	r.Use(injectClaimsFromHeader)

	// Webhook management
	r.Route("/api/v1/webhooks", func(r chi.Router) {
		r.Post("/bulk", wh.BulkCreateWebhooks)
		r.Get("/", wh.List)
		r.Post("/", wh.Create)
		// CR-009: literal paths BEFORE /{id} to avoid routing conflict
		if dh != nil {
			r.Get("/deliveries", dh.ListWebhookDeliveries)           // CR-009: flat delivery list
			r.Get("/stats/hourly", dh.GetWebhookHourlyStats)         // CR-009: 24h hourly chart
			r.Get("/stats", dh.GetWebhookHourlyStats)                // Alias for test script
			r.Post("/deliveries/{id}/retry", dh.RetryWebhookDelivery) // CR-009: retry failed
		}
		r.Delete("/{id}", wh.Delete)
		r.Get("/{id}/deliveries", wh.ListDeliveries)
		r.Post("/{id}/test", wh.TestWebhook)
	})

	// Subscription management
	r.Route("/api/v2/subscriptions", func(r chi.Router) {
		r.Post("/bulk", sh.BulkCreateSubscriptions)
		r.Get("/", sh.List)
		r.Post("/", sh.Create)
		r.Delete("/{id}", sh.Delete)
	})

	// Health
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, 200, map[string]string{"status": "ok"})
	})

	// Internal Events
	r.Post("/internal/events/cve", ih.ReceiveCVEEvents)

	// Webhook Deliveries Root Alias
	if dh != nil {
		r.Get("/api/v1/webhook_deliveries", dh.ListWebhookDeliveries)
	}

	// In-app Notifications
	// MOCK-014 FIX: guard nil AlertsHandler/SSEHandler — trả 503 thay vì panic
	if ah != nil {
		r.Route("/api/v2/notifications", func(r chi.Router) {
			r.Get("/", ah.ListNotifications)
			r.Patch("/{id}/read", ah.MarkRead)
			r.Post("/mark-all-read", ah.MarkAllRead)
			r.Get("/unread-count", ah.UnreadCount)
			// SSE stream — only mount when SSEHandler is wired
			if sse != nil {
				r.Get("/stream", sse.Stream)
			}
		})
	} else {
		// Graceful stub: return 503 so clients can distinguish "not wired" from "not found"
		unavailable := func(w http.ResponseWriter, r *http.Request) {
			respondJSON(w, http.StatusServiceUnavailable,
				map[string]string{"error": "notification service not fully initialized"})
		}
		r.Get("/api/v2/notifications", unavailable)
		r.Get("/api/v2/notifications/stream", unavailable)
	}

	// Rules management
	if rh != nil {
		r.Route("/api/v2/notification-rules", func(r chi.Router) {
			r.Post("/bulk", rh.BulkCreateNotificationRules)
			r.Get("/", rh.List)
			r.Post("/", rh.Create)
			r.Put("/{id}", rh.Update)
			r.Delete("/{id}", rh.Delete)
		})
	}

	// TASK-011: v1 notification routes (gateway forwards /api/v1/notifications/* → notification-service)
	// IMPORTANT: literal paths MUST be before /{id} wildcard
	if ah != nil {
		r.Post("/api/v1/notifications/mark-all-read", ah.MarkAllRead)  // literal BEFORE /{id}
		r.Get("/api/v1/notifications/unread-count", ah.UnreadCount)    // literal BEFORE /{id}
		r.Get("/api/v1/notifications", ah.ListNotifications)
		r.Patch("/api/v1/notifications/{id}/read", ah.MarkRead)
	}

	return r
}
