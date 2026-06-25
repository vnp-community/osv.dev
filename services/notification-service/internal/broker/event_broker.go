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
