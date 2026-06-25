package sse

import (
    "fmt"
    "net/http"
    "sync"
    "time"

    "github.com/google/uuid"
    "github.com/rs/zerolog"
)

// ProgressEvent represents a scan progress update.
type ProgressEvent struct {
    ScanID   uuid.UUID `json:"scan_id"`
    Progress int       `json:"progress"` // 0-100
    Message  string    `json:"message"`
    At       time.Time `json:"at"`
}

// Hub manages SSE client connections per scan.
type Hub struct {
    mu      sync.RWMutex
    clients map[uuid.UUID]map[chan ProgressEvent]struct{}
    logger  zerolog.Logger
}

// NewHub creates an SSE Hub.
func NewHub(logger zerolog.Logger) *Hub {
    return &Hub{
        clients: make(map[uuid.UUID]map[chan ProgressEvent]struct{}),
        logger:  logger,
    }
}

// Subscribe registers an SSE channel for a scan.
func (h *Hub) Subscribe(scanID uuid.UUID) chan ProgressEvent {
    h.mu.Lock()
    defer h.mu.Unlock()

    if _, ok := h.clients[scanID]; !ok {
        h.clients[scanID] = make(map[chan ProgressEvent]struct{})
    }

    ch := make(chan ProgressEvent, 32)
    h.clients[scanID][ch] = struct{}{}
    return ch
}

// Unsubscribe removes an SSE channel.
func (h *Hub) Unsubscribe(scanID uuid.UUID, ch chan ProgressEvent) {
    h.mu.Lock()
    defer h.mu.Unlock()

    if clients, ok := h.clients[scanID]; ok {
        delete(clients, ch)
        if len(clients) == 0 {
            delete(h.clients, scanID)
        }
    }
    close(ch)
}

// Publish sends a progress event to all subscribers of a scan.
func (h *Hub) Publish(scanID uuid.UUID, progress int, message string) {
    h.mu.RLock()
    defer h.mu.RUnlock()

    event := ProgressEvent{
        ScanID:   scanID,
        Progress: progress,
        Message:  message,
        At:       time.Now().UTC(),
    }

    if clients, ok := h.clients[scanID]; ok {
        for ch := range clients {
            select {
            case ch <- event:
            default:
                // Buffer full: skip this update
                h.logger.Warn().Str("scan_id", scanID.String()).Msg("sse buffer full, skipping event")
            }
        }
    }
}

// ServeSSE is the HTTP handler for SSE scan progress streaming.
// GET /scans/{id}/stream
func (h *Hub) ServeSSE(scanID uuid.UUID) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Verify SSE support
        flusher, ok := w.(http.Flusher)
        if !ok {
            http.Error(w, "SSE not supported", http.StatusInternalServerError)
            return
        }

        // Set SSE headers
        w.Header().Set("Content-Type", "text/event-stream")
        w.Header().Set("Cache-Control", "no-cache")
        w.Header().Set("Connection", "keep-alive")
        w.Header().Set("X-Accel-Buffering", "no") // Nginx: disable buffering

        // Subscribe
        ch := h.Subscribe(scanID)
        defer h.Unsubscribe(scanID, ch)

        // Keep-alive ticker
        ticker := time.NewTicker(15 * time.Second)
        defer ticker.Stop()

        for {
            select {
            case <-r.Context().Done():
                return

            case event, ok := <-ch:
                if !ok {
                    return
                }
                fmt.Fprintf(w, "data: {\"scan_id\":%q,\"progress\":%d,\"message\":%q}\n\n",
                    event.ScanID, event.Progress, event.Message)
                flusher.Flush()

                // Auto-close stream when scan is done
                if event.Progress == 100 {
                    return
                }

            case <-ticker.C:
                // Heartbeat
                fmt.Fprintf(w, ": ping\n\n")
                flusher.Flush()
            }
        }
    }
}
