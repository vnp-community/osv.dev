// Package http — inapp_handler.go
// InAppHandler serves SSE stream and REST CRUD for in-app notifications.
//
// S3-NOTIF-01: In-App Notifications SSE
// Routes (add to existing router):
//   GET  /notifications/stream      → SSE stream (real-time push)
//   GET  /notifications             → Paginated list (polling fallback)
//   POST /notifications/{id}/read   → Mark one as read
//   POST /notifications/read-all    → Mark all as read
package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/osv/notification-service/internal/infra/adapters/inapp"
)

// InAppHandler handles in-app notification endpoints including SSE.
type InAppHandler struct {
	store *inapp.Store
	log   zerolog.Logger
}

// NewInAppHandler creates a new InAppHandler.
func NewInAppHandler(store *inapp.Store, log zerolog.Logger) *InAppHandler {
	return &InAppHandler{store: store, log: log}
}

// Stream handles GET /notifications/stream — Server-Sent Events endpoint.
// Polls the database every 5 seconds for new unread notifications.
// Client must send X-User-ID header (populated by gateway JWT middleware).
func (h *InAppHandler) Stream(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.extractUserID(w, r)
	if !ok {
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering
	w.WriteHeader(http.StatusOK)

	// Send initial keep-alive comment
	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			notifications, err := h.store.GetUnread(r.Context(), userID, 20)
			if err != nil {
				h.log.Warn().Err(err).Msg("inapp SSE: GetUnread failed")
				continue
			}
			for _, n := range notifications {
				data, _ := json.Marshal(n)
				fmt.Fprintf(w, "id: %s\n", n.ID)
				fmt.Fprintf(w, "event: notification\n")
				fmt.Fprintf(w, "data: %s\n\n", data)
			}
			flusher.Flush()

		case <-r.Context().Done():
			return
		}
	}
}

// List handles GET /notifications — paginated unread notifications.
func (h *InAppHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.extractUserID(w, r)
	if !ok {
		return
	}

	limit := parseIntQuery(r, "limit", 50)
	notifications, err := h.store.GetUnread(r.Context(), userID, limit)
	if err != nil {
		h.log.Error().Err(err).Msg("inapp.List")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list notifications"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"notifications": notifications,
		"total":         len(notifications),
	})
}

// MarkRead handles POST /notifications/{id}/read.
func (h *InAppHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.extractUserID(w, r)
	if !ok {
		return
	}

	notifID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid notification id"})
		return
	}

	if err := h.store.MarkRead(r.Context(), notifID, userID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "notification not found or already read"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// MarkAllRead handles POST /notifications/read-all.
func (h *InAppHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.extractUserID(w, r)
	if !ok {
		return
	}

	count, err := h.store.MarkAllRead(r.Context(), userID)
	if err != nil {
		h.log.Error().Err(err).Msg("inapp.MarkAllRead")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to mark all read"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]int64{"marked_read": count})
}

// ── helpers ──────────────────────────────────────────────────────────────────

func (h *InAppHandler) extractUserID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	raw := r.Header.Get("X-User-ID")
	id, err := uuid.Parse(raw)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing or invalid X-User-ID header"})
		return uuid.Nil, false
	}
	return id, true
}

func parseIntQuery(r *http.Request, key string, defaultVal int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return defaultVal
	}
	n := 0
	fmt.Sscanf(v, "%d", &n)
	if n <= 0 {
		return defaultVal
	}
	return n
}
