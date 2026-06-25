# TASK-BE-012 — notification-service: SSE Event Broker + Stream Endpoint

| Field | Value |
|-------|-------|
| **Task ID** | TASK-BE-012 |
| **Service** | `services/notification-service` |
| **Solution Ref** | [SOL-UI-002 §2.5](../solutions/SOL-UI-002-dashboard-bff-sse.md) |
| **Priority** | 🟡 P1 |
| **Depends On** | TASK-BE-011 (alert store phải tồn tại) |
| **Estimated** | 4h |

---

## Context

Frontend dùng `EventSource` để nhận real-time notifications. notification-service cần:
1. `EventBroker` — in-memory pub/sub cho SSE connections
2. NATS subscriber gọi `broker.Push()` khi nhận events
3. SSE endpoint `GET /notifications/stream`

---

## Target Files

| Action | File Path |
|--------|-----------|
| CREATE | `services/notification-service/internal/broker/event_broker.go` |
| CREATE | `services/notification-service/internal/adapter/http/sse_handler.go` |
| MODIFY | `services/notification-service/internal/adapter/nats/subscriber.go` |
| MODIFY | `services/notification-service/internal/adapter/http/router.go` |

---

## Implementation

### File 1: `services/notification-service/internal/broker/event_broker.go`

```go
package broker

import (
	"sync"
)

// NotificationEvent is pushed to SSE clients
type NotificationEvent struct {
	Type       string `json:"type"`        // "kev.new" | "finding.sla.breached" | ...
	Title      string `json:"title"`
	Message    string `json:"message"`
	Severity   string `json:"severity"`    // "Critical"|"High"|"Info"
	EntityType string `json:"entity_type"` // "cve"|"finding"
	EntityID   string `json:"entity_id"`
}

// EventBroker manages SSE client subscriptions
type EventBroker struct {
	mu          sync.RWMutex
	subscribers map[string][]chan NotificationEvent // userID → channels
}

func New() *EventBroker {
	return &EventBroker{
		subscribers: make(map[string][]chan NotificationEvent),
	}
}

// Subscribe registers a new SSE channel for a user
func (b *EventBroker) Subscribe(userID string) chan NotificationEvent {
	ch := make(chan NotificationEvent, 10) // buffered: 10 events
	b.mu.Lock()
	b.subscribers[userID] = append(b.subscribers[userID], ch)
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes an SSE channel and closes it
func (b *EventBroker) Unsubscribe(userID string, ch chan NotificationEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	subs := b.subscribers[userID]
	for i, s := range subs {
		if s == ch {
			b.subscribers[userID] = append(subs[:i], subs[i+1:]...)
			close(ch)
			return
		}
	}
}

// Push sends an event to a specific user's SSE connections
func (b *EventBroker) Push(userID string, evt NotificationEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subscribers[userID] {
		select {
		case ch <- evt:
		default: // drop if buffer full — non-blocking
		}
	}
}

// PushAll broadcasts event to all connected users
func (b *EventBroker) PushAll(evt NotificationEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, channels := range b.subscribers {
		for _, ch := range channels {
			select {
			case ch <- evt:
			default:
			}
		}
	}
}

// ConnectedCount returns number of active SSE connections (for metrics)
func (b *EventBroker) ConnectedCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	count := 0
	for _, channels := range b.subscribers {
		count += len(channels)
	}
	return count
}
```

### File 2: `services/notification-service/internal/adapter/http/sse_handler.go`

```go
package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/your-org/osv/services/notification-service/internal/broker"
)

type SSEHandler struct {
	broker     *broker.EventBroker
	tokenSvc   TokenValidator // validates Bearer token for SSE (can't use cookies with EventSource)
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
		return h.tokenSvc.ValidateAndGetUserID(auth[7:])
	}

	// Option 2: ?token=<jwt> (for browser EventSource that can't set headers)
	token := r.URL.Query().Get("token")
	if token != "" {
		return h.tokenSvc.ValidateAndGetUserID(token)
	}

	// Option 3: X-User-ID from gateway (if gateway already validated and proxied)
	if userID := r.Header.Get("X-User-ID"); userID != "" {
		return userID, nil
	}

	return "", fmt.Errorf("no valid authentication")
}
```

### File 3: NATS subscriber update (add broker.Push calls):

