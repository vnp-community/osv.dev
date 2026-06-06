// interface/http/handler/health_handler.go
package handler

import (
	"encoding/json"
	"net/http"
	"time"
)

// HealthHandler serves /health/live and /health/ready endpoints.
type HealthHandler struct{}

func NewHealthHandler() *HealthHandler { return &HealthHandler{} }

func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	// TODO: check Firestore + NATS connectivity
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
		"status": "ready",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}
