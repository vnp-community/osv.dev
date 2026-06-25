// health.go — Health and readiness handlers for the OSV orchestrator.
// Aggregates health status from all registered services.
package orchestrator

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// ServiceStatus tracks the health state of a single service.
type ServiceStatus struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "ok", "degraded", "down"
	Message string `json:"message,omitempty"`
	Since   string `json:"since,omitempty"`
}

// HealthRegistry tracks the runtime status of all managed services.
type HealthRegistry struct {
	mu       sync.RWMutex
	statuses map[string]*ServiceStatus
	startedAt time.Time
}

// NewHealthRegistry creates a new HealthRegistry.
func NewHealthRegistry() *HealthRegistry {
	return &HealthRegistry{
		statuses:  make(map[string]*ServiceStatus),
		startedAt: time.Now(),
	}
}

// Register adds a service to the registry with initial "starting" status.
func (h *HealthRegistry) Register(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.statuses[name] = &ServiceStatus{
		Name:   name,
		Status: "starting",
		Since:  time.Now().UTC().Format(time.RFC3339),
	}
}

// SetStatus updates the health status of a service.
func (h *HealthRegistry) SetStatus(name, status, message string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.statuses[name] = &ServiceStatus{
		Name:    name,
		Status:  status,
		Message: message,
		Since:   time.Now().UTC().Format(time.RFC3339),
	}
}

// HandleHealth is an HTTP handler for GET /health.
func (h *HealthRegistry) HandleHealth(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	type response struct {
		Status   string           `json:"status"`
		Services []*ServiceStatus `json:"services"`
		Uptime   string           `json:"uptime"`
	}

	overall := "ok"
	var services []*ServiceStatus
	for _, s := range h.statuses {
		services = append(services, s)
		if s.Status == "down" {
			overall = "degraded"
		}
	}

	resp := response{
		Status:   overall,
		Services: services,
		Uptime:   time.Since(h.startedAt).Truncate(time.Second).String(),
	}

	httpStatus := http.StatusOK
	if overall != "ok" {
		httpStatus = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	json.NewEncoder(w).Encode(resp) //nolint:errcheck
}

// HandleReady is an HTTP handler for GET /ready (readiness probe).
func (h *HealthRegistry) HandleReady(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, s := range h.statuses {
		if s.Status == "starting" || s.Status == "down" {
			http.Error(w, `{"ready":false}`, http.StatusServiceUnavailable)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"ready":true}`)) //nolint:errcheck
}