```go
// services/notification-service/internal/adapter/nats/subscriber.go
// MODIFY: thêm broker field và Push calls vào existing subscribers

type NATSSubscriber struct {
	js         nats.JetStreamContext
	alertStore AlertRepository
	broker     *broker.EventBroker  // ADD THIS FIELD
}

// Existing KEV handler — ADD broker.PushAll call
func (s *NATSSubscriber) onKEVNew(msg *nats.Msg) {
	var event struct {
		CveID   string `json:"cve_id"`
		Vendor  string `json:"vendor"`
		Product string `json:"product"`
	}
	json.Unmarshal(msg.Data, &event)

	// Existing: store in DB
	alert := &Alert{
		Type:       "kev.new",
		Title:      fmt.Sprintf("New KEV: %s %s (%s)", event.Vendor, event.Product, event.CveID),
		Message:    "Added to CISA KEV catalog",
		Severity:   "Critical",
		EntityType: "cve",
		EntityID:   event.CveID,
	}
	s.alertStore.CreateForAll(context.Background(), alert)

	// NEW: push to all SSE clients
	s.broker.PushAll(broker.NotificationEvent{
		Type:       alert.Type,
		Title:      alert.Title,
		Message:    alert.Message,
		Severity:   alert.Severity,
		EntityType: alert.EntityType,
		EntityID:   alert.EntityID,
	})

	msg.Ack()
}

// Existing SLA breach handler — ADD broker.Push call
func (s *NATSSubscriber) onSLABreached(msg *nats.Msg) {
	var event struct {
		FindingID      string `json:"finding_id"`
		AssignedUserID string `json:"assigned_user_id"`
		FindingTitle   string `json:"finding_title"`
		Severity       string `json:"severity"`
	}
	json.Unmarshal(msg.Data, &event)

	// Existing: store in DB for assigned user
	alert := &Alert{
		Type:       "finding.sla.breached",
		Title:      "SLA Breach: " + event.FindingTitle,
		Message:    "This finding has exceeded its SLA deadline",
		Severity:   "High",
		EntityType: "finding",
		EntityID:   event.FindingID,
		UserID:     event.AssignedUserID,
	}
	s.alertStore.CreateForUser(context.Background(), alert, event.AssignedUserID)

	// NEW: push to specific user
	if event.AssignedUserID != "" {
		s.broker.Push(event.AssignedUserID, broker.NotificationEvent{
			Type:       alert.Type,
			Title:      alert.Title,
			Message:    alert.Message,
			Severity:   alert.Severity,
			EntityType: alert.EntityType,
			EntityID:   alert.EntityID,
		})
	}

	msg.Ack()
}
```

### Router:

```go
// services/notification-service/internal/adapter/http/router.go
mux.HandleFunc("GET /notifications/stream", h.SSE.Stream)
```

---

## Verification

```bash
cd services/notification-service
go build ./...

# Connect to SSE stream
curl -N -H "Authorization: Bearer $TOKEN" \
  http://localhost:8087/notifications/stream
# Expected output:
# event: connected
# data: {"user_id":"..."}
#
# (every 30s)
# event: ping
# data: {"ts":"2026-..."}

# Trigger an event (via NATS):
nats pub kev.new '{"cve_id":"CVE-2026-9999","vendor":"Test","product":"Product"}'
# Expected: SSE client receives "event: notification" within 2s
```

---

## Checklist

- [x] `EventBroker` thread-safe via `sync.RWMutex`
- [x] `Subscribe` tạo buffered channel (size 10) để tránh blocking
- [x] `Unsubscribe` close channel khi client disconnect
- [x] `Push` non-blocking: dùng `select { case ch <- evt: default: }` (drop nếu buffer đầy)
- [x] `PushAll` broadcast đến tất cả connected users
- [x] SSE handler set đúng headers: `Content-Type: text/event-stream`, `X-Accel-Buffering: no`
- [x] Ping mỗi 30s để giữ kết nối qua nginx/proxy
- [x] `onKEVNew` gọi `broker.PushAll()` sau khi store vào DB
- [x] `onSLABreached` gọi `broker.Push(assignedUserID, ...)` sau khi store
- [x] `go build ./...` thành công

## Notes for AI

- SSE path `GET /notifications/stream` phải được gateway proxy với `ForwardSSE()` (no timeout, no buffering)
- `TokenValidator` interface cần được inject; nếu gateway đã validate JWT và inject `X-User-ID` header, dùng Option 3 trong `extractUserID`
