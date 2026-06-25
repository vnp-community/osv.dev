// Package health provides /health and /info HTTP handlers for the unified gateway.
package health

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"
)

var startTime = time.Now()

// InfoResponse is the JSON response for /info.
type InfoResponse struct {
	Service   string    `json:"service"`
	Version   string    `json:"version"`
	GoVersion string    `json:"go_version"`
	StartTime time.Time `json:"start_time"`
	Uptime    string    `json:"uptime"`
}

// HealthResponse is the JSON response for the simple /health endpoint.
type HealthResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}

// HandleHealth returns 200 OK when the gateway is running (simple local check).
// Deprecated: prefer AggregateHandler for full system health visibility.
func HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(HealthResponse{Status: "ok"}) //nolint:errcheck
}

// HandleInfo returns service metadata.
func HandleInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(InfoResponse{ //nolint:errcheck
		Service:   "gateway-service",
		Version:   "1.0.0",
		GoVersion: runtime.Version(),
		StartTime: startTime,
		Uptime:    time.Since(startTime).Round(time.Second).String(),
	})
}

// AggregateHandler wraps AggregateUseCase as an HTTP handler for GET /health.
// Returns 200 regardless of upstream status (degraded is not a gateway error).
type AggregateHandler struct {
	uc *AggregateUseCase
}

// NewAggregateHandler creates a handler backed by AggregateUseCase.
func NewAggregateHandler(uc *AggregateUseCase) *AggregateHandler {
	return &AggregateHandler{uc: uc}
}

// ServeHTTP handles GET /health.
// Always returns HTTP 200 — downstream clients check "status" field for "degraded".
func (h *AggregateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	health := h.uc.Check(r.Context())
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(health) //nolint:errcheck
}

