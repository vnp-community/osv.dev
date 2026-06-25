package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/osv/notification-service/internal/broker"
)

type TokenValidator interface {
	ValidateAndGetUserID(token string) (string, error)
}

type SSEHandler struct {
	broker     *broker.EventBroker
	tokenSvc   TokenValidator // validates Bearer token for SSE (can't use cookies with EventSource)
}

func NewSSEHandler(broker *broker.EventBroker, tokenSvc TokenValidator) *SSEHandler {
	return &SSEHandler{broker: broker, tokenSvc: tokenSvc}
}

// GET /notifications/stream
// Auth: Bearer token via Authorization header OR ?token= query param
// (EventSource API cannot set custom headers in browser, so ?token= fallback is needed)
func (h *SSEHandler) Stream(w http.ResponseWriter, r *http.Request) {
	// Extract user ID
	userID, err := h.extractUserID(r)
	if err != nil || userID == "" {
		http.Error(w, `{"error":"UNAUTHORIZED"}`, http.StatusUnauthorized)
		return
	}

	// Verify SSE is supported
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	// Subscribe to events
	ch := h.broker.Subscribe(userID)
	defer h.broker.Unsubscribe(userID, ch)

	// Send initial "connected" event
	fmt.Fprintf(w, "event: connected\ndata: {\"user_id\":\"%s\"}\n\n", userID)
	flusher.Flush()

	// Keep-alive ping every 30s
	ping := time.NewTicker(30 * time.Second)
	defer ping.Stop()

	for {
		select {
		case <-r.Context().Done():
			// Client disconnected
			return

		case evt, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(evt)
			fmt.Fprintf(w, "event: notification\ndata: %s\n\n", data)
			flusher.Flush()

		case <-ping.C:
			fmt.Fprintf(w, "event: ping\ndata: {\"ts\":\"%s\"}\n\n",
				time.Now().UTC().Format(time.RFC3339))
			flusher.Flush()
		}
	}
}

// extractUserID gets user ID from Bearer token or ?token= query param
func (h *SSEHandler) extractUserID(r *http.Request) (string, error) {
	// Option 1: Authorization: Bearer <jwt>
	auth := r.Header.Get("Authorization")
	if auth != "" && len(auth) > 7 && auth[:7] == "Bearer " {
		if h.tokenSvc != nil {
			return h.tokenSvc.ValidateAndGetUserID(auth[7:])
		}
	}

	// Option 2: ?token=<jwt> (for browser EventSource that can't set headers)
	token := r.URL.Query().Get("token")
	if token != "" {
		if h.tokenSvc != nil {
			return h.tokenSvc.ValidateAndGetUserID(token)
		}
	}

	// Option 3: X-User-ID from gateway (if gateway already validated and proxied)
	if userID := r.Header.Get("X-User-ID"); userID != "" {
		return userID, nil
	}

	return "", fmt.Errorf("no valid authentication")
}
