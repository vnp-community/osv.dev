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

// HealthResponse is the JSON response for /health.
type HealthResponse struct {
	Status  string            `json:"status"`
	Checks  map[string]string `json:"checks,omitempty"`
}

// HandleHealth returns 200 OK when the gateway is running.
func HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(HealthResponse{Status: "ok"})
}

// HandleInfo returns service metadata.
func HandleInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(InfoResponse{
		Service:   "gateway-service",
		Version:   "1.0.0",
		GoVersion: runtime.Version(),
		StartTime: startTime,
		Uptime:    time.Since(startTime).Round(time.Second).String(),
	})
}
